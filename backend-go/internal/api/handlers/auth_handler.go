package handlers

import (
	_ "agentic-npc-backend/internal/db/ent"
	"agentic-npc-backend/internal/domain/user_logic"
	"context"
	"fmt"
	"log"

	"github.com/gorilla/websocket"
)

// HandleAuthEvent processes authentication-related WebSocket events (LOGIN, REGISTER).
// It returns the new playerID, authentication status, and any error.
func (h *WebSocketHandler) HandleAuthEvent(conn *websocket.Conn, ctx context.Context, event EventMessage) (string, bool, error) {
	switch event.EventType {
	case "REGISTER_PLAYER":
		log.Printf("Attempting to register new player: %s", event.Username)
		_, err := user_logic.RegisterPlayer(ctx, h.dbClient, event.Username, event.Password)
		if err != nil {
			log.Printf("Registration failed: %v", err)
			return "", false, err
		}
		// After successful registration, fall through to login
		fallthrough

	case "LOGIN_PLAYER":
		log.Printf("Attempting to log in player: %s", event.Username)
		player, err := user_logic.LoginPlayer(ctx, h.dbClient, event.Username, event.Password)
		if err != nil {
			log.Printf("Login failed: %v", err)
			return "", false, err
		}

		log.Printf("--- Player '%s' successfully authenticated. ---", player.PlayerID)

		// Send success message
		responseMap := map[string]string{
			"action_type": "LOGIN_SUCCESS",
			"content":     fmt.Sprintf("Welcome, %s!", player.PlayerName),
		}
		if err := conn.WriteJSON(responseMap); err != nil {
			log.Println("WebSocket write error:", err)
			return "", false, err
		}
		// Return the new authenticated state
		return player.PlayerID, true, nil

	default:
		log.Printf("Auth failed: Received event '%s' from unauthenticated client.", event.EventType)
		return "", false, fmt.Errorf("authentication required. Please LOGIN_PLAYER or REGISTER_PLAYER first")
	}
}
