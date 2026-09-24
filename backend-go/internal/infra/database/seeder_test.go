package database

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"agentic-npc-backend/internal/db/ent"
	"agentic-npc-backend/internal/db/ent/enttest"
	"agentic-npc-backend/internal/db/ent/npc"

	_ "github.com/mattn/go-sqlite3"
)

const (
	npcDir   = "../../../../gamedata/npcs"
	questDir = "../../../../gamedata/quests"
)

func openDB(t *testing.T) (context.Context, *ent.Client) {
	t.Helper()
	db := enttest.Open(t, "sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared&_fk=1")
	t.Cleanup(func() { db.Close() })
	return context.Background(), db
}

// npcTypes returns name -> npc_type for every NPC row.
func npcTypes(t *testing.T, ctx context.Context, db *ent.Client) map[string]string {
	t.Helper()
	rows, err := db.NPC.Query().All(ctx)
	if err != nil {
		t.Fatalf("query NPCs: %v", err)
	}
	out := map[string]string{}
	for _, n := range rows {
		out[n.Name] = n.NpcType
	}
	return out
}

// questNames returns id -> name for every quest row.
func questNames(t *testing.T, ctx context.Context, db *ent.Client) map[string]string {
	t.Helper()
	rows, err := db.Quest.Query().All(ctx)
	if err != nil {
		t.Fatalf("query quests: %v", err)
	}
	out := map[string]string{}
	for _, q := range rows {
		out[q.ID] = q.Name
	}
	return out
}

var wantNPCs = map[string]string{
	"Baelor": "Blacksmith",
	"Elara":  "Herbalist",
	"Elian":  "Villager", // occupation is ["Villager", "Smart Brother"]; the first is kept
	"Kaelen": "Magistrate",
	"Lian":   "Villager",
	"Marcus": "Guard",
	"Rook":   "Bandit",
	"Silas":  "Merchant",
}

var wantQuests = map[string]string{
	"sq_0_prove_your_worth": "A Growing Threat",
	"sq_1_elian_intro":      "An Anxious Brother",
	"sq_2_elara_research":   "The Herbalist's Knowledge",
	"sq_3_baelor_gate":      "The Distrustful Blacksmith",
	"sq_5_petal_return":     "The Final Component",
	"sq_b1_proof_of_mettle": "Proof of Mettle",
	"sq_e1_missing_herbs":   "A Simple Errand",
}

func seedAll(t *testing.T, db *ent.Client) {
	t.Helper()
	if err := SeedNPCs(db, npcDir); err != nil {
		t.Fatalf("SeedNPCs: %v", err)
	}
	if err := SeedQuests(db, questDir); err != nil {
		t.Fatalf("SeedQuests: %v", err)
	}
}

