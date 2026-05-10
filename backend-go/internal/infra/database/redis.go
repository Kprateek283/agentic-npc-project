package database

import (
	"agentic-npc-backend/internal/config"
	"context"
	"log"

	"github.com/go-redis/redis/v8"
)

// NewRedisClient creates and returns a new Redis client.
func NewRedisClient(cfg *config.Config) *redis.Client {
	// Create a new client using the address from our config
	rdb := redis.NewClient(&redis.Options{
		Addr: cfg.RedisAddr,
	})

	// Ping the Redis server to ensure the connection is successful
	ctx := context.Background()
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("failed to connect to redis: %v", err)
	}

	log.Println("Successfully connected to Redis")
	return rdb
}
