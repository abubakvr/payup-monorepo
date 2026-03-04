package kafka

import (
	"context"
	"encoding/json"
	"time"

	"github.com/segmentio/kafka-go"
)

const notificationTopic = "notification-events"

// NotificationEvent matches the payload consumed by the notification service.
type NotificationEvent struct {
	Type     string                 `json:"type"`
	Channel  string                 `json:"channel"`
	Metadata map[string]interface{} `json:"metadata"`
}

// NotificationProducer writes to the notification-events topic (SMS, email, etc.).
type NotificationProducer struct {
	writer *kafka.Writer
}

// NewNotificationProducer creates a producer for notification-events.
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

// Send publishes a notification event. Safe to call with nil producer (no-op).
func (p *NotificationProducer) Send(ev NotificationEvent) error {
	if p == nil || p.writer == nil {
		return nil
	}
	payload, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return p.writer.WriteMessages(ctx, kafka.Message{Value: payload})
}
