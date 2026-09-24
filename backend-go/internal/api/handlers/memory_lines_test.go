package handlers

import (
	"reflect"
	"testing"
	"time"

	"agentic-npc-backend/internal/dto"
)

// ep describes one memory row seeded straight into the database, so each case controls weight
// exactly. With the default intensity of 0 the half-life is exactly one hour, and with last_at
// equal to the evaluation time the decay factor is exactly 1, so a row's weight is simply its
// largest |delta| times the repetition and forgiveness factors.
type ep struct {
	actor    string
	desc     string
	delta    map[string]float64
	count    float64 // 0 means 1
	harmful  bool
	forgiven float64
	betrayal bool
	ago      time.Duration // time between last_at and t0
	text     string
}

// seedLines creates the rows in order; each is created one second after the previous one,
// so a later row in the slice is "newer" for the tie-break.
func seedLines(t *testing.T, e *episodeEnv, eps []ep) {
	t.Helper()
	for i, s := range eps {
		count := s.count
		if count == 0 {
			count = 1
		}
		op := e.db.Memory.Create().
			SetOwner(e.npc).
			SetActor(s.actor).
			SetEventType("TEST").
			SetDescription(s.desc).
			SetParticipants([]string{s.actor, e.npc.ID.String()}).
			SetCount(count).
			SetHarmful(s.harmful).
			SetForgiven(s.forgiven).
			SetBetrayal(s.betrayal).
			SetText(s.text).
			SetCreatedAt(t0.Add(-time.Hour + time.Duration(i)*time.Second)).
			SetFirstAt(t0.Add(-s.ago)).
			SetLastAt(t0.Add(-s.ago))
		if s.delta != nil {
			op.SetDelta(s.delta)
		}
		if _, err := op.Save(e.ctx); err != nil {
			t.Fatalf("seed memory %q: %v", s.desc, err)
		}
	}
}

// linesFor builds the AI request for speaker at t0 and returns its memory lines.
func linesFor(t *testing.T, e *episodeEnv, speaker string) []string {
	t.Helper()
	ev := dto.EventMessage{EventType: "PLAYER_INTERACT", SourceEntityId: speaker, TargetNpcName: e.npc.Name}
	args, err := e.h.buildAIArgsAt(e.ctx, ev, e.npc, e.players[speaker], t0)
	if err != nil {
		t.Fatalf("buildAIArgsAt(%s): %v", speaker, err)
	}
	return args.memoryLines
}

func wantLines(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("memory lines:\n got  %q\n want %q", got, want)
	}
}

func trust(v float64) map[string]float64 { return map[string]float64{"trust": v} }
func anger(v float64) map[string]float64 { return map[string]float64{"anger": v} }

