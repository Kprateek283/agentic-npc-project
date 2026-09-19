package config

import (
	"fmt"
	"os"
)

type Config struct {
	PostgresDSN   string
	RedisAddr     string
	ServerPort    string
	AIServiceAddr string
	GamedataDir   string
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

	serverPort := os.Getenv("SERVER_PORT")
	if serverPort == "" {
		serverPort = "8080"
	}

	aiServiceAddr := os.Getenv("AI_SERVICE_ADDR")
	if aiServiceAddr == "" {
		aiServiceAddr = "localhost:50051"
	}

	gamedataDir := os.Getenv("GAMEDATA_DIR")
	if gamedataDir == "" {
		gamedataDir = "../gamedata"
	}

	return &Config{
		PostgresDSN:   postgresDSN,
		RedisAddr:     redisAddr,
		ServerPort:    serverPort,
		AIServiceAddr: aiServiceAddr,
		GamedataDir:   gamedataDir,
	}, nil
}
