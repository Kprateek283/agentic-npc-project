package handlers

import (
	"context"
	"math"
	"strings"
	"testing"

	"agentic-npc-backend/internal/db/ent"
	"agentic-npc-backend/internal/db/ent/enttest"
	entmemory "agentic-npc-backend/internal/db/ent/memory"
	"agentic-npc-backend/internal/domain/npcstate"
	"agentic-npc-backend/internal/domain/quest_logic"
	"agentic-npc-backend/internal/domain/rules"
	"agentic-npc-backend/internal/dto"

	_ "github.com/mattn/go-sqlite3"
)

func seedTestNPC(t *testing.T, ctx context.Context, db *ent.Client, name string) *ent.NPC {
	t.Helper()
	npc, err := db.NPC.Create().
		SetName(name).
		SetPersonalityPath("gamedata/npcs/" + strings.ToLower(name) + "/personality.json").
		SetBackstoryPath("gamedata/npcs/" + strings.ToLower(name) + "/backstory.json").
		SetLorePath("gamedata/npcs/" + strings.ToLower(name) + "/lore.json").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to seed NPC %s: %v", name, err)
	}
	return npc
}

func seedTestPlayer(t *testing.T, ctx context.Context, db *ent.Client, playerID string) *ent.Player {
	t.Helper()
	player, err := db.Player.Create().
		SetPlayerID(playerID).
		SetPlayerName("Player " + playerID).
		SetPassword("dummy_password").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to seed player %s: %v", playerID, err)
	}
	return player
}

func approxEqual(a, b, tolerance float64) bool {
	return math.Abs(a-b) <= tolerance
}

