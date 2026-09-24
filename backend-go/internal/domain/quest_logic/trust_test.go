package quest_logic

import (
	"context"
	"math"
	"testing"
	"time"

	"agentic-npc-backend/internal/db/ent"
	"agentic-npc-backend/internal/db/ent/enttest"
	"agentic-npc-backend/internal/domain/npcstate"
	"agentic-npc-backend/internal/domain/rules"
	"agentic-npc-backend/internal/dto"

	_ "github.com/mattn/go-sqlite3"
)

// Trust is never stored: it is recomputed from memory rows at the instant it is read. Every
// case here writes rows at the fixed instant t0 and reads trust at fixed offsets from it.
//
// Half-lives the expected values rely on, from gamedata/events.json (min 1h, factor 8760):
//   - a gift of intensity 0.5 (Moonshadow Petal): 8760^0.5 h = 93.595 h = 93h35m42s
//   - a gift of intensity 1 (Mithril Ingot):      8760 h = 365 days
//   - a quest reward or admin set, intensity 0.3: 8760^0.3 h = 15.232 h = 15h13m54s
//
// Floats are compared with a 1% relative tolerance (absolute 1e-9 when the expected value is 0).
const relTol = 0.01

var t0 = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

const (
	moonHalfLife   = 93*time.Hour + 35*time.Minute + 42*time.Second
	rewardHalfLife = 15*time.Hour + 13*time.Minute + 54*time.Second
	year           = 8760 * time.Hour
)

func approx(got, want float64) bool {
	if want == 0 {
		return math.Abs(got) < 1e-9
	}
	return math.Abs(got-want) <= relTol*math.Abs(want)
}

type trustEnv struct {
	ctx     context.Context
	db      *ent.Client
	qm      *QuestManager
	npcs    map[string]*ent.NPC
	players map[string]*ent.Player
}

func newTrustEnv(t *testing.T) *trustEnv {
	t.Helper()
	db := enttest.Open(t, "sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared&_fk=1")
	t.Cleanup(func() { db.Close() })
	qm, err := NewQuestManager("../../../../gamedata")
	if err != nil {
		t.Fatalf("NewQuestManager: %v", err)
	}
	// The app loads the rules file into the manager at startup; do the same so the half-lives
	// come from gamedata/events.json.
	qm.Rules, err = rules.Load("../../../../gamedata/events.json")
	if err != nil {
		t.Fatalf("rules.Load: %v", err)
	}
	ctx := context.Background()
	e := &trustEnv{ctx: ctx, db: db, qm: qm, npcs: map[string]*ent.NPC{}, players: map[string]*ent.Player{}}
	for _, name := range []string{"Elara", "Baelor"} {
		e.npcs[name] = seedNPC(t, ctx, db, name)
	}
	for _, id := range []string{"p1", "p2"} {
		e.players[id] = seedPlayer(t, ctx, db, id)
	}
	return e
}

func (e *trustEnv) gift(t *testing.T, player, npc, item string, at time.Time) {
	t.Helper()
	if err := e.qm.handleGiftingAt(e.ctx, e.db, e.players[player], e.npcs[npc], item, at); err != nil {
		t.Fatalf("gift %s from %s to %s: %v", item, player, npc, err)
	}
}

// trustAt reads the NPC's trust toward the player from the database at the given instant.
func (e *trustEnv) trustAt(t *testing.T, npc, player string, at time.Time) float64 {
	t.Helper()
	eps, _, err := npcstate.Load(e.ctx, e.db, e.npcs[npc])
	if err != nil {
		t.Fatalf("npcstate.Load: %v", err)
	}
	return npcstate.TrustToward(eps, player, at, e.qm.Rules.Config)
}

func (e *trustEnv) memoryCount(t *testing.T, npc string) int {
	t.Helper()
	n, err := e.npcs[npc].QueryMemories().Count(e.ctx)
	if err != nil {
		t.Fatalf("count memories: %v", err)
	}
	return n
}

