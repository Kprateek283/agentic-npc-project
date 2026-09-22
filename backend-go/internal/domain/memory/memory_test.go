package memory

import (
	"math"
	"testing"
	"time"
)

func approxEqual(a, b, relTol float64) bool {
	if math.Abs(a-b) <= 1e-9 {
		return true
	}
	diff := math.Abs(a - b)
	denom := math.Max(math.Abs(a), math.Abs(b))
	return diff/denom <= relTol
}

func TestHalfLives(t *testing.T) {
	cfg := DefaultConfig()

	tests := []struct {
		name         string
		intensity    float64
		count        float64
		expectedEffI float64
		expectedDur  time.Duration
		expectedHrs  float64
	}{
		{
			name:         "one stone",
			intensity:    0.2,
			count:        1,
			expectedEffI: 0.2,
			expectedHrs:  6.1,
		},
		{
			name:         "five stones",
			intensity:    0.2,
			count:        5,
			expectedEffI: 1.0 - math.Pow(0.8, 5), // ~0.67232
			expectedHrs:  447.0,
		},
		{
			name:         "an arrow",
			intensity:    0.9,
			count:        1,
			expectedEffI: 0.9,
			expectedHrs:  3534.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			effI := EffectiveIntensity(tc.intensity, tc.count)
			if !approxEqual(effI, tc.expectedEffI, 0.01) {
				t.Fatalf("EffectiveIntensity(%v, %v) = %v, expected %v", tc.intensity, tc.count, effI, tc.expectedEffI)
			}
			hl := HalfLife(effI, cfg)
			hlHours := hl.Hours()
			if !approxEqual(hlHours, tc.expectedHrs, 0.01) {
				t.Fatalf("HalfLife hours = %v, expected ~%v (within 1%%)", hlHours, tc.expectedHrs)
			}
		})
	}
}

func TestFiveStonesInitial(t *testing.T) {
	cfg := DefaultConfig()
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)

	eps := []Episode{
		{
			Actor:     "player1",
			EventType: "PLAYER_THREW_STONE",
			Delta:     map[string]float64{"anger": 0.1, "trust": -0.1},
			Intensity: 0.2,
			Count:     5,
			Harmful:   true,
			FirstAt:   now,
			LastAt:    now,
		},
	}

	emotions := EmotionsToward("player1", eps, now, cfg)
	if !approxEqual(emotions["anger"], 1.0, 0.01) {
		t.Fatalf("anger = %v, expected 1.0", emotions["anger"])
	}
	if !approxEqual(emotions["trust"], -1.0, 0.01) {
		t.Fatalf("trust = %v, expected -1.0", emotions["trust"])
	}

	mood := GeneralMood(eps, now, cfg)
	if !approxEqual(mood["anger"], 0.25, 0.01) {
		t.Fatalf("GeneralMood anger = %v, expected 0.25", mood["anger"])
	}
}

func TestApology(t *testing.T) {
	cfg := DefaultConfig()
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)

	eps := []Episode{
		{
			Actor:     "player1",
			EventType: "PLAYER_THREW_STONE",
			Delta:     map[string]float64{"anger": 0.1, "trust": -0.1},
			Intensity: 0.2,
			Count:     5,
			Harmful:   true,
			Forgiven:  0.52938,
			FirstAt:   now,
			LastAt:    now,
		},
	}

	emotions := EmotionsToward("player1", eps, now, cfg)
	if !approxEqual(emotions["anger"], 0.47, 0.01) {
		t.Fatalf("anger = %v, expected ~0.47", emotions["anger"])
	}
	if !approxEqual(emotions["trust"], -0.74, 0.01) {
		t.Fatalf("trust = %v, expected ~-0.74", emotions["trust"])
	}
}

