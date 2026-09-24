package npcstate

import (
	"context"
	"time"

	"agentic-npc-backend/internal/db/ent"
	entmemory "agentic-npc-backend/internal/db/ent/memory"
	"agentic-npc-backend/internal/domain/memory"
)

// Load loads all memory rows for the given NPC (newest first) and converts each to a memory.Episode.
// Returns the episodes and the matching database rows in the same order.
func Load(ctx context.Context, db *ent.Client, npc *ent.NPC) ([]memory.Episode, []*ent.Memory, error) {
	rows, err := npc.QueryMemories().
		Order(ent.Desc(entmemory.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, nil, err
	}

	episodes := make([]memory.Episode, 0, len(rows))
	for _, m := range rows {
		actor := m.Actor
		if actor == "" && len(m.Participants) > 0 {
			actor = m.Participants[0]
		}
		lastAt := m.LastAt
		if lastAt.IsZero() {
			lastAt = m.CreatedAt
		}
		firstAt := m.FirstAt
		if firstAt.IsZero() {
			firstAt = m.CreatedAt
		}
		episodes = append(episodes, memory.Episode{
			Actor:     actor,
			EventType: m.EventType,
			Subject:   m.Subject,
			Delta:     m.Delta,
			Intensity: m.Intensity,
			Count:     m.Count,
			Harmful:   m.Harmful,
			Forgiven:  m.Forgiven,
			Betrayal:  m.Betrayal,
			FirstAt:   firstAt,
			LastAt:    lastAt,
			Text:      m.Text,
		})
	}

	return episodes, rows, nil
}

// TrustToward computes the net trust emotion toward a specific actor.
func TrustToward(eps []memory.Episode, actor string, now time.Time, cfg memory.Config) float64 {
	return memory.EmotionsToward(actor, eps, now, cfg)["trust"]
}
