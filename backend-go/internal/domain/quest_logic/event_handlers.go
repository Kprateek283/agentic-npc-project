package quest_logic

import (
	"agentic-npc-backend/internal/db/ent"
	entnpc "agentic-npc-backend/internal/db/ent/npc"
	entplayer "agentic-npc-backend/internal/db/ent/player"
	entplayerqueststate "agentic-npc-backend/internal/db/ent/playerqueststate"
	entquest "agentic-npc-backend/internal/db/ent/quest"
	"agentic-npc-backend/internal/domain/memory"
	"agentic-npc-backend/internal/domain/npcstate"
	"agentic-npc-backend/internal/dto"
	"context"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"
)

// --- SPECIFIC EVENT HANDLERS ---

// handleGifting applies the episode-based memory and diminishing returns logic for gifts
func (qm *QuestManager) handleGifting(ctx context.Context, db *ent.Client, p *ent.Player, n *ent.NPC, itemID string) error {
	return qm.handleGiftingAt(ctx, db, p, n, itemID, time.Now())
}

// handleGiftingAt is handleGifting recorded at the given instant.
func (qm *QuestManager) handleGiftingAt(ctx context.Context, db *ent.Client, p *ent.Player, n *ent.NPC, itemID string, now time.Time) error {
	itemDef, err := qm.getItemDefinition(itemID)
	if err != nil {
		return err
	}

	_, err = qm.GetOrCreateRelationship(ctx, db, p, n)
	if err != nil {
		return err
	}

	v := clamp(itemDef.BaseTrustValue, -1.0, 1.0)
	intensity := math.Abs(v)

	cfg := memory.DefaultConfig()
	if qm.Rules != nil {
		cfg = qm.Rules.Config
	}

	episodes, rows, err := npcstate.Load(ctx, db, n)
	if err != nil {
		return err
	}

	actorHasForgiven := false
	for _, ep := range episodes {
		if ep.Actor == p.PlayerID && ep.Forgiven > 0 {
			actorHasForgiven = true
			break
		}
	}

	mergeIndex := -1
	for i, ep := range episodes {
		if ep.Actor == p.PlayerID && ep.EventType == "PLAYER_GAVE_GIFT" && ep.Subject == itemID {
			if memory.CanMerge(ep, now, cfg, false, actorHasForgiven) {
				mergeIndex = i
				break
			}
		}
	}

	if mergeIndex >= 0 {
		ep := episodes[mergeIndex]
		memory.Merge(&ep, now, cfg)
		row := rows[mergeIndex]
		err = db.Memory.UpdateOne(row).
			SetCount(ep.Count).
			SetLastAt(ep.LastAt).
			Exec(ctx)
		if err != nil {
			return err
		}
		log.Printf("Gifting merged: NPC %s updated gift %s count to %f", n.Name, itemID, ep.Count)
		return nil
	}

	memoryDesc := fmt.Sprintf("%s gave %s to %s", p.PlayerID, itemID, n.Name)
	_, err = db.Memory.Create().
		SetOwner(n).
		SetActor(p.PlayerID).
		SetEventType("PLAYER_GAVE_GIFT").
		SetSubject(itemID).
		SetDelta(map[string]float64{"trust": v}).
		SetIntensity(intensity).
		SetHarmful(false).
		SetCount(1.0).
		SetFirstAt(now).
		SetLastAt(now).
		SetDescription(memoryDesc).
		SetParticipants([]string{p.PlayerID, n.ID.String()}).
		Save(ctx)
	if err != nil {
		return err
	}

	log.Printf("Gifting recorded: NPC %s received gift %s (trust delta: %f)", n.Name, itemID, v)
	return nil
}

// HandleAdminCommand processes debug commands
func (qm *QuestManager) HandleAdminCommand(ctx context.Context, db *ent.Client, event dto.EventMessage) error {
	if !qm.AdminEnabled {
		return fmt.Errorf("admin commands are disabled (set ADMIN_ENABLED=true)")
	}
	log.Printf("Dungeon Master: Received ADMIN COMMAND: %s", event.EventType)
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

		// Find the target NPC
		n, err := db.NPC.Query().Where(entnpc.NameEQ(npcName)).Only(ctx)
		if err != nil {
			return fmt.Errorf("admin command failed: could not find NPC %s", npcName)
		}

		// Get or create the relationship
		_, err = qm.GetOrCreateRelationship(ctx, db, p, n)
		if err != nil {
			return fmt.Errorf("failed to get/create relationship for admin command: %w", err)
		}

		// Delete existing memory rows for this player with this NPC
		existingMems, err := n.QueryMemories().All(ctx)
		if err != nil {
			return fmt.Errorf("admin command failed: could not query NPC memories: %w", err)
		}
		for _, m := range existingMems {
			actor := m.Actor
			if actor == "" && len(m.Participants) > 0 {
				actor = m.Participants[0]
			}
			if actor == p.PlayerID {
				if err := db.Memory.DeleteOne(m).Exec(ctx); err != nil {
					return fmt.Errorf("admin command failed: could not delete memory %d: %w", m.ID, err)
				}
			}
		}

		v := clamp(trustValue, -1.0, 1.0)
		now := time.Now()
		_, err = db.Memory.Create().
			SetOwner(n).
			SetActor(p.PlayerID).
			SetEventType("QUEST_REWARD").
			SetDelta(map[string]float64{"trust": v}).
			SetIntensity(0.3).
			SetFirstAt(now).
			SetLastAt(now).
			SetDescription(fmt.Sprintf("Admin set trust with %s to %.2f", npcName, trustValue)).
			SetParticipants([]string{p.PlayerID, n.ID.String()}).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to set trust level: %w", err)
		}

		log.Printf("ADMIN: Set player %s trust with NPC %s to %.2f", p.PlayerID, npcName, trustValue)

	default:
		return fmt.Errorf("unknown admin command: %s", event.EventType)
	}
	return nil
}