func TestSeedRealGamedata(t *testing.T) {
	t.Run("a fresh database gets all eight NPCs with their occupations", func(t *testing.T) {
		ctx, db := openDB(t)
		seedAll(t, db)
		if got := npcTypes(t, ctx, db); !reflect.DeepEqual(got, wantNPCs) {
			t.Errorf("NPCs = %v, want %v", got, wantNPCs)
		}
	})

	t.Run("each NPC points at the three files in its own directory", func(t *testing.T) {
		ctx, db := openDB(t)
		seedAll(t, db)
		rows, err := db.NPC.Query().All(ctx)
		if err != nil {
			t.Fatalf("query NPCs: %v", err)
		}
		for _, n := range rows {
			dir := filepath.Join(npcDir, map[string]string{
				"Baelor": "baelor", "Elara": "elara", "Elian": "elian", "Kaelen": "kaelen",
				"Lian": "lian", "Marcus": "marcus", "Rook": "rook", "Silas": "silas",
			}[n.Name])
			want := [3]string{filepath.Join(dir, "personality.json"), filepath.Join(dir, "backstory.json"), filepath.Join(dir, "lore.json")}
			got := [3]string{n.PersonalityPath, n.BackstoryPath, n.LorePath}
			if got != want {
				t.Errorf("%s paths = %q, want %q", n.Name, got, want)
			}
			for _, p := range got {
				if _, err := os.Stat(p); err != nil {
					t.Errorf("%s: %s does not exist: %v", n.Name, p, err)
				}
			}
		}
	})

	t.Run("a fresh database gets all seven side quests and nothing else", func(t *testing.T) {
		ctx, db := openDB(t)
		seedAll(t, db)
		if got := questNames(t, ctx, db); !reflect.DeepEqual(got, wantQuests) {
			t.Errorf("quests = %v, want %v", got, wantQuests)
		}
		rows, err := db.Quest.Query().All(ctx)
		if err != nil {
			t.Fatalf("query quests: %v", err)
		}
		for _, q := range rows {
			if want := filepath.Join(questDir, "definitions", q.ID+".json"); q.StaticDataPath != want {
				t.Errorf("%s static_data_path = %q, want %q", q.ID, q.StaticDataPath, want)
			}
		}
	})

	t.Run("seeding twice adds nothing and keeps the same rows", func(t *testing.T) {
		ctx, db := openDB(t)
		seedAll(t, db)
		before, err := db.NPC.Query().IDs(ctx)
		if err != nil {
			t.Fatalf("NPC ids: %v", err)
		}
		seedAll(t, db)
		after, err := db.NPC.Query().IDs(ctx)
		if err != nil {
			t.Fatalf("NPC ids: %v", err)
		}
		sort.Slice(before, func(i, j int) bool { return before[i].String() < before[j].String() })
		sort.Slice(after, func(i, j int) bool { return after[i].String() < after[j].String() })
		if len(after) != 8 || !reflect.DeepEqual(before, after) {
			t.Errorf("NPC ids after a second seed = %v, want the same 8 as before %v", after, before)
		}
		if got := questNames(t, ctx, db); !reflect.DeepEqual(got, wantQuests) {
			t.Errorf("quests after a second seed = %v, want %v", got, wantQuests)
		}
	})

	t.Run("an NPC that already exists is left exactly as it was", func(t *testing.T) {
		ctx, db := openDB(t)
		if _, err := db.NPC.Create().SetName("Elara").SetNpcType("villager").
			SetPersonalityPath("old/personality.json").SetBackstoryPath("old/backstory.json").SetLorePath("old/lore.json").
			Save(ctx); err != nil {
			t.Fatalf("pre-create Elara: %v", err)
		}
		seedAll(t, db)
		got := npcTypes(t, ctx, db)
		if len(got) != 8 || got["Elara"] != "villager" {
			t.Errorf("NPCs = %v, want 8 with Elara still a villager", got)
		}
		elara, err := db.NPC.Query().Where(npc.NameEQ("Elara")).Only(ctx)
		if err != nil {
			t.Fatalf("load Elara: %v", err)
		}
		if elara.PersonalityPath != "old/personality.json" {
			t.Errorf("Elara personality_path = %q, want the pre-existing %q", elara.PersonalityPath, "old/personality.json")
		}
	})
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSeedNPCsFromFiles(t *testing.T) {
	cases := []struct {
		name    string
		files   map[string]string // relative path -> personality.json body
		wantErr bool
		want    map[string]string // name -> npc_type
	}{
		{
			name:  "an empty directory seeds nothing and is not an error",
			files: nil,
			want:  map[string]string{},
		},
		{
			name: "a malformed personality file is skipped and the rest are seeded",
			files: map[string]string{
				"good/personality.json": `{"name": "Good", "occupation": "Baker"}`,
				"bad/personality.json":  `{"name": "Bad", `,
			},
			want: map[string]string{"Good": "Baker"},
		},
		{
			name: "occupations that are missing, empty, numeric or non-string lists become Unknown",
			files: map[string]string{
				"a/personality.json": `{"name": "A"}`,
				"b/personality.json": `{"name": "B", "occupation": []}`,
				"c/personality.json": `{"name": "C", "occupation": 7}`,
				"d/personality.json": `{"name": "D", "occupation": [3, "Smith"]}`,
				"e/personality.json": `{"name": "E", "occupation": null}`,
			},
			want: map[string]string{"A": "Unknown", "B": "Unknown", "C": "Unknown", "D": "Unknown", "E": "Unknown"},
		},
		{
			name: "one personality file with no name fails the whole seed and nothing is written",
			files: map[string]string{
				"good/personality.json":     `{"name": "Good", "occupation": "Baker"}`,
				"nameless/personality.json": `{"occupation": "Ghost"}`,
			},
			wantErr: true,
			want:    map[string]string{},
		},
		{
			name: "two files claiming the same name seed one NPC",
			files: map[string]string{
				"a/personality.json": `{"name": "Twin", "occupation": "First"}`,
				"b/personality.json": `{"name": "Twin", "occupation": "Second"}`,
			},
			want: map[string]string{"Twin": "First"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for rel, body := range tc.files {
				writeFile(t, filepath.Join(dir, rel), body)
			}
			ctx, db := openDB(t)
			err := SeedNPCs(db, dir)
			if (err != nil) != tc.wantErr {
				t.Fatalf("SeedNPCs err = %v, want error: %v", err, tc.wantErr)
			}
			if got := npcTypes(t, ctx, db); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("NPCs = %v, want %v", got, tc.want)
			}
		})
	}

	t.Run("a directory that does not exist seeds nothing and reports no error", func(t *testing.T) {
		ctx, db := openDB(t)
		if err := SeedNPCs(db, filepath.Join(t.TempDir(), "no-such-dir")); err != nil {
			t.Fatalf("SeedNPCs: %v", err)
		}
		if got := npcTypes(t, ctx, db); len(got) != 0 {
			t.Errorf("NPCs = %v, want none", got)
		}
	})
}

