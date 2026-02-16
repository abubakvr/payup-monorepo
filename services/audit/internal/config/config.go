package config

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/lib/pq"
)

type Config struct {
	Port        string
	DatabaseURL string
}

func LoadConfig() *Config {
	port := os.Getenv("AUDIT_SERVICE_PORT")
	if port == "" {
		port = "8003"
	}
	dbURL := os.Getenv("AUDIT_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/audit_db?sslmode=disable"
	}

	return &Config{
		Port:        port,
		DatabaseURL: dbURL,
	}
}

// OpenDB opens a Postgres connection from cfg and pings it. Caller must close the DB when done.
func OpenDB(cfg *Config) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	log.Println("audit: database connected")
	return db, nil
}
