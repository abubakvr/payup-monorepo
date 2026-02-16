package kafka

import (
	"context"
	"encoding/json"
	"log"

	"github.com/abubakvr/payup-backend/services/audit/internal/model"
	"github.com/abubakvr/payup-backend/services/audit/internal/service"

	"github.com/segmentio/kafka-go"
)

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

		if err := c.service.ProcessAuditEvent(event); err != nil {
			log.Println("Failed tp save audit log:", err)
		}
	}
}
