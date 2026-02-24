package termii

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// Client sends SMS via Termii.
type Client struct {
	APIKey     string
	SenderID   string
	BaseURL    string
	HTTPClient *http.Client
}

// NewClient returns a Termii SMS client.
func NewClient(apiKey, senderID, baseURL string) *Client {
	if baseURL == "" {
		baseURL = "https://api.termii.com"
	}
	return &Client{
		APIKey:     apiKey,
		SenderID:   senderID,
		BaseURL:    baseURL,
		HTTPClient: http.DefaultClient,
	}
}

// SendRequest matches Termii send SMS API (single or bulk).
type SendRequest struct {
	APIKey string `json:"api_key"`
	To     string `json:"to"`     // single number e.g. 23490126727
	From   string `json:"from"`   // alphanumeric sender ID
	SMS    string `json:"sms"`
	Type   string `json:"type"`   // plain
	Channel string `json:"channel"` // generic | dnd
}

// Send sends one SMS. Phone should be E.164 (e.g. 23490126727). Use channel "dnd" for OTP/transactional.
func (c *Client) Send(phone, message, channel string) error {
	if c.APIKey == "" {
		return fmt.Errorf("termii: missing API key")
	}
	if channel == "" {
		channel = "generic"
	}
	req := SendRequest{
		APIKey:  c.APIKey,
		To:      phone,
		From:    c.SenderID,
		SMS:     message,
		Type:    "plain",
		Channel: channel,
	}
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	url := c.BaseURL + "/api/sms/send"
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("content-type", "application/json")
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("termii: unexpected status %d", resp.StatusCode)
	}
	return nil
}
