package user_logic

import (
	"context"
	"strings"
	"testing"

	"agentic-npc-backend/internal/db/ent"
	"agentic-npc-backend/internal/db/ent/enttest"
	entplayer "agentic-npc-backend/internal/db/ent/player"

	_ "github.com/mattn/go-sqlite3"
)

// Hashes of "lantern-oath-1" made outside the code under test. Cost 14 is what the service
// used before d783cf5 switched to bcrypt.DefaultCost (10), so rows written then carry it.
// Verifying the cost-14 hash takes about a second; it is the only slow case.
const (
	legacyPassword = "lantern-oath-1"
	legacyCost14   = "$2a$14$zbVkkbA2etHcLnXYxwiVReD1KT1jAOau7qeiPNc55ohPF28t2xEXS"
	legacyCost4    = "$2a$04$pbvf1vwX59p79hvcRWwsDuSydjPyvE6kcpxH27755eG53dJd.5xOW"
)

func openDB(t *testing.T) (context.Context, *ent.Client) {
	t.Helper()
	db := enttest.Open(t, "sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared&_fk=1")
	t.Cleanup(func() { db.Close() })
	return context.Background(), db
}

// seedHash stores a player with a ready-made hash, as an older version of the service would have.
func seedHash(t *testing.T, ctx context.Context, db *ent.Client, username, hash string) {
	t.Helper()
	if _, err := db.Player.Create().SetPlayerID(username).SetPlayerName(username).SetPassword(hash).Save(ctx); err != nil {
		t.Fatalf("seed %s: %v", username, err)
	}
}

func playerCount(t *testing.T, ctx context.Context, db *ent.Client) int {
	t.Helper()
	n, err := db.Player.Query().Count(ctx)
	if err != nil {
		t.Fatalf("count players: %v", err)
	}
	return n
}

