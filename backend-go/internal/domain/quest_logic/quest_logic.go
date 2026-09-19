package quest_logic

import (
	"agentic-npc-backend/internal/db/ent"
	entplayer "agentic-npc-backend/internal/db/ent/player"
	entplayerqueststate "agentic-npc-backend/internal/db/ent/playerqueststate"
	"agentic-npc-backend/internal/dto"
	"context"
	"fmt"
	"log"
	"log/slog"
	"strconv"
	"strings"

	"github.com/go-redis/redis/v8"
)

// --- CORE QUEST LOGIC FUNCTIONS ---

// trustMet is the pure trust-precondition decision: does a trust level satisfy the
// operator/threshold a quest step requires. An unknown or missing operator fails closed.
func trustMet(level float64, operator string, value float64) bool {
	switch operator {
	case ">=":
		return level >= value
	case "GREATER_THAN":
		return level > value
	default:
		return false
	}
}

// checkQuestCompletion is the main quest logic loop
func (qm *QuestManager) checkQuestCompletion(ctx context.Context, db *ent.Client, rdb *redis.Client, p *ent.Player, n *ent.NPC, event dto.EventMessage) (failResponse *FailResponseAction, err error) {
	// Find all active quests for this player
	activeQuests, err := db.PlayerQuestState.Query().Where(
		entplayerqueststate.HasPlayerWith(entplayer.IDEQ(p.ID)),
		entplayerqueststate.IsCompletedEQ(false),
	).All(ctx)
	if err != nil {
		return nil, err
	}
	slog.Debug("found active quests", "count", len(activeQuests), "player_id", p.PlayerID)
	if len(activeQuests) == 0 {
		return nil, nil // No active quests, nothing to do
	}

	// Loop over all active quests to see if this event triggers any
	for _, questState := range activeQuests {
		questDef, ok := qm.Quests[questState.QuestIdentifier]
		if !ok {
			continue // Should not happen if data is clean
		}

		currentStepStr := strconv.Itoa(questState.CurrentStep)
		stepDef, ok := questDef.Steps[currentStepStr]
		if !ok {
			continue // Player is on a step that doesn't exist
		}

		// Check if the event matches the trigger for this step
		trigger := stepDef.CompletionTrigger
		eventMatchesTrigger := false // Start assuming it doesn't match

		// Check base conditions (EventType and TargetNPC)
		if trigger.EventType == event.EventType && trigger.TargetNPCName == n.Name {
			// Now check the keyword based on the event type
			var textToMatch string
			if event.EventType == "PLAYER_ASKED_QUESTION" {
				textToMatch = event.QuestionText // Use QuestionText for questions
			} else {
				textToMatch = event.Keyword // Use Keyword for gifts, item submissions, etc.
			}

			// Perform the keyword check (case-insensitive)
			if trigger.Keyword == "" || strings.EqualFold(trigger.Keyword, textToMatch) {
				eventMatchesTrigger = true // All conditions met!
			}
		}

		if eventMatchesTrigger {
			slog.Debug("event matches trigger", "quest_id", questState.QuestIdentifier, "step", currentStepStr)

			// Event matches! Now check preconditions
			preconditionsMet, err := qm.checkPreconditions(ctx, db, rdb, p, n, stepDef.Preconditions)
			if err != nil {
				return nil, err
			}
			if !preconditionsMet {
				// Preconditions failed, return the specific fail message
				return &stepDef.FailResponse, nil
			}

			// Preconditions met! Apply rewards
			if err := qm.applyRewards(ctx, db, rdb, p, n, stepDef.Rewards); err != nil {
				log.Printf("Warning: failed to apply rewards: %v", err)
				// Don't block quest completion on reward failure
			} else {
				log.Printf("Successfully applied rewards for Quest '%s', Step %s.", questState.QuestIdentifier, currentStepStr)
			}

			// Update quest state
			if stepDef.Rewards.UnlocksStep != "" {
				// Advance to the next step
				nextStep, err := strconv.Atoi(stepDef.Rewards.UnlocksStep)
				if err != nil {
					return nil, err
				}
				err = db.PlayerQuestState.UpdateOne(questState).SetCurrentStep(nextStep).Exec(ctx)
				if err == nil {
					log.Printf("Updated Quest '%s' to Step %d.", questState.QuestIdentifier, nextStep)
				}
			} else {
				// This was the last step, complete the quest
				err = db.PlayerQuestState.UpdateOne(questState).SetIsCompleted(true).Exec(ctx)
				if err == nil {
					log.Printf("Completed Quest '%s'.", questState.QuestIdentifier)
				}
			}
			if err != nil {
				return nil, err // Return error if DB update failed
			}
			return nil, nil // Quest step completed successfully
		}
	}
	return nil, nil // Event didn't trigger any active quest steps
}

