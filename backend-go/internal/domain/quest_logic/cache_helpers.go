package quest_logic

import (
	"agentic-npc-backend/internal/db/ent"
	entnpc "agentic-npc-backend/internal/db/ent/npc"
	entplayer "agentic-npc-backend/internal/db/ent/player"
	entplayernpcrelationship "agentic-npc-backend/internal/db/ent/playernpcrelationship"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

// --- CACHING & DB HELPER FUNCTIONS ---

// GetPlayer finds a player by their source_entity_id, using cache-aside logic
func (qm *QuestManager) GetPlayer(ctx context.Context, db *ent.Client, rdb *redis.Client, playerID string) (*ent.Player, error) {
	cacheKey := "player:id_for:" + playerID

	// 1. Try to get the PLAYER'S UUID from Redis
	val, err := rdb.Get(ctx, cacheKey).Result()
	if err == nil {
		if playerUUID, perr := uuid.Parse(val); perr == nil {
			// Fetch the full, hydrated player object by its primary key. On any error
			// (e.g. a stale id left over from a DB reset), fall through to a fresh
			// by-id lookup below that re-caches, rather than returning the error.
			if p, gerr := db.Player.Get(ctx, playerUUID); gerr == nil {
				return p, nil
			}
		}
	}

	// 2. Get from PostgreSQL (Cache Miss)
	p, err := db.Player.Query().Where(entplayer.PlayerIDEQ(playerID)).Only(ctx)
	if err != nil {
		return nil, err
	}

	// 3. Save the PLAYER'S UUID to Redis
	rdb.Set(ctx, cacheKey, p.ID.String(), time.Minute*10).Err()
	return p, nil
}

// GetNpc finds an NPC by their in-game name, using cache-aside logic
func (qm *QuestManager) GetNpc(ctx context.Context, db *ent.Client, rdb *redis.Client, npcName string) (*ent.NPC, error) {
	cacheKey := "npc:id_for:" + npcName

	// 1. Try to get the NPC'S UUID from Redis
	val, err := rdb.Get(ctx, cacheKey).Result()
	if err == nil {
		if npcUUID, perr := uuid.Parse(val); perr == nil {
			// Fetch the full, hydrated object by its primary key. On any error (e.g. a
			// stale id cached before a DB re-seed gave NPCs new UUIDs), fall through to a
			// fresh by-name lookup below that re-caches, rather than returning "not found".
			if n, gerr := db.NPC.Get(ctx, npcUUID); gerr == nil {
				return n, nil
			}
		}
	}

	// 2. Get from PostgreSQL (Cache Miss)
	n, err := db.NPC.Query().Where(entnpc.NameEQ(npcName)).Only(ctx)
	if err != nil {
		return nil, err
	}

	// 3. Save the NPC'S UUID to Redis
	rdb.Set(ctx, cacheKey, n.ID.String(), time.Minute*10).Err()
	return n, nil
}

// getItemDefinition gets a pre-loaded item's static data
func (qm *QuestManager) getItemDefinition(itemID string) (ItemDefinition, error) {
	itemDef, ok := qm.Items[itemID]
	if !ok {
		return ItemDefinition{}, fmt.Errorf("item definition not found for item_id: %s", itemID)
	}
	return itemDef, nil
}

// GetOrCreateRelationship --- THIS IS THE FIX: Capitalized 'GetOrCreateRelationship' ---
// getOrCreateRelationship finds the relationship between a player and NPC, using cache-aside
func (qm *QuestManager) GetOrCreateRelationship(ctx context.Context, db *ent.Client, rdb *redis.Client, p *ent.Player, n *ent.NPC) (*ent.PlayerNPCRelationship, error) {
	cacheKey := fmt.Sprintf("relationship:%s:%s", p.ID.String(), n.ID.String())
	val, err := rdb.Get(ctx, cacheKey).Result()
	if err == nil {
		var rel ent.PlayerNPCRelationship
		if err := json.Unmarshal([]byte(val), &rel); err == nil {
			// Re-fetch a hydrated object by id. On any error (e.g. a stale id from a DB
			// reset), fall through to the query/create path below instead of erroring.
			if hydrated, gerr := db.PlayerNPCRelationship.Get(ctx, rel.ID); gerr == nil {
				return hydrated, nil
			}
		}
	}

	// Cache miss, query the DB
	rel, err := db.PlayerNPCRelationship.
		Query().
		Where(
			entplayernpcrelationship.HasPlayerWith(entplayer.IDEQ(p.ID)),
			entplayernpcrelationship.HasNpcWith(entnpc.IDEQ(n.ID)),
		).
		Only(ctx)

	if ent.IsNotFound(err) {
		// Create it if it doesn't exist
		rel, err = db.PlayerNPCRelationship.Create().SetPlayer(p).SetNpc(n).SetTrustLevel(0.0).SetGiftCount(0).Save(ctx)
	}
	if err != nil {
		return nil, err
	}

	// Save the full object to cache
	jsonData, _ := json.Marshal(rel)
	rdb.Set(ctx, cacheKey, jsonData, time.Minute*10).Err()
	return rel, nil
}
