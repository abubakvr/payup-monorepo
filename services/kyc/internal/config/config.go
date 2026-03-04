package config

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Config struct {
	Port                string
	DatabaseURL         string
	EncryptionKey       string // 32-byte hex for AES-256 (KYC_ENCRYPTION_KEY)
	JWTSecret           string // same as user service for auth_request
	UserServiceGrpcAddr string // user service gRPC address for GetUserForKYC (e.g. user-service:9001)
	KafkaBroker         string // comma-separated brokers for audit-events and notification-events
	// Dojah BVN: set DOJAH_APP_ID, DOJAH_AUTHORIZATION_KEY; optional BVN_SELFIE_MIN_CONFIDENCE (default 70)
}

func LoadConfig() *Config {
	port := os.Getenv("KYC_SERVICE_PORT")
	if port == "" {
		port = "8002"
	}
	dbURL := os.Getenv("KYC_DATABASE_URL")
	if dbURL == "" {
		dbURL = fmt.Sprintf(
			"postgres://%s:%s@%s:%s/%s?sslmode=%s",
			os.Getenv("KYC_DB_USER"),
			os.Getenv("KYC_DB_PASSWORD"),
			os.Getenv("KYC_DB_HOST"),
			os.Getenv("KYC_DB_PORT"),
			os.Getenv("KYC_DB_NAME"),
			os.Getenv("KYC_DB_SSLMODE"),
		)
	}
	encKey := os.Getenv("KYC_ENCRYPTION_KEY")
	if encKey == "" {
		encKey = os.Getenv("ENCRYPTION_KEY")
	}
	userGrpc := os.Getenv("USER_SERVICE_GRPC_ADDR")
	if userGrpc == "" {
		userGrpc = "user-service:9001"
	}
	kafkaBroker := os.Getenv("KAFKA_BROKER")
	if kafkaBroker == "" {
		kafkaBroker = "redpanda:9092"
	}
	return &Config{
		Port:                port,
		DatabaseURL:         dbURL,
		EncryptionKey:       encKey,
		JWTSecret:           os.Getenv("JWT_SECRET"),
		UserServiceGrpcAddr: userGrpc,
		KafkaBroker:         kafkaBroker,
	}
}

func OpenDB(cfg *Config) (*sql.DB, error) {
	return sql.Open("pgx", cfg.DatabaseURL)
}
