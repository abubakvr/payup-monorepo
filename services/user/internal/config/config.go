package config

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Config struct {
	Port                      string
	DatabaseURL               string
	KafkaBroker               string
	EmailVerificationBaseURL   string
	PasswordResetBaseURL      string
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
	return &Config{
		Port:                    port,
		DatabaseURL:             dbURL,
		KafkaBroker:             kafkaBroker,
		EmailVerificationBaseURL: os.Getenv("EMAIL_VERIFICATION_BASE_URL"),
		PasswordResetBaseURL:    os.Getenv("PASSWORD_RESET_BASE_URL"),
	}
}

// OpenDB opens a Postgres connection. Caller must close when done.
func OpenDB(cfg *Config) (*sql.DB, error) {
	return sql.Open("pgx", cfg.DatabaseURL)
}