func TestSeedQuestsFromFiles(t *testing.T) {
	cases := []struct {
		name    string
		files   map[string]string // relative path -> body
		wantErr bool
		want    map[string]string // id -> name
	}{
		{
			name: "only definitions/sq_*.json files are seeded",
			files: map[string]string{
				"definitions/sq_a.json":   `{"quest_id": "sq_a", "name": "Alpha"}`,
				"definitions/main_b.json": `{"quest_id": "main_b", "name": "Beta"}`,
				"sq_c.json":               `{"quest_id": "sq_c", "name": "Gamma"}`,
			},
			want: map[string]string{"sq_a": "Alpha"},
		},
		{
			name: "a quest with no quest_id or malformed JSON is skipped",
			files: map[string]string{
				"definitions/sq_a.json": `{"quest_id": "sq_a", "name": "Alpha"}`,
				"definitions/sq_b.json": `{"name": "No id"}`,
				"definitions/sq_c.json": `{"quest_id": `,
			},
			want: map[string]string{"sq_a": "Alpha"},
		},
		{
			name: "the quest_id inside the file is the key, not the file name",
			files: map[string]string{
				"definitions/sq_file.json": `{"quest_id": "sq_inner", "name": "Inner"}`,
			},
			want: map[string]string{"sq_inner": "Inner"},
		},
		{
			name: "one quest with no name fails the whole seed and nothing is written",
			files: map[string]string{
				"definitions/sq_a.json": `{"quest_id": "sq_a", "name": "Alpha"}`,
				"definitions/sq_b.json": `{"quest_id": "sq_b"}`,
			},
			wantErr: true,
			want:    map[string]string{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for rel, body := range tc.files {
				writeFile(t, filepath.Join(dir, rel), body)
			}
			ctx, db := openDB(t)
			err := SeedQuests(db, dir)
			if (err != nil) != tc.wantErr {
				t.Fatalf("SeedQuests err = %v, want error: %v", err, tc.wantErr)
			}
			if got := questNames(t, ctx, db); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("quests = %v, want %v", got, tc.want)
			}
		})
	}
}
