package rules_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"agentic-npc-backend/internal/domain/memory"
	"agentic-npc-backend/internal/domain/rules"
)

func TestLoad_RealEventsFile(t *testing.T) {
	r, err := rules.Load("../../../../gamedata/events.json")
	if err != nil {
		t.Fatalf("unexpected error loading real events.json: %v", err)
	}

	stoneRule, ok := r.Rule("PLAYER_THREW_STONE")
	if !ok {
		t.Fatalf("PLAYER_THREW_STONE not found")
	}
	if !stoneRule.Harmful {
		t.Errorf("expected PLAYER_THREW_STONE to be harmful")
	}
	if stoneRule.Delta["anger"] != 0.1 || stoneRule.Delta["trust"] != -0.1 {
		t.Errorf("unexpected deltas for PLAYER_THREW_STONE: %v", stoneRule.Delta)
	}

	apologyRule, ok := r.Rule("PLAYER_APOLOGIZED")
	if !ok {
		t.Fatalf("PLAYER_APOLOGIZED not found")
	}
	if !apologyRule.Apology {
		t.Errorf("expected PLAYER_APOLOGIZED to have Apology=true")
	}
	if len(apologyRule.Delta) != 0 {
		t.Errorf("expected PLAYER_APOLOGIZED to have no delta, got: %v", apologyRule.Delta)
	}

	questionRule, ok := r.Rule("PLAYER_ASKED_QUESTION")
	if !ok {
		t.Fatalf("PLAYER_ASKED_QUESTION not found")
	}
	if !questionRule.Conversation {
		t.Errorf("expected PLAYER_ASKED_QUESTION to have Conversation=true")
	}

	expectedCfg := memory.DefaultConfig()
	if r.Config.Spillover != expectedCfg.Spillover {
		t.Errorf("Spillover mismatch: got %v, want %v", r.Config.Spillover, expectedCfg.Spillover)
	}
	if r.Config.EscalationExp != expectedCfg.EscalationExp {
		t.Errorf("EscalationExp mismatch: got %v, want %v", r.Config.EscalationExp, expectedCfg.EscalationExp)
	}
	if r.Config.DiminishExp != expectedCfg.DiminishExp {
		t.Errorf("DiminishExp mismatch: got %v, want %v", r.Config.DiminishExp, expectedCfg.DiminishExp)
	}
	if r.Config.MinHalfLife != expectedCfg.MinHalfLife {
		t.Errorf("MinHalfLife mismatch: got %v, want %v", r.Config.MinHalfLife, expectedCfg.MinHalfLife)
	}
	if r.Config.MaxHalfLifeFactor != expectedCfg.MaxHalfLifeFactor {
		t.Errorf("MaxHalfLifeFactor mismatch: got %v, want %v", r.Config.MaxHalfLifeFactor, expectedCfg.MaxHalfLifeFactor)
	}
	if r.Config.MergeThreshold != expectedCfg.MergeThreshold {
		t.Errorf("MergeThreshold mismatch: got %v, want %v", r.Config.MergeThreshold, expectedCfg.MergeThreshold)
	}
	if r.Config.NotabilityThreshold != expectedCfg.NotabilityThreshold {
		t.Errorf("NotabilityThreshold mismatch: got %v, want %v", r.Config.NotabilityThreshold, expectedCfg.NotabilityThreshold)
	}
	if r.Config.ForgiveStep != expectedCfg.ForgiveStep {
		t.Errorf("ForgiveStep mismatch: got %v, want %v", r.Config.ForgiveStep, expectedCfg.ForgiveStep)
	}
	if r.Config.ForgiveCap != expectedCfg.ForgiveCap {
		t.Errorf("ForgiveCap mismatch: got %v, want %v", r.Config.ForgiveCap, expectedCfg.ForgiveCap)
	}
	if r.Config.BetrayalTrust != expectedCfg.BetrayalTrust {
		t.Errorf("BetrayalTrust mismatch: got %v, want %v", r.Config.BetrayalTrust, expectedCfg.BetrayalTrust)
	}
	if !reflect.DeepEqual(r.Config.ForgiveWeights, expectedCfg.ForgiveWeights) {
		t.Errorf("ForgiveWeights mismatch: got %v, want %v", r.Config.ForgiveWeights, expectedCfg.ForgiveWeights)
	}
	if !reflect.DeepEqual(r.Config.SignedEmotions, expectedCfg.SignedEmotions) {
		t.Errorf("SignedEmotions mismatch: got %v, want %v", r.Config.SignedEmotions, expectedCfg.SignedEmotions)
	}
}

