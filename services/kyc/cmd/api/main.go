package main

import (
	"log"
	"net/http"

	"github.com/abubakvr/payup-backend/services/kyc/internal/config"
	"github.com/abubakvr/payup-backend/services/kyc/internal/kafka"
	"github.com/abubakvr/payup-backend/services/kyc/internal/router"
)

func main() {
	cfg := config.LoadConfig()
	r := router.SetupRouter(cfg)

	consumer := kafka.NewConsumer([]string{"redpanda:9092"})
	go consumer.Start()

	log.Printf("KYC service running on port %s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		log.Fatal(err)
	}
}
