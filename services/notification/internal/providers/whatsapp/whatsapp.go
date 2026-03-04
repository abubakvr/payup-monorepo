package whatsapp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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
// URL pattern: POST https://graph.facebook.com/{version}/{phone_id}/messages
func NewClient(token, phoneID, apiVersion string) *Client {
	if apiVersion == "" {
		apiVersion = "v25.0"
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
		"recipient_type":   "individual",
		"to":               toPhone,
		"type":             "text",
		"text":             map[string]string{"body": text},
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
		"recipient_type":   "individual",
		"to":               toPhone,
		"type":             "template",
		"template":         template,
	}
	return c.post(payload)
}

// SendOTP sends a template message with body (OTP text) and optional URL button (same OTP).
// Template must have body {{1}} and optionally a URL button with dynamic part {{1}}.
func (c *Client) SendOTP(toPhone, templateName, otp string) error {
	if c.Token == "" || c.PhoneID == "" {
		return fmt.Errorf("whatsapp: missing token or phone number id")
	}
	if templateName == "" {
		templateName = "basic_otp"
	}
	components := []TemplateComponent{
		{Type: "body", Parameters: []TemplateParam{{Type: "text", Text: otp}}},
		{Type: "button", SubType: "url", Index: "0", Parameters: []TemplateParam{{Type: "text", Text: otp}}},
	}
	return c.SendTemplate(toPhone, templateName, "en_US", components)
}

// TemplateComponent for template variables (body, button, etc.).
type TemplateComponent struct {
	Type       string         `json:"type"`                 // body, button, etc.
	SubType    string         `json:"sub_type,omitempty"`   // url, quick_reply (for button)
	Index      string         `json:"index,omitempty"`      // button index e.g. "0"
	Parameters []TemplateParam `json:"parameters,omitempty"`
}

// TemplateParam is one variable (text type).
type TemplateParam struct {
	Type string `json:"type"` // text
	Text string `json:"text"`
}

// post sends POST to https://graph.facebook.com/{version}/{phone_id}/messages
// with Authorization: Bearer <token> and JSON body (messaging_product, recipient_type, to, type, template).
func (c *Client) post(payload map[string]interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	reqURL := c.BaseURL + "/" + c.PhoneID + "/messages"
	httpReq, err := http.NewRequest(http.MethodPost, reqURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.Token)
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		msg := fmt.Sprintf("whatsapp: status %d", resp.StatusCode)
		if len(body) > 0 {
			var errResp struct {
				Error struct {
					Message string `json:"message"`
					Code    int    `json:"code"`
					Type    string `json:"type"`
				} `json:"error"`
			}
			if json.Unmarshal(body, &errResp) == nil && errResp.Error.Message != "" {
				msg = fmt.Sprintf("whatsapp: %s (status %d)", errResp.Error.Message, resp.StatusCode)
			} else {
				msg = fmt.Sprintf("whatsapp: status %d body=%s", resp.StatusCode, string(body))
			}
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}
