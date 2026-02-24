package model

// NotificationEvent is the payload consumed from the notification-events Kafka topic.
// Type identifies the kind of notification; Channel determines the provider (email, sms, whatsapp).
// Metadata holds channel-specific fields (to, subject, body, template_id, params, etc.).
type NotificationEvent struct {
	Type     string                 `json:"type"`               // e.g. email_verification, sms_otp, whatsapp_alert, transfer_receipt
	Channel  string                 `json:"channel"`             // email | sms | whatsapp
	Metadata map[string]interface{} `json:"metadata"`             // to, subject, body, html, template_id, params, etc.
}
