package quest_logic

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"

	"agentic-npc-backend/internal/db/ent"
	"agentic-npc-backend/internal/db/ent/enttest"
	"agentic-npc-backend/internal/domain/memory"
	"agentic-npc-backend/internal/domain/npcstate"
	"agentic-npc-backend/internal/dto"

	_ "github.com/mattn/go-sqlite3"
)

func seedNPC(t *testing.T, ctx context.Context, db *ent.Client, name string) *ent.NPC {
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

func seedPlayer(t *testing.T, ctx context.Context, db *ent.Client, playerID string) *ent.Player {
	t.Helper()
	player, err := db.Player.Create().
		SetPlayerID(playerID).
		SetPlayerName("Adventurer").
		SetPassword("dummy_password").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to seed player %s: %v", playerID, err)
	}
	return player
}

func seedQuest(t *testing.T, ctx context.Context, db *ent.Client, questID, name, staticDataPath string) *ent.Quest {
	t.Helper()
	quest, err := db.Quest.Create().
		SetID(questID).
		SetName(name).
		SetStaticDataPath(staticDataPath).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to seed quest %s: %v", questID, err)
	}
	return quest
}

func seedPlayerQuestState(t *testing.T, ctx context.Context, db *ent.Client, p *ent.Player, q *ent.Quest, questIdentifier string, step int) *ent.PlayerQuestState {
	t.Helper()
	state, err := db.PlayerQuestState.Create().
		SetPlayer(p).
		SetQuest(q).
		SetQuestIdentifier(questIdentifier).
		SetCurrentStep(step).
		SetIsCompleted(false).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to seed player quest state: %v", err)
	}
	return state
}

func seedTrustMemory(t *testing.T, ctx context.Context, db *ent.Client, p *ent.Player, n *ent.NPC, trust float64) *ent.Memory {
	t.Helper()
	now := time.Now()
	mem, err := db.Memory.Create().
		SetOwner(n).
		SetActor(p.PlayerID).
		SetEventType("QUEST_REWARD").
		SetDelta(map[string]float64{"trust": trust}).
		SetIntensity(0.3).
		SetFirstAt(now).
		SetLastAt(now).
		SetDescription("Seeded trust reward").
		SetParticipants([]string{p.PlayerID, n.ID.String()}).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to seed trust memory: %v", err)
	}
	return mem
}

func seedRelationship(t *testing.T, ctx context.Context, db *ent.Client, p *ent.Player, n *ent.NPC, trust float64) *ent.PlayerNPCRelationship {
	t.Helper()
	rel, err := db.PlayerNPCRelationship.Create().
		SetPlayer(p).
		SetNpc(n).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to seed player NPC relationship: %v", err)
	}
	if trust != 0 {
		seedTrustMemory(t, ctx, db, p, n, trust)
	}
	return rel
}

func TestQuestStepCompletesQuest(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	ctx := context.Background()
	qm, err := NewQuestManager("../../../../gamedata")
	if err != nil {
		t.Fatalf("NewQuestManager failed: %v", err)
	}

	npc := seedNPC(t, ctx, client, "Elara")
	player := seedPlayer(t, ctx, client, "player1")
	quest := seedQuest(t, ctx, client, "sq_2_elara_research", "The Herbalist's Knowledge", "quests/definitions/sq_2_elara_research.json")
	state := seedPlayerQuestState(t, ctx, client, player, quest, "sq_2_elara_research", 1)
	seedRelationship(t, ctx, client, player, npc, 0.6)

	event := dto.EventMessage{
		EventType:      "PLAYER_ASKED_QUESTION",
		SourceEntityId: "player1",
		TargetNpcName:  "Elara",
		QuestionText:   "Do you know of a cure?",
	}

	failResp, err := qm.checkQuestCompletion(ctx, client, player, npc, event)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if failResp != nil {
		t.Fatalf("expected nil fail response, got: %+v", failResp)
	}

	updatedState, err := client.PlayerQuestState.Get(ctx, state.ID)
	if err != nil {
		t.Fatalf("failed to get updated quest state: %v", err)
	}
	if !updatedState.IsCompleted {
		t.Errorf("expected is_completed to be true, got false")
	}
}

