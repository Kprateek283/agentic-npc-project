package quest_logic

import (
	"agentic-npc-backend/internal/db/ent"
	entnpc "agentic-npc-backend/internal/db/ent/npc"
	entplayer "agentic-npc-backend/internal/db/ent/player"
	entplayerqueststate "agentic-npc-backend/internal/db/ent/playerqueststate"
	entquest "agentic-npc-backend/internal/db/ent/quest"
	"agentic-npc-backend/internal/dto"
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/go-redis/redis/v8"
)

// --- SPECIFIC EVENT HANDLERS ---

// handleGifting applies the diminishing returns logic for gifts
func (qm *QuestManager) handleGifting(ctx context.Context, db *ent.Client, rdb *redis.Client, p *ent.Player, n *ent.NPC, itemID string) error {
	itemDef, err := qm.getItemDefinition(itemID)
	if err != nil {
		return err
	}

	rel, err := qm.GetOrCreateRelationship(ctx, db, rdb, p, n)
	if err != nil {
		return err
	}

	// Apply diminishing returns logic
	var trustGained float64
	giftCount := rel.GiftCount
	if giftCount == 0 {
		trustGained = itemDef.BaseTrustValue * 1.0 // 100% value
	} else if giftCount == 1 {
		trustGained = itemDef.BaseTrustValue / 5.0 // 20% value
	} else {
		trustGained = 0.0 // 0% value
	}

	newTrustLevel := rel.TrustLevel + trustGained
	newGiftCount := giftCount + 1

	// Update the relationship in the DB
	err = db.PlayerNPCRelationship.UpdateOne(rel).SetTrustLevel(newTrustLevel).SetGiftCount(newGiftCount).Exec(ctx)
	if err != nil {
		return err
	}

	// Invalidate this relationship's cache
	cacheKey := fmt.Sprintf("relationship:%s:%s", p.ID.String(), n.ID.String())
	rdb.Del(ctx, cacheKey)

	log.Printf("Gifting successful: NPC %s trust is now %f (Gift #%d)", n.Name, newTrustLevel, newGiftCount)
	return nil
}

// HandleAdminCommand processes debug commands
func (qm *QuestManager) HandleAdminCommand(ctx context.Context, db *ent.Client, rdb *redis.Client, event dto.EventMessage) error { // <-- ADDED rdb argument
	log.Printf("Dungeon Master: Received ADMIN COMMAND: %s", event.EventType)
	// Admin commands don't use cache to get the player, ensuring fresh data
	p, err := db.Player.Query().Where(entplayer.PlayerIDEQ(event.SourceEntityId)).Only(ctx)
	if err != nil {
		return fmt.Errorf("admin command failed: could not find player %s", event.SourceEntityId)
	}

	switch event.EventType {
	case "ADMIN_SET_QUEST_STAGE":
		parts := strings.Split(event.Keyword, ",")
		if len(parts) != 2 {
			return fmt.Errorf("invalid admin keyword: %s. Expected 'quest_id,step'", event.Keyword)
		}
		questID := parts[0]
		step, err := strconv.Atoi(parts[1])
		if err != nil {
			return fmt.Errorf("invalid admin step number: %w", err)
		}

		// Check if the quest definition exists in our loaded gamedata
		if _, ok := qm.Quests[questID]; !ok {
			return fmt.Errorf("quest definition not found for %s", questID)
		}

		// Find the actual Quest entity in the database
		questEntity, err := db.Quest.Query().Where(entquest.IDEQ(questID)).Only(ctx)
		if err != nil {
			return fmt.Errorf("failed to find quest entity %s in database: %w", questID, err)
		}

		// Find the player's quest state for this quest
		questState, err := db.PlayerQuestState.Query().Where(
			entplayerqueststate.HasPlayerWith(entplayer.IDEQ(p.ID)),
			entplayerqueststate.QuestIdentifierEQ(questID),
		).Only(ctx)

		if ent.IsNotFound(err) {
			// Player doesn't have this quest, create it
			_, err = db.PlayerQuestState.Create().
				SetPlayer(p).
				SetQuest(questEntity).
				SetQuestIdentifier(questID).
				SetCurrentStep(step).
				SetIsCompleted(false).
				Save(ctx)
		} else if err == nil {
			// Player already has this quest, update it
			_, err = db.PlayerQuestState.UpdateOne(questState).
				SetCurrentStep(step).
				SetIsCompleted(false).
				Save(ctx)
		}
		if err != nil {
			return fmt.Errorf("failed to set quest state: %w", err)
		}
		log.Printf("ADMIN: Set player %s quest %s to step %d", p.PlayerID, questID, step)

	case "ADMIN_SET_TRUST":
		parts := strings.Split(event.Keyword, ",")
		if len(parts) != 2 {
			return fmt.Errorf("invalid admin keyword: %s. Expected 'NpcName,trustValue'", event.Keyword)
		}
		npcName := parts[0]
		trustValue, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return fmt.Errorf("invalid admin trust value: %w", err)
		}

		// Find the target NPC (don't use cache for admin commands)
		n, err := db.NPC.Query().Where(entnpc.NameEQ(npcName)).Only(ctx)
		if err != nil {
			return fmt.Errorf("admin command failed: could not find NPC %s", npcName)
		}

		// Get or create the relationship
		// NOTE: Use GetOrCreateRelationship which handles DB+Cache logic
		rel, err := qm.GetOrCreateRelationship(ctx, db, rdb, p, n)
		if err != nil {
			return fmt.Errorf("failed to get/create relationship for admin command: %w", err)
		}

		// Update the trust level
		err = db.PlayerNPCRelationship.UpdateOne(rel).SetTrustLevel(trustValue).Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to set trust level: %w", err)
		}

		// Invalidate cache explicitly (GetOrCreateRelationship already does this on create, but not update)
		cacheKey := fmt.Sprintf("relationship:%s:%s", p.ID.String(), n.ID.String())
		rdb.Del(ctx, cacheKey)

		log.Printf("ADMIN: Set player %s trust with NPC %s to %.2f", p.PlayerID, npcName, trustValue)

	default:
		return fmt.Errorf("unknown admin command: %s", event.EventType)
	}
	return nil
}
