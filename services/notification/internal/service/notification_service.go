package service

import (
	"fmt"
	"log"

	"github.com/abubakvr/payup-backend/services/notification/internal/model"
	"github.com/abubakvr/payup-backend/services/notification/internal/providers/brevo"
	"github.com/abubakvr/payup-backend/services/notification/internal/providers/termii"
	"github.com/abubakvr/payup-backend/services/notification/internal/providers/whatsapp"
)

// NotificationService processes notification events and sends via the appropriate provider.
type NotificationService struct {
	brevo    *brevo.Client
	termii   *termii.Client
	whatsapp *whatsapp.Client
}

// NewNotificationService builds the service with the given provider clients.
func NewNotificationService(brevo *brevo.Client, termii *termii.Client, whatsapp *whatsapp.Client) *NotificationService {
	return &NotificationService{
		brevo:    brevo,
		termii:   termii,
		whatsapp: whatsapp,
	}
}

// Process handles one notification event: routes by channel and calls the right provider.
func (s *NotificationService) Process(event model.NotificationEvent) error {
	meta := event.Metadata
	if meta == nil {
		meta = make(map[string]interface{})
	}

	switch event.Channel {
	case "email":
		return s.sendEmail(event.Type, meta)
	case "sms":
		return s.sendSMS(event.Type, meta)
	case "whatsapp":
		return s.sendWhatsApp(event.Type, meta)
	default:
		return fmt.Errorf("unknown channel: %s", event.Channel)
	}
}

func (s *NotificationService) sendEmail(evType string, meta map[string]interface{}) error {
	if s.brevo == nil {
		log.Printf("notification: email skipped type=%s (Brevo client not configured)", evType)
		return fmt.Errorf("email provider not configured")
	}
	to := getStr(meta, "to")
	if to == "" {
		log.Printf("notification: email skipped type=%s (missing metadata.to)", evType)
		return fmt.Errorf("email: missing metadata.to")
	}
	subject := getStr(meta, "subject")
	body := getStr(meta, "body")
	html := getStr(meta, "html")
	toName := getStr(meta, "to_name")
	templateID := getInt64(meta, "template_id")
	params := getMap(meta, "params")

	log.Printf("notification: sending email via Brevo type=%s to=%s subject=%s has_html=%v has_body=%v template_id=%d",
		evType, to, subject, html != "", body != "", templateID)

	err := s.brevo.Send(to, toName, subject, html, body, templateID, params)
	if err != nil {
		log.Printf("notification: email send failed type=%s to=%s err=%v", evType, to, err)
		return err
	}
	log.Printf("notification: email sent successfully type=%s to=%s", evType, to)
	return nil
}

func (s *NotificationService) sendSMS(evType string, meta map[string]interface{}) error {
	if s.termii == nil {
		return fmt.Errorf("sms provider not configured")
	}
	to := getStr(meta, "to")
	if to == "" {
		return fmt.Errorf("sms: missing metadata.to")
	}
	message := getStr(meta, "body")
	if message == "" {
		message = getStr(meta, "message")
	}
	if message == "" {
		return fmt.Errorf("sms: missing metadata.body or metadata.message")
	}
	channel := getStr(meta, "channel")
	if channel == "" {
		channel = "generic"
	}

	err := s.termii.Send(to, message, channel)
	if err != nil {
		log.Printf("notification: sms send failed type=%s to=%s err=%v", evType, to, err)
		return err
	}
	log.Printf("notification: sms sent type=%s to=%s", evType, to)
	return nil
}

func (s *NotificationService) sendWhatsApp(evType string, meta map[string]interface{}) error {
	if s.whatsapp == nil {
		return fmt.Errorf("whatsapp provider not configured")
	}
	to := getStr(meta, "to")
	if to == "" {
		return fmt.Errorf("whatsapp: missing metadata.to")
	}
	// Remove leading + for WhatsApp API
	if len(to) > 0 && to[0] == '+' {
		to = to[1:]
	}

	templateName := getStr(meta, "template_name")
	if templateName != "" {
		lang := getStr(meta, "template_language")
		var components []whatsapp.TemplateComponent
		if params, ok := meta["template_params"].([]interface{}); ok {
			var bodyParams []whatsapp.TemplateParam
			for _, p := range params {
				if s, ok := p.(string); ok {
					bodyParams = append(bodyParams, whatsapp.TemplateParam{Type: "text", Text: s})
				}
			}
			if len(bodyParams) > 0 {
				components = []whatsapp.TemplateComponent{
					{Type: "body", Parameters: bodyParams},
				}
			}
		}
		err := s.whatsapp.SendTemplate(to, templateName, lang, components)
		if err != nil {
			log.Printf("notification: whatsapp template send failed type=%s to=%s err=%v", evType, to, err)
			return err
		}
		log.Printf("notification: whatsapp template sent type=%s to=%s", evType, to)
		return nil
	}

	text := getStr(meta, "body")
	if text == "" {
		text = getStr(meta, "message")
	}
	if text == "" {
		return fmt.Errorf("whatsapp: need metadata.template_name or metadata.body")
	}
	err := s.whatsapp.SendText(to, text)
	if err != nil {
		log.Printf("notification: whatsapp text send failed type=%s to=%s err=%v", evType, to, err)
		return err
	}
	log.Printf("notification: whatsapp text sent type=%s to=%s", evType, to)
	return nil
}

func getStr(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func getInt64(m map[string]interface{}, key string) int64 {
	if m == nil {
		return 0
	}
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int:
		return int64(n)
	case int64:
		return n
	}
	return 0
}

func getMap(m map[string]interface{}, key string) map[string]interface{} {
	if m == nil {
		return nil
	}
	v, ok := m[key]
	if !ok {
		return nil
	}
	out, _ := v.(map[string]interface{})
	return out
}
