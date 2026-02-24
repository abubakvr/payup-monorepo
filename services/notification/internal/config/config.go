package config

import "os"

type Config struct {
	Port string

	// Kafka
	KafkaBroker string

	// Brevo (email)
	BrevoAPIKey string
	BrevoSenderEmail string
	BrevoSenderName  string

	// Termii (SMS)
	TermiiAPIKey   string
	TermiiSenderID string
	TermiiBaseURL  string

	// WhatsApp Business Cloud API
	WhatsAppToken      string
	WhatsAppPhoneID    string
	WhatsAppAPIVersion string
}

func LoadConfig() *Config {
	port := os.Getenv("NOTIFICATION_SERVICE_PORT")
	if port == "" {
		port = "8004"
	}
	kafkaBroker := os.Getenv("KAFKA_BROKER")
	if kafkaBroker == "" {
		kafkaBroker = "redpanda:9092"
	}
	brevoSender := os.Getenv("BREVO_SENDER_EMAIL")
	if brevoSender == "" {
		brevoSender = "noreply@example.com"
	}
	brevoName := os.Getenv("BREVO_SENDER_NAME")
	if brevoName == "" {
		brevoName = "PayUp"
	}
	termiiBase := os.Getenv("TERMII_BASE_URL")
	if termiiBase == "" {
		termiiBase = "https://api.termii.com"
	}
	waVersion := os.Getenv("WHATSAPP_API_VERSION")
	if waVersion == "" {
		waVersion = "v21.0"
	}

	return &Config{
		Port:               port,
		KafkaBroker:        kafkaBroker,
		BrevoAPIKey:        os.Getenv("BREVO_API_KEY"),
		BrevoSenderEmail:   brevoSender,
		BrevoSenderName:    brevoName,
		TermiiAPIKey:       os.Getenv("TERMII_API_KEY"),
		TermiiSenderID:     os.Getenv("TERMII_SENDER_ID"),
		TermiiBaseURL:      termiiBase,
		WhatsAppToken:      os.Getenv("WHATSAPP_ACCESS_TOKEN"),
		WhatsAppPhoneID:     os.Getenv("WHATSAPP_PHONE_NUMBER_ID"),
		WhatsAppAPIVersion: waVersion,
	}
}
