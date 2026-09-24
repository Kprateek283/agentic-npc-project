package quest_logic

import (
	"agentic-npc-backend/internal/db/ent"
	entnpc "agentic-npc-backend/internal/db/ent/npc"
	entplayer "agentic-npc-backend/internal/db/ent/player"
	entplayernpcrelationship "agentic-npc-backend/internal/db/ent/playernpcrelationship"
	"context"
	"fmt"
)

// --- DB HELPER FUNCTIONS ---

// GetPlayer finds a player by their source_entity_id.
func (qm *QuestManager) GetPlayer(ctx context.Context, db *ent.Client, playerID string) (*ent.Player, error) {
	return db.Player.Query().Where(entplayer.PlayerIDEQ(playerID)).Only(ctx)
}

// GetNpc finds an NPC by their in-game name.
func (qm *QuestManager) GetNpc(ctx context.Context, db *ent.Client, npcName string) (*ent.NPC, error) {
	return db.NPC.Query().Where(entnpc.NameEQ(npcName)).Only(ctx)
}

// getItemDefinition gets a pre-loaded item's static data
func (qm *QuestManager) getItemDefinition(itemID string) (ItemDefinition, error) {
	itemDef, ok := qm.Items[itemID]
	if !ok {
		return ItemDefinition{}, fmt.Errorf("item definition not found for item_id: %s", itemID)
	}
	return itemDef, nil
}

// GetOrCreateRelationship finds the relationship between a player and NPC.
func (qm *QuestManager) GetOrCreateRelationship(ctx context.Context, db *ent.Client, p *ent.Player, n *ent.NPC) (*ent.PlayerNPCRelationship, error) {
	rel, err := db.PlayerNPCRelationship.
		Query().
		Where(
			entplayernpcrelationship.HasPlayerWith(entplayer.IDEQ(p.ID)),
			entplayernpcrelationship.HasNpcWith(entnpc.IDEQ(n.ID)),
		).
		Only(ctx)

	if ent.IsNotFound(err) {
		// Create it if it doesn't exist
		rel, err = db.PlayerNPCRelationship.Create().SetPlayer(p).SetNpc(n).Save(ctx)
	}
	if err != nil {
		return nil, err
	}

	return rel, nil
}
