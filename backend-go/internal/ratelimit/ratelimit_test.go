package ratelimit

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
)

func TestAllowDisabled(t *testing.T) {
	ctx := context.Background()
	allowed, count, err := Allow(ctx, nil, "test:key", 0, time.Minute)
	if err != nil {
		t.Fatalf("expected no error when limit is 0, got: %v", err)
	}
	if !allowed {
		t.Fatalf("expected allowed=true when limit is 0, got false")
	}
	if count != 0 {
		t.Fatalf("expected count=0 when limit is 0, got %d", count)
	}
}

func TestAllowAgainstRedis(t *testing.T) {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		t.Skip("REDIS_ADDR not set")
	}

	ctx := context.Background()
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	defer rdb.Close()

	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Fatalf("failed to ping redis at %s: %v", redisAddr, err)
	}

	key := fmt.Sprintf("test:ratelimit:%d", time.Now().UnixNano())
	defer rdb.Del(ctx, key)

	limit := 3
	window := 2 * time.Second

	// Call 1: allowed, count 1
	allowed, count, err := Allow(ctx, rdb, key, limit, window)
	if err != nil {
		t.Fatalf("call 1 failed: %v", err)
	}
	if !allowed || count != 1 {
		t.Fatalf("call 1: expected allowed=true, count=1; got allowed=%v, count=%d", allowed, count)
	}

	// rdb.TTL on the key is > 0 after the first call
	ttl1, err := rdb.TTL(ctx, key).Result()
	if err != nil {
		t.Fatalf("failed to get TTL after call 1: %v", err)
	}
	if ttl1 <= 0 {
		t.Fatalf("expected TTL > 0 after call 1, got %v", ttl1)
	}

	// Small pause so any TTL extension would be measurable
	time.Sleep(100 * time.Millisecond)

	// Call 2: allowed, count 2
	allowed, count, err = Allow(ctx, rdb, key, limit, window)
	if err != nil {
		t.Fatalf("call 2 failed: %v", err)
	}
	if !allowed || count != 2 {
		t.Fatalf("call 2: expected allowed=true, count=2; got allowed=%v, count=%d", allowed, count)
	}

	// Call 3: allowed, count 3
	allowed, count, err = Allow(ctx, rdb, key, limit, window)
	if err != nil {
		t.Fatalf("call 3 failed: %v", err)
	}
	if !allowed || count != 3 {
		t.Fatalf("call 3: expected allowed=true, count=3; got allowed=%v, count=%d", allowed, count)
	}

	// TTL is not extended by later calls
	ttl2, err := rdb.TTL(ctx, key).Result()
	if err != nil {
		t.Fatalf("failed to get TTL after call 3: %v", err)
	}
	if ttl2 > ttl1 {
		t.Fatalf("TTL was extended by later calls: initial ttl=%v, subsequent ttl=%v", ttl1, ttl2)
	}

	// Call 4: denied, count 4
	allowed, count, err = Allow(ctx, rdb, key, limit, window)
	if err != nil {
		t.Fatalf("call 4 failed: %v", err)
	}
	if allowed || count != 4 {
		t.Fatalf("call 4: expected allowed=false, count=4; got allowed=%v, count=%d", allowed, count)
	}

	// After sleeping past the window, the next call is allowed with count 1
	time.Sleep(window + 100*time.Millisecond)

	allowed, count, err = Allow(ctx, rdb, key, limit, window)
	if err != nil {
		t.Fatalf("call after window expiry failed: %v", err)
	}
	if !allowed || count != 1 {
		t.Fatalf("call after window expiry: expected allowed=true, count=1; got allowed=%v, count=%d", allowed, count)
	}
}
