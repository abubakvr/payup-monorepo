package kafka

import (
	"context"
	"encoding/json"
	"log"

	"github.com/abubakvr/payup-backend/services/notification/internal/model"
	"github.com/abubakvr/payup-backend/services/notification/internal/service"

	"github.com/segmentio/kafka-go"
)

const topic = "notification-events"
const groupID = "notification-service"

// Consumer reads notification events from Kafka and hands them to the notification service.
type Consumer struct {
	reader  *kafka.Reader
	service *service.NotificationService
}

// NewConsumer creates a Kafka consumer for the notification-events topic.
func NewConsumer(brokers []string, svc *service.NotificationService) *Consumer {
	return &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers: brokers,
			Topic:   topic,
			GroupID: groupID,
		}),
		service: svc,
	}
}

func metadataKeys(m map[string]interface{}) []string {
	if m == nil {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Start runs the consumer loop. Call it in a goroutine.
func (c *Consumer) Start() {
	log.Println("notification consumer: starting read loop topic=notification-events")
	for {
		msg, err := c.reader.ReadMessage(context.Background())
		if err != nil {
			log.Println("notification consumer: kafka error:", err)
			continue
		}

		var event model.NotificationEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			log.Printf("notification consumer: invalid event err=%v raw_len=%d", err, len(msg.Value))
			continue
		}

		log.Printf("notification consumer: received type=%s channel=%s metadata_keys=%v", event.Type, event.Channel, metadataKeys(event.Metadata))
		if event.Channel == "email" && event.Metadata != nil {
			if to, ok := event.Metadata["to"].(string); ok {
				log.Printf("notification consumer: email to=%s subject=%v", to, event.Metadata["subject"])
			}
		}

		if err := c.service.Process(event); err != nil {
			log.Printf("notification consumer: process failed type=%s err=%v", event.Type, err)
		} else {
			log.Printf("notification consumer: process ok type=%s channel=%s", event.Type, event.Channel)
		}
	}
}