func TestQuestPreconditionFails(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	ctx := context.Background()
	qm, err := NewQuestManager("../../../../gamedata")
	if err != nil {
		t.Fatalf("NewQuestManager failed: %v", err)
	}

	npc := seedNPC(t, ctx, client, "Elara")
	player := seedPlayer(t, ctx, client, "player1")
	quest := seedQuest(t, ctx, client, "sq_2_elara_research", "The Herbalist's Knowledge", "quests/definitions/sq_2_elara_research.json")
	state := seedPlayerQuestState(t, ctx, client, player, quest, "sq_2_elara_research", 1)
	seedRelationship(t, ctx, client, player, npc, 0.2)

	event := dto.EventMessage{
		EventType:      "PLAYER_ASKED_QUESTION",
		SourceEntityId: "player1",
		TargetNpcName:  "Elara",
		QuestionText:   "Do you know of a cure?",
	}

	failResp, err := qm.checkQuestCompletion(ctx, client, player, npc, event)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if failResp == nil {
		t.Fatalf("expected non-nil fail response, got nil")
	}

	expectedFail := qm.Quests["sq_2_elara_research"].Steps["1"].FailResponse
	if failResp.ActionType != expectedFail.ActionType {
		t.Errorf("failResponse.ActionType = %q, want %q", failResp.ActionType, expectedFail.ActionType)
	}
	if failResp.Content != expectedFail.Content {
		t.Errorf("failResponse.Content = %q, want %q", failResp.Content, expectedFail.Content)
	}

	updatedState, err := client.PlayerQuestState.Get(ctx, state.ID)
	if err != nil {
		t.Fatalf("failed to get updated quest state: %v", err)
	}
	if updatedState.IsCompleted {
		t.Errorf("expected is_completed to be false, got true")
	}
	if updatedState.CurrentStep != 1 {
		t.Errorf("expected current_step to be 1, got %d", updatedState.CurrentStep)
	}
}

func TestQuestQuestionMustMatchWholeWord(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	ctx := context.Background()
	qm, err := NewQuestManager("../../../../gamedata")
	if err != nil {
		t.Fatalf("NewQuestManager failed: %v", err)
	}

	npc := seedNPC(t, ctx, client, "Elara")
	player := seedPlayer(t, ctx, client, "player1")
	quest := seedQuest(t, ctx, client, "sq_2_elara_research", "The Herbalist's Knowledge", "quests/definitions/sq_2_elara_research.json")
	state := seedPlayerQuestState(t, ctx, client, player, quest, "sq_2_elara_research", 1)
	seedRelationship(t, ctx, client, player, npc, 0.6)

	event := dto.EventMessage{
		EventType:      "PLAYER_ASKED_QUESTION",
		SourceEntityId: "player1",
		TargetNpcName:  "Elara",
		QuestionText:   "Is the gate secure?",
	}

	failResp, err := qm.checkQuestCompletion(ctx, client, player, npc, event)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if failResp != nil {
		t.Fatalf("expected nil fail response, got: %+v", failResp)
	}

	updatedState, err := client.PlayerQuestState.Get(ctx, state.ID)
	if err != nil {
		t.Fatalf("failed to get updated quest state: %v", err)
	}
	if updatedState.IsCompleted {
		t.Errorf("expected is_completed to be false, got true")
	}
	if updatedState.CurrentStep != 1 {
		t.Errorf("expected current_step to be 1, got %d", updatedState.CurrentStep)
	}
}

func TestQuestStepAdvances(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	ctx := context.Background()
	qm, err := NewQuestManager("../../../../gamedata")
	if err != nil {
		t.Fatalf("NewQuestManager failed: %v", err)
	}

	npc := seedNPC(t, ctx, client, "Kaelen")
	player := seedPlayer(t, ctx, client, "player1")
	quest := seedQuest(t, ctx, client, "sq_0_prove_your_worth", "A Growing Threat", "quests/definitions/sq_0_prove_your_worth.json")
	state := seedPlayerQuestState(t, ctx, client, player, quest, "sq_0_prove_your_worth", 1)

	event := dto.EventMessage{
		EventType:      "PLAYER_INTERACT",
		SourceEntityId: "player1",
		TargetNpcName:  "Kaelen",
	}

	failResp, err := qm.checkQuestCompletion(ctx, client, player, npc, event)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if failResp != nil {
		t.Fatalf("expected nil fail response, got: %+v", failResp)
	}

	updatedState, err := client.PlayerQuestState.Get(ctx, state.ID)
	if err != nil {
		t.Fatalf("failed to get updated quest state: %v", err)
	}
	if updatedState.CurrentStep != 2 {
		t.Errorf("expected current_step to be 2, got %d", updatedState.CurrentStep)
	}
	if updatedState.IsCompleted {
		t.Errorf("expected is_completed to be false, got true")
	}
}

