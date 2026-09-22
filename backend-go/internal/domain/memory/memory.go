package memory

import (
	"math"
	"time"
)

// Episode represents a remembered event and its emotional impact.
type Episode struct {
	Actor     string
	EventType string
	Subject   string
	Delta     map[string]float64
	Intensity float64
	Count     float64
	Harmful   bool
	Forgiven  float64
	Betrayal  bool
	FirstAt   time.Time
	LastAt    time.Time
	Text      string
}

// Config defines parameters governing emotional scaling, decay, and forgiveness.
type Config struct {
	Spillover           float64
	EscalationExp       float64
	DiminishExp         float64
	MinHalfLife         time.Duration
	MaxHalfLifeFactor   float64
	MergeThreshold      float64
	NotabilityThreshold float64
	ForgiveStep         float64
	ForgiveCap          float64
	BetrayalTrust       float64
	ForgiveWeights      map[string]float64
	SignedEmotions      map[string]bool
}

// DefaultConfig returns standard baseline parameters for NPC emotion calculation.
func DefaultConfig() Config {
	return Config{
		Spillover:           0.25,
		EscalationExp:       1.5,
		DiminishExp:         0.5,
		MinHalfLife:         time.Hour,
		MaxHalfLifeFactor:   8760,
		MergeThreshold:      0.1,
		NotabilityThreshold: 0.3,
		ForgiveStep:         0.8,
		ForgiveCap:          0.7,
		BetrayalTrust:       -0.2,
		ForgiveWeights: map[string]float64{
			"anger":   1.0,
			"joy":     1.0,
			"sadness": 0.7,
			"trust":   0.5,
			"fear":    0.2,
		},
		SignedEmotions: map[string]bool{
			"trust": true,
		},
	}
}

// EffectiveIntensity computes cumulative intensity across repeated occurrences of an event.
func EffectiveIntensity(i0, n float64) float64 {
	return 1.0 - math.Pow(1.0-i0, n)
}

// HalfLife computes the retention half-life based on event intensity.
func HalfLife(intensity float64, cfg Config) time.Duration {
	return time.Duration(float64(cfg.MinHalfLife) * math.Pow(cfg.MaxHalfLifeFactor, intensity))
}

// DecayFactor computes the remaining emotional retention since the last occurrence.
func DecayFactor(e Episode, now time.Time, cfg Config) float64 {
	dt := now.Sub(e.LastAt)
	if dt <= 0 {
		return 1.0
	}
	hl := HalfLife(EffectiveIntensity(e.Intensity, e.Count), cfg)
	if hl <= 0 {
		return 0.0
	}
	return math.Pow(0.5, float64(dt)/float64(hl))
}

// Escalation computes the repetition multiplier for an episode.
func Escalation(e Episode, cfg Config) float64 {
	if e.Count <= 0 {
		return 0.0
	}
	if e.Harmful {
		return math.Pow(e.Count, cfg.EscalationExp)
	}
	return math.Pow(e.Count, cfg.DiminishExp)
}

// Contribution calculates the decayed and forgiven emotional delta for a specific emotion.
func Contribution(e Episode, emotion string, now time.Time, cfg Config) float64 {
	delta, ok := e.Delta[emotion]
	if !ok {
		return 0.0
	}

	scaled := delta * Escalation(e, cfg)
	if scaled > 1.0 {
		scaled = 1.0
	} else if scaled < -1.0 {
		scaled = -1.0
	}

	weight := 0.0
	if cfg.ForgiveWeights != nil {
		weight = cfg.ForgiveWeights[emotion]
	}

	forgiveFactor := 1.0 - e.Forgiven*weight
	return scaled * forgiveFactor * DecayFactor(e, now, cfg)
}