func TestMemoryLines(t *testing.T) {
	tests := []struct {
		name    string
		seed    []ep
		run     func(t *testing.T, e *episodeEnv) // optional, for cases driven through recordEpisodeAt
		speaker string
		want    []string
	}{
		{
			name: "an NPC with no memories sends no lines",
			want: nil,
		},
		{
			name: "the speaker's own top five by weight reach the prompt and the sixth is dropped",
			seed: []ep{
				{actor: "P1", desc: "P1 did thing 30", delta: trust(0.3)},
				{actor: "P1", desc: "P1 did thing 60", delta: trust(0.6)},
				{actor: "P1", desc: "P1 did thing 10", delta: trust(-0.1)},
				{actor: "P1", desc: "P1 did thing 50", delta: trust(-0.5)},
				{actor: "P1", desc: "P1 did thing 20", delta: trust(0.2)},
				{actor: "P1", desc: "P1 did thing 40", delta: anger(0.4)},
			},
			want: []string{
				"You did thing 60",
				"You did thing 50",
				"You did thing 40",
				"You did thing 30",
				"You did thing 20",
			},
		},
		{
			// Weights 0.02 and 0: well below notability, but notability only filters bystanders.
			// Note: the conversation's question text is stored but is not part of the line (finding 2).
			name: "the speaker's faint episodes and conversations still reach the prompt",
			seed: []ep{
				{actor: "P1", desc: "P1 triggered PLAYER_INTERACT on Elara", delta: map[string]float64{"joy": 0.02, "trust": 0.01}},
				{actor: "P1", desc: "P1 triggered PLAYER_ASKED_QUESTION on Elara", delta: map[string]float64{}, text: "where is the well?"},
			},
			want: []string{
				"You triggered PLAYER_INTERACT on Elara",
				"You triggered PLAYER_ASKED_QUESTION on Elara",
			},
		},
		{
			name: "other players' episodes are notable only, capped at three, heaviest first",
			seed: []ep{
				{actor: "P2", desc: "P2 did thing 50", delta: anger(0.5)},
				{actor: "P3", desc: "P3 did thing 29", delta: anger(0.29)},
				{actor: "P2", desc: "P2 did thing 90", delta: anger(0.9)},
				{actor: "P3", desc: "P3 did thing 35", delta: anger(0.35)},
				{actor: "P2", desc: "P2 did thing 10", delta: anger(0.1)},
				{actor: "P3", desc: "P3 did thing 40", delta: trust(-0.4)},
			},
			want: []string{
				"Someone did thing 90",
				"Someone did thing 50",
				"Someone did thing 40",
			},
		},
		{
			name: "a bystander's episode at exactly the notability threshold is included, just under is not",
			seed: []ep{
				{actor: "P2", desc: "P2 did thing 30", delta: anger(0.3)},
				{actor: "P3", desc: "P3 did thing 29", delta: anger(0.29)},
			},
			want: []string{"Someone did thing 30"},
		},
		{
			name: "the speaker's lines come before bystanders' even when the bystander's weigh more",
			seed: []ep{
				{actor: "P2", desc: "P2 did thing 90", delta: anger(0.9)},
				{actor: "P1", desc: "P1 did thing 10", delta: trust(0.1)},
			},
			want: []string{"You did thing 10", "Someone did thing 90"},
		},
		{
			// Half-life is one hour at intensity 0: anger 0.8 is 0.4 after 1h, 0.2 after 2h,
			// and 0.9 is 0.1125 after 3h.
			name: "memories fade: a bystander's drops out of the prompt and the speaker's old one sinks",
			seed: []ep{
				{actor: "P1", desc: "P1 did old thing", delta: anger(0.9), ago: 3 * time.Hour},
				{actor: "P1", desc: "P1 did fresh thing", delta: trust(0.2)},
				{actor: "P2", desc: "P2 did thing an hour ago", delta: anger(0.8), ago: time.Hour},
				{actor: "P2", desc: "P2 did thing two hours ago", delta: anger(0.8), ago: 2 * time.Hour},
			},
			want: []string{
				"You did fresh thing",
				"You did old thing",
				"Someone did thing an hour ago",
			},
		},
		{
			// Forgiveness weight for anger is 1.0: 0.8 forgiven by 0.75 weighs 0.2, by 0.5 weighs 0.4.
			name: "forgiveness can pull a bystander's attack below notability",
			seed: []ep{
				{actor: "P2", desc: "P2 shoved Elara", delta: anger(0.8), harmful: true, forgiven: 0.75},
				{actor: "P3", desc: "P3 kicked Elara", delta: anger(0.8), harmful: true, forgiven: 0.5},
			},
			want: []string{"Someone kicked Elara"},
		},
		{
			// Non-harmful escalation is count^0.5, so these weigh 0.854, 0.614, 0.424 and 0.173.
			name: "a repeat count is appended, rounded to the nearest whole time",
			seed: []ep{
				{actor: "P1", desc: "P1 did thing thrice", delta: trust(0.1), count: 3},
				{actor: "P1", desc: "P1 did thing twice", delta: trust(0.3), count: 2},
				{actor: "P1", desc: "P1 did thing 1.508 times", delta: trust(0.5), count: 1.508},
				{actor: "P1", desc: "P1 did thing 1.49 times", delta: trust(0.7), count: 1.49},
			},
			want: []string{
				"You did thing 1.49 times",
				"You did thing 1.508 times (2 times)",
				"You did thing twice (2 times)",
				"You did thing thrice (3 times)",
			},
		},
		{
			name: "a betrayal is marked after the count, for the speaker and for a bystander",
			seed: []ep{
				{actor: "P2", desc: "P2 triggered PLAYER_ATTACKED on Elara", delta: anger(0.6), harmful: true, betrayal: true},
				{actor: "P1", desc: "P1 triggered PLAYER_THREW_STONE on Elara", delta: map[string]float64{"anger": 0.1, "trust": -0.3}, harmful: true, betrayal: true, count: 2},
			},
			want: []string{
				"You triggered PLAYER_THREW_STONE on Elara (2 times) — after apologising",
				"Someone triggered PLAYER_ATTACKED on Elara — after apologising",
			},
		},
		{
			name: "equal weights keep the newer episode first",
			seed: []ep{
				{actor: "P1", desc: "P1 did older thing", delta: trust(0.2)},
				{actor: "P2", desc: "P2 did older thing", delta: anger(0.5)},
				{actor: "P1", desc: "P1 did newer thing", delta: trust(0.2)},
				{actor: "P2", desc: "P2 did newer thing", delta: anger(0.5)},
			},
			want: []string{
				"You did newer thing",
				"You did older thing",
				"Someone did newer thing",
				"Someone did older thing",
			},
		},
		{
			// The real rules: a stone is anger 0.1 / trust -0.1, harmful (escalation count^1.5).
			// One stone weighs 0.1 and is not notable; five weigh 5^1.5 × 0.1 = 1.12, capped at 1.
			name: "one stone from a bystander is not notable, five are",
			run: func(t *testing.T, e *episodeEnv) {
				e.players["P3"] = seedTestPlayer(t, e.ctx, e.db, "P3")
				e.record(t, "P2", "PLAYER_THREW_STONE", 0)
				for i := 0; i < 5; i++ {
					e.record(t, "P3", "PLAYER_THREW_STONE", 0)
				}
			},
			want: []string{"Someone triggered PLAYER_THREW_STONE on Elara (5 times)"},
		},
		{
			// The quest layer writes rewards as "<player> completed a quest for me: ...", so a
			// bystander's reward is attributed to someone else, never to the speaker.
			name: "a bystander's quest reward is attributed to someone else",
			seed: []ep{
				{actor: "P2", desc: "P2 completed a quest for me: trust changed by 0.50", delta: trust(0.5)},
			},
			want: []string{"Someone completed a quest for me: trust changed by 0.50"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := newEpisodeEnv(t)
			if tc.run != nil {
				tc.run(t, e)
			}
			seedLines(t, e, tc.seed)
			speaker := tc.speaker
			if speaker == "" {
				speaker = "P1"
			}
			wantLines(t, linesFor(t, e, speaker), tc.want)
		})
	}
}

