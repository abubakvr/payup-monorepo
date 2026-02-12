package kafka

import (
	"context"

	"log"

	"github.com/segmentio/kafka-go"
)

type Consumer struct {
	reader *kafka.Reader
}

func NewConsumer(brokers []string) *Consumer {
	return &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers: brokers,
			Topic:   "user-created",
			GroupID: "kyc-service",
		}),
	}
}

func (c *Consumer) Start() {
	for {
		msg, err := c.reader.ReadMessage(context.Background())

		if err != nil {
			log.Println("kafka error:", err)
			continue
		}

		log.Printf("KYC received user event: %s\n", string(msg.Value))
	}
}
