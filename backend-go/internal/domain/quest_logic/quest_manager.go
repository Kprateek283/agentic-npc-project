package quest_logic

import (
	"agentic-npc-backend/internal/db/ent"
	_ "agentic-npc-backend/internal/db/ent/item"
	_ "agentic-npc-backend/internal/db/ent/npc"
	_ "agentic-npc-backend/internal/db/ent/player"
	_ "agentic-npc-backend/internal/db/ent/playernpcrelationship"
	_ "agentic-npc-backend/internal/db/ent/playerqueststate"
	"agentic-npc-backend/internal/dto"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-redis/redis/v8"
	_ "github.com/google/uuid"
)

// QuestManager holds the preloaded static game data ("rulebook")
type QuestManager struct {
	Items  map[string]ItemDefinition
	Quests map[string]QuestDefinition
}

// NewQuestManager creates a new manager and loads all gamedata
func NewQuestManager(gamedataPath string) (*QuestManager, error) {
	qm := &QuestManager{
		Items:  make(map[string]ItemDefinition),
		Quests: make(map[string]QuestDefinition),
	}
	err := qm.loadGameData(gamedataPath)
	if err != nil {
		return nil, err
	}
	return qm, nil
}

// loadGameData parses all JSON definitions from the gamedata directory
func (qm *QuestManager) loadGameData(gamedataPath string) error {
	log.Println("Loading game data 'rulebook'...")

	// 1. Load Items
	itemPath := filepath.Join(gamedataPath, "items.json")
	var items []ItemDefinition
	itemData, err := os.ReadFile(itemPath)
	if err != nil {
		return fmt.Errorf("failed to read items.json: %w", err)
	}
	if err := json.Unmarshal(itemData, &items); err != nil {
		return fmt.Errorf("failed to parse items.json: %w", err)
	}
	for _, itemDef := range items {
		qm.Items[itemDef.ItemID] = itemDef
	}
	log.Printf("Successfully loaded %d items.", len(qm.Items))

	// 2. Load all Quest Definitions
	questPath := filepath.Join(gamedataPath, "quests", "definitions")
	questFiles, err := filepath.Glob(filepath.Join(questPath, "sq_*.json"))
	if err != nil {
		return fmt.Errorf("failed to scan quest definitions: %w", err)
	}
	for _, questFile := range questFiles {
		var quest QuestDefinition
		questData, err := os.ReadFile(questFile)
		if err != nil {
			log.Printf("Warning: failed to read quest file %s: %v", questFile, err)
			continue
		}
		if err := json.Unmarshal(questData, &quest); err != nil {
			log.Printf("Warning: failed to parse quest file %s: %v", questFile, err)
			continue
		}
		if quest.QuestID == "" {
			log.Printf("Warning: quest file %s has no quest_id, skipping.", questFile)
			continue
		}
		qm.Quests[quest.QuestID] = quest
	}
	log.Printf("Successfully loaded %d quest definitions.", len(qm.Quests))
	log.Println("Game data 'rulebook' successfully loaded.")
	return nil
}

// ProcessEvent is the main "brain" of the Go backend.
func (qm *QuestManager) ProcessEvent(ctx context.Context, db *ent.Client, rdb *redis.Client, event dto.EventMessage) (*FailResponseAction, error) {
	log.Println("Dungeon Master is processing the event...")

	// 1. Get the Player
	p, err := qm.GetPlayer(ctx, db, rdb, event.SourceEntityId)
	if err != nil {
		return nil, err
	}

	// 2. Route to Admin Handler
	if strings.HasPrefix(event.EventType, "ADMIN_") {
		err := qm.HandleAdminCommand(ctx, db, rdb, event)
		return nil, err // Admin commands don't have fail responses
	}

	// 3. Get the Target NPC (required for all other events)
	n, err := qm.GetNpc(ctx, db, rdb, event.TargetNpcName)
	if err != nil {
		return nil, err
	}

	// 4. Route to Gifting Handler
	if event.EventType == "PLAYER_GAVE_GIFT" {
		err := qm.handleGifting(ctx, db, rdb, p, n, event.Keyword)
		return nil, err // Gifting doesn't trigger quest checks, return directly
	}

	// 5. Route to Quest Logic Handler
	// This will check for quest completion and return a failResponse if preconditions fail
	return qm.checkQuestCompletion(ctx, db, rdb, p, n, event)
}
