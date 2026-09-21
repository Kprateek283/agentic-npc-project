package handlers

import "testing"

func TestOriginAllowed(t *testing.T) {
	tests := []struct {
		name    string
		origin  string
		host    string
		allowed []string
		want    bool
	}{
		// Rule 1: origin == "" -> true (non-browser clients send no Origin header)
		{
			name:    "empty origin allowed without allowed list",
			origin:  "",
			host:    "localhost:8080",
			allowed: nil,
			want:    true,
		},
		{
			name:    "empty origin allowed with allowed list",
			origin:  "",
			host:    "localhost:8080",
			allowed: []string{"http://example.com"},
			want:    true,
		},

		// Rule 2: allowed contains "*" or contains origin exactly -> true
		{
			name:    "wildcard in allowed list allows any origin",
			origin:  "http://evil.com",
			host:    "localhost:8080",
			allowed: []string{"*"},
			want:    true,
		},
		{
			name:    "exact match in allowed list",
			origin:  "http://localhost:5500",
			host:    "localhost:8080",
			allowed: []string{"http://localhost:5500", "http://example.com"},
			want:    true,
		},
		{
			name:    "null origin allowed when explicitly in allowed list",
			origin:  "null",
			host:    "localhost:8080",
			allowed: []string{"null"},
			want:    true,
		},
		{
			name:    "null origin allowed when wildcard in allowed list",
			origin:  "null",
			host:    "localhost:8080",
			allowed: []string{"*"},
			want:    true,
		},

		// Rule 3: allowed is empty -> same-origin only: parse origin with net/url; true only if Host non-empty and equals host (case-insensitive).
		{
			name:    "empty allowed same origin match",
			origin:  "http://localhost:8080",
			host:    "localhost:8080",
			allowed: []string{},
			want:    true,
		},
		{
			name:    "empty allowed same origin match case insensitive",
			origin:  "http://LOCALHOST:8080",
			host:    "localhost:8080",
			allowed: []string{},
			want:    true,
		},
		{
			name:    "nil allowed same origin match",
			origin:  "http://localhost:8080",
			host:    "localhost:8080",
			allowed: nil,
			want:    true,
		},
		{
			name:    "empty allowed different host rejected",
			origin:  "http://localhost:3000",
			host:    "localhost:8080",
			allowed: []string{},
			want:    false,
		},
		{
			name:    "empty allowed null origin rejected",
			origin:  "null",
			host:    "localhost:8080",
			allowed: []string{},
			want:    false,
		},
		{
			name:    "empty allowed unparseable origin rejected",
			origin:  "://invalid-url",
			host:    "localhost:8080",
			allowed: []string{},
			want:    false,
		},

		// Rule 4: otherwise false
		{
			name:    "not matching allowed list rejected",
			origin:  "http://unknown.com",
			host:    "localhost:8080",
			allowed: []string{"http://localhost:5500"},
			want:    false,
		},
		{
			name:    "null origin rejected when allowed list has other entries",
			origin:  "null",
			host:    "localhost:8080",
			allowed: []string{"http://localhost:5500"},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := originAllowed(tt.origin, tt.host, tt.allowed); got != tt.want {
				t.Errorf("originAllowed(%q, %q, %v) = %v, want %v", tt.origin, tt.host, tt.allowed, got, tt.want)
			}
		})
	}
}
