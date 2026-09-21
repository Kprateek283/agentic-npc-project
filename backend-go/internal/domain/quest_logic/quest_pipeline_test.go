package quest_logic

import (
	"context"
	"strings"
	"testing"

	"agentic-npc-backend/internal/db/ent"
	"agentic-npc-backend/internal/db/ent/enttest"
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

func seedRelationship(t *testing.T, ctx context.Context, db *ent.Client, p *ent.Player, n *ent.NPC, trust float64) *ent.PlayerNPCRelationship {
	t.Helper()
	rel, err := db.PlayerNPCRelationship.Create().
		SetPlayer(p).
		SetNpc(n).
		SetTrustLevel(trust).
		SetGiftCount(0).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to seed player NPC relationship: %v", err)
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
