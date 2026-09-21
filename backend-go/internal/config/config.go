package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	PostgresDSN    string
	RedisAddr      string
	ServerPort     string
	AIServiceAddr  string
	GamedataDir    string
	AdminEnabled   bool
	AllowedOrigins []string
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

	adminEnabled := os.Getenv("ADMIN_ENABLED") == "true"

	var allowedOrigins []string
	if rawOrigins := os.Getenv("ALLOWED_ORIGINS"); rawOrigins != "" {
		for _, o := range strings.Split(rawOrigins, ",") {
			trimmed := strings.TrimSpace(o)
			if trimmed != "" {
				allowedOrigins = append(allowedOrigins, trimmed)
			}
		}
	}

	return &Config{
		PostgresDSN:    postgresDSN,
		RedisAddr:      redisAddr,
		ServerPort:     serverPort,
		AIServiceAddr:  aiServiceAddr,
		GamedataDir:    gamedataDir,
		AdminEnabled:   adminEnabled,
		AllowedOrigins: allowedOrigins,
	}, nil
}