func TestForgiveHalves(t *testing.T) {
	cfg := DefaultConfig()
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)

	eps := []Episode{
		{
			Actor:     "player1",
			EventType: "MINOR_OFFENSE",
			Delta:     map[string]float64{"anger": 0.1},
			Intensity: 0.05,
			Count:     1,
			Harmful:   true,
			FirstAt:   now,
			LastAt:    now,
		},
	}

	// 1st apology
	changed := Forgive(eps, "player1", 0, cfg)
	if changed != 1 {
		t.Fatalf("1st apology changed = %v, expected 1", changed)
	}
	if !approxEqual(eps[0].Forgiven, 0.8, 0.01) {
		t.Fatalf("1st apology Forgiven = %v, expected 0.8", eps[0].Forgiven)
	}

	// 2nd apology (priorApologies = 1)
	changed = Forgive(eps, "player1", 1, cfg)
	if changed != 1 {
		t.Fatalf("2nd apology changed = %v, expected 1", changed)
	}
	if !approxEqual(eps[0].Forgiven, 0.965, 0.01) {
		t.Fatalf("2nd apology Forgiven = %v, expected 0.965", eps[0].Forgiven)
	}

	// 3rd apology (priorApologies = 2) - capped at 0.965, should not lower or exceed
	changed = Forgive(eps, "player1", 2, cfg)
	if changed != 0 {
		t.Fatalf("3rd apology changed = %v, expected 0", changed)
	}
	if !approxEqual(eps[0].Forgiven, 0.965, 0.01) {
		t.Fatalf("3rd apology Forgiven = %v, expected 0.965", eps[0].Forgiven)
	}
}

func TestRevokeForgiveness(t *testing.T) {
	cfg := DefaultConfig()
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)

	eps := []Episode{
		{
			Actor:     "player1",
			EventType: "PLAYER_THREW_STONE",
			Delta:     map[string]float64{"anger": 0.1, "trust": -0.1},
			Intensity: 0.2,
			Count:     5,
			Harmful:   true,
			Forgiven:  0.52938,
			FirstAt:   now,
			LastAt:    now,
		},
	}

	changed := RevokeForgiveness(eps, "player1")
	if changed != 1 {
		t.Fatalf("RevokeForgiveness changed = %v, expected 1", changed)
	}
	if eps[0].Forgiven != 0 {
		t.Fatalf("Forgiven after revoke = %v, expected 0", eps[0].Forgiven)
	}

	emotions := EmotionsToward("player1", eps, now, cfg)
	if !approxEqual(emotions["anger"], 1.0, 0.01) {
		t.Fatalf("anger = %v, expected 1.0", emotions["anger"])
	}
}

func TestHarmfulEscalatesVsNonHarmful(t *testing.T) {
	cfg := DefaultConfig()
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)

	// 5 merged gifts (non-harmful)
	mergedGifts := []Episode{
		{
			Actor:     "player1",
			EventType: "PLAYER_GAVE_GIFT",
			Delta:     map[string]float64{"joy": 0.1},
			Intensity: 0.2,
			Count:     5,
			Harmful:   false,
			FirstAt:   now,
			LastAt:    now,
		},
	}
	emotionsMerged := EmotionsToward("player1", mergedGifts, now, cfg)
	expectedMergedJoy := 0.1 * math.Sqrt(5) // ~0.2236
	if !approxEqual(emotionsMerged["joy"], expectedMergedJoy, 0.01) {
		t.Fatalf("merged joy = %v, expected ~%v", emotionsMerged["joy"], expectedMergedJoy)
	}

	// 5 separate single-gift episodes
	separateGifts := make([]Episode, 5)
	for i := 0; i < 5; i++ {
		separateGifts[i] = Episode{
			Actor:     "player1",
			EventType: "PLAYER_GAVE_GIFT",
			Delta:     map[string]float64{"joy": 0.1},
			Intensity: 0.2,
			Count:     1,
			Harmful:   false,
			FirstAt:   now,
			LastAt:    now,
		}
	}
	emotionsSeparate := EmotionsToward("player1", separateGifts, now, cfg)
	if !approxEqual(emotionsSeparate["joy"], 0.5, 0.01) {
		t.Fatalf("separate joy = %v, expected 0.5", emotionsSeparate["joy"])
	}

	if emotionsMerged["joy"] >= emotionsSeparate["joy"] {
		t.Fatalf("expected merged joy (%v) < separate joy (%v)", emotionsMerged["joy"], emotionsSeparate["joy"])
	}
}

