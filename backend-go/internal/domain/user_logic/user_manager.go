package user_logic

import (
	"agentic-npc-backend/internal/db/ent"
	entplayer "agentic-npc-backend/internal/db/ent/player"
	"context"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// hashPassword securely hashes a password using bcrypt.
func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

// checkPasswordHash compares a plain-text password with a stored hash.
func checkPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// RegisterPlayer handles creating a new player in the database.
// It checks if the player already exists and hashes their password.
func RegisterPlayer(ctx context.Context, db *ent.Client, username string, password string) (*ent.Player, error) {
	// 1. Check if player already exists
	exists, err := db.Player.
		Query().
		Where(entplayer.PlayerIDEQ(username)).
		Exist(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check if player exists: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("player with username '%s' already exists", username)
	}

	// 2. Hash the password
	hashedPassword, err := hashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// 3. Create the new player
	newPlayer, err := db.Player.
		Create().
		SetPlayerID(username).
		SetPlayerName(username). // Defaults display name to username
		SetPassword(hashedPassword).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create new player: %w", err)
	}

	return newPlayer, nil
}

// LoginPlayer handles logging in an existing player.
// It finds the player and securely compares their password hash.
func LoginPlayer(ctx context.Context, db *ent.Client, username string, password string) (*ent.Player, error) {
	// 1. Find the player by their username (PlayerID)
	player, err := db.Player.
		Query().
		Where(entplayer.PlayerIDEQ(username)).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("player '%s' not found", username)
	}

	// 2. Securely compare the provided password with the stored hash
	if !checkPasswordHash(password, player.Password) {
		return nil, fmt.Errorf("invalid password")
	}

	// 3. Success!
	return player, nil
}
