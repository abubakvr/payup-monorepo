package config

import "os"

type Config struct {
	Port string
}

func LoadConfig() *Config {
	port := os.Getenv("KYC_SERVICE_PORT")
	if port == "" {
		port = "8002"
	}

	return &Config{
		Port: port,
	}
}
