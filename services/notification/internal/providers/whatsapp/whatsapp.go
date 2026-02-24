package whatsapp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// Client sends messages via WhatsApp Business Cloud API.
type Client struct {
	Token       string
	PhoneID     string
	APIVersion  string
	BaseURL     string
	HTTPClient  *http.Client
}

// NewClient returns a WhatsApp Cloud API client.
func NewClient(token, phoneID, apiVersion string) *Client {
	if apiVersion == "" {
		apiVersion = "v21.0"
	}
	baseURL := "https://graph.facebook.com/" + apiVersion
	return &Client{
		Token:      token,
		PhoneID:    phoneID,
		APIVersion: apiVersion,
		BaseURL:    baseURL,
		HTTPClient: http.DefaultClient,
	}
}

// SendText sends a plain text message (only allowed within 24h customer reply window; otherwise use template).
func (c *Client) SendText(toPhone, text string) error {
	if c.Token == "" || c.PhoneID == "" {
		return fmt.Errorf("whatsapp: missing token or phone number id")
	}
	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"to":                toPhone,
		"type":              "text",
		"text":              map[string]string{"body": text},
	}
	return c.post(payload)
}

// SendTemplate sends a pre-approved template. components pass variables for {{1}}, {{2}}, etc.
func (c *Client) SendTemplate(toPhone, templateName, langCode string, components []TemplateComponent) error {
	if c.Token == "" || c.PhoneID == "" {
		return fmt.Errorf("whatsapp: missing token or phone number id")
	}
	lang := langCode
	if lang == "" {
		lang = "en_US"
	}
	template := map[string]interface{}{
		"name":       templateName,
		"language":   map[string]string{"code": lang},
		"components": components,
	}
	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"to":                toPhone,
		"type":              "template",
		"template":          template,
	}
	return c.post(payload)
}

// TemplateComponent for template variables (e.g. body with {{1}} {{2}}).
type TemplateComponent struct {
	Type       string          `json:"type"` // body, button, etc.
	Parameters []TemplateParam  `json:"parameters,omitempty"`
}

// TemplateParam is one variable (text type).
type TemplateParam struct {
	Type string `json:"type"` // text
	Text string `json:"text"`
}

func (c *Client) post(payload map[string]interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	url := c.BaseURL + "/" + c.PhoneID + "/messages"
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.Token)
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("whatsapp: unexpected status %d", resp.StatusCode)
	}
	return nil
}
