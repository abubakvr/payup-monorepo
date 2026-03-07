package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

const auditTopic = "audit-events"
const notificationTopic = "notification-events"
const serviceName = "admin"

// AuditEvent matches the payload consumed by the audit service (topic: audit-events).
type AuditEvent struct {
	Service       string                 `json:"service"`
	UserID        *string                `json:"user_id,omitempty"`
	Action        string                 `json:"action"`
	Entity        string                 `json:"entity"`
	EntityID      *string                `json:"entity_id,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	CorrelationID *string                `json:"correlation_id,omitempty"`
	Timestamp     time.Time              `json:"timestamp"`
}

// AuditProducer writes audit events to the audit-events topic.
type AuditProducer struct {
	writer *kafka.Writer
}

// NewAuditProducer creates a producer for audit-events. brokers e.g. []string{"redpanda:9092"}.
func NewAuditProducer(brokers []string) *AuditProducer {
	if len(brokers) == 0 {
		return nil
	}
	return &AuditProducer{
		writer: kafka.NewWriter(kafka.WriterConfig{
			Brokers: brokers,
			Topic:   auditTopic,
		}),
	}
}

// SendAdminCreated sends an admin_created audit event. Safe to call with nil producer (no-op).
func (p *AuditProducer) SendAdminCreated(createdByAdminID, newAdminID string, metadata map[string]interface{}) error {
	return p.SendAudit("admin_created", "admin", newAdminID, createdByAdminID, metadata)
}

// SendAudit sends a generic audit event to audit-events. Safe to call with nil producer (no-op).
func (p *AuditProducer) SendAudit(action, entity, entityID, userID string, metadata map[string]interface{}) error {
	if p == nil || p.writer == nil {
		return nil
	}
	event := AuditEvent{
		Service:   serviceName,
		UserID:    strPtr(userID),
		Action:    action,
		Entity:    entity,
		EntityID:  strPtr(entityID),
		Metadata:  metadata,
		Timestamp: time.Now(),
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return p.writer.WriteMessages(ctx, kafka.Message{Value: payload})
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// NotificationEvent is the payload for notification-events topic (consumed by notification service).
type NotificationEvent struct {
	Type     string                 `json:"type"`
	Channel  string                 `json:"channel"`
	Metadata map[string]interface{} `json:"metadata"`
}

// NotificationProducer writes to the notification-events topic (email, sms, etc.).
type NotificationProducer struct {
	writer *kafka.Writer
}

// NewNotificationProducer creates a producer for notification-events. brokers e.g. []string{"redpanda:9092"}.
func NewNotificationProducer(brokers []string) *NotificationProducer {
	if len(brokers) == 0 {
		return nil
	}
	return &NotificationProducer{
		writer: kafka.NewWriter(kafka.WriterConfig{
			Brokers: brokers,
			Topic:   notificationTopic,
		}),
	}
}

// SendAdminWelcomeEmail sends an email to the new admin with login details and temporary password, prompting them to log in and change it immediately.
// Safe to call with nil producer (no-op). portalURL is optional (e.g. https://admin.payup.ng); if empty, the email does not include a link.
func (p *NotificationProducer) SendAdminWelcomeEmail(email, firstName, lastName, temporaryPassword, portalURL string) error {
	if p == nil || p.writer == nil {
		return nil
	}
	toName := firstName
	if lastName != "" {
		if toName != "" {
			toName += " "
		}
		toName += lastName
	}
	if toName == "" {
		toName = email
	}
	subject := "Your Admin Portal Login Details"
	body := fmt.Sprintf("Hello %s,\n\nYour admin account has been created. Please use the following details to log in:\n\nEmail: %s\nTemporary password: %s\n\nYou must change your password immediately after your first login.\n\n", toName, email, temporaryPassword)
	if portalURL != "" {
		body += fmt.Sprintf("Log in here: %s\n\n", portalURL)
	}
	body += "If you did not expect this email, please contact your administrator."

	html := fmt.Sprintf(`<p>Hello %s,</p>
<p>Your admin account has been created. Please use the following details to log in:</p>
<ul>
<li><strong>Email:</strong> %s</li>
<li><strong>Temporary password:</strong> %s</li>
</ul>
<p>You <strong>must change your password immediately</strong> after your first login.</p>
`, escapeHTML(toName), escapeHTML(email), escapeHTML(temporaryPassword))
	if portalURL != "" {
		html += fmt.Sprintf(`<p><a href="%s">Log in here</a></p>
`, portalURL)
	}
	html += `<p>If you did not expect this email, please contact your administrator.</p>`

	ev := NotificationEvent{
		Type:    "admin_welcome",
		Channel: "email",
		Metadata: map[string]interface{}{
			"to":       email,
			"to_name":  toName,
			"subject":  subject,
			"body":     body,
			"html":     html,
		},
	}
	payload, err := json.Marshal(ev)
	if err != nil {
		log.Printf("admin: notification marshal err=%v", err)
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := p.writer.WriteMessages(ctx, kafka.Message{Value: payload}); err != nil {
		log.Printf("admin: notification send failed err=%v", err)
		return err
	}
	log.Printf("admin: welcome email queued for %s", email)
	return nil
}

func escapeHTML(s string) string {
	var out []rune
	for _, r := range s {
		switch r {
		case '<':
			out = append(out, '&', 'l', 't', ';')
		case '>':
			out = append(out, '&', 'g', 't', ';')
		case '&':
			out = append(out, '&', 'a', 'm', 'p', ';')
		case '"':
			out = append(out, '&', 'q', 'u', 'o', 't', ';')
		default:
			out = append(out, r)
		}
	}
	return string(out)
}
