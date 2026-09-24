package handlers

import (
	"context"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"agentic-npc-backend/internal/db/ent"
	"agentic-npc-backend/internal/db/ent/enttest"
	entmemory "agentic-npc-backend/internal/db/ent/memory"
	"agentic-npc-backend/internal/domain/quest_logic"
	"agentic-npc-backend/internal/domain/rules"
	"agentic-npc-backend/internal/dto"

	_ "github.com/mattn/go-sqlite3"
)

// House style: floats are compared with a 1% relative tolerance.
const relTol = 0.01

func near(got, want float64) bool {
	if want == 0 {
		return math.Abs(got) < 1e-9
	}
	return math.Abs(got-want) <= relTol*math.Abs(want)
}

// t0 is the fixed clock every episode test starts from.
var t0 = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

type episodeEnv struct {
	ctx     context.Context
	db      *ent.Client
	h       *WebSocketHandler
	qm      *quest_logic.QuestManager
	npc     *ent.NPC
	players map[string]*ent.Player
}

// newEpisodeEnv opens a fresh in-memory database with the real rules file, one NPC and players P1, P2.
func newEpisodeEnv(t *testing.T) *episodeEnv {
	t.Helper()
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db := enttest.Open(t, "sqlite3", "file:"+dbName+"?mode=memory&cache=shared&_fk=1")
	t.Cleanup(func() { db.Close() })

	qm, err := quest_logic.NewQuestManager("../../../../gamedata")
	if err != nil {
		t.Fatalf("NewQuestManager: %v", err)
	}
	qm.Rules, err = rules.Load("../../../../gamedata/events.json")
	if err != nil {
		t.Fatalf("rules.Load: %v", err)
	}

	ctx := context.Background()
	env := &episodeEnv{
		ctx:     ctx,
		db:      db,
		h:       &WebSocketHandler{dbClient: db, questManager: qm},
		qm:      qm,
		npc:     seedTestNPC(t, ctx, db, "Elara"),
		players: map[string]*ent.Player{},
	}
	for _, id := range []string{"P1", "P2"} {
		env.players[id] = seedTestPlayer(t, ctx, db, id)
	}
	return env
}

// record sends one event from actor at t0+offset.
func (e *episodeEnv) record(t *testing.T, actor, eventType string, offset time.Duration, opts ...func(*dto.EventMessage)) {
	t.Helper()
	ev := dto.EventMessage{EventType: eventType, SourceEntityId: actor, TargetNpcName: e.npc.Name}
	for _, o := range opts {
		o(&ev)
	}
	if err := e.h.recordEpisodeAt(e.ctx, ev, e.npc, e.players[actor], t0.Add(offset)); err != nil {
		t.Fatalf("recordEpisodeAt(%s %s): %v", actor, eventType, err)
	}
}

func withText(s string) func(*dto.EventMessage) {
	return func(ev *dto.EventMessage) { ev.QuestionText = s }
}
func withKeyword(s string) func(*dto.EventMessage) {
	return func(ev *dto.EventMessage) { ev.Keyword = s }
}

// rows returns every memory row in insertion order.
func (e *episodeEnv) rows(t *testing.T) []*ent.Memory {
	t.Helper()
	rows, err := e.db.Memory.Query().Order(ent.Asc(entmemory.FieldID)).All(e.ctx)
	if err != nil {
		t.Fatalf("query memories: %v", err)
	}
	return rows
}

func wantRowCount(t *testing.T, rows []*ent.Memory, n int) {
	t.Helper()
	if len(rows) != n {
		var got []string
		for _, r := range rows {
			got = append(got, r.Actor+":"+r.EventType)
		}
		t.Fatalf("want %d memory rows, got %d: %v", n, len(rows), got)
	}
}

