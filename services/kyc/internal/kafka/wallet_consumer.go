package kafka

import (
	"context"
	"encoding/json"
	"log"

	"github.com/segmentio/kafka-go"
)

const walletEventsTopic = "wallet-events"
const walletEventsGroupID = "kyc-service-wallet"

// WalletCreatedMessage matches the event published by payment service when a wallet is created.
type WalletCreatedMessage struct {
	EventType string `json:"event_type"`
	UserID    string `json:"user_id"`
}

// WalletEventsConsumer consumes wallet-events and invokes onWalletCreated for each wallet_created event.
type WalletEventsConsumer struct {
	reader          *kafka.Reader
	onWalletCreated func(ctx context.Context, userID string)
}

// NewWalletEventsConsumer creates a consumer for wallet-events. onWalletCreated is called for each wallet_created (e.g. update kyc_level=1).
func NewWalletEventsConsumer(brokers []string, onWalletCreated func(ctx context.Context, userID string)) *WalletEventsConsumer {
	if len(brokers) == 0 || onWalletCreated == nil {
		return nil
	}
	return &WalletEventsConsumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers: brokers,
			Topic:   walletEventsTopic,
			GroupID: walletEventsGroupID,
		}),
		onWalletCreated: onWalletCreated,
	}
}

// Start runs the consumer loop. Call in a goroutine.
func (c *WalletEventsConsumer) Start() {
	if c == nil {
		return
	}
	for {
		msg, err := c.reader.ReadMessage(context.Background())
		if err != nil {
			log.Printf("kyc wallet-events consumer: %v", err)
			continue
		}
		var ev WalletCreatedMessage
		if err := json.Unmarshal(msg.Value, &ev); err != nil {
			log.Printf("kyc wallet-events: invalid JSON: %v", err)
			continue
		}
		if ev.EventType != "wallet_created" || ev.UserID == "" {
			continue
		}
		c.onWalletCreated(context.Background(), ev.UserID)
	}
}
