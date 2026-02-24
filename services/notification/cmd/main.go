package main

import (
	"log"
	"net/http"
	"os"

	"github.com/abubakvr/payup-backend/services/notification/internal/config"
	"github.com/abubakvr/payup-backend/services/notification/internal/controller"
	"github.com/abubakvr/payup-backend/services/notification/internal/kafka"
	"github.com/abubakvr/payup-backend/services/notification/internal/providers/brevo"
	"github.com/abubakvr/payup-backend/services/notification/internal/providers/termii"
	"github.com/abubakvr/payup-backend/services/notification/internal/providers/whatsapp"
	"github.com/abubakvr/payup-backend/services/notification/internal/router"
	"github.com/abubakvr/payup-backend/services/notification/internal/service"
)

func main() {
	cfg := config.LoadConfig()

	var brevoClient *brevo.Client
	if cfg.BrevoAPIKey != "" {
		brevoClient = brevo.NewClient(cfg.BrevoAPIKey, cfg.BrevoSenderEmail, cfg.BrevoSenderName, "")
		log.Printf("notification: Brevo configured sender=%s", cfg.BrevoSenderEmail)
	} else {
		log.Printf("notification: Brevo not configured (BREVO_API_KEY empty)")
	}

	var termiiClient *termii.Client
	if cfg.TermiiAPIKey != "" {
		termiiClient = termii.NewClient(cfg.TermiiAPIKey, cfg.TermiiSenderID, cfg.TermiiBaseURL)
		log.Printf("notification: Termii configured")
	} else {
		log.Printf("notification: Termii not configured")
	}

	var whatsappClient *whatsapp.Client
	if cfg.WhatsAppToken != "" && cfg.WhatsAppPhoneID != "" {
		whatsappClient = whatsapp.NewClient(cfg.WhatsAppToken, cfg.WhatsAppPhoneID, cfg.WhatsAppAPIVersion)
		log.Printf("notification: WhatsApp configured")
	} else {
		log.Printf("notification: WhatsApp not configured")
	}

	svc := service.NewNotificationService(brevoClient, termiiClient, whatsappClient)
	log.Printf("notification: Kafka broker=%s topic=notification-events", cfg.KafkaBroker)
	ctrl := controller.NewController()
	r := router.SetupRouter(ctrl)

	brokers := []string{cfg.KafkaBroker}
	if b := os.Getenv("KAFKA_BROKER"); b != "" {
		brokers = []string{b}
	}
	consumer := kafka.NewConsumer(brokers, svc)
	go consumer.Start()

	log.Printf("Notification service running on port %s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		log.Fatal(err)
	}
}