// TestMemoryLinesManyEpisodes seeds fifteen episodes from four players on one NPC and reads the
// prompt from two different speakers' points of view.
func TestMemoryLinesManyEpisodes(t *testing.T) {
	e := newEpisodeEnv(t)
	for _, id := range []string{"P3", "P4"} {
		e.players[id] = seedTestPlayer(t, e.ctx, e.db, id)
	}
	seedLines(t, e, []ep{
		// P1 — weights: f 1.0 (0.6 × 3^0.5 = 1.04, capped), a 0.9, c 0.4, g 0.25, b 0.15, d 0.05, e 0.
		{actor: "P1", desc: "P1 did a", delta: trust(0.9)},
		{actor: "P1", desc: "P1 did b", delta: trust(0.15)},
		{actor: "P1", desc: "P1 did c", delta: anger(0.8), ago: time.Hour},
		{actor: "P1", desc: "P1 did d", delta: trust(0.05)},
		{actor: "P1", desc: "P1 did e", delta: map[string]float64{}},
		{actor: "P1", desc: "P1 did f", delta: trust(0.6), count: 3},
		{actor: "P1", desc: "P1 did g", delta: trust(0.25)},
		// P2 — h 0.7, i 0.2, j 0.45 (betrayal).
		{actor: "P2", desc: "P2 did h", delta: anger(0.7)},
		{actor: "P2", desc: "P2 did i", delta: anger(0.2)},
		{actor: "P2", desc: "P2 did j", delta: anger(0.45), harmful: true, betrayal: true},
		// P3 — k 0.2 (0.8 two half-lives ago), l 0.55, m 0.3.
		{actor: "P3", desc: "P3 did k", delta: anger(0.8), ago: 2 * time.Hour},
		{actor: "P3", desc: "P3 did l", delta: anger(0.55)},
		{actor: "P3", desc: "P3 did m", delta: trust(-0.3)},
		// P4 — n 0.25 (0.5 half forgiven), o 0.65 on an emotion no other episode uses.
		{actor: "P4", desc: "P4 did n", delta: anger(0.5), harmful: true, forgiven: 0.5},
		{actor: "P4", desc: "P4 did o", delta: map[string]float64{"awe": 0.65}},
	})

	t.Run("the first player hears their own top five and the three most notable others", func(t *testing.T) {
		wantLines(t, linesFor(t, e, "P1"), []string{
			"You did f (3 times)",
			"You did a",
			"You did c",
			"You did g",
			"You did b",
			"Someone did h",
			"Someone did o",
			"Someone did l",
		})
	})
	t.Run("the second player hears all three of their own and the first player's episodes as someone's", func(t *testing.T) {
		wantLines(t, linesFor(t, e, "P2"), []string{
			"You did h",
			"You did j — after apologising",
			"You did i",
			"Someone did f (3 times)",
			"Someone did a",
			"Someone did o",
		})
	})
}
