package kafka

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

const (
	auditTopic        = "audit-events"
	notificationTopic = "notification-events"
	walletTopic       = "wallet-events"
)

// NotificationEvent matches the payload consumed by the notification service (SMS, email, etc.).
type NotificationEvent struct {
	Type     string                 `json:"type"`
	Channel  string                 `json:"channel"`
	Metadata map[string]interface{} `json:"metadata"`
}

// WalletCreatedEvent is published to wallet-events when a wallet is created. KYC service consumes to set kyc_level=1.
type WalletCreatedEvent struct {
	EventType string `json:"event_type"` // "wallet_created"
	UserID    string `json:"user_id"`
}

// Producer sends to audit-events, notification-events, and wallet-events.
type Producer struct {
	auditWriter        *kafka.Writer
	notificationWriter *kafka.Writer
	walletWriter       *kafka.Writer
}

// NewProducer creates a producer for audit, notification, and wallet topics.
func NewProducer(brokers []string) *Producer {
	if len(brokers) == 0 {
		return nil
	}
	return &Producer{
		auditWriter: kafka.NewWriter(kafka.WriterConfig{
			Brokers: brokers,
			Topic:   auditTopic,
		}),
		notificationWriter: kafka.NewWriter(kafka.WriterConfig{
			Brokers: brokers,
			Topic:   notificationTopic,
		}),
		walletWriter: kafka.NewWriter(kafka.WriterConfig{
			Brokers: brokers,
			Topic:   walletTopic,
		}),
	}
}

// SendAuditEvent sends an event to audit-events (consumed by audit service).
func (p *Producer) SendAuditEvent(event AuditEvent) error {
	if p == nil || p.auditWriter == nil {
		return nil
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return p.auditWriter.WriteMessages(ctx, kafka.Message{Value: payload})
}

// SendAuditLog builds an audit event from params and sends it. Safe to call with nil producer.
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

// SendNotification publishes to notification-events (SMS, email, etc.). Safe to call with nil producer.
func (p *Producer) SendNotification(ev NotificationEvent) error {
	if p == nil || p.notificationWriter == nil {
		return nil
	}
	payload, err := json.Marshal(ev)
	if err != nil {
		log.Printf("payment kafka: SendNotification marshal err=%v", err)
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = p.notificationWriter.WriteMessages(ctx, kafka.Message{Value: payload})
	if err != nil {
		log.Printf("payment kafka: SendNotification err=%v", err)
		return err
	}
	return nil
}

// PublishWalletCreated sends a wallet_created event to wallet-events. KYC service consumes and sets kyc_level=1.
func (p *Producer) PublishWalletCreated(ctx context.Context, userID string) error {
	if p == nil || p.walletWriter == nil {
		return nil
	}
	ev := WalletCreatedEvent{EventType: "wallet_created", UserID: userID}
	payload, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return p.walletWriter.WriteMessages(cctx, kafka.Message{Value: payload})
}

func ptr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