// seedAttack writes the row recordEpisode makes for one PLAYER_ATTACKED at the given instant.
func (e *trustEnv) seedAttack(t *testing.T, player, npc string, at time.Time) {
	t.Helper()
	_, err := e.db.Memory.Create().
		SetOwner(e.npcs[npc]).
		SetActor(player).
		SetEventType("PLAYER_ATTACKED").
		SetDelta(map[string]float64{"joy": -1.0, "anger": 0.6, "fear": 0.4, "trust": -0.5}).
		SetIntensity(0.9).
		SetHarmful(true).
		SetDescription(player + " attacked " + npc).
		SetParticipants([]string{player, e.npcs[npc].ID.String()}).
		SetCreatedAt(at).
		SetFirstAt(at).
		SetLastAt(at).
		Save(e.ctx)
	if err != nil {
		t.Fatalf("seed attack: %v", err)
	}
}

// seedLegacy writes a row in the pre-Memory-v1 shape: no actor, no delta, the player first in
// participants.
func (e *trustEnv) seedLegacy(t *testing.T, player, npc string, at time.Time) {
	t.Helper()
	_, err := e.db.Memory.Create().
		SetOwner(e.npcs[npc]).
		SetEventType("PLAYER_GAVE_GIFT").
		SetDescription(player + " gave a Mithril Ingot to " + npc).
		SetParticipants([]string{player, e.npcs[npc].ID.String()}).
		SetCreatedAt(at).
		SetFirstAt(at).
		SetLastAt(at).
		Save(e.ctx)
	if err != nil {
		t.Fatalf("seed legacy row: %v", err)
	}
}

func TestTrustFromGifts(t *testing.T) {
	type gift struct {
		player, item string
		at           time.Duration // after t0
	}
	type read struct {
		player string
		at     time.Duration // after t0
		want   float64
	}
	cases := []struct {
		name  string
		gifts []gift
		reads []read
		rows  int
	}{
		{
			name:  "a gift raises trust by the item's value",
			gifts: []gift{{"p1", "moonshadow_petal", 0}},
			reads: []read{{"p1", 0, 0.5}},
			rows:  1,
		},
		{
			name:  "trust from a gift halves with each half-life of its intensity",
			gifts: []gift{{"p1", "moonshadow_petal", 0}},
			reads: []read{{"p1", moonHalfLife, 0.25}, {"p1", 2 * moonHalfLife, 0.125}},
			rows:  1,
		},
		{
			name:  "a full-value gift is remembered for a year",
			gifts: []gift{{"p1", "mithril_ingot", 0}},
			reads: []read{{"p1", 0, 1.0}, {"p1", year, 0.5}},
			rows:  1,
		},
		{
			name:  "an item valued above one counts as exactly full trust",
			gifts: []gift{{"p1", "dragon_heartscale", 0}},
			reads: []read{{"p1", 0, 1.0}, {"p1", year, 0.5}},
			rows:  1,
		},
		{
			name:  "the same gift again a half-life later merges into a faded count of 1.5",
			gifts: []gift{{"p1", "moonshadow_petal", 0}, {"p1", "moonshadow_petal", moonHalfLife}},
			// 0.5 x sqrt(1.5): diminishing returns on the merged count, no decay at that instant.
			reads: []read{{"p1", moonHalfLife, 0.6124}},
			rows:  1,
		},
		{
			name:  "different gifts are separate memories that add up",
			gifts: []gift{{"p1", "moonshadow_petal", 0}, {"p1", "kingsfoil_leaf", 0}},
			reads: []read{{"p1", 0, 0.7}},
			rows:  2,
		},
		{
			name:  "positive trust from gifts is capped at one",
			gifts: []gift{{"p1", "moonshadow_petal", 0}, {"p1", "mithril_ingot", 0}},
			reads: []read{{"p1", 0, 1.0}},
			rows:  2,
		},
		{
			name:  "a trash gift lowers trust",
			gifts: []gift{{"p1", "rotten_fish", 0}, {"p1", "moonshadow_petal", 0}},
			reads: []read{{"p1", 0, 0.4}},
			rows:  2,
		},
		{
			name:  "a gift from one player does not raise trust toward another",
			gifts: []gift{{"p2", "mithril_ingot", 0}},
			reads: []read{{"p1", 0, 0}, {"p2", 0, 1.0}},
			rows:  1,
		},
		{
			name:  "each player's trust counts only their own gifts",
			gifts: []gift{{"p2", "mithril_ingot", 0}, {"p1", "kingsfoil_leaf", 0}},
			reads: []read{{"p1", 0, 0.2}, {"p2", 0, 1.0}},
			rows:  2,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newTrustEnv(t)
			for _, g := range tc.gifts {
				e.gift(t, g.player, "Elara", g.item, t0.Add(g.at))
			}
			for _, r := range tc.reads {
				if got := e.trustAt(t, "Elara", r.player, t0.Add(r.at)); !approx(got, r.want) {
					t.Errorf("trust toward %s at t0+%v = %.4f, want %.4f", r.player, r.at, got, r.want)
				}
			}
			if got := e.memoryCount(t, "Elara"); got != tc.rows {
				t.Errorf("memory rows = %d, want %d", got, tc.rows)
			}
		})
	}
}

