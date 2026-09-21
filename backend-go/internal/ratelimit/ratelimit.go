// Package ratelimit provides a Redis-backed fixed-window rate limiter for protecting shared resources.
package ratelimit

import (
	"context"
	"errors"
	"time"

	"github.com/go-redis/redis/v8"
)

// Allow checks whether an event for key is permitted under a fixed-window rate limit.
// A limit <= 0 disables rate limiting and returns allowed=true without touching Redis (rdb may be nil).
// Otherwise, it runs INCR and EXPIRE ... NX together in one transaction (rdb.TxPipelined).
// This guarantees the key always has a TTL without extending it on subsequent calls.
// On any Redis error, it returns false, 0, err so the caller can decide whether to fail open.
func Allow(ctx context.Context, rdb *redis.Client, key string, limit int, window time.Duration) (allowed bool, count int64, err error) {
	if limit <= 0 {
		return true, 0, nil
	}
	if rdb == nil {
		return false, 0, errors.New("redis client is nil")
	}

	var incrCmd *redis.IntCmd
	_, err = rdb.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		incrCmd = pipe.Incr(ctx, key)
		pipe.ExpireNX(ctx, key, window)
		return nil
	})
	if err != nil {
		return false, 0, err
	}

	count = incrCmd.Val()
	return count <= int64(limit), count, nil
}