// checkPreconditions validates all rules for a quest step
func (qm *QuestManager) checkPreconditions(ctx context.Context, db *ent.Client, rdb *redis.Client, p *ent.Player, n *ent.NPC, preconditions []Precondition) (bool, error) {
	slog.Debug("checking preconditions", "player_id", p.PlayerID, "npc_name", n.Name)
	for _, precond := range preconditions {
		slog.Debug("checking precondition type", "type", precond.Type)
		switch precond.Type {
		case "RELATIONSHIP_TRUST":
			rel, err := qm.GetOrCreateRelationship(ctx, db, rdb, p, n)
			if err != nil {
				slog.Debug("error getting relationship", "error", err)
				return false, err
			}
			slog.Debug("got relationship", "trust_level", rel.TrustLevel)
			slog.Debug("comparing trust", "current", rel.TrustLevel, "operator", precond.Operator, "target", precond.Value)

			trustMet := trustMet(rel.TrustLevel, precond.Operator, precond.Value)
			slog.Debug("trust condition evaluated", "met", trustMet)
			if !trustMet {
				slog.Debug("precondition failed")
				return false, nil // Failed this precondition
			}
			slog.Debug("precondition passed")

		case "QUEST_COMPLETED":
			count, err := db.PlayerQuestState.
				Query().
				Where(
					entplayerqueststate.HasPlayerWith(entplayer.IDEQ(p.ID)),
					entplayerqueststate.QuestIdentifierEQ(precond.QuestID),
					entplayerqueststate.IsCompletedEQ(true),
				).
				Count(ctx)
			if err != nil {
				return false, err
			}
			if count == 0 {
				slog.Debug("precondition failed: quest not completed", "quest_id", precond.QuestID)
				return false, nil // Required quest is not complete
			}
			slog.Debug("precondition passed: quest completed", "quest_id", precond.QuestID)
		}
	}
	slog.Debug("all preconditions passed")
	return true, nil // All preconditions passed
}

// applyRewards gives the player items, XP, or relationship changes
func (qm *QuestManager) applyRewards(ctx context.Context, db *ent.Client, rdb *redis.Client, p *ent.Player, n *ent.NPC, rewards Rewards) error {
	// 1. Apply Relationship Change
	if rewards.RelationshipChange.TargetNPCName != "" {
		targetNpcName := rewards.RelationshipChange.TargetNPCName
		if targetNpcName == "SELF" {
			targetNpcName = n.Name // "SELF" means the NPC they're talking to
		}

		rewardNpc, err := qm.GetNpc(ctx, db, rdb, targetNpcName)
		if err != nil {
			return err
		}

		rel, err := qm.GetOrCreateRelationship(ctx, db, rdb, p, rewardNpc)
		if err != nil {
			return err
		}

		trustChange, err := strconv.ParseFloat(rewards.RelationshipChange.Trust, 64)
		if err != nil {
			return err
		}

		newTrustLevel := rel.TrustLevel + trustChange
		err = db.PlayerNPCRelationship.UpdateOne(rel).SetTrustLevel(newTrustLevel).Exec(ctx)
		if err != nil {
			return err
		}
		log.Printf("Applied Trust Reward: Player %s trust with NPC %s is now %.2f", p.PlayerID, rewardNpc.Name, newTrustLevel)

		// Invalidate this relationship's cache
		cacheKey := fmt.Sprintf("relationship:%s:%s", p.ID.String(), rewardNpc.ID.String())
		rdb.Del(ctx, cacheKey)
	}

	return nil
}