func TestTrustPreconditionFollowsMemory(t *testing.T) {
	type questCase struct {
		quest, npc, question string
	}
	elaraCure := questCase{"sq_2_elara_research", "Elara", "Do you know of a cure?"}        // trust > 0.5
	baelorTool := questCase{"sq_3_baelor_gate", "Baelor", "Can you make an obsidian tool?"} // trust > 0.4

	cases := []struct {
		name          string
		q             questCase
		seed          func(t *testing.T, e *trustEnv)
		askAt         time.Duration // after t0
		wantCompleted bool
	}{
		{
			name:          "a year-long gift passes the Elara check a day before its half-life",
			q:             elaraCure,
			seed:          func(t *testing.T, e *trustEnv) { e.gift(t, "p1", "Elara", "mithril_ingot", t0) },
			askAt:         year - 24*time.Hour, // trust 0.501
			wantCompleted: true,
		},
		{
			name:          "the same gift fails the Elara check once it has faded to exactly 0.5",
			q:             elaraCure,
			seed:          func(t *testing.T, e *trustEnv) { e.gift(t, "p1", "Elara", "mithril_ingot", t0) },
			askAt:         year,
			wantCompleted: false,
		},
		{
			name:          "a petal passes Baelor's check a day later",
			q:             baelorTool,
			seed:          func(t *testing.T, e *trustEnv) { e.gift(t, "p1", "Baelor", "moonshadow_petal", t0) },
			askAt:         24 * time.Hour, // trust 0.419
			wantCompleted: true,
		},
		{
			name:          "the same petal fails Baelor's check two days later",
			q:             baelorTool,
			seed:          func(t *testing.T, e *trustEnv) { e.gift(t, "p1", "Baelor", "moonshadow_petal", t0) },
			askAt:         48 * time.Hour, // trust 0.350
			wantCompleted: false,
		},
		{
			name:          "another player's gift does not unlock the quest",
			q:             elaraCure,
			seed:          func(t *testing.T, e *trustEnv) { e.gift(t, "p2", "Elara", "mithril_ingot", t0) },
			wantCompleted: false,
		},
		{
			name: "an attack cancels enough trust to lock the quest",
			q:    elaraCure,
			seed: func(t *testing.T, e *trustEnv) {
				e.gift(t, "p1", "Elara", "mithril_ingot", t0)
				e.seedAttack(t, "p1", "Elara", t0) // 1.0 - 0.5 = 0.5, not above 0.5
			},
			wantCompleted: false,
		},
		{
			name: "gifts beyond full trust are not banked against a later attack",
			q:    elaraCure,
			seed: func(t *testing.T, e *trustEnv) {
				e.gift(t, "p1", "Elara", "mithril_ingot", t0)
				e.gift(t, "p1", "Elara", "moonshadow_petal", t0)
				e.seedAttack(t, "p1", "Elara", t0) // min(1, 1.5) - 0.5 = 0.5, not above 0.5
			},
			wantCompleted: false,
		},
		{
			name: "legacy rows with no delta contribute nothing to the check",
			q:    elaraCure,
			seed: func(t *testing.T, e *trustEnv) {
				e.seedLegacy(t, "p1", "Elara", t0)
				e.seedLegacy(t, "p1", "Elara", t0)
			},
			wantCompleted: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newTrustEnv(t)
			quest := seedQuest(t, e.ctx, e.db, tc.q.quest, tc.q.quest, "quests/definitions/"+tc.q.quest+".json")
			state := seedPlayerQuestState(t, e.ctx, e.db, e.players["p1"], quest, tc.q.quest, 1)
			tc.seed(t, e)

			ev := dto.EventMessage{
				EventType:      "PLAYER_ASKED_QUESTION",
				SourceEntityId: "p1",
				TargetNpcName:  tc.q.npc,
				QuestionText:   tc.q.question,
			}
			fail, err := e.qm.checkQuestCompletionAt(e.ctx, e.db, e.players["p1"], e.npcs[tc.q.npc], ev, t0.Add(tc.askAt))
			if err != nil {
				t.Fatalf("checkQuestCompletionAt: %v", err)
			}
			got, err := e.db.PlayerQuestState.Get(e.ctx, state.ID)
			if err != nil {
				t.Fatalf("reload quest state: %v", err)
			}
			if got.IsCompleted != tc.wantCompleted {
				t.Errorf("quest completed = %v, want %v", got.IsCompleted, tc.wantCompleted)
			}
			wantFail := e.qm.Quests[tc.q.quest].Steps["1"].FailResponse
			if tc.wantCompleted && fail != nil {
				t.Errorf("fail response = %+v, want none", fail)
			}
			if !tc.wantCompleted && (fail == nil || *fail != wantFail) {
				t.Errorf("fail response = %+v, want %+v", fail, wantFail)
			}
		})
	}
}

