package config

import (
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name              string
		env               map[string]string
		wantErr           bool
		wantAICallTimeout time.Duration
		wantLLMRateLimit  int
		wantLLMRateWindow time.Duration
	}{
		{
			name: "defaults",
			env: map[string]string{
				"POSTGRES_DSN": "postgres://dummy:dummy@localhost:5432/db",
			},
			wantErr:           false,
			wantAICallTimeout: 40 * time.Second,
			wantLLMRateLimit:  20,
			wantLLMRateWindow: 60 * time.Second,
		},
		{
			name: "valid overrides",
			env: map[string]string{
				"POSTGRES_DSN":            "postgres://dummy:dummy@localhost:5432/db",
				"AI_CALL_TIMEOUT":         "30",
				"LLM_RATE_LIMIT":          "10",
				"LLM_RATE_WINDOW_SECONDS": "120",
			},
			wantErr:           false,
			wantAICallTimeout: 30 * time.Second,
			wantLLMRateLimit:  10,
			wantLLMRateWindow: 120 * time.Second,
		},
		{
			name: "invalid AI_CALL_TIMEOUT non-integer",
			env: map[string]string{
				"POSTGRES_DSN":    "postgres://dummy:dummy@localhost:5432/db",
				"AI_CALL_TIMEOUT": "abc",
			},
			wantErr: true,
		},
		{
			name: "invalid AI_CALL_TIMEOUT zero",
			env: map[string]string{
				"POSTGRES_DSN":    "postgres://dummy:dummy@localhost:5432/db",
				"AI_CALL_TIMEOUT": "0",
			},
			wantErr: true,
		},
		{
			name: "invalid LLM_RATE_LIMIT negative",
			env: map[string]string{
				"POSTGRES_DSN":   "postgres://dummy:dummy@localhost:5432/db",
				"LLM_RATE_LIMIT": "-1",
			},
			wantErr: true,
		},
		{
			name: "invalid LLM_RATE_WINDOW_SECONDS non-integer",
			env: map[string]string{
				"POSTGRES_DSN":            "postgres://dummy:dummy@localhost:5432/db",
				"LLM_RATE_WINDOW_SECONDS": "x",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("POSTGRES_DSN", "")
			t.Setenv("REDIS_ADDR", "")
			t.Setenv("SERVER_PORT", "")
			t.Setenv("AI_SERVICE_ADDR", "")
			t.Setenv("AI_CALL_TIMEOUT", "")
			t.Setenv("GAMEDATA_DIR", "")
			t.Setenv("ADMIN_ENABLED", "")
			t.Setenv("ALLOWED_ORIGINS", "")
			t.Setenv("LLM_RATE_LIMIT", "")
			t.Setenv("LLM_RATE_WINDOW_SECONDS", "")

			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			cfg, err := Load()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Load() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if cfg.AICallTimeout != tt.wantAICallTimeout {
				t.Errorf("cfg.AICallTimeout = %v, want %v", cfg.AICallTimeout, tt.wantAICallTimeout)
			}
			if cfg.LLMRateLimit != tt.wantLLMRateLimit {
				t.Errorf("cfg.LLMRateLimit = %v, want %v", cfg.LLMRateLimit, tt.wantLLMRateLimit)
			}
			if cfg.LLMRateWindow != tt.wantLLMRateWindow {
				t.Errorf("cfg.LLMRateWindow = %v, want %v", cfg.LLMRateWindow, tt.wantLLMRateWindow)
			}
		})
	}
}
