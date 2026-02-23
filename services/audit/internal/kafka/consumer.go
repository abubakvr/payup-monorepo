package kafka

import (
	"context"
	"encoding/json"
	"log"

	"github.com/abubakvr/payup-backend/services/audit/internal/model"
	"github.com/abubakvr/payup-backend/services/audit/internal/service"

	"github.com/segmentio/kafka-go"
)

func logAuditEvent(event model.AuditEvent) {
	userID := "<nil>"
	if event.UserID != nil {
		userID = *event.UserID
	}
	entityID := "<nil>"
	if event.EntityID != nil {
		entityID = *event.EntityID
	}
	correlationID := "<nil>"
	if event.CorrelationID != nil {
		correlationID = *event.CorrelationID
	}
	log.Printf("audit received: service=%s action=%s entity=%s entity_id=%s user_id=%s correlation_id=%s timestamp=%s metadata=%v",
		event.Service, event.Action, event.Entity, entityID, userID, correlationID, event.Timestamp.Format("2006-01-02T15:04:05.000Z07:00"), event.Metadata)
}

type Consumer struct {
	reader  *kafka.Reader
	service *service.AuditService
}

func NewConsumer(brokers []string, svc *service.AuditService) *Consumer {
	return &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers: brokers,
			Topic:   "audit-events",
			GroupID: "audit-service",
		}),
		service: svc,
	}
}

func (c *Consumer) Start() {
	for {
		msg, err := c.reader.ReadMessage(context.Background())
		if err != nil {
			log.Println("kafka error:", err)
			continue
		}

		var event model.AuditEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			log.Println("Invalid audit event:", err)
			continue
		}

		logAuditEvent(event)

		if err := c.service.ProcessAuditEvent(event); err != nil {
			log.Println("Failed to save audit log:", err)
		}
	}
}