func TestQuestRewardTrust(t *testing.T) {
	e := newTrustEnv(t)
	herbs := seedQuest(t, e.ctx, e.db, "sq_e1_missing_herbs", "A Simple Errand", "quests/definitions/sq_e1_missing_herbs.json")
	seedPlayerQuestState(t, e.ctx, e.db, e.players["p1"], herbs, "sq_e1_missing_herbs", 2)
	research := seedQuest(t, e.ctx, e.db, "sq_2_elara_research", "The Herbalist's Knowledge", "quests/definitions/sq_2_elara_research.json")
	researchState := seedPlayerQuestState(t, e.ctx, e.db, e.players["p1"], research, "sq_2_elara_research", 1)

	// Handing in the herbs completes sq_e1 and writes its "+0.4" trust reward at t0.
	submit := dto.EventMessage{EventType: "PLAYER_SUBMITTED_QUEST_ITEM", SourceEntityId: "p1", TargetNpcName: "Elara", Keyword: "Sunpetal"}
	if _, err := e.qm.checkQuestCompletionAt(e.ctx, e.db, e.players["p1"], e.npcs["Elara"], submit, t0); err != nil {
		t.Fatalf("submit herbs: %v", err)
	}

	for _, r := range []struct {
		name string
		at   time.Duration
		want float64
	}{
		{"the herb reward raises trust by 0.4", 0, 0.4},
		{"the herb reward halves after its 15-hour half-life", rewardHalfLife, 0.2},
	} {
		t.Run(r.name, func(t *testing.T) {
			if got := e.trustAt(t, "Elara", "p1", t0.Add(r.at)); !approx(got, r.want) {
				t.Errorf("trust at t0+%v = %.4f, want %.4f", r.at, got, r.want)
			}
		})
	}

	t.Run("the reward alone does not open the cure question, the reward plus a petal does", func(t *testing.T) {
		ask := dto.EventMessage{EventType: "PLAYER_ASKED_QUESTION", SourceEntityId: "p1", TargetNpcName: "Elara", QuestionText: "Is there a cure?"}
		fail, err := e.qm.checkQuestCompletionAt(e.ctx, e.db, e.players["p1"], e.npcs["Elara"], ask, t0)
		if err != nil || fail == nil {
			t.Fatalf("with 0.4 trust: fail = %+v, err = %v; want the fail response", fail, err)
		}
		e.gift(t, "p1", "Elara", "moonshadow_petal", t0) // 0.4 + 0.5 = 0.9
		fail, err = e.qm.checkQuestCompletionAt(e.ctx, e.db, e.players["p1"], e.npcs["Elara"], ask, t0)
		if err != nil || fail != nil {
			t.Fatalf("with 0.9 trust: fail = %+v, err = %v; want completion", fail, err)
		}
		got, err := e.db.PlayerQuestState.Get(e.ctx, researchState.ID)
		if err != nil || !got.IsCompleted {
			t.Errorf("cure quest completed = %v (err %v), want true", got != nil && got.IsCompleted, err)
		}
	})
}

