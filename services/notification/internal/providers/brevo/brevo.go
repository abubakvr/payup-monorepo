package brevo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

const defaultBaseURL = "https://api.brevo.com/v3/smtp/email"

// Client sends transactional emails via Brevo.
type Client struct {
	APIKey        string
	SenderEmail   string
	SenderName    string
	BaseURL       string
	HTTPClient    *http.Client
}

// NewClient returns a Brevo email client.
func NewClient(apiKey, senderEmail, senderName, baseURL string) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Client{
		APIKey:      apiKey,
		SenderEmail: senderEmail,
		SenderName:  senderName,
		BaseURL:     baseURL,
		HTTPClient:  http.DefaultClient,
	}
}

// SendRequest matches Brevo's send transactional email API.
type SendRequest struct {
	Sender      Sender    `json:"sender"`
	To          []To      `json:"to"`
	Subject     string    `json:"subject"`
	HTMLContent string    `json:"htmlContent,omitempty"`
	TextContent string    `json:"textContent,omitempty"`
	TemplateID  int64     `json:"templateId,omitempty"`
	Params      map[string]interface{} `json:"params,omitempty"`
}

type Sender struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type To struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

// Send sends one transactional email. Prefer HTML or Text; TemplateID can be used with Params.
func (c *Client) Send(toEmail, toName, subject, htmlBody, textBody string, templateID int64, params map[string]interface{}) error {
	if c.APIKey == "" {
		log.Printf("brevo: Send aborted (API key empty)")
		return fmt.Errorf("brevo: missing API key")
	}
	log.Printf("brevo: calling API url=%s to=%s from=%s subject=%s", c.BaseURL, toEmail, c.SenderEmail, subject)

	req := SendRequest{
		Sender:  Sender{Name: c.SenderName, Email: c.SenderEmail},
		To:      []To{{Email: toEmail, Name: toName}},
		Subject: subject,
		Params:  params,
	}
	if templateID > 0 {
		req.TemplateID = templateID
	} else if htmlBody != "" {
		req.HTMLContent = htmlBody
	} else {
		req.TextContent = textBody
	}
	body, err := json.Marshal(req)
	if err != nil {
		log.Printf("brevo: marshal err=%v", err)
		return err
	}
	httpReq, err := http.NewRequest(http.MethodPost, c.BaseURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("accept", "application/json")
	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("api-key", c.APIKey)
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		log.Printf("brevo: request err=%v", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("brevo: API error status=%d body=%s", resp.StatusCode, string(respBody))
		return fmt.Errorf("brevo: unexpected status %d body=%s", resp.StatusCode, string(respBody))
	}
	log.Printf("brevo: email accepted status=%d to=%s", resp.StatusCode, toEmail)
	return nil
}
