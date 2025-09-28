package config

import (
	"fmt"
	"os"
)

type Config struct {
	PostgresDSN string
	RedisAddr   string
}

func Load() (*Config, error) {
	postgresDSN := os.Getenv("POSTGRES_DSN")
	if postgresDSN == "" {
		return nil, fmt.Errorf("POSTGRES_DSN environment variable is not set")
	}

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	return &Config{
		PostgresDSN: postgresDSN,
		RedisAddr:   redisAddr,
	}, nil
}
