// cmd/api/main.go
package main

import (
	"log"
	"net/http"

	"github.com/abubakvr/payup-backend/services/user/internal/config"
	"github.com/abubakvr/payup-backend/services/user/internal/kafka"
	"github.com/abubakvr/payup-backend/services/user/internal/router"
)

func main() {
	cfg := config.LoadConfig()
	r := router.SetupRouter(cfg)

	producer := kafka.NewProducer([]string{"redpanda:9092"})
	producer.UserCreated([]byte(`{"user_id":"123","email":"test@payup.com"}`))

	log.Printf("User service running on port %s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		log.Fatalf("Failed to start user service: %v", err)
	}
}
