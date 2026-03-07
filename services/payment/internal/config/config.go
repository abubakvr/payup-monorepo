package config

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Config struct {
	Port        string // HTTP server port
	GrpcPort    string // gRPC server port
	DatabaseURL string
	KafkaBroker string // comma-separated brokers for audit-events, notification-events

	// Redis for idempotency (X-Idempotency-Key). Optional; if empty, idempotency is disabled.
	RedisAddr     string
	RedisPassword string

	// 9PSB authentication (do not log; use env or secrets manager)
	PsbBaseURL      string // e.g. https://api.9psb.com.ng (auth, open_wallet, etc.)
	PsbBaseURL2     string // optional; for wallet_other_banks if different from PsbBaseURL
	PsbWaasBaseURL  string // optional; for WaaS debit/credit e.g. http://102.216.128.75:9090/waas
	PsbUsername     string
	PsbPassword     string
	PsbClientID     string
	PsbClientSecret string

	// 64 hex chars (32 bytes) for AES-256; encrypts auth tokens at rest in auth_tokens
	EncryptionKey string

	// JWT secret for decoding user_id from Bearer token on transfer route (same as user service).
	JWTSecret string

	KYCServiceGrpcAddr  string // e.g. kyc-service:9002
	UserServiceGrpcAddr string // e.g. user-service:9001
}

func Load() *Config {
	port := os.Getenv("PAYMENT_SERVICE_PORT")
	if port == "" {
		port = "8006"
	}
	grpcPort := os.Getenv("PAYMENT_GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "9004"
	}
	dbURL := os.Getenv("PAYMENT_DATABASE_URL")
	if dbURL == "" {
		dbURL = fmt.Sprintf(
			"postgres://%s:%s@%s:%s/%s?sslmode=%s",
			os.Getenv("PAYMENT_DB_USER"),
			os.Getenv("PAYMENT_DB_PASSWORD"),
			os.Getenv("PAYMENT_DB_HOST"),
			os.Getenv("PAYMENT_DB_PORT"),
			os.Getenv("PAYMENT_DB_NAME"),
			os.Getenv("PAYMENT_DB_SSLMODE"),
		)
	}
	kafkaBroker := os.Getenv("KAFKA_BROKER")
	if kafkaBroker == "" {
		kafkaBroker = "redpanda:9092"
	}
	return &Config{
		Port:                port,
		GrpcPort:            grpcPort,
		DatabaseURL:         dbURL,
		KafkaBroker:         kafkaBroker,
		RedisAddr:           os.Getenv("REDIS_ADDR"),
		RedisPassword:       os.Getenv("REDIS_PASSWORD"),
		PsbBaseURL:          os.Getenv("PSB_BASE_URL"),
		PsbBaseURL2:         os.Getenv("PSB_BASE_URL"), // if empty, use PsbBaseURL for wallet_other_banks
		PsbWaasBaseURL:      os.Getenv("PSB_BASE_URL"), // e.g. http://102.216.128.75:9090/waas for debit/credit
		PsbUsername:         os.Getenv("PSB_USERNAME"),
		PsbPassword:         os.Getenv("PSB_PASSWORD"),
		PsbClientID:         os.Getenv("PSB_CLIENT_ID"),
		PsbClientSecret:     os.Getenv("PSB_CLIENT_SECRET"),
		EncryptionKey:       os.Getenv("PAYMENT_ENCRYPTION_KEY"),
		JWTSecret:           os.Getenv("JWT_SECRET"),
		KYCServiceGrpcAddr:  os.Getenv("KYC_SERVICE_GRPC_ADDR"),
		UserServiceGrpcAddr: os.Getenv("USER_SERVICE_GRPC_ADDR"),
	}
}

func OpenDB(cfg *Config) (*sql.DB, error) {
	return sql.Open("pgx", cfg.DatabaseURL)
}
