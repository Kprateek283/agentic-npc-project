package quest_logic

import (
	"agentic-npc-backend/internal/db/ent"
	entplayer "agentic-npc-backend/internal/db/ent/player"
	entplayerqueststate "agentic-npc-backend/internal/db/ent/playerqueststate"
	"agentic-npc-backend/internal/domain/memory"
	"agentic-npc-backend/internal/domain/npcstate"
	"agentic-npc-backend/internal/dto"
	"context"
	"fmt"
	"log"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// --- CORE QUEST LOGIC FUNCTIONS ---

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

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

// keywordMatches checks whether the event's text matches the quest step trigger keyword.
// An empty keyword matches any text. For question events, the keyword must appear as a
// whole word or phrase (case-insensitive). Other event types require an exact case-insensitive match.
func keywordMatches(eventType, keyword, text string) bool {
	if keyword == "" {
		return true
	}
	if eventType == "PLAYER_ASKED_QUESTION" {
		pattern := `(?i)\b` + regexp.QuoteMeta(keyword) + `\b`
		matched, err := regexp.MatchString(pattern, text)
		if err != nil {
			return false
		}
		return matched
	}
	return strings.EqualFold(keyword, text)
}

// checkQuestCompletion is the main quest logic loop
func (qm *QuestManager) checkQuestCompletion(ctx context.Context, db *ent.Client, p *ent.Player, n *ent.NPC, event dto.EventMessage) (failResponse *FailResponseAction, err error) {
	return qm.checkQuestCompletionAt(ctx, db, p, n, event, time.Now())
}

// checkQuestCompletionAt is checkQuestCompletion evaluated at the given instant.
func (qm *QuestManager) checkQuestCompletionAt(ctx context.Context, db *ent.Client, p *ent.Player, n *ent.NPC, event dto.EventMessage, now time.Time) (failResponse *FailResponseAction, err error) {
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
			if keywordMatches(event.EventType, trigger.Keyword, textToMatch) {
				eventMatchesTrigger = true // All conditions met!
			}
		}

		if eventMatchesTrigger {
			slog.Debug("event matches trigger", "quest_id", questState.QuestIdentifier, "step", currentStepStr)

			// Event matches! Now check preconditions
			preconditionsMet, err := qm.checkPreconditionsAt(ctx, db, p, n, stepDef.Preconditions, now)
			if err != nil {
				return nil, err
			}
			if !preconditionsMet {
				// Preconditions failed, return the specific fail message
				return &stepDef.FailResponse, nil
			}

			// Preconditions met! Apply rewards
			if err := qm.applyRewards(ctx, db, p, n, stepDef.Rewards, now); err != nil {
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

// checkPreconditionsAt validates all rules for a quest step at the given instant
func (qm *QuestManager) checkPreconditionsAt(ctx context.Context, db *ent.Client, p *ent.Player, n *ent.NPC, preconditions []Precondition, now time.Time) (bool, error) {
	slog.Debug("checking preconditions", "player_id", p.PlayerID, "npc_name", n.Name)
	for _, precond := range preconditions {
		slog.Debug("checking precondition type", "type", precond.Type)
		switch precond.Type {
		case "RELATIONSHIP_TRUST":
			cfg := memory.DefaultConfig()
			if qm.Rules != nil {
				cfg = qm.Rules.Config
			}
			eps, _, err := npcstate.Load(ctx, db, n)
			if err != nil {
				slog.Debug("error loading npc memories", "error", err)
				return false, err
			}
			currentTrust := npcstate.TrustToward(eps, p.PlayerID, now, cfg)
			slog.Debug("got relationship trust", "trust_level", currentTrust)
			slog.Debug("comparing trust", "current", currentTrust, "operator", precond.Operator, "target", precond.Value)

			trustMet := trustMet(currentTrust, precond.Operator, precond.Value)
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
func (qm *QuestManager) applyRewards(ctx context.Context, db *ent.Client, p *ent.Player, n *ent.NPC, rewards Rewards, now time.Time) error {
	// 1. Apply Relationship Change
	if rewards.RelationshipChange.TargetNPCName != "" {
		targetNpcName := rewards.RelationshipChange.TargetNPCName
		if targetNpcName == "SELF" {
			targetNpcName = n.Name // "SELF" means the NPC they're talking to
		}

		rewardNpc, err := qm.GetNpc(ctx, db, targetNpcName)
		if err != nil {
			return err
		}

		_, err = qm.GetOrCreateRelationship(ctx, db, p, rewardNpc)
		if err != nil {
			return err
		}

		trustChange, err := strconv.ParseFloat(rewards.RelationshipChange.Trust, 64)
		if err != nil {
			return err
		}

		v := clamp(trustChange, -1.0, 1.0)
		_, err = db.Memory.Create().
			SetOwner(rewardNpc).
			SetActor(p.PlayerID).
			SetEventType("QUEST_REWARD").
			SetDelta(map[string]float64{"trust": v}).
			SetIntensity(0.3).
			SetFirstAt(now).
			SetLastAt(now).
			SetDescription(fmt.Sprintf("Quest reward: trust changed by %.2f", trustChange)).
			SetParticipants([]string{p.PlayerID, rewardNpc.ID.String()}).
			Save(ctx)
		if err != nil {
			return err
		}
		log.Printf("Applied Trust Reward: Player %s trust with NPC %s added QUEST_REWARD episode delta=%.2f", p.PlayerID, rewardNpc.Name, v)
	}

	return nil
}
