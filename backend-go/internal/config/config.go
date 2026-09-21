package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	PostgresDSN    string
	RedisAddr      string
	ServerPort     string
	AIServiceAddr  string
	GamedataDir    string
	AdminEnabled   bool
	AllowedOrigins []string
	LLMRateLimit   int
	LLMRateWindow  time.Duration
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

	llmRateLimit := 20
	if rawLimit := os.Getenv("LLM_RATE_LIMIT"); rawLimit != "" {
		v, err := strconv.Atoi(rawLimit)
		if err != nil || v < 0 {
			return nil, fmt.Errorf("invalid LLM_RATE_LIMIT: %q", rawLimit)
		}
		llmRateLimit = v
	}

	llmRateWindow := 60 * time.Second
	if rawWindow := os.Getenv("LLM_RATE_WINDOW_SECONDS"); rawWindow != "" {
		v, err := strconv.Atoi(rawWindow)
		if err != nil || v < 0 {
			return nil, fmt.Errorf("invalid LLM_RATE_WINDOW_SECONDS: %q", rawWindow)
		}
		llmRateWindow = time.Duration(v) * time.Second
	}

	return &Config{
		PostgresDSN:    postgresDSN,
		RedisAddr:      redisAddr,
		ServerPort:     serverPort,
		AIServiceAddr:  aiServiceAddr,
		GamedataDir:    gamedataDir,
		AdminEnabled:   adminEnabled,
		AllowedOrigins: allowedOrigins,
		LLMRateLimit:   llmRateLimit,
		LLMRateWindow:  llmRateWindow,
	}, nil
}

