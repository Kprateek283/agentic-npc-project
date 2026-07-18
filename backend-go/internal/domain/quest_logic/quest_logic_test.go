package quest_logic

import "testing"

func TestTrustMet(t *testing.T) {
	tests := []struct {
		name     string
		level    float64
		operator string
		value    float64
		want     bool
	}{
		{"gte satisfied", 0.5, ">=", 0.5, true},
		{"gte below", 0.49, ">=", 0.5, false},
		{"greater_than satisfied", 0.6, "GREATER_THAN", 0.5, true},
		{"greater_than equal is not greater", 0.5, "GREATER_THAN", 0.5, false},
		{"unknown operator fails closed", 0.9, "LESS_THAN", 0.5, false},
		{"missing operator fails closed", 0.9, "", 0.5, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := trustMet(tt.level, tt.operator, tt.value); got != tt.want {
				t.Errorf("trustMet(%v, %q, %v) = %v, want %v", tt.level, tt.operator, tt.value, got, tt.want)
			}
		})
	}
}

func TestGetItemDefinition(t *testing.T) {
	qm := &QuestManager{Items: map[string]ItemDefinition{
		"apple": {ItemID: "apple", Name: "Apple"},
	}}

	got, err := qm.getItemDefinition("apple")
	if err != nil {
		t.Fatalf("unexpected error for known item: %v", err)
	}
	if got.Name != "Apple" {
		t.Errorf("got name %q, want Apple", got.Name)
	}

	if _, err := qm.getItemDefinition("dragon_egg"); err == nil {
		t.Error("expected error for unknown item, got nil")
	}
}

// TestNewQuestManagerLoadsGamedata reads the real gamedata directory (file access only,
// no DB/Redis) and asserts the static definitions load.
func TestNewQuestManagerLoadsGamedata(t *testing.T) {
	qm, err := NewQuestManager("../../../gamedata")
	if err != nil {
		t.Fatalf("NewQuestManager: %v", err)
	}
	if len(qm.Items) == 0 {
		t.Error("expected item definitions to load, got none")
	}
	if _, ok := qm.Items["apple"]; !ok {
		t.Error("expected 'apple' item to be loaded from gamedata")
	}
}
