// Command benchcache measures the cache-aside read path: Redis hit vs PostgreSQL read.
//
// It exercises the real quest_logic.GetPlayer used by the game handler, twice per
// iteration: once cold (the Redis key is deleted first, so the lookup falls through to
// PostgreSQL and repopulates the cache) and once warm (the Redis key is present).
//
// Note what the warm path actually does, because it shapes the result: on a cache hit
// GetPlayer reads the player's UUID from Redis and *still* fetches the row from PostgreSQL
// by primary key. The cache replaces an indexed lookup on player_id with a PK fetch — it
// does not remove the database round-trip. The measurement tells us what that is worth.
//
// Usage (from backend-go/, with .env pointing at the running Postgres and Redis):
//
//	go run ./cmd/benchcache -n 200
package main

import (
	"agentic-npc-backend/internal/config"
	"agentic-npc-backend/internal/domain/quest_logic"
	"agentic-npc-backend/internal/infra/database"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	"github.com/joho/godotenv"
)

type summary struct {
	N        int     `json:"n"`
	MedianMs float64 `json:"median_ms"`
	P95Ms    float64 `json:"p95_ms"`
	MinMs    float64 `json:"min_ms"`
	MaxMs    float64 `json:"max_ms"`
}

func summarise(samples []float64) summary {
	if len(samples) == 0 {
		return summary{}
	}
	sorted := append([]float64(nil), samples...)
	sort.Float64s(sorted)
	p := func(pct float64) float64 {
		k := int(pct/100.0*float64(len(sorted))+0.5) - 1
		if k < 0 {
			k = 0
		}
		if k >= len(sorted) {
			k = len(sorted) - 1
		}
		return sorted[k]
	}
	return summary{
		N: len(sorted), MedianMs: p(50), P95Ms: p(95),
		MinMs: sorted[0], MaxMs: sorted[len(sorted)-1],
	}
}

func main() {
	n := flag.Int("n", 200, "iterations after warm-up")
	player := flag.String("player", "bench_player", "existing player_id to read")
	out := flag.String("out", "../benchmarks/results/cache_vs_db.json", "results file")
	flag.Parse()

	if err := godotenv.Load(); err != nil {
		log.Println("no .env file; using system environment")
	}
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	db := database.NewClient(cfg)
	defer db.Close()
	rdb := database.NewRedisClient(cfg)
	defer rdb.Close()

	qm, err := quest_logic.NewQuestManager("gamedata")
	if err != nil {
		log.Fatalf("quest manager: %v", err)
	}

	ctx := context.Background()
	cacheKey := "player:id_for:" + *player

	if _, err := qm.GetPlayer(ctx, db, rdb, *player); err != nil {
		log.Fatalf("player %q not found — create it first (the ws_e2e benchmark registers it): %v", *player, err)
	}

	// Warm-up: let connection pools and query plans settle.
	for i := 0; i < 20; i++ {
		rdb.Del(ctx, cacheKey)
		qm.GetPlayer(ctx, db, rdb, *player)
		qm.GetPlayer(ctx, db, rdb, *player)
	}

	var cold, warm []float64
	for i := 0; i < *n; i++ {
		// Cold: no cache entry -> indexed PostgreSQL lookup + cache repopulate.
		rdb.Del(ctx, cacheKey)
		t0 := time.Now()
		if _, err := qm.GetPlayer(ctx, db, rdb, *player); err != nil {
			log.Fatalf("cold read: %v", err)
		}
		cold = append(cold, float64(time.Since(t0).Microseconds())/1000.0)

		// Warm: cache entry present -> Redis GET + PostgreSQL primary-key fetch.
		t1 := time.Now()
		if _, err := qm.GetPlayer(ctx, db, rdb, *player); err != nil {
			log.Fatalf("warm read: %v", err)
		}
		warm = append(warm, float64(time.Since(t1).Microseconds())/1000.0)
	}

	coldS, warmS := summarise(cold), summarise(warm)
	fmt.Printf("cache MISS (PostgreSQL indexed lookup + cache set): median %.2fms p95 %.2fms (n=%d)\n",
		coldS.MedianMs, coldS.P95Ms, coldS.N)
	fmt.Printf("cache HIT  (Redis GET + PostgreSQL PK fetch):       median %.2fms p95 %.2fms (n=%d)\n",
		warmS.MedianMs, warmS.P95Ms, warmS.N)
	if warmS.MedianMs > 0 {
		fmt.Printf("speedup (miss/hit medians): %.2fx\n", coldS.MedianMs/warmS.MedianMs)
	}

	payload := map[string]any{
		"benchmark": "cache_vs_db",
		"run_date":  time.Now().Format("2006-01-02"),
		"description": "quest_logic.GetPlayer cache-aside read path. cache_miss deletes the " +
			"Redis key first, so it measures an indexed PostgreSQL lookup plus the cache " +
			"write. cache_hit measures a Redis GET followed by the PostgreSQL primary-key " +
			"fetch that GetPlayer still performs on a hit — the cache does not remove the " +
			"database round-trip.",
		"player_id":       *player,
		"cache_miss":      coldS,
		"cache_hit":       warmS,
		"miss_samples_ms": cold,
		"hit_samples_ms":  warm,
	}
	blob, _ := json.MarshalIndent(payload, "", "  ")
	if err := os.WriteFile(*out, append(blob, '\n'), 0o644); err != nil {
		log.Fatalf("write results: %v", err)
	}
	fmt.Printf("\nresults -> %s\n", *out)
}