func TestDemoSequenceMemoryAndEmotions(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	ctx := context.Background()

	qm, err := quest_logic.NewQuestManager("../../../../gamedata")
	if err != nil {
		t.Fatalf("failed to create QuestManager: %v", err)
	}
	rulesData, err := rules.Load("../../../../gamedata/events.json")
	if err != nil {
		t.Fatalf("failed to load rules: %v", err)
	}
	qm.Rules = rulesData

	handler := &WebSocketHandler{
		dbClient:     client,
		questManager: qm,
	}

	npc := seedTestNPC(t, ctx, client, "Elara")
	p1 := seedTestPlayer(t, ctx, client, "P1")
	p2 := seedTestPlayer(t, ctx, client, "P2")

	tolerance := 0.02

	// 1. P1 throws five stones (five separate PLAYER_THREW_STONE events)
	stoneEventP1 := dto.EventMessage{
		EventType:      "PLAYER_THREW_STONE",
		SourceEntityId: p1.PlayerID,
		TargetNpcName:  npc.Name,
	}

	for i := 0; i < 5; i++ {
		if err := handler.recordEpisode(ctx, stoneEventP1, npc, p1); err != nil {
			t.Fatalf("step 1: recordEpisode stone %d failed: %v", i+1, err)
		}
	}

	epsP1, rowsP1, err := npcstate.Load(ctx, client, npc)
	if err != nil {
		t.Fatalf("step 1: npcstate.Load failed: %v", err)
	}
	if len(epsP1) != 1 || len(rowsP1) != 1 {
		t.Fatalf("step 1: expected exactly 1 merged episode, got %d episodes (%d rows)", len(epsP1), len(rowsP1))
	}
	if !approxEqual(epsP1[0].Count, 5.0, 1e-4) {
		t.Errorf("step 1: expected episode count 5, got %f", epsP1[0].Count)
	}

	argsP1, err := handler.buildAIArgs(ctx, stoneEventP1, npc, p1)
	if err != nil {
		t.Fatalf("step 1: buildAIArgs failed: %v", err)
	}
	if !approxEqual(argsP1.emotions["anger"], 1.0, tolerance) {
		t.Errorf("step 1: expected anger toward P1 ~1.0, got %f", argsP1.emotions["anger"])
	}
	if !approxEqual(argsP1.emotions["trust"], -1.0, tolerance) {
		t.Errorf("step 1: expected trust toward P1 ~-1.0, got %f", argsP1.emotions["trust"])
	}

	// 2. P2 sends any event (e.g. PLAYER_INTERACT)
	interactEventP2 := dto.EventMessage{
		EventType:      "PLAYER_INTERACT",
		SourceEntityId: p2.PlayerID,
		TargetNpcName:  npc.Name,
	}
	if err := handler.recordEpisode(ctx, interactEventP2, npc, p2); err != nil {
		t.Fatalf("step 2: recordEpisode for P2 failed: %v", err)
	}

	argsP2, err := handler.buildAIArgs(ctx, interactEventP2, npc, p2)
	if err != nil {
		t.Fatalf("step 2: buildAIArgs for P2 failed: %v", err)
	}
	if !approxEqual(argsP2.emotions["anger"], 0.0, tolerance) {
		t.Errorf("step 2: expected anger toward P2 ~0.0, got %f", argsP2.emotions["anger"])
	}
	if !approxEqual(argsP2.generalMood["anger"], 0.25, tolerance) {
		t.Errorf("step 2: expected general mood anger ~0.25, got %f", argsP2.generalMood["anger"])
	}

	var hasSomeoneStoneLine bool
	var hasYouStoneLine bool
	for _, line := range argsP2.memoryLines {
		if strings.Contains(line, "PLAYER_THREW_STONE") {
			if strings.HasPrefix(line, "Someone") {
				hasSomeoneStoneLine = true
			}
			if strings.HasPrefix(line, "You") {
				hasYouStoneLine = true
			}
		}
	}
	if !hasSomeoneStoneLine {
		t.Errorf("step 2: expected P2 memory lines to include a 'Someone' line about stones, got: %v", argsP2.memoryLines)
	}
	if hasYouStoneLine {
		t.Errorf("step 2: expected P2 memory lines to NOT include a 'You' line about stones, got: %v", argsP2.memoryLines)
	}

	// 3. P1 apologises (PLAYER_APOLOGIZED)
	apologyEventP1 := dto.EventMessage{
		EventType:      "PLAYER_APOLOGIZED",
		SourceEntityId: p1.PlayerID,
		TargetNpcName:  npc.Name,
	}
	if err := handler.recordEpisode(ctx, apologyEventP1, npc, p1); err != nil {
		t.Fatalf("step 3: recordEpisode apology failed: %v", err)
	}

	stoneRow, err := client.Memory.Query().
		Where(
			entmemory.EventTypeEQ("PLAYER_THREW_STONE"),
			entmemory.ActorEQ(p1.PlayerID),
			entmemory.BetrayalEQ(false),
		).
		Only(ctx)
	if err != nil {
		t.Fatalf("step 3: failed to find stone memory row: %v", err)
	}
	if !approxEqual(stoneRow.Forgiven, 0.53, tolerance) {
		t.Errorf("step 3: expected stone episode Forgiven ~0.53, got %f", stoneRow.Forgiven)
	}

	apologyRow, err := client.Memory.Query().
		Where(
			entmemory.EventTypeEQ("PLAYER_APOLOGIZED"),
			entmemory.ActorEQ(p1.PlayerID),
		).
		Only(ctx)
	if err != nil {
		t.Fatalf("step 3: failed to find apology memory row: %v", err)
	}
	if len(apologyRow.Covers) == 0 {
		t.Errorf("step 3: expected apology covers to be non-empty, got: %v", apologyRow.Covers)
	}

	argsP1Apology, err := handler.buildAIArgs(ctx, apologyEventP1, npc, p1)
	if err != nil {
		t.Fatalf("step 3: buildAIArgs failed: %v", err)
	}
	if !approxEqual(argsP1Apology.emotions["anger"], 0.47, tolerance) {
		t.Errorf("step 3: expected anger toward P1 ~0.47 after apology, got %f", argsP1Apology.emotions["anger"])
	}
	if !approxEqual(argsP1Apology.emotions["trust"], -0.74, tolerance) {
		t.Errorf("step 3: expected trust toward P1 ~-0.74 after apology, got %f", argsP1Apology.emotions["trust"])
	}

	// 4. P1 throws a sixth stone
	if err := handler.recordEpisode(ctx, stoneEventP1, npc, p1); err != nil {
		t.Fatalf("step 4: recordEpisode 6th stone failed: %v", err)
	}

	betrayalRow, err := client.Memory.Query().
		Where(
			entmemory.EventTypeEQ("PLAYER_THREW_STONE"),
			entmemory.ActorEQ(p1.PlayerID),
			entmemory.BetrayalEQ(true),
		).
		Only(ctx)
	if err != nil {
		t.Fatalf("step 4: expected a NEW episode flagged betrayal, got error: %v", err)
	}
	if betrayalRow == nil {
		t.Fatalf("step 4: expected betrayal episode to exist")
	}

	oldStoneRow, err := client.Memory.Query().
		Where(
			entmemory.EventTypeEQ("PLAYER_THREW_STONE"),
			entmemory.ActorEQ(p1.PlayerID),
			entmemory.BetrayalEQ(false),
		).
		Only(ctx)
	if err != nil {
		t.Fatalf("step 4: failed to find old stone row: %v", err)
	}
	if oldStoneRow.Forgiven != 0 {
		t.Errorf("step 4: expected old stone row Forgiven to be reset to 0, got %f", oldStoneRow.Forgiven)
	}

	argsP1Sixth, err := handler.buildAIArgs(ctx, stoneEventP1, npc, p1)
	if err != nil {
		t.Fatalf("step 4: buildAIArgs after 6th stone failed: %v", err)
	}
	if !approxEqual(argsP1Sixth.emotions["anger"], 1.0, tolerance) {
		t.Errorf("step 4: expected anger toward P1 ~1.0 again, got %f", argsP1Sixth.emotions["anger"])
	}

	var hasBetrayalWording bool
	for _, line := range argsP1Sixth.memoryLines {
		if strings.Contains(line, "after apologising") {
			hasBetrayalWording = true
			if !strings.HasPrefix(line, "You") {
				t.Errorf("step 4: expected betrayal line to start with 'You' for P1, got: %s", line)
			}
		}
	}
	if !hasBetrayalWording {
		t.Errorf("step 4: expected P1 memory lines to contain 'after apologising' wording, got: %v", argsP1Sixth.memoryLines)
	}
}

