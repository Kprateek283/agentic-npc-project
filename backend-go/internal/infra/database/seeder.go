package database

import (
	"agentic-npc-backend/internal/db/ent"
	"agentic-npc-backend/internal/db/ent/npc"   // Keep this import
	"agentic-npc-backend/internal/db/ent/quest" // <-- NEW IMPORT
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"entgo.io/ent/dialect/sql"
)

// tempPersonality struct (Keep this)
type tempPersonality struct {
	Name       string      `json:"name"`
	Occupation interface{} `json:"occupation"`
}

// occupationToString function (Keep this)
func occupationToString(occ interface{}) string {
	// ... (implementation unchanged) ...
	if occ == nil {
		return "Unknown"
	}
	// Try to cast to string first
	if str, ok := occ.(string); ok {
		return str
	}
	// Try to cast to slice of strings
	if arr, ok := occ.([]interface{}); ok {
		if len(arr) > 0 {
			// Just return the first element as a string
			if str, ok := arr[0].(string); ok {
				return str
			}
		}
	}
	// Fallback
	return "Unknown"
}

// SeedNPCs function (Keep this)
func SeedNPCs(client *ent.Client, gamedataPath string) error {
	// ... (implementation unchanged) ...
	log.Println("Checking database seeding for NPCs...")

	searchPath := filepath.Join(gamedataPath, "*", "personality.json")
	personalityFiles, err := filepath.Glob(searchPath)
	if err != nil {
		return fmt.Errorf("failed to scan NPC gamedata directory: %w", err)
	}

	if len(personalityFiles) == 0 {
		log.Println("No NPC personality files found in gamedata, skipping NPC seeding.")
		return nil
	}

	log.Printf("Found %d NPC configurations. Synchronizing with database...", len(personalityFiles))
	ctx := context.Background()

	creators := make([]*ent.NPCCreate, 0, len(personalityFiles))

	for _, pPath := range personalityFiles {
		data, err := os.ReadFile(pPath)
		if err != nil {
			log.Printf("Warning: failed to read %s, skipping NPC: %v", pPath, err)
			continue
		}

		var p tempPersonality
		if err := json.Unmarshal(data, &p); err != nil {
			log.Printf("Warning: failed to parse JSON from %s, skipping NPC: %v", pPath, err)
			continue
		}

		baseDir := filepath.Dir(pPath)
		backstoryPath := filepath.Join(baseDir, "backstory.json")
		lorePath := filepath.Join(baseDir, "lore.json")
		occupationStr := occupationToString(p.Occupation)

		creator := client.NPC.
			Create().
			SetName(p.Name).
			SetNpcType(occupationStr).
			SetPersonalityPath(pPath).
			SetBackstoryPath(backstoryPath).
			SetLorePath(lorePath)

		creators = append(creators, creator)
	}

	if len(creators) == 0 {
		log.Println("No valid NPC creators to add. NPC Seeding complete.")
		return nil
	}

	err = client.NPC.
		CreateBulk(creators...).
		OnConflict(sql.ConflictColumns(npc.FieldName)). // Conflict on the "name" field
		DoNothing().                                    // If name exists, do nothing
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to bulk seed NPCs: %w", err)
	}

	log.Println("NPC database seeding complete.")
	return nil
}

// --- NEW FUNCTION ---
// tempQuest is used to parse just the ID and Name from the quest JSON
type tempQuest struct {
	QuestID string `json:"quest_id"`
	Name    string `json:"name"`
}

// SeedQuests scans the gamedata directory and creates Quest records
// if they do not already exist.
func SeedQuests(client *ent.Client, gamedataPath string) error {
	log.Println("Checking database seeding for Quests...")

	searchPath := filepath.Join(gamedataPath, "definitions", "sq_*.json") // Path to quest files
	questFiles, err := filepath.Glob(searchPath)
	if err != nil {
		return fmt.Errorf("failed to scan quest gamedata directory: %w", err)
	}

	if len(questFiles) == 0 {
		log.Println("No quest definition files found in gamedata, skipping quest seeding.")
		return nil
	}

	log.Printf("Found %d quest definitions. Synchronizing with database...", len(questFiles))
	ctx := context.Background()

	creators := make([]*ent.QuestCreate, 0, len(questFiles))

	for _, qPath := range questFiles {
		data, err := os.ReadFile(qPath)
		if err != nil {
			log.Printf("Warning: failed to read %s, skipping quest: %v", qPath, err)
			continue
		}

		var q tempQuest
		if err := json.Unmarshal(data, &q); err != nil {
			log.Printf("Warning: failed to parse JSON from %s, skipping quest: %v", qPath, err)
			continue
		}

		if q.QuestID == "" {
			log.Printf("Warning: Quest ID missing in %s, skipping.", qPath)
			continue
		}

		creator := client.Quest.
			Create().
			SetID(q.QuestID). // Use the quest_id from JSON as the primary key
			SetName(q.Name).
			SetStaticDataPath(qPath) // Store the path to the full definition

		creators = append(creators, creator)
	}

	if len(creators) == 0 {
		log.Println("No valid quest creators to add. Quest Seeding complete.")
		return nil
	}

	// Use CreateBulk with OnConflict to insert or ignore existing quests
	err = client.Quest.
		CreateBulk(creators...).
		OnConflict(sql.ConflictColumns(quest.FieldID)). // Conflict on the "id" field
		DoNothing().                                    // If id exists, do nothing
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to bulk seed quests: %w", err)
	}

	log.Println("Quest database seeding complete.")
	return nil
}