func TestLoad_ValidationErrors(t *testing.T) {
	validConfigJSON := `"_config": {
		"spillover": 0.25,
		"escalation_exp": 1.5,
		"diminish_exp": 0.5,
		"min_half_life_hours": 1,
		"max_half_life_factor": 8760,
		"merge_threshold": 0.1,
		"notability_threshold": 0.3,
		"forgive_step": 0.8,
		"forgive_cap": 0.7,
		"betrayal_trust": -0.2,
		"forgive_weights": {"anger": 1.0, "joy": 1.0, "sadness": 0.7, "trust": 0.5, "fear": 0.2},
		"signed_emotions": ["trust"]
	}`

	tests := []struct {
		name        string
		filename    string
		content     string
		isMissing   bool
		expectedErr string
	}{
		{
			name:        "missing file",
			isMissing:   true,
			filename:    "nonexistent.json",
			expectedErr: "nonexistent.json",
		},
		{
			name:        "malformed JSON",
			filename:    "malformed.json",
			content:     `{ invalid json }`,
			expectedErr: "malformed.json",
		},
		{
			name:        "missing _config",
			filename:    "missing_config.json",
			content:     `{"SOME_EVENT": {"intensity": 0.5, "delta": {"joy": 0.1}}}`,
			expectedErr: "_config",
		},
		{
			name:     "intensity 1.5",
			filename: "invalid_intensity.json",
			content: "{\n" + validConfigJSON + ",\n" +
				`"TEST_EVENT": {"intensity": 1.5, "delta": {"joy": 0.1}}` + "\n}",
			expectedErr: "intensity",
		},
		{
			name:     "delta of 2.0",
			filename: "invalid_delta.json",
			content: "{\n" + validConfigJSON + ",\n" +
				`"TEST_EVENT": {"intensity": 0.5, "delta": {"joy": 2.0}}` + "\n}",
			expectedErr: "delta",
		},
		{
			name:     "apology carrying a delta",
			filename: "apology_with_delta.json",
			content: "{\n" + validConfigJSON + ",\n" +
				`"PLAYER_APOLOGIZED": {"intensity": 0.1, "apology": true, "delta": {"joy": 0.1}}` + "\n}",
			expectedErr: "apology",
		},
		{
			name:     "harmful event with no delta",
			filename: "harmful_no_delta.json",
			content: "{\n" + validConfigJSON + ",\n" +
				`"PLAYER_ATTACKED": {"intensity": 0.9, "harmful": true}` + "\n}",
			expectedErr: "harmful",
		},
		{
			name:     "spillover 5",
			filename: "invalid_spillover.json",
			content: `{
				"_config": {
					"spillover": 5.0,
					"escalation_exp": 1.5,
					"diminish_exp": 0.5,
					"min_half_life_hours": 1,
					"max_half_life_factor": 8760,
					"merge_threshold": 0.1,
					"notability_threshold": 0.3,
					"forgive_step": 0.8,
					"forgive_cap": 0.7,
					"betrayal_trust": -0.2,
					"forgive_weights": {"anger": 1.0},
					"signed_emotions": ["trust"]
				}
			}`,
			expectedErr: "spillover",
		},
		{
			name:     "_config without forgive_weights",
			filename: "missing_forgive_weights.json",
			content: `{
				"_config": {
					"spillover": 0.25,
					"escalation_exp": 1.5,
					"diminish_exp": 0.5,
					"min_half_life_hours": 1,
					"max_half_life_factor": 8760,
					"merge_threshold": 0.1,
					"notability_threshold": 0.3,
					"forgive_step": 0.8,
					"forgive_cap": 0.7,
					"betrayal_trust": -0.2,
					"signed_emotions": ["trust"]
				}
			}`,
			expectedErr: "forgive_weights",
		},
		{
			name:     "_config without betrayal_trust",
			filename: "missing_betrayal_trust.json",
			content: `{
				"_config": {
					"spillover": 0.25,
					"escalation_exp": 1.5,
					"diminish_exp": 0.5,
					"min_half_life_hours": 1,
					"max_half_life_factor": 8760,
					"merge_threshold": 0.1,
					"notability_threshold": 0.3,
					"forgive_step": 0.8,
					"forgive_cap": 0.7,
					"forgive_weights": {"anger": 1.0},
					"signed_emotions": ["trust"]
				}
			}`,
			expectedErr: "betrayal_trust",
		},
	}

	tempDir := t.TempDir()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			filePath := filepath.Join(tempDir, tc.filename)
			if !tc.isMissing {
				if err := os.WriteFile(filePath, []byte(tc.content), 0644); err != nil {
					t.Fatalf("failed to write test file: %v", err)
				}
			}

			_, err := rules.Load(filePath)
			if err == nil {
				t.Fatalf("expected error for case %q, got nil", tc.name)
			}

			if !strings.Contains(err.Error(), tc.expectedErr) {
				t.Errorf("expected error message to contain %q, got %q", tc.expectedErr, err.Error())
			}
		})
	}
}