func TestEmotionsFrameContent(t *testing.T) {
	tests := []struct {
		name    string
		npc     string
		toward  map[string]float64
		general map[string]float64
		want    string
	}{
		{
			name: "sample emotions matching doc specification",
			npc:  "Elara",
			toward: map[string]float64{
				"anger": 1.0,
				"trust": -1.0,
			},
			general: map[string]float64{
				"anger": 0.25,
			},
			want: `{"npc":"Elara","toward_you":{"anger":1,"trust":-1},"general":{"anger":0.25}}`,
		},
		{
			name:    "empty and nil maps encode as empty json object",
			npc:     "Elara",
			toward:  map[string]float64{},
			general: nil,
			want:    `{"npc":"Elara","toward_you":{},"general":{}}`,
		},
		{
			name: "two decimal rounding and key sorting",
			npc:  "Baelor",
			toward: map[string]float64{
				"trust": 0.333333,
				"anger": 0.666666,
			},
			general: map[string]float64{
				"joy":     0.125,
				"sadness": -0.0001,
			},
			want: `{"npc":"Baelor","toward_you":{"anger":0.67,"trust":0.33},"general":{"joy":0.13,"sadness":0}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := emotionsFrameContent(tt.npc, tt.toward, tt.general)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("emotionsFrameContent() =\n  got:  %s\n  want: %s", got, tt.want)
			}
		})
	}
}
