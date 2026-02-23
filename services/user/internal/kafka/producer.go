package kafka

import (
	"context"
	"encoding/json"
	"time"

	"github.com/segmentio/kafka-go"
)

type Producer struct {
	writer      *kafka.Writer
	auditWriter *kafka.Writer
}

func NewProducer(brokers []string) *Producer {
	return &Producer{
		writer: kafka.NewWriter(kafka.WriterConfig{
			Brokers: brokers,
			Topic:   "user-events",
		}),
		auditWriter: kafka.NewWriter(kafka.WriterConfig{
			Brokers: brokers,
			Topic:   "audit-events",
		}),
	}
}

func (p *Producer) UserCreated(payload []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return p.writer.WriteMessages(ctx, kafka.Message{
		Value: payload,
	})
}

// SendAuditEvent sends an audit event to the audit-events topic for the audit service to consume.
func (p *Producer) SendAuditEvent(event AuditEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return p.auditWriter.WriteMessages(ctx, kafka.Message{
		Value: payload,
	})
}

// AuditLogParams holds parameters for a reusable audit log. Use SendAuditLog to publish.
type AuditLogParams struct {
	Service       string
	Action        string
	Entity        string
	EntityID      string
	UserID        *string
	Metadata      map[string]interface{}
	CorrelationID *string
}

// SendAuditLog builds an audit event from params and sends it to the audit-events topic.
// Safe to call with a nil producer (no-op). Reusable from any service (user, payment, etc.).
func (p *Producer) SendAuditLog(params AuditLogParams) error {
	if p == nil {
		return nil
	}
	event := AuditEvent{
		Service:       params.Service,
		UserID:        params.UserID,
		Action:        params.Action,
		Entity:        params.Entity,
		EntityID:      ptr(params.EntityID),
		Metadata:      params.Metadata,
		CorrelationID: params.CorrelationID,
		Timestamp:     time.Now(),
	}
	return p.SendAuditEvent(event)
}

func ptr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
