// Package rules provides configuration and event definitions for NPC emotional reactions.
package rules

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"agentic-npc-backend/internal/domain/memory"
)

// EventRule describes the emotional and behavioral effects of a game event.
type EventRule struct {
	Delta        map[string]float64
	Intensity    float64
	Harmful      bool
	Apology      bool
	Conversation bool
	// Memory is how the NPC remembers the event, written after the actor's id, e.g.
	// "threw a stone at me". Empty falls back to naming the raw event type.
	Memory string
}

// Rules contains memory calculation settings and event definitions.
type Rules struct {
	Config memory.Config
	Events map[string]EventRule
}

type rawConfigFile struct {
	Spillover           *float64           `json:"spillover"`
	EscalationExp       *float64           `json:"escalation_exp"`
	DiminishExp         *float64           `json:"diminish_exp"`
	MinHalfLifeHours    *float64           `json:"min_half_life_hours"`
	MaxHalfLifeFactor   *float64           `json:"max_half_life_factor"`
	MergeThreshold      *float64           `json:"merge_threshold"`
	NotabilityThreshold *float64           `json:"notability_threshold"`
	ForgiveStep         *float64           `json:"forgive_step"`
	ForgiveCap          *float64           `json:"forgive_cap"`
	BetrayalTrust       *float64           `json:"betrayal_trust"`
	ForgiveWeights      map[string]float64 `json:"forgive_weights"`
	SignedEmotions      []string           `json:"signed_emotions"`
}

type rawEventRule struct {
	Delta        map[string]float64 `json:"delta"`
	Intensity    float64            `json:"intensity"`
	Harmful      bool               `json:"harmful"`
	Apology      bool               `json:"apology"`
	Conversation bool               `json:"conversation"`
	Memory       string             `json:"memory"`
}