// EmotionsToward calculates net emotional stances toward a specific actor.
func EmotionsToward(actor string, eps []Episode, now time.Time, cfg Config) map[string]float64 {
	emotions := make(map[string]struct{})
	for _, e := range eps {
		if e.Actor == actor {
			for em := range e.Delta {
				emotions[em] = struct{}{}
			}
		}
	}

	result := make(map[string]float64, len(emotions))
	for em := range emotions {
		var sumPos, sumNeg float64
		for _, e := range eps {
			if e.Actor != actor {
				continue
			}
			c := Contribution(e, em, now, cfg)
			if c > 0 {
				sumPos += c
			} else if c < 0 {
				sumNeg += -c
			}
		}

		pos := math.Min(1.0, sumPos)
		neg := math.Min(1.0, sumNeg)
		val := pos - neg

		if cfg.SignedEmotions != nil && cfg.SignedEmotions[em] {
			if val > 1.0 {
				val = 1.0
			} else if val < -1.0 {
				val = -1.0
			}
		} else {
			if val > 1.0 {
				val = 1.0
			} else if val < 0.0 {
				val = 0.0
			}
		}

		result[em] = val
	}

	return result
}

// GeneralMood computes diffuse ambient emotions aggregated across all known actors.
func GeneralMood(eps []Episode, now time.Time, cfg Config) map[string]float64 {
	actors := make(map[string]struct{})
	for _, e := range eps {
		actors[e.Actor] = struct{}{}
	}

	totals := make(map[string]float64)
	for actor := range actors {
		actorEmotions := EmotionsToward(actor, eps, now, cfg)
		for em, val := range actorEmotions {
			totals[em] += val
		}
	}

	result := make(map[string]float64, len(totals))
	for em, total := range totals {
		val := total * cfg.Spillover
		if cfg.SignedEmotions != nil && cfg.SignedEmotions[em] {
			if val > 1.0 {
				val = 1.0
			} else if val < -1.0 {
				val = -1.0
			}
		} else {
			if val > 1.0 {
				val = 1.0
			} else if val < 0.0 {
				val = 0.0
			}
		}
		result[em] = val
	}

	return result
}

// Weight determines the peak emotional salience of an episode across all affected emotions.
func Weight(e Episode, now time.Time, cfg Config) float64 {
	maxW := 0.0
	for em := range e.Delta {
		c := math.Abs(Contribution(e, em, now, cfg))
		if c > maxW {
			maxW = c
		}
	}
	return maxW
}

// CanMerge checks whether an incoming event can be combined into an existing episode.
func CanMerge(e Episode, now time.Time, cfg Config, isConversation, actorHasForgivenEpisodes bool) bool {
	if isConversation {
		return false
	}
	if actorHasForgivenEpisodes && e.Harmful {
		return false
	}
	return DecayFactor(e, now, cfg) >= cfg.MergeThreshold
}

// Merge incorporates a new event recurrence into an existing episode after decaying prior counts.
func Merge(e *Episode, now time.Time, cfg Config) {
	df := DecayFactor(*e, now, cfg)
	e.Count = e.Count*df + 1.0
	e.LastAt = now
}

// Forgive increments the forgiveness level of an actor's harmful episodes up to their severity cap.
func Forgive(eps []Episode, actor string, priorApologies int, cfg Config) int {
	changed := 0
	step := cfg.ForgiveStep * math.Pow(0.5, float64(priorApologies))
	for i := range eps {
		if eps[i].Actor != actor || !eps[i].Harmful {
			continue
		}
		effInt := EffectiveIntensity(eps[i].Intensity, eps[i].Count)
		capVal := 1.0 - cfg.ForgiveCap*effInt
		newF := math.Max(eps[i].Forgiven, math.Min(eps[i].Forgiven+step, capVal))
		if newF > eps[i].Forgiven {
			eps[i].Forgiven = newF
			changed++
		}
	}
	return changed
}

// RevokeForgiveness resets forgiveness on all episodes associated with an actor.
func RevokeForgiveness(eps []Episode, actor string) int {
	changed := 0
	for i := range eps {
		if eps[i].Actor == actor && eps[i].Forgiven > 0 {
			eps[i].Forgiven = 0
			changed++
		}
	}
	return changed
}