func TestPasswords(t *testing.T) {
	type step struct {
		op       string // "register" or "login"
		user     string
		password string
		wantErr  string // "" means success; otherwise a substring of the error
	}
	cases := []struct {
		name        string
		seed        map[string]string // username -> stored hash
		steps       []step
		wantPlayers int
	}{
		{
			name: "a registered password logs in",
			steps: []step{
				{"register", "alice", "correct horse", ""},
				{"login", "alice", "correct horse", ""},
			},
			wantPlayers: 1,
		},
		{
			name: "a wrong password is refused",
			steps: []step{
				{"register", "alice", "correct horse", ""},
				{"login", "alice", "correct horse ", "invalid password"},
				{"login", "alice", "Correct horse", "invalid password"},
				{"login", "alice", "", "invalid password"},
			},
			wantPlayers: 1,
		},
		{
			name: "a second registration with the same name is refused and the first password still works",
			steps: []step{
				{"register", "alice", "first secret", ""},
				{"register", "alice", "second secret", "already exists"},
				{"login", "alice", "second secret", "invalid password"},
				{"login", "alice", "first secret", ""},
			},
			wantPlayers: 1,
		},
		{
			name: "usernames are case-sensitive, so two players can differ only by case",
			steps: []step{
				{"register", "alice", "lower", ""},
				{"register", "Alice", "upper", ""},
				{"login", "Alice", "lower", "invalid password"},
				{"login", "Alice", "upper", ""},
			},
			wantPlayers: 2,
		},
		{
			name:        "logging in as an unknown player fails",
			steps:       []step{{"login", "nobody", "anything", "not found"}},
			wantPlayers: 0,
		},
		{
			name:        "an empty username cannot be registered",
			steps:       []step{{"register", "", "secret", "failed to create new player"}},
			wantPlayers: 0,
		},
		{
			name: "a password longer than bcrypt's 72 bytes is refused at registration",
			steps: []step{
				{"register", "alice", strings.Repeat("x", 73), "failed to hash password"},
			},
			wantPlayers: 0,
		},
		{
			name: "an empty password is accepted and then required",
			steps: []step{
				{"register", "alice", "", ""},
				{"login", "alice", "", ""},
				{"login", "alice", "x", "invalid password"},
			},
			wantPlayers: 1,
		},
		{
			name: "only the first 72 bytes of a password count at login",
			steps: []step{
				{"register", "alice", strings.Repeat("x", 72), ""},
				{"login", "alice", strings.Repeat("x", 72) + "anything", ""},
				{"login", "alice", strings.Repeat("x", 71), "invalid password"},
			},
			wantPlayers: 1,
		},
		{
			name:        "a hash made at the older cost 14 still verifies",
			seed:        map[string]string{"veteran": legacyCost14},
			steps:       []step{{"login", "veteran", legacyPassword, ""}},
			wantPlayers: 1,
		},
		{
			name: "any cost is read from the stored hash, and a wrong password fails against it",
			seed: map[string]string{"veteran": legacyCost4},
			steps: []step{
				{"login", "veteran", legacyPassword, ""},
				{"login", "veteran", "lantern-oath-2", "invalid password"},
			},
			wantPlayers: 1,
		},
		{
			name:        "a row holding a plain-text password cannot log in with it",
			seed:        map[string]string{"legacy": "lantern-oath-1"},
			steps:       []step{{"login", "legacy", "lantern-oath-1", "invalid password"}},
			wantPlayers: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, db := openDB(t)
			for user, hash := range tc.seed {
				seedHash(t, ctx, db, user, hash)
			}
			for i, s := range tc.steps {
				var p *ent.Player
				var err error
				switch s.op {
				case "register":
					p, err = RegisterPlayer(ctx, db, s.user, s.password)
				case "login":
					p, err = LoginPlayer(ctx, db, s.user, s.password)
				default:
					t.Fatalf("unknown op %q", s.op)
				}
				if s.wantErr == "" {
					if err != nil {
						t.Fatalf("step %d: %s %q: %v", i+1, s.op, s.user, err)
					}
					if p == nil || p.PlayerID != s.user {
						t.Fatalf("step %d: %s %q returned %v", i+1, s.op, s.user, p)
					}
					continue
				}
				if err == nil || !strings.Contains(err.Error(), s.wantErr) {
					t.Fatalf("step %d: %s %q: err = %v, want one containing %q", i+1, s.op, s.user, err, s.wantErr)
				}
				if p != nil {
					t.Fatalf("step %d: %s %q returned a player alongside the error", i+1, s.op, s.user)
				}
			}
			if got := playerCount(t, ctx, db); got != tc.wantPlayers {
				t.Errorf("players = %d, want %d", got, tc.wantPlayers)
			}
		})
	}
}

func TestRegisteredRow(t *testing.T) {
	ctx, db := openDB(t)
	if _, err := RegisterPlayer(ctx, db, "alice", "correct horse"); err != nil {
		t.Fatalf("RegisterPlayer: %v", err)
	}
	row, err := db.Player.Query().Where(entplayer.PlayerIDEQ("alice")).Only(ctx)
	if err != nil {
		t.Fatalf("load row: %v", err)
	}

	t.Run("the stored password is a cost-10 bcrypt hash, not the password", func(t *testing.T) {
		if row.Password == "correct horse" || strings.Contains(row.Password, "correct horse") {
			t.Fatalf("stored password contains the plain text: %q", row.Password)
		}
		if !strings.HasPrefix(row.Password, "$2a$10$") || len(row.Password) != 60 {
			t.Errorf("stored password = %q, want a 60-character $2a$10$ bcrypt hash", row.Password)
		}
	})
	t.Run("the display name defaults to the username", func(t *testing.T) {
		if row.PlayerName != "alice" {
			t.Errorf("player_name = %q, want %q", row.PlayerName, "alice")
		}
	})
	t.Run("registering the same password twice gives different hashes", func(t *testing.T) {
		if _, err := RegisterPlayer(ctx, db, "bob", "correct horse"); err != nil {
			t.Fatalf("RegisterPlayer bob: %v", err)
		}
		bob, err := db.Player.Query().Where(entplayer.PlayerIDEQ("bob")).Only(ctx)
		if err != nil {
			t.Fatalf("load bob: %v", err)
		}
		if bob.Password == row.Password {
			t.Errorf("alice and bob share the hash %q; want a fresh salt per player", bob.Password)
		}
	})
}