func TestRecordEpisode(t *testing.T) {
	tests := []struct {
		name  string
		run   func(t *testing.T, e *episodeEnv)
		check func(t *testing.T, e *episodeEnv, rows []*ent.Memory)
	}{
		{
			name: "a plain event creates one row carrying its rule",
			run: func(t *testing.T, e *episodeEnv) {
				e.record(t, "P1", "PLAYER_INTERACT", 0)
			},
			check: func(t *testing.T, e *episodeEnv, rows []*ent.Memory) {
				wantRowCount(t, rows, 1)
				r := rows[0]
				if r.Actor != "P1" || r.EventType != "PLAYER_INTERACT" || r.Subject != "" {
					t.Errorf("identity: got actor=%q event=%q subject=%q", r.Actor, r.EventType, r.Subject)
				}
				if !reflect.DeepEqual(r.Delta, map[string]float64{"joy": 0.02, "trust": 0.01}) {
					t.Errorf("delta: got %v", r.Delta)
				}
				if r.Intensity != 0.05 || r.Count != 1 || r.Harmful || r.Betrayal || r.Forgiven != 0 {
					t.Errorf("got intensity=%v count=%v harmful=%v betrayal=%v forgiven=%v",
						r.Intensity, r.Count, r.Harmful, r.Betrayal, r.Forgiven)
				}
				if !r.FirstAt.Equal(t0) || !r.LastAt.Equal(t0) {
					t.Errorf("times: first=%v last=%v, want both %v", r.FirstAt, r.LastAt, t0)
				}
				if r.Description != "P1 triggered PLAYER_INTERACT on Elara" {
					t.Errorf("description: got %q", r.Description)
				}
				if !reflect.DeepEqual(r.Participants, []string{"P1", e.npc.ID.String()}) {
					t.Errorf("participants: got %v", r.Participants)
				}
			},
		},
		{
			name: "a repeat at the same instant merges to a count of two",
			run: func(t *testing.T, e *episodeEnv) {
				e.record(t, "P1", "PLAYER_THREW_STONE", 0)
				e.record(t, "P1", "PLAYER_THREW_STONE", 0)
			},
			check: func(t *testing.T, e *episodeEnv, rows []*ent.Memory) {
				wantRowCount(t, rows, 1)
				if !near(rows[0].Count, 2.0) {
					t.Errorf("count: got %v, want 2", rows[0].Count)
				}
			},
		},
		{
			name: "a repeat six hours later merges with the old count decayed",
			run: func(t *testing.T, e *episodeEnv) {
				e.record(t, "P1", "PLAYER_THREW_STONE", 0)
				e.record(t, "P1", "PLAYER_THREW_STONE", 6*time.Hour)
			},
			check: func(t *testing.T, e *episodeEnv, rows []*ent.Memory) {
				wantRowCount(t, rows, 1)
				// Stone half-life is 1h * 8760^0.2 = 6.14h, so after 6h the first
				// stone is worth 0.508 and the merged count is 1.508, not 2.
				if !near(rows[0].Count, 1.508) {
					t.Errorf("count: got %v, want 1.508", rows[0].Count)
				}
				if !rows[0].FirstAt.Equal(t0) || !rows[0].LastAt.Equal(t0.Add(6*time.Hour)) {
					t.Errorf("times: first=%v last=%v", rows[0].FirstAt, rows[0].LastAt)
				}
			},
		},
		{
			name: "a repeat after the memory has faded past the merge threshold starts a new row",
			run: func(t *testing.T, e *episodeEnv) {
				// 48h is ~7.8 half-lives: decay 0.004, below the 0.1 merge threshold.
				e.record(t, "P1", "PLAYER_THREW_STONE", 0)
				e.record(t, "P1", "PLAYER_THREW_STONE", 48*time.Hour)
			},
			check: func(t *testing.T, e *episodeEnv, rows []*ent.Memory) {
				wantRowCount(t, rows, 2)
				for i, r := range rows {
					if r.Count != 1 {
						t.Errorf("row %d count: got %v, want 1", i, r.Count)
					}
				}
			},
		},
		{
			name: "the same event from two players never merges across actors",
			run: func(t *testing.T, e *episodeEnv) {
				e.record(t, "P1", "PLAYER_THREW_STONE", 0)
				e.record(t, "P2", "PLAYER_THREW_STONE", 0)
			},
			check: func(t *testing.T, e *episodeEnv, rows []*ent.Memory) {
				wantRowCount(t, rows, 2)
				if rows[0].Actor != "P1" || rows[1].Actor != "P2" {
					t.Errorf("actors: got %q, %q", rows[0].Actor, rows[1].Actor)
				}
			},
		},
		{
			name: "submitting two different quest items keeps one row per item",
			run: func(t *testing.T, e *episodeEnv) {
				e.record(t, "P1", "PLAYER_SUBMITTED_QUEST_ITEM", 0, withKeyword("herb"))
				e.record(t, "P1", "PLAYER_SUBMITTED_QUEST_ITEM", 0, withKeyword("axe"))
				e.record(t, "P1", "PLAYER_SUBMITTED_QUEST_ITEM", 0, withKeyword("herb"))
			},
			check: func(t *testing.T, e *episodeEnv, rows []*ent.Memory) {
				wantRowCount(t, rows, 2)
				if rows[0].Subject != "herb" || rows[1].Subject != "axe" {
					t.Errorf("subjects: got %q, %q", rows[0].Subject, rows[1].Subject)
				}
				if !near(rows[0].Count, 2.0) || rows[1].Count != 1 {
					t.Errorf("counts: herb=%v axe=%v, want 2 and 1", rows[0].Count, rows[1].Count)
				}
			},
		},
		{
			name: "a conversation never merges and keeps each question's text",
			run: func(t *testing.T, e *episodeEnv) {
				e.record(t, "P1", "PLAYER_ASKED_QUESTION", 0, withText("Who built the tower?"))
				e.record(t, "P1", "PLAYER_ASKED_QUESTION", 0, withText("Where is the well?"))
			},
			check: func(t *testing.T, e *episodeEnv, rows []*ent.Memory) {
				wantRowCount(t, rows, 2)
				if rows[0].Text != "Who built the tower?" || rows[1].Text != "Where is the well?" {
					t.Errorf("texts: got %q, %q", rows[0].Text, rows[1].Text)
				}
				for i, r := range rows {
					if r.Count != 1 || len(r.Delta) != 0 {
						t.Errorf("row %d: count=%v delta=%v, want 1 and empty", i, r.Count, r.Delta)
					}
				}
			},
		},
		{
			name: "an apology forgives only that actor's harmful episodes and records what it covered",
			run: func(t *testing.T, e *episodeEnv) {
				e.record(t, "P1", "PLAYER_THREW_STONE", 0)
				e.record(t, "P2", "PLAYER_THREW_STONE", 0)
				e.record(t, "P1", "PLAYER_INTERACT", 0)
				e.record(t, "P1", "PLAYER_APOLOGIZED", time.Minute)
			},
			check: func(t *testing.T, e *episodeEnv, rows []*ent.Memory) {
				wantRowCount(t, rows, 4)
				p1Stone, p2Stone, p1Interact, apology := rows[0], rows[1], rows[2], rows[3]
				// First apology: step 0.8, under the stone's cap of 1 - 0.7*0.2 = 0.86.
				if !near(p1Stone.Forgiven, 0.8) {
					t.Errorf("P1 stone forgiven: got %v, want 0.8", p1Stone.Forgiven)
				}
				if p2Stone.Forgiven != 0 {
					t.Errorf("P2 stone forgiven by P1's apology: got %v", p2Stone.Forgiven)
				}
				if p1Interact.Forgiven != 0 {
					t.Errorf("non-harmful interact forgiven: got %v", p1Interact.Forgiven)
				}
				if apology.EventType != "PLAYER_APOLOGIZED" || apology.Actor != "P1" {
					t.Fatalf("last row: got %s from %s", apology.EventType, apology.Actor)
				}
				if !reflect.DeepEqual(apology.Covers, []int{p1Stone.ID}) {
					t.Errorf("covers: got %v, want [%d]", apology.Covers, p1Stone.ID)
				}
				if apology.Harmful || apology.Count != 1 || apology.Intensity != 0.1 || len(apology.Delta) != 0 {
					t.Errorf("apology row: harmful=%v count=%v intensity=%v delta=%v",
						apology.Harmful, apology.Count, apology.Intensity, apology.Delta)
				}
			},
		},
		{
			name: "an apology cannot forgive a severe attack past its cap",
			run: func(t *testing.T, e *episodeEnv) {
				e.record(t, "P1", "PLAYER_ATTACKED", 0)
				e.record(t, "P1", "PLAYER_APOLOGIZED", time.Minute)
			},
			check: func(t *testing.T, e *episodeEnv, rows []*ent.Memory) {
				wantRowCount(t, rows, 2)
				// Cap is 1 - 0.7*0.9 = 0.37, below the 0.8 step.
				if !near(rows[0].Forgiven, 0.37) {
					t.Errorf("attack forgiven: got %v, want 0.37", rows[0].Forgiven)
				}
			},
		},
		{
			name: "a second apology is worth half the first",
			run: func(t *testing.T, e *episodeEnv) {
				// The first apology has nothing to forgive, so the stone that
				// follows is not a betrayal; the second apology then steps by 0.4.
				e.record(t, "P1", "PLAYER_APOLOGIZED", 0)
				e.record(t, "P1", "PLAYER_THREW_STONE", time.Minute)
				e.record(t, "P1", "PLAYER_APOLOGIZED", 2*time.Minute)
			},
			check: func(t *testing.T, e *episodeEnv, rows []*ent.Memory) {
				wantRowCount(t, rows, 3)
				stone := rows[1]
				if stone.EventType != "PLAYER_THREW_STONE" || stone.Betrayal {
					t.Fatalf("row 1: got %s betrayal=%v", stone.EventType, stone.Betrayal)
				}
				if !near(stone.Forgiven, 0.4) {
					t.Errorf("stone forgiven after second apology: got %v, want 0.4", stone.Forgiven)
				}
				if len(rows[0].Covers) != 0 {
					t.Errorf("first apology covers: got %v, want none", rows[0].Covers)
				}
			},
		},
		{
			name: "harm after forgiveness is a new betrayal episode with the extra trust penalty",
			run: func(t *testing.T, e *episodeEnv) {
				e.record(t, "P1", "PLAYER_THREW_STONE", 0)
				e.record(t, "P2", "PLAYER_THREW_STONE", 0)
				e.record(t, "P2", "PLAYER_APOLOGIZED", time.Minute)
				e.record(t, "P1", "PLAYER_APOLOGIZED", time.Minute)
				e.record(t, "P1", "PLAYER_THREW_STONE", 2*time.Minute)
			},
			check: func(t *testing.T, e *episodeEnv, rows []*ent.Memory) {
				wantRowCount(t, rows, 5)
				p1Old, p2Stone, betrayal := rows[0], rows[1], rows[4]
				if p1Old.Forgiven != 0 || p1Old.Count != 1 {
					t.Errorf("P1's first stone: forgiven=%v count=%v, want 0 and 1 (revoked, not merged)",
						p1Old.Forgiven, p1Old.Count)
				}
				if !near(p2Stone.Forgiven, 0.8) {
					t.Errorf("P2's forgiveness touched by P1's betrayal: got %v, want 0.8", p2Stone.Forgiven)
				}
				if betrayal.Actor != "P1" || betrayal.EventType != "PLAYER_THREW_STONE" || !betrayal.Betrayal {
					t.Fatalf("new row: got %s %s betrayal=%v", betrayal.Actor, betrayal.EventType, betrayal.Betrayal)
				}
				// Stone trust -0.1 plus the -0.2 betrayal penalty.
				if !near(betrayal.Delta["trust"], -0.3) || !near(betrayal.Delta["anger"], 0.1) {
					t.Errorf("betrayal delta: got %v, want trust -0.3 anger 0.1", betrayal.Delta)
				}
				if !betrayal.Harmful || betrayal.Count != 1 || !betrayal.FirstAt.Equal(t0.Add(2*time.Minute)) {
					t.Errorf("betrayal row: harmful=%v count=%v first=%v", betrayal.Harmful, betrayal.Count, betrayal.FirstAt)
				}
				rule, _ := e.qm.Rules.Rule("PLAYER_THREW_STONE")
				if rule.Delta["trust"] != -0.1 {
					t.Errorf("betrayal penalty leaked into the shared rule: trust=%v", rule.Delta["trust"])
				}
			},
		},
		{
			name: "a harmless event after forgiveness is not a betrayal and keeps the forgiveness",
			run: func(t *testing.T, e *episodeEnv) {
				e.record(t, "P1", "PLAYER_THREW_STONE", 0)
				e.record(t, "P1", "PLAYER_APOLOGIZED", time.Minute)
				e.record(t, "P1", "PLAYER_INTERACT", 2*time.Minute)
			},
			check: func(t *testing.T, e *episodeEnv, rows []*ent.Memory) {
				wantRowCount(t, rows, 3)
				if rows[2].Betrayal {
					t.Errorf("interact flagged as betrayal")
				}
				if !near(rows[0].Forgiven, 0.8) {
					t.Errorf("stone forgiveness: got %v, want 0.8 kept", rows[0].Forgiven)
				}
			},
		},
		{
			name: "an unknown event type records nothing",
			run: func(t *testing.T, e *episodeEnv) {
				e.record(t, "P1", "PLAYER_DANCED_BADLY", 0)
			},
			check: func(t *testing.T, e *episodeEnv, rows []*ent.Memory) {
				wantRowCount(t, rows, 0)
			},
		},
		{
			name: "gifts and quest rewards are left to the quest layer",
			run: func(t *testing.T, e *episodeEnv) {
				e.record(t, "P1", "PLAYER_GAVE_GIFT", 0, withKeyword("apple"))
				e.record(t, "P1", "QUEST_REWARD", 0)
			},
			check: func(t *testing.T, e *episodeEnv, rows []*ent.Memory) {
				wantRowCount(t, rows, 0)
			},
		},
		{
			name: "a gift through the full path is written once, by the quest layer",
			run: func(t *testing.T, e *episodeEnv) {
				gift := dto.EventMessage{EventType: "PLAYER_GAVE_GIFT", SourceEntityId: "P1", TargetNpcName: "Elara", Keyword: "apple"}
				if _, err := e.qm.ProcessEvent(e.ctx, e.db, gift); err != nil {
					t.Fatalf("ProcessEvent gift: %v", err)
				}
				e.record(t, "P1", "PLAYER_GAVE_GIFT", 0, withKeyword("apple"))
			},
			check: func(t *testing.T, e *episodeEnv, rows []*ent.Memory) {
				wantRowCount(t, rows, 1)
				// The quest layer's delta comes from items.json (apple: trust 0.01).
				// A second write would merge rather than add a row, so the count must stay 1.
				r := rows[0]
				if r.Subject != "apple" || r.Count != 1 || !reflect.DeepEqual(r.Delta, map[string]float64{"trust": 0.01}) {
					t.Errorf("gift row: subject=%q count=%v delta=%v, want apple, 1, trust 0.01", r.Subject, r.Count, r.Delta)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEpisodeEnv(t)
			tt.run(t, e)
			tt.check(t, e, e.rows(t))
		})
	}
}
