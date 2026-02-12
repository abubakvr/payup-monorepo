package config

import "os"

type Config struct {
	Port string
}

func LoadConfig() *Config {
	port := os.Getenv("USER_SERVICE_PORT")
	if port == "" {
		port = "8001"
	}

	return &Config{
		Port: port,
	}
}