func TestTrustedFriendThrowsStone(t *testing.T) {
	cfg := DefaultConfig()
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)

	eps := []Episode{
		{
			Actor:     "friend",
			EventType: "SAVED_LIFE",
			Delta:     map[string]float64{"trust": 1.0},
			Intensity: 0.5,
			Count:     1,
			Harmful:   false,
			FirstAt:   now,
			LastAt:    now,
		},
		{
			Actor:     "friend",
			EventType: "PLAYER_THREW_STONE",
			Delta:     map[string]float64{"anger": 0.1, "trust": -0.1},
			Intensity: 0.2,
			Count:     1,
			Harmful:   true,
			FirstAt:   now,
			LastAt:    now,
		},
	}

	emotions := EmotionsToward("friend", eps, now, cfg)
	if !approxEqual(emotions["trust"], 0.9, 0.01) {
		t.Fatalf("trust = %v, expected 0.9", emotions["trust"])
	}
}

func TestTenPlayersMaxAngerGeneralMood(t *testing.T) {
	cfg := DefaultConfig()
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)

	actors := []string{"p1", "p2", "p3", "p4", "p5", "p6", "p7", "p8", "p9", "p10"}
	eps := make([]Episode, len(actors))
	for i, actor := range actors {
		eps[i] = Episode{
			Actor:     actor,
			EventType: "ATTACK",
			Delta:     map[string]float64{"anger": 1.0},
			Intensity: 0.8,
			Count:     1,
			Harmful:   true,
			FirstAt:   now,
			LastAt:    now,
		}
	}

	mood := GeneralMood(eps, now, cfg)
	if !approxEqual(mood["anger"], 1.0, 0.01) {
		t.Fatalf("GeneralMood anger = %v, expected 1.0 (clamped from 2.5)", mood["anger"])
	}
}

func TestMergeDecaysOldCount(t *testing.T) {
	cfg := DefaultConfig()
	t0 := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)

	ep := Episode{
		Actor:     "player1",
		EventType: "PLAYER_THREW_STONE",
		Delta:     map[string]float64{"anger": 0.1, "trust": -0.1},
		Intensity: 0.2,
		Count:     5,
		Harmful:   true,
		FirstAt:   t0,
		LastAt:    t0,
	}

	effI := EffectiveIntensity(ep.Intensity, ep.Count)
	hl := HalfLife(effI, cfg)
	now := t0.Add(hl)

	Merge(&ep, now, cfg)

	if !approxEqual(ep.Count, 3.5, 0.01) {
		t.Fatalf("merged Count = %v, expected 3.5", ep.Count)
	}
	if !ep.LastAt.Equal(now) {
		t.Fatalf("merged LastAt = %v, expected %v", ep.LastAt, now)
	}
}

func TestCanMerge(t *testing.T) {
	cfg := DefaultConfig()
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name                     string
		ep                       Episode
		isConversation           bool
		actorHasForgivenEpisodes bool
		expected                 bool
	}{
		{
			name: "fresh harmful repeat",
			ep: Episode{
				Intensity: 0.2,
				Count:     1,
				Harmful:   true,
				LastAt:    now,
			},
			isConversation:           false,
			actorHasForgivenEpisodes: false,
			expected:                 true,
		},
		{
			name: "decay factor below threshold",
			ep: Episode{
				Intensity: 0.2,
				Count:     1,
				Harmful:   true,
				LastAt:    now.Add(-40 * time.Hour), // hl is ~6.1h; 40h -> df = 0.5^(40/6.136) ~ 0.01 < 0.1
			},
			isConversation:           false,
			actorHasForgivenEpisodes: false,
			expected:                 false,
		},
		{
			name: "conversation event",
			ep: Episode{
				Intensity: 0.2,
				Count:     1,
				Harmful:   false,
				LastAt:    now,
			},
			isConversation:           true,
			actorHasForgivenEpisodes: false,
			expected:                 false,
		},
		{
			name: "harmful event when actor has forgiven episodes",
			ep: Episode{
				Intensity: 0.2,
				Count:     1,
				Harmful:   true,
				LastAt:    now,
			},
			isConversation:           false,
			actorHasForgivenEpisodes: true,
			expected:                 false,
		},
		{
			name: "non-harmful event when actor has forgiven episodes",
			ep: Episode{
				Intensity: 0.2,
				Count:     1,
				Harmful:   false,
				LastAt:    now,
			},
			isConversation:           false,
			actorHasForgivenEpisodes: true,
			expected:                 true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := CanMerge(tc.ep, now, cfg, tc.isConversation, tc.actorHasForgivenEpisodes)
			if actual != tc.expected {
				t.Fatalf("CanMerge() = %v, expected %v", actual, tc.expected)
			}
		})
	}
}