func TestGiftingCreatesAndMergesEpisodes(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	ctx := context.Background()
	qm, err := NewQuestManager("../../../../gamedata")
	if err != nil {
		t.Fatalf("NewQuestManager failed: %v", err)
	}

	npc := seedNPC(t, ctx, client, "Elara")
	player := seedPlayer(t, ctx, client, "player1")

	// First gift of a +0.5 item (moonshadow_petal)
	giftEvent := dto.EventMessage{
		EventType:      "PLAYER_GAVE_GIFT",
		SourceEntityId: player.PlayerID,
		TargetNpcName:  "Elara",
		Keyword:        "moonshadow_petal",
	}

	failResp, err := qm.ProcessEvent(ctx, client, giftEvent)
	if err != nil {
		t.Fatalf("ProcessEvent first gift failed: %v", err)
	}
	if failResp != nil {
		t.Fatalf("expected nil fail response, got: %+v", failResp)
	}

	cfg := memory.DefaultConfig()
	if qm.Rules != nil {
		cfg = qm.Rules.Config
	}

	eps, rows, err := npcstate.Load(ctx, client, npc)
	if err != nil {
		t.Fatalf("npcstate.Load failed: %v", err)
	}
	if len(eps) != 1 || len(rows) != 1 {
		t.Fatalf("expected 1 episode after first gift, got %d", len(eps))
	}
	if eps[0].Count != 1.0 {
		t.Errorf("expected count 1.0, got %f", eps[0].Count)
	}

	trust1 := npcstate.TrustToward(eps, "player1", time.Now(), cfg)
	if math.Abs(trust1-0.5) > 1e-6 {
		t.Errorf("expected trust 0.5 after first gift, got %f", trust1)
	}

	// Second identical gift should merge
	failResp, err = qm.ProcessEvent(ctx, client, giftEvent)
	if err != nil {
		t.Fatalf("ProcessEvent second gift failed: %v", err)
	}
	if failResp != nil {
		t.Fatalf("expected nil fail response, got: %+v", failResp)
	}

	eps2, rows2, err := npcstate.Load(ctx, client, npc)
	if err != nil {
		t.Fatalf("npcstate.Load failed: %v", err)
	}
	if len(eps2) != 1 || len(rows2) != 1 {
		t.Fatalf("expected 1 merged episode after second gift, got %d", len(eps2))
	}
	if math.Abs(eps2[0].Count-2.0) > 1e-4 {
		t.Errorf("expected count ~2.0, got %f", eps2[0].Count)
	}

	trust2 := npcstate.TrustToward(eps2, "player1", time.Now(), cfg)
	if trust2 >= 1.0 {
		t.Errorf("expected trust < 1.0 due to diminishing returns, got %f", trust2)
	}
	if trust2 <= 0.5 {
		t.Errorf("expected trust > 0.5 after second gift, got %f", trust2)
	}
}

func TestAdminSetTrustOverwritesHistory(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	ctx := context.Background()
	qm, err := NewQuestManager("../../../../gamedata")
	if err != nil {
		t.Fatalf("NewQuestManager failed: %v", err)
	}
	qm.AdminEnabled = true

	npc := seedNPC(t, ctx, client, "Elara")
	player := seedPlayer(t, ctx, client, "player1")

	// 1. Give a player trust via a gift (+0.5)
	giftEvent := dto.EventMessage{
		EventType:      "PLAYER_GAVE_GIFT",
		SourceEntityId: player.PlayerID,
		TargetNpcName:  "Elara",
		Keyword:        "moonshadow_petal",
	}
	if _, err := qm.ProcessEvent(ctx, client, giftEvent); err != nil {
		t.Fatalf("ProcessEvent gift failed: %v", err)
	}

	cfg := memory.DefaultConfig()
	if qm.Rules != nil {
		cfg = qm.Rules.Config
	}

	eps, rows, err := npcstate.Load(ctx, client, npc)
	if err != nil {
		t.Fatalf("npcstate.Load failed: %v", err)
	}
	if len(eps) != 1 || len(rows) != 1 {
		t.Fatalf("expected 1 episode after gift, got %d", len(eps))
	}
	trustBefore := npcstate.TrustToward(eps, player.PlayerID, time.Now(), cfg)
	if math.Abs(trustBefore-0.5) > 1e-6 {
		t.Fatalf("expected trust 0.5 before admin command, got %f", trustBefore)
	}

	// 2. Run ADMIN_SET_TRUST for that player to 0.6
	adminEvent := dto.EventMessage{
		EventType:      "ADMIN_SET_TRUST",
		SourceEntityId: player.PlayerID,
		Keyword:        "Elara,0.6",
	}
	if _, err := qm.ProcessEvent(ctx, client, adminEvent); err != nil {
		t.Fatalf("ProcessEvent ADMIN_SET_TRUST failed: %v", err)
	}

	// 3. Assert computed trust is 0.6 (not 0.5 + 0.6 = 1.0) and only one memory row remains
	epsAfter, rowsAfter, err := npcstate.Load(ctx, client, npc)
	if err != nil {
		t.Fatalf("npcstate.Load failed: %v", err)
	}
	if len(rowsAfter) != 1 || len(epsAfter) != 1 {
		t.Fatalf("expected exactly 1 memory row after ADMIN_SET_TRUST, got %d rows, %d eps", len(rowsAfter), len(epsAfter))
	}
	if epsAfter[0].Actor != player.PlayerID {
		t.Errorf("expected actor %q, got %q", player.PlayerID, epsAfter[0].Actor)
	}
	if epsAfter[0].EventType != "QUEST_REWARD" {
		t.Errorf("expected event_type QUEST_REWARD, got %q", epsAfter[0].EventType)
	}

	trustAfter := npcstate.TrustToward(epsAfter, player.PlayerID, time.Now(), cfg)
	if math.Abs(trustAfter-0.6) > 1e-6 {
		t.Errorf("expected computed trust 0.6 after ADMIN_SET_TRUST, got %f", trustAfter)
	}
}
