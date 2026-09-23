package npcstate

import (
	"context"
	"testing"
	"time"

	"agentic-npc-backend/internal/db/ent/enttest"
	"agentic-npc-backend/internal/domain/memory"

	_ "github.com/mattn/go-sqlite3"
)

func TestLoad_OldStyleRow(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	ctx := context.Background()
	npc, err := client.NPC.Create().
		SetName("OldNpc").
		SetPersonalityPath("personality.json").
		SetBackstoryPath("backstory.json").
		SetLorePath("lore.json").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create NPC: %v", err)
	}

	// Create old-style row: no actor, no delta, participants set
	createdMem, err := client.Memory.Create().
		SetOwner(npc).
		SetEventType("PLAYER_INTERACT").
		SetParticipants([]string{"player123", npc.ID.String()}).
		SetDescription("Old memory description").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create old-style memory: %v", err)
	}

	eps, rows, err := Load(ctx, client, npc)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(eps) != 1 || len(rows) != 1 {
		t.Fatalf("expected 1 episode and 1 row, got %d eps and %d rows", len(eps), len(rows))
	}
	if rows[0].ID != createdMem.ID {
		t.Errorf("expected row ID %d, got %d", createdMem.ID, rows[0].ID)
	}

	ep := eps[0]
	if ep.Actor != "player123" {
		t.Errorf("expected Actor %q, got %q", "player123", ep.Actor)
	}
	if ep.Delta != nil {
		t.Errorf("expected nil Delta, got %v", ep.Delta)
	}

	// Verify no emotional contribution
	cfg := memory.DefaultConfig()
	trust := TrustToward(eps, "player123", time.Now(), cfg)
	if trust != 0.0 {
		t.Errorf("expected trust 0.0, got %f", trust)
	}
	allEmotions := memory.EmotionsToward("player123", eps, time.Now(), cfg)
	if len(allEmotions) != 0 {
		t.Errorf("expected empty emotions map, got %v", allEmotions)
	}
}