// Load parses and validates game event rules and memory configuration from the specified JSON file.
func Load(path string) (*Rules, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("rules %s: %w", path, err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("rules %s: %w", path, err)
	}

	configRaw, ok := raw["_config"]
	if !ok {
		return nil, fmt.Errorf("rules %s: missing key %q", path, "_config")
	}

	var rawCfg rawConfigFile
	if err := json.Unmarshal(configRaw, &rawCfg); err != nil {
		return nil, fmt.Errorf("rules %s: _config: %w", path, err)
	}

	if rawCfg.Spillover == nil || *rawCfg.Spillover < 0 || *rawCfg.Spillover > 1 {
		return nil, fmt.Errorf("rules %s: _config key %q must be within 0..1", path, "spillover")
	}
	if rawCfg.MergeThreshold == nil || *rawCfg.MergeThreshold < 0 || *rawCfg.MergeThreshold > 1 {
		return nil, fmt.Errorf("rules %s: _config key %q must be within 0..1", path, "merge_threshold")
	}
	if rawCfg.NotabilityThreshold == nil || *rawCfg.NotabilityThreshold < 0 || *rawCfg.NotabilityThreshold > 1 {
		return nil, fmt.Errorf("rules %s: _config key %q must be within 0..1", path, "notability_threshold")
	}
	if rawCfg.ForgiveStep == nil || *rawCfg.ForgiveStep < 0 || *rawCfg.ForgiveStep > 1 {
		return nil, fmt.Errorf("rules %s: _config key %q must be within 0..1", path, "forgive_step")
	}
	if rawCfg.ForgiveCap == nil || *rawCfg.ForgiveCap < 0 || *rawCfg.ForgiveCap > 1 {
		return nil, fmt.Errorf("rules %s: _config key %q must be within 0..1", path, "forgive_cap")
	}
	if rawCfg.EscalationExp == nil || *rawCfg.EscalationExp <= 0 {
		return nil, fmt.Errorf("rules %s: _config key %q must be > 0", path, "escalation_exp")
	}
	if rawCfg.DiminishExp == nil || *rawCfg.DiminishExp <= 0 {
		return nil, fmt.Errorf("rules %s: _config key %q must be > 0", path, "diminish_exp")
	}
	if rawCfg.MinHalfLifeHours == nil || *rawCfg.MinHalfLifeHours <= 0 {
		return nil, fmt.Errorf("rules %s: _config key %q must be > 0", path, "min_half_life_hours")
	}
	if rawCfg.MaxHalfLifeFactor == nil || *rawCfg.MaxHalfLifeFactor <= 1 {
		return nil, fmt.Errorf("rules %s: _config key %q must be > 1", path, "max_half_life_factor")
	}
	if len(rawCfg.ForgiveWeights) == 0 {
		return nil, fmt.Errorf("rules %s: _config key %q must be present and non-empty", path, "forgive_weights")
	}
	for em, w := range rawCfg.ForgiveWeights {
		if w < 0 || w > 1 {
			return nil, fmt.Errorf("rules %s: _config forgive_weights key %q must be within 0..1", path, em)
		}
	}
	if rawCfg.BetrayalTrust == nil || *rawCfg.BetrayalTrust < -1 || *rawCfg.BetrayalTrust > 0 {
		return nil, fmt.Errorf("rules %s: _config key %q must be within -1..0", path, "betrayal_trust")
	}

	signedEmotions := make(map[string]bool, len(rawCfg.SignedEmotions))
	for _, em := range rawCfg.SignedEmotions {
		signedEmotions[em] = true
	}

	cfg := memory.Config{
		Spillover:           *rawCfg.Spillover,
		EscalationExp:       *rawCfg.EscalationExp,
		DiminishExp:         *rawCfg.DiminishExp,
		MinHalfLife:         time.Duration(*rawCfg.MinHalfLifeHours * float64(time.Hour)),
		MaxHalfLifeFactor:   *rawCfg.MaxHalfLifeFactor,
		MergeThreshold:      *rawCfg.MergeThreshold,
		NotabilityThreshold: *rawCfg.NotabilityThreshold,
		ForgiveStep:         *rawCfg.ForgiveStep,
		ForgiveCap:          *rawCfg.ForgiveCap,
		BetrayalTrust:       *rawCfg.BetrayalTrust,
		ForgiveWeights:      rawCfg.ForgiveWeights,
		SignedEmotions:      signedEmotions,
	}

	events := make(map[string]EventRule, len(raw)-1)
	for key, rawVal := range raw {
		if key == "_config" {
			continue
		}

		var er rawEventRule
		if err := json.Unmarshal(rawVal, &er); err != nil {
			return nil, fmt.Errorf("rules %s: event %q: %w", path, key, err)
		}

		if er.Intensity < 0 || er.Intensity > 1 {
			return nil, fmt.Errorf("rules %s: event %q: key %q must be within 0..1", path, key, "intensity")
		}
		for em, deltaVal := range er.Delta {
			if deltaVal < -1 || deltaVal > 1 {
				return nil, fmt.Errorf("rules %s: event %q: delta key %q must be within -1..1", path, key, em)
			}
		}
		if er.Apology && len(er.Delta) > 0 {
			return nil, fmt.Errorf("rules %s: event %q: apology event must have no delta", path, key)
		}
		if er.Harmful && len(er.Delta) == 0 {
			return nil, fmt.Errorf("rules %s: event %q: harmful event must have non-empty delta", path, key)
		}
		if er.Apology && er.Conversation {
			return nil, fmt.Errorf("rules %s: event %q: apology and conversation cannot both be true", path, key)
		}

		events[key] = EventRule{
			Delta:        er.Delta,
			Intensity:    er.Intensity,
			Harmful:      er.Harmful,
			Apology:      er.Apology,
			Conversation: er.Conversation,
			Memory:       er.Memory,
		}
	}

	return &Rules{
		Config: cfg,
		Events: events,
	}, nil
}

// Rule returns the event rule for the specified event type, if found.
func (r *Rules) Rule(eventType string) (EventRule, bool) {
	if r == nil || r.Events == nil {
		return EventRule{}, false
	}
	rule, ok := r.Events[eventType]
	return rule, ok
}
