package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
)

// The limiter must never block play: when Redis is missing or unreachable, the call goes
// ahead. A denial needs a live Redis and is covered by ratelimit.TestAllowAgainstRedis.
func TestAllowLLMFailsOpen(t *testing.T) {
	unreachable := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", MaxRetries: -1})
	defer unreachable.Close()

	cases := []struct {
		name  string
		rdb   *redis.Client
		limit int
	}{
		{"limiting disabled, no Redis", nil, 0},
		{"limiting on, no Redis client", nil, 5},
		{"limiting on, Redis unreachable", unreachable, 5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := &WebSocketHandler{redisClient: tc.rdb, llmRateLimit: tc.limit, llmRateWindow: time.Minute}
			if !h.allowLLM(context.Background(), "req-1", "player-1") {
				t.Fatal("allowLLM = false, want true: a limiter failure must not block the call")
			}
		})
	}
}
