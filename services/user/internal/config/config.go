package config

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Config struct {
	Port                     string
	GrpcPort                 string // gRPC port for KYC service (e.g. GetUserForKYC)
	DatabaseURL              string
	KafkaBroker              string
	EmailVerificationBaseURL string
	PasswordResetBaseURL     string
	AdminAPIKey              string // If set, admin routes require X-Admin-Key header
	// UserExistsCacheTTLSeconds is the TTL for Redis cache "user exists" (auth validate). Default 900 (15 min).
	UserExistsCacheTTLSeconds int
}

func LoadConfig() *Config {
	port := os.Getenv("USER_SERVICE_PORT")
	if port == "" {
		port = "8001"
	}
	dbURL := os.Getenv("USER_DATABASE_URL")
	if dbURL == "" {
		dbURL = fmt.Sprintf(
			"postgres://%s:%s@%s:%s/%s?sslmode=%s",
			os.Getenv("USER_DB_USER"),
			os.Getenv("USER_DB_PASSWORD"),
			os.Getenv("USER_DB_HOST"),
			os.Getenv("USER_DB_PORT"),
			os.Getenv("USER_DB_NAME"),
			os.Getenv("USER_DB_SSLMODE"),
		)
	}
	kafkaBroker := os.Getenv("KAFKA_BROKER")
	if kafkaBroker == "" {
		kafkaBroker = "redpanda:9092"
	}
	grpcPort := os.Getenv("USER_GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "9001"
	}
	adminKey := os.Getenv("ADMIN_API_KEY")
	if adminKey == "" {
		adminKey = os.Getenv("USER_ADMIN_API_KEY")
	}
	ttl := 900
	if s := os.Getenv("USER_EXISTS_CACHE_TTL_SECONDS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			ttl = n
		}
	}
	return &Config{
		Port:                     port,
		GrpcPort:                 grpcPort,
		DatabaseURL:              dbURL,
		KafkaBroker:              kafkaBroker,
		EmailVerificationBaseURL: os.Getenv("EMAIL_VERIFICATION_BASE_URL"),
		PasswordResetBaseURL:     os.Getenv("PASSWORD_RESET_BASE_URL"),
		AdminAPIKey:              adminKey,
		UserExistsCacheTTLSeconds: ttl,
	}
}

// OpenDB opens a Postgres connection. Caller must close when done.
func OpenDB(cfg *Config) (*sql.DB, error) {
	return sql.Open("pgx", cfg.DatabaseURL)
}
