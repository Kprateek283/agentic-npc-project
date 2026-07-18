package dto

import (
	"encoding/json"
	"testing"
)

// TestEventMessageRoundTrip checks that each event shape a client sends survives
// unmarshal, and that omitempty fields absent from the wire stay zero-valued.
func TestEventMessageRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want EventMessage
	}{
		{
			name: "player asks question",
			raw:  `{"event_type":"PLAYER_ASKED_QUESTION","target_npc_name":"Elara","question_text":"Who is the mayor?"}`,
			want: EventMessage{EventType: "PLAYER_ASKED_QUESTION", TargetNpcName: "Elara", QuestionText: "Who is the mayor?"},
		},
		{
			name: "player gives gift",
			raw:  `{"event_type":"PLAYER_GAVE_GIFT","target_npc_name":"Baelor","keyword":"apple"}`,
			want: EventMessage{EventType: "PLAYER_GAVE_GIFT", TargetNpcName: "Baelor", Keyword: "apple"},
		},
		{
			name: "login carries credentials only",
			raw:  `{"event_type":"LOGIN_PLAYER","username":"alice","password":"secret"}`,
			want: EventMessage{EventType: "LOGIN_PLAYER", Username: "alice", Password: "secret"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got EventMessage
			if err := json.Unmarshal([]byte(tt.raw), &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}

			// Re-marshal and unmarshal again: the struct must be a stable round-trip.
			blob, err := json.Marshal(got)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var again EventMessage
			if err := json.Unmarshal(blob, &again); err != nil {
				t.Fatalf("re-unmarshal: %v", err)
			}
			if again != tt.want {
				t.Errorf("round-trip drift: got %+v, want %+v", again, tt.want)
			}
		})
	}
}