func TestAdminSetTrust(t *testing.T) {
	cases := []struct {
		name    string
		seed    func(t *testing.T, e *trustEnv)
		keyword []string // one ADMIN_SET_TRUST per entry, in order
		want    float64
		// Rows left on Elara and Baelor after the command(s).
		elaraRows, baelorRows int
		// Trust toward p2 on Elara at t0 afterwards, to prove it was not touched.
		wantP2 float64
	}{
		{
			name:      "setting trust with no history lands exactly",
			keyword:   []string{"Elara,0.8"},
			want:      0.8,
			elaraRows: 1,
		},
		{
			name: "setting trust replaces gifts, an attack and legacy rows rather than adding to them",
			seed: func(t *testing.T, e *trustEnv) {
				e.gift(t, "p1", "Elara", "moonshadow_petal", t0)
				e.gift(t, "p1", "Elara", "kingsfoil_leaf", t0)
				e.seedAttack(t, "p1", "Elara", t0)
				e.seedLegacy(t, "p1", "Elara", t0)
			},
			keyword:   []string{"Elara,0.3"},
			want:      0.3,
			elaraRows: 1,
		},
		{
			name:      "setting twice keeps only the second value",
			keyword:   []string{"Elara,0.8", "Elara,0.2"},
			want:      0.2,
			elaraRows: 1,
		},
		{
			name:      "a negative value lands exactly",
			keyword:   []string{"Elara,-0.4"},
			want:      -0.4,
			elaraRows: 1,
		},
		{
			name:      "a value above one is clamped to one",
			keyword:   []string{"Elara,1.5"},
			want:      1.0,
			elaraRows: 1,
		},
		{
			name:      "setting to zero clears trust",
			seed:      func(t *testing.T, e *trustEnv) { e.gift(t, "p1", "Elara", "mithril_ingot", t0) },
			keyword:   []string{"Elara,0"},
			want:      0,
			elaraRows: 1,
		},
		{
			name: "other players' memories and other NPCs' memories are left alone",
			seed: func(t *testing.T, e *trustEnv) {
				e.gift(t, "p1", "Elara", "moonshadow_petal", t0)
				e.gift(t, "p2", "Elara", "kingsfoil_leaf", t0)
				e.seedLegacy(t, "p2", "Elara", t0)
				e.gift(t, "p1", "Baelor", "moonshadow_petal", t0)
			},
			keyword:    []string{"Elara,0.6"},
			want:       0.6,
			elaraRows:  3,
			baelorRows: 1,
			wantP2:     0.2,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newTrustEnv(t)
			e.qm.AdminEnabled = true
			if tc.seed != nil {
				tc.seed(t, e)
			}
			for _, kw := range tc.keyword {
				ev := dto.EventMessage{EventType: "ADMIN_SET_TRUST", SourceEntityId: "p1", Keyword: kw}
				if err := e.qm.HandleAdminCommand(e.ctx, e.db, ev); err != nil {
					t.Fatalf("ADMIN_SET_TRUST %s: %v", kw, err)
				}
			}
			at := adminRowTime(t, e, "p1")
			if got := e.trustAt(t, "Elara", "p1", at); !approx(got, tc.want) {
				t.Errorf("trust toward p1 = %.4f, want %.4f", got, tc.want)
			}
			if got := e.trustAt(t, "Elara", "p2", t0); !approx(got, tc.wantP2) {
				t.Errorf("trust toward p2 = %.4f, want %.4f", got, tc.wantP2)
			}
			if got := e.memoryCount(t, "Elara"); got != tc.elaraRows {
				t.Errorf("Elara memory rows = %d, want %d", got, tc.elaraRows)
			}
			if got := e.memoryCount(t, "Baelor"); got != tc.baelorRows {
				t.Errorf("Baelor memory rows = %d, want %d", got, tc.baelorRows)
			}
		})
	}

	t.Run("a set trust is a memory like any other and fades with a quest reward's half-life", func(t *testing.T) {
		e := newTrustEnv(t)
		e.qm.AdminEnabled = true
		ev := dto.EventMessage{EventType: "ADMIN_SET_TRUST", SourceEntityId: "p1", Keyword: "Elara,0.8"}
		if err := e.qm.HandleAdminCommand(e.ctx, e.db, ev); err != nil {
			t.Fatalf("ADMIN_SET_TRUST: %v", err)
		}
		at := adminRowTime(t, e, "p1")
		if got := e.trustAt(t, "Elara", "p1", at.Add(rewardHalfLife)); !approx(got, 0.4) {
			t.Errorf("trust one reward half-life later = %.4f, want 0.4", got)
		}
	})

	t.Run("with admin commands disabled nothing is replaced", func(t *testing.T) {
		e := newTrustEnv(t)
		e.gift(t, "p1", "Elara", "moonshadow_petal", t0)
		ev := dto.EventMessage{EventType: "ADMIN_SET_TRUST", SourceEntityId: "p1", Keyword: "Elara,0.9"}
		if err := e.qm.HandleAdminCommand(e.ctx, e.db, ev); err == nil {
			t.Fatal("ADMIN_SET_TRUST with admin disabled: want an error, got nil")
		}
		if got := e.trustAt(t, "Elara", "p1", t0); !approx(got, 0.5) {
			t.Errorf("trust = %.4f, want the gift's 0.5", got)
		}
		if got := e.memoryCount(t, "Elara"); got != 1 {
			t.Errorf("memory rows = %d, want 1", got)
		}
	})
}

