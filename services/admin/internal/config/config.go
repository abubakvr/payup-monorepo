package config

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Config struct {
	Port                  string
	DatabaseURL           string
	JWTSecret             string
	BootstrapEmail        string
	BootstrapPassword     string
	BootstrapFirstName    string
	BootstrapLastName     string
	UserServiceGrpcAddr   string // e.g. user-service:9001
	KYCServiceGrpcAddr    string // e.g. kyc-service:9002
	AuditServiceGrpcAddr  string // e.g. audit-service:9003
	PaymentServiceGrpcAddr string // e.g. payment-service:9004
	KYCAdminAPIKey        string // X-Admin-Key used to call KYC HTTP admin image endpoint
	KafkaBroker           string // e.g. redpanda:9092 (for audit-events, notification-events)
	AdminPortalURL        string // optional; e.g. https://admin.payup.ng (included in welcome email login link)
}

func LoadConfig() *Config {
	port := os.Getenv("ADMIN_SERVICE_PORT")
	if port == "" {
		port = "8005"
	}
	dbURL := os.Getenv("ADMIN_DATABASE_URL")
	if dbURL == "" {
		dbURL = fmt.Sprintf(
			"postgres://%s:%s@%s:%s/%s?sslmode=%s",
			os.Getenv("ADMIN_DB_USER"),
			os.Getenv("ADMIN_DB_PASSWORD"),
			os.Getenv("ADMIN_DB_HOST"),
			os.Getenv("ADMIN_DB_PORT"),
			os.Getenv("ADMIN_DB_NAME"),
			os.Getenv("ADMIN_DB_SSLMODE"),
		)
	}
	jwtSecret := os.Getenv("ADMIN_JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = os.Getenv("JWT_SECRET")
	}
	userGrpc := os.Getenv("USER_SERVICE_GRPC_ADDR")
	if userGrpc == "" {
		userGrpc = "user-service:9001"
	}
	kycGrpc := os.Getenv("KYC_SERVICE_GRPC_ADDR")
	if kycGrpc == "" {
		kycGrpc = "kyc-service:9002"
	}
	auditGrpc := os.Getenv("AUDIT_SERVICE_GRPC_ADDR")
	if auditGrpc == "" {
		auditGrpc = "audit-service:9003"
	}
	paymentGrpc := os.Getenv("PAYMENT_SERVICE_GRPC_ADDR")
	if paymentGrpc == "" {
		paymentGrpc = "payment-service:9004"
	}
	kycAdminKey := os.Getenv("KYC_ADMIN_API_KEY")
	if kycAdminKey == "" {
		kycAdminKey = os.Getenv("ADMIN_API_KEY")
	}
	kafkaBroker := os.Getenv("KAFKA_BROKER")
	if kafkaBroker == "" {
		kafkaBroker = "redpanda:9092"
	}
	portalURL := os.Getenv("ADMIN_PORTAL_URL")
	return &Config{
		Port:                 port,
		DatabaseURL:          dbURL,
		JWTSecret:            jwtSecret,
		BootstrapEmail:       os.Getenv("ADMIN_BOOTSTRAP_EMAIL"),
		BootstrapPassword:    os.Getenv("ADMIN_BOOTSTRAP_PASSWORD"),
		BootstrapFirstName:   os.Getenv("ADMIN_BOOTSTRAP_FIRST_NAME"),
		BootstrapLastName:    os.Getenv("ADMIN_BOOTSTRAP_LAST_NAME"),
		UserServiceGrpcAddr:  userGrpc,
		KYCServiceGrpcAddr:   kycGrpc,
		AuditServiceGrpcAddr:  auditGrpc,
		PaymentServiceGrpcAddr: paymentGrpc,
		KYCAdminAPIKey:       kycAdminKey,
		KafkaBroker:          kafkaBroker,
		AdminPortalURL:       portalURL,
	}
}

func OpenDB(cfg *Config) (*sql.DB, error) {
	return sql.Open("pgx", cfg.DatabaseURL)
}
