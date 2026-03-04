package kafka

import (
	"context"
	"encoding/json"
	"time"

	"github.com/segmentio/kafka-go"
)

const auditTopic = "audit-events"

// AuditProducer writes audit events to the audit-events topic.
type AuditProducer struct {
	writer *kafka.Writer
}

// NewAuditProducer creates a producer that writes to audit-events. brokers can be one or more, e.g. []string{"redpanda:9092"}.
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

// SendAuditLog builds an audit event from params and sends it. Safe to call with nil producer (no-op).
func (p *AuditProducer) SendAuditLog(params AuditLogParams) error {
	if p == nil || p.writer == nil {
		return nil
	}
	event := AuditEvent{
		Service:       params.Service,
		UserID:        params.UserID,
		Action:        params.Action,
		Entity:        params.Entity,
		EntityID:      strPtr(params.EntityID),
		Metadata:      params.Metadata,
		CorrelationID: params.CorrelationID,
		Timestamp:     time.Now(),
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