// adminRowTime returns when the admin command wrote its row. HandleAdminCommand stamps the row
// with the wall clock, so trust is read at exactly that instant rather than at a guessed time.
func adminRowTime(t *testing.T, e *trustEnv, player string) time.Time {
	t.Helper()
	eps, _, err := npcstate.Load(e.ctx, e.db, e.npcs["Elara"])
	if err != nil {
		t.Fatalf("npcstate.Load: %v", err)
	}
	for _, ep := range eps {
		if ep.Actor == player && ep.EventType == "QUEST_REWARD" {
			return ep.LastAt
		}
	}
	t.Fatalf("no admin-set row for %s", player)
	return time.Time{}
}

// Memory descriptions must begin with the actor, so the prompt formatter can rewrite them to
// "You ..." for the speaker and "Someone ..." for a bystander. Both show the clamped value the
// row actually holds.
func TestTrustRowDescriptionsStartWithTheActor(t *testing.T) {
	description := func(t *testing.T, e *trustEnv) string {
		t.Helper()
		_, rows, err := npcstate.Load(e.ctx, e.db, e.npcs["Elara"])
		if err != nil {
			t.Fatalf("npcstate.Load: %v", err)
		}
		for _, r := range rows {
			if r.EventType == "QUEST_REWARD" {
				return r.Description
			}
		}
		t.Fatal("no QUEST_REWARD row")
		return ""
	}

	t.Run("a quest reward", func(t *testing.T) {
		e := newTrustEnv(t)
		herbs := seedQuest(t, e.ctx, e.db, "sq_e1_missing_herbs", "A Simple Errand", "quests/definitions/sq_e1_missing_herbs.json")
		seedPlayerQuestState(t, e.ctx, e.db, e.players["p1"], herbs, "sq_e1_missing_herbs", 2)
		submit := dto.EventMessage{EventType: "PLAYER_SUBMITTED_QUEST_ITEM", SourceEntityId: "p1", TargetNpcName: "Elara", Keyword: "Sunpetal"}
		if _, err := e.qm.checkQuestCompletionAt(e.ctx, e.db, e.players["p1"], e.npcs["Elara"], submit, t0); err != nil {
			t.Fatalf("submit herbs: %v", err)
		}
		if got, want := description(t, e), "p1 completed a quest for me: trust changed by 0.40"; got != want {
			t.Errorf("description = %q, want %q", got, want)
		}
	})

	t.Run("an admin set, clamped to 1", func(t *testing.T) {
		e := newTrustEnv(t)
		e.qm.AdminEnabled = true
		ev := dto.EventMessage{EventType: "ADMIN_SET_TRUST", SourceEntityId: "p1", Keyword: "Elara,1.5"}
		if err := e.qm.HandleAdminCommand(e.ctx, e.db, ev); err != nil {
			t.Fatalf("ADMIN_SET_TRUST: %v", err)
		}
		if got, want := description(t, e), "p1 had trust with Elara set to 1.00 by an admin"; got != want {
			t.Errorf("description = %q, want %q", got, want)
		}
	})
}
