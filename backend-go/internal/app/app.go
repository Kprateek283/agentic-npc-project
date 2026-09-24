package app

import (
	"agentic-npc-backend/internal/api/handlers"
	"agentic-npc-backend/internal/config"
	"agentic-npc-backend/internal/db/ent"
	"agentic-npc-backend/internal/domain/quest_logic"
	"agentic-npc-backend/internal/domain/rules"
	"agentic-npc-backend/internal/infra/database"
	"agentic-npc-backend/internal/infra/grpc_client"
	"agentic-npc-backend/internal/infra/httpsapi"
	"fmt"
	"log"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/joho/godotenv"
)

// App holds all the core components of the application.
type App struct {
	Config       *config.Config
	DBClient     *ent.Client
	RedisClient  *redis.Client
	AIClient     *grpc_client.AIClient
	QuestManager *quest_logic.QuestManager
	Server       *gin.Engine
}

// New creates and initializes a new application instance.
func New() (*App, error) {
	// 1. Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: .env file not found, using system environment variables")
	}

	// 2. Load configuration
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	log.Println("Configuration loaded successfully")

	// 3. Initialize Database Client
	dbClient := database.NewClient(cfg)
	log.Println("Database client initialized")

	// 4. Initialize Redis Client
	redisClient := database.NewRedisClient(cfg)
	log.Println("Redis client initialized")

	// 5. Seed Database (NPCs)
	err = database.SeedNPCs(dbClient, filepath.Join(cfg.GamedataDir, "npcs"))
	if err != nil {
		err := dbClient.Close()
		if err != nil {
			return nil, err
		}
		redisClient.Close()
		return nil, fmt.Errorf("failed to seed NPCs: %w", err)
	}
	log.Println("NPC database seeded successfully")

	// 5b. Seed Database (Quests)
	err = database.SeedQuests(dbClient, filepath.Join(cfg.GamedataDir, "quests"))
	if err != nil {
		err := dbClient.Close()
		if err != nil {
			return nil, err
		}
		redisClient.Close()
		return nil, fmt.Errorf("failed to seed quests: %w", err)
	}
	log.Println("Quest database seeded successfully")

	// 6. Initialize QuestManager
	questManager, err := quest_logic.NewQuestManager(cfg.GamedataDir)
	if err != nil {
		err := dbClient.Close()
		if err != nil {
			return nil, err
		}
		redisClient.Close()
		return nil, fmt.Errorf("failed to create quest manager: %w", err)
	}
	questManager.AdminEnabled = cfg.AdminEnabled

	// 6b. Load Rules
	rulesData, err := rules.Load(filepath.Join(cfg.GamedataDir, "events.json"))
	if err != nil {
		err := dbClient.Close()
		if err != nil {
			return nil, err
		}
		redisClient.Close()
		return nil, fmt.Errorf("failed to load rules: %w", err)
	}
	questManager.Rules = rulesData
	log.Println("Dungeon Master (QuestManager) initialized successfully")

	// 7. Initialize gRPC AI Client
	aiClient, err := grpc_client.NewAIClient(cfg.AIServiceAddr, cfg.AICallTimeout)
	if err != nil {
		err := dbClient.Close()
		if err != nil {
			return nil, err
		}
		redisClient.Close()
		return nil, fmt.Errorf("failed to create AI client: %w", err)
	}
	log.Println("Successfully connected to AI gRPC server")

	// 8. Create Handlers
	healthHandler := handlers.HealthHandler
	wsHandler := handlers.NewWebSocketHandler(dbClient, aiClient, questManager, redisClient, cfg.AllowedOrigins, cfg.LLMRateLimit, cfg.LLMRateWindow)
	log.Println("API Handlers initialized")

	// 9. Initialize HTTP Server
	server := httpsapi.NewServer(healthHandler, wsHandler)
	log.Println("HTTP server initialized")

	// 10. Return the fully assembled application
	return &App{
		Config:       cfg,
		DBClient:     dbClient,
		RedisClient:  redisClient,
		AIClient:     aiClient,
		QuestManager: questManager,
		Server:       server,
	}, nil
}

// Run starts the application's HTTP server.
func (a *App) Run() error {
	addr := ":" + a.Config.ServerPort
	log.Printf("Starting server on %s", addr)
	err := a.Server.Run(addr)
	if err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}
	return nil
}

// Shutdown gracefully closes all application resources.
func (a *App) Shutdown() {
	log.Println("Shutting down application...")
	if a.DBClient != nil {
		if err := a.DBClient.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}
	if a.RedisClient != nil {
		if err := a.RedisClient.Close(); err != nil {
			log.Printf("Error closing redis: %v", err)
		}
	}
	log.Println("Shutdown complete.")
}
