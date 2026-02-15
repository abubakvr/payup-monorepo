// cmd/api/main.go
package main

import (
	"log"
	"net/http"

	"github.com/abubakvr/payup-backend/services/user/internal/config"
	"github.com/abubakvr/payup-backend/services/user/internal/kafka"
	"github.com/abubakvr/payup-backend/services/user/internal/router"
	"github.com/abubakvr/payup-backend/services/user/redis"
)

func main() {
	cfg := config.LoadConfig()
	r := router.SetupRouter(cfg)

	redis.InitRedis()

	producer := kafka.NewProducer([]string{"redpanda:9092"})
	if err := producer.UserCreated([]byte(`{"user_id":"123","email":"test@payup.com"}`)); err != nil {
		log.Printf("Kafka produce failed: %v", err)
	} else {
		log.Printf("Produced user-created event to user-events topic")
	}

	log.Printf("User service running on port %s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		log.Fatalf("Failed to start user service: %v", err)
	}
}
