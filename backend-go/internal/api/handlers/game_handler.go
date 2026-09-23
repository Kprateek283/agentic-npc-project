package handlers

import (
	"agentic-npc-backend/internal/db/ent"
	entplayerqueststate "agentic-npc-backend/internal/db/ent/playerqueststate"
	"agentic-npc-backend/internal/domain/memory"
	"agentic-npc-backend/internal/domain/npcstate"
	"agentic-npc-backend/internal/domain/rules"
	pb "agentic-npc-backend/internal/proto"
	"agentic-npc-backend/internal/ratelimit"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/grpc/metadata"
)

const rateLimitedLine = "Easy, friend — you're talking faster than I can think. Give me a moment."

// newRequestID returns a short random hex id used to correlate the Go and Python
// log lines for one conversation event.
func newRequestID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

type emotionsPayload struct {
	NPC       string             `json:"npc"`
	TowardYou map[string]float64 `json:"toward_you"`
	General   map[string]float64 `json:"general"`
}

func roundEmotionMap(m map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(m))
	for k, v := range m {
		val := math.Round(v*100) / 100
		if math.Abs(val) < 1e-9 {
			val = 0
		}
		out[k] = val
	}
	return out
}

// emotionsFrameContent builds the JSON payload for an EMOTIONS frame with rounded values.
func emotionsFrameContent(npc string, toward, general map[string]float64) (string, error) {
	payload := emotionsPayload{
		NPC:       npc,
		TowardYou: roundEmotionMap(toward),
		General:   roundEmotionMap(general),
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// HandleGameEvent processes all in-game logic for an authenticated player.
func (h *WebSocketHandler) HandleGameEvent(conn *websocket.Conn, ctx context.Context, event EventMessage) {
	reqID := newRequestID()
	start := time.Now()

	// 1. Process all game logic (quests, gifting, AND admin commands)
	questStart := time.Now()
	failResponse, err := h.questManager.ProcessEvent(ctx, h.dbClient, event)
	questMs := time.Since(questStart).Milliseconds()
	if err != nil {
		log.Printf("Error processing event in QuestManager: %v", err)
		h.sendError(conn, err.Error())
		return
	}

	// 2. Check for "Fail Response" from quest preconditions
	if failResponse != nil {
		log.Printf("Dungeon Master: Precondition failed. Sending fail-response: %s", failResponse.Content)
		h.sendSimpleResponse(conn, failResponse.ActionType, failResponse.Content)
		return
	}

	// 3. Check if it was an Admin command that succeeded
	if strings.HasPrefix(event.EventType, "ADMIN_") {
		log.Println("Dungeon Master: Admin command processed successfully.")
		h.sendSimpleResponse(conn, "ADMIN_ACK", "Admin command received and processed.")
		return
	}

	// Find Target NPC and Player
	targetNPC, err := h.questManager.GetNpc(ctx, h.dbClient, event.TargetNpcName)
	if err != nil {
		log.Printf("Error finding NPC: %v", err)
		h.sendError(conn, "target NPC not found")
		return
	}

	player, err := h.questManager.GetPlayer(ctx, h.dbClient, event.SourceEntityId)
	if err != nil {
		log.Printf("Error finding Player: %v", err)
		h.sendError(conn, "player not found")
		return
	}

	// 4. Record episode before rate limit check
	if err := h.recordEpisode(ctx, event, targetNPC, player); err != nil {
		log.Printf("Error recording episode: %v", err)
	}

	// Send EMOTIONS frame before rate limit check
	cfg := memory.DefaultConfig()
	if h.questManager != nil && h.questManager.Rules != nil {
		cfg = h.questManager.Rules.Config
	}
	if eps, _, err := npcstate.Load(ctx, h.dbClient, targetNPC); err != nil {
		log.Printf("Error loading NPC state for emotions frame: %v", err)
	} else {
		now := time.Now()
		speakerEmotions := memory.EmotionsToward(player.PlayerID, eps, now, cfg)
		generalMood := memory.GeneralMood(eps, now, cfg)
		if content, err := emotionsFrameContent(targetNPC.Name, speakerEmotions, generalMood); err != nil {
			log.Printf("Error encoding emotions frame: %v", err)
		} else {
			h.sendSimpleResponse(conn, "EMOTIONS", content)
		}
	}

	// 5. Rate limit check before calling the AI service
	rlCtx, rlCancel := context.WithTimeout(ctx, 200*time.Millisecond)
	allowed, count, err := ratelimit.Allow(rlCtx, h.redisClient, "ratelimit:llm:"+event.SourceEntityId, h.llmRateLimit, h.llmRateWindow)
	rlCancel()
	if err != nil {
		slog.Warn("rate_limit_unavailable", "req_id", reqID, "player", event.SourceEntityId, "err", err.Error())
	} else if !allowed {
		slog.Warn("rate_limited", "req_id", reqID, "player", event.SourceEntityId, "count", count)
		h.sendSimpleResponse(conn, "SPEAK", rateLimitedLine)
		return
	}

	// 6. Normal event -> stream the AI response (C3).
	grpcMs, err := h.streamAI(conn, ctx, reqID, event, targetNPC, player)
	if err != nil {
		if ctx.Err() != nil {
			slog.Info("client_disconnected", "req_id", reqID, "npc", event.TargetNpcName)
			return
		}
		slog.Warn("ai_stream_failed", "req_id", reqID, "npc", event.TargetNpcName,
			"event", event.EventType, "err", err.Error())
		h.sendSimpleResponse(conn, "SPEAK",
			"Hmm? Forgive me — my mind wandered just now. Ask me again in a moment.")
		return
	}

	slog.Info("game_event",
		"req_id", reqID,
		"player", event.SourceEntityId,
		"npc", event.TargetNpcName,
		"event", event.EventType,
		"quest_ms", questMs,
		"grpc_ms", grpcMs,
		"total_ms", time.Since(start).Milliseconds(),
	)
}

// recordEpisode writes the event into the memory table using the rules from h.questManager.Rules.
func (h *WebSocketHandler) recordEpisode(ctx context.Context, event EventMessage, npc *ent.NPC, player *ent.Player) error {
	if event.EventType == "PLAYER_GAVE_GIFT" || event.EventType == "QUEST_REWARD" {
		return nil
	}

	cfg := memory.DefaultConfig()
	var rule rules.EventRule
	var hasRule bool
	if h.questManager != nil && h.questManager.Rules != nil {
		cfg = h.questManager.Rules.Config
		rule, hasRule = h.questManager.Rules.Rule(event.EventType)
	}
	if !hasRule {
		return nil // unknown event type records nothing and is not an error
	}

	eps, rows, err := npcstate.Load(ctx, h.dbClient, npc)
	if err != nil {
		return err
	}

	subject := ""
	if event.EventType == "PLAYER_GAVE_GIFT" || event.EventType == "PLAYER_SUBMITTED_QUEST_ITEM" {
		subject = event.Keyword
	}

	now := time.Now()
	memoryDesc := fmt.Sprintf("%s triggered %s on %s", player.PlayerID, event.EventType, npc.Name)
	if event.EventType == "PLAYER_GAVE_GIFT" {
		memoryDesc = fmt.Sprintf("%s gave %s to %s", player.PlayerID, event.Keyword, npc.Name)
	}

	if rule.Apology {
		priorApologies := 0
		for _, ep := range eps {
			if ep.Actor == player.PlayerID && ep.EventType == event.EventType {
				priorApologies++
			}
		}

		_ = memory.Forgive(eps, player.PlayerID, priorApologies, cfg)

		var covers []int
		for i := range eps {
			if eps[i].Forgiven != rows[i].Forgiven {
				if err := h.dbClient.Memory.UpdateOne(rows[i]).SetForgiven(eps[i].Forgiven).Exec(ctx); err != nil {
					return err
				}
				covers = append(covers, rows[i].ID)
			}
		}

		createOp := h.dbClient.Memory.Create().
			SetOwner(npc).
			SetActor(player.PlayerID).
			SetEventType(event.EventType).
			SetSubject(subject).
			SetIntensity(rule.Intensity).
			SetHarmful(rule.Harmful).
			SetCount(1.0).
			SetFirstAt(now).
			SetLastAt(now).
			SetParticipants([]string{player.PlayerID, npc.ID.String()}).
			SetDescription(memoryDesc)
		if len(rule.Delta) > 0 {
			createOp.SetDelta(rule.Delta)
		}
		if len(covers) > 0 {
			createOp.SetCovers(covers)
		}
		_, err := createOp.Save(ctx)
		return err
	}

	actorHasForgiven := false
	for _, ep := range eps {
		if ep.Actor == player.PlayerID && ep.Forgiven > 0 {
			actorHasForgiven = true
			break
		}
	}

	if rule.Harmful && actorHasForgiven {
		_ = memory.RevokeForgiveness(eps, player.PlayerID)
		for i := range eps {
			if rows[i].Actor == player.PlayerID && rows[i].Forgiven > 0 && eps[i].Forgiven == 0 {
				if err := h.dbClient.Memory.UpdateOne(rows[i]).SetForgiven(0).Exec(ctx); err != nil {
					return err
				}
			}
		}

		delta := make(map[string]float64, len(rule.Delta)+1)
		for k, v := range rule.Delta {
			delta[k] = v
		}
		delta["trust"] += cfg.BetrayalTrust

		createOp := h.dbClient.Memory.Create().
			SetOwner(npc).
			SetActor(player.PlayerID).
			SetEventType(event.EventType).
			SetSubject(subject).
			SetDelta(delta).
			SetIntensity(rule.Intensity).
			SetHarmful(rule.Harmful).
			SetBetrayal(true).
			SetCount(1.0).
			SetFirstAt(now).
			SetLastAt(now).
			SetParticipants([]string{player.PlayerID, npc.ID.String()}).
			SetDescription(memoryDesc)
		_, err := createOp.Save(ctx)
		return err
	}

	if rule.Conversation {
		createOp := h.dbClient.Memory.Create().
			SetOwner(npc).
			SetActor(player.PlayerID).
			SetEventType(event.EventType).
			SetSubject(subject).
			SetText(event.QuestionText).
			SetIntensity(rule.Intensity).
			SetHarmful(rule.Harmful).
			SetCount(1.0).
			SetFirstAt(now).
			SetLastAt(now).
			SetParticipants([]string{player.PlayerID, npc.ID.String()}).
			SetDescription(memoryDesc)
		if len(rule.Delta) > 0 {
			createOp.SetDelta(rule.Delta)
		}
		_, err := createOp.Save(ctx)
		return err
	}

	// Otherwise: merge into existing episode if CanMerge allows, else insert new row.
	mergeIndex := -1
	for i, ep := range eps {
		if ep.Actor == player.PlayerID && ep.EventType == event.EventType && ep.Subject == subject {
			if memory.CanMerge(ep, now, cfg, false, actorHasForgiven) {
				mergeIndex = i
				break
			}
		}
	}

	if mergeIndex >= 0 {
		ep := eps[mergeIndex]
		memory.Merge(&ep, now, cfg)
		return h.dbClient.Memory.UpdateOne(rows[mergeIndex]).
			SetCount(ep.Count).
			SetLastAt(ep.LastAt).
			Exec(ctx)
	}

	createOp := h.dbClient.Memory.Create().
		SetOwner(npc).
		SetActor(player.PlayerID).
		SetEventType(event.EventType).
		SetSubject(subject).
		SetIntensity(rule.Intensity).
		SetHarmful(rule.Harmful).
		SetCount(1.0).
		SetFirstAt(now).
		SetLastAt(now).
		SetParticipants([]string{player.PlayerID, npc.ID.String()}).
		SetDescription(memoryDesc)
	if len(rule.Delta) > 0 {
		createOp.SetDelta(rule.Delta)
	}
	_, err = createOp.Save(ctx)
	return err
}

type episodeRanked struct {
	ep     memory.Episode
	desc   string
	weight float64
	order  int
}

func formatMemoryLine(desc string, actor string, speakerID string, count float64, betrayal bool) string {
	line := desc
	if actor != "" {
		if strings.HasPrefix(line, actor+" ") {
			if actor == speakerID {
				line = "You " + line[len(actor)+1:]
			} else {
				line = "Someone " + line[len(actor)+1:]
			}
		} else if strings.HasPrefix(line, actor) {
			if actor == speakerID {
				line = "You" + line[len(actor):]
			} else {
				line = "Someone" + line[len(actor):]
			}
		}
	}
	roundCount := int(math.Round(count))
	if roundCount >= 2 {
		line = fmt.Sprintf("%s (%d times)", line, roundCount)
	}
	if betrayal {
		line = line + " — after apologising"
	}
	return line
}

// aiRequestArgs is the fully-gathered per-event context the AI service needs.
type aiRequestArgs struct {
	personalityPath string
	backstoryPath   string
	lorePath        string
	emotions        map[string]float64
	generalMood     map[string]float64
	memoryLines     []string
	eventType       string
	text            string
	sourceEntityId  string
	questStep       int
	completionRate  float32
}

// buildAIArgs reads the state and computes emotions, general mood, and ranked memory lines.
func (h *WebSocketHandler) buildAIArgs(ctx context.Context, event EventMessage, npc *ent.NPC, player *ent.Player) (*aiRequestArgs, error) {
	if npc == nil {
		var err error
		npc, err = h.questManager.GetNpc(ctx, h.dbClient, event.TargetNpcName)
		if err != nil {
			return nil, fmt.Errorf("target NPC not found")
		}
	}
	if player == nil {
		var err error
		player, err = h.questManager.GetPlayer(ctx, h.dbClient, event.SourceEntityId)
		if err != nil {
			return nil, fmt.Errorf("player not found")
		}
	}

	cfg := memory.DefaultConfig()
	if h.questManager != nil && h.questManager.Rules != nil {
		cfg = h.questManager.Rules.Config
	}

	eps, rows, err := npcstate.Load(ctx, h.dbClient, npc)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	speakerEmotions := memory.EmotionsToward(player.PlayerID, eps, now, cfg)
	generalMood := memory.GeneralMood(eps, now, cfg)

	var speakerEpisodes []episodeRanked
	var otherEpisodes []episodeRanked

	for i, ep := range eps {
		w := memory.Weight(ep, now, cfg)
		desc := ""
		if i < len(rows) && rows[i] != nil {
			desc = rows[i].Description
		}
		item := episodeRanked{
			ep:     ep,
			desc:   desc,
			weight: w,
			order:  i,
		}
		if ep.Actor == player.PlayerID {
			speakerEpisodes = append(speakerEpisodes, item)
		} else {
			if w >= cfg.NotabilityThreshold {
				otherEpisodes = append(otherEpisodes, item)
			}
		}
	}

	sort.SliceStable(speakerEpisodes, func(i, j int) bool {
		if speakerEpisodes[i].weight != speakerEpisodes[j].weight {
			return speakerEpisodes[i].weight > speakerEpisodes[j].weight
		}
		return speakerEpisodes[i].order < speakerEpisodes[j].order
	})

	sort.SliceStable(otherEpisodes, func(i, j int) bool {
		if otherEpisodes[i].weight != otherEpisodes[j].weight {
			return otherEpisodes[i].weight > otherEpisodes[j].weight
		}
		return otherEpisodes[i].order < otherEpisodes[j].order
	})

	var memoryLines []string
	limitSpeaker := 5
	if len(speakerEpisodes) < limitSpeaker {
		limitSpeaker = len(speakerEpisodes)
	}
	for i := 0; i < limitSpeaker; i++ {
		item := speakerEpisodes[i]
		line := formatMemoryLine(item.desc, item.ep.Actor, player.PlayerID, item.ep.Count, item.ep.Betrayal)
		memoryLines = append(memoryLines, line)
	}

	limitOther := 3
	if len(otherEpisodes) < limitOther {
		limitOther = len(otherEpisodes)
	}
	for i := 0; i < limitOther; i++ {
		item := otherEpisodes[i]
		line := formatMemoryLine(item.desc, item.ep.Actor, player.PlayerID, item.ep.Count, item.ep.Betrayal)
		memoryLines = append(memoryLines, line)
	}

	currentQuestStep, completionRate := h.getPlayerQuestState(ctx, player)

	text := event.QuestionText
	if event.EventType == "PLAYER_GAVE_GIFT" || event.EventType == "PLAYER_SUBMITTED_QUEST_ITEM" {
		text = event.Keyword
	}

	return &aiRequestArgs{
		personalityPath: npc.PersonalityPath,
		backstoryPath:   npc.BackstoryPath,
		lorePath:        npc.LorePath,
		emotions:        speakerEmotions,
		generalMood:     generalMood,
		memoryLines:     memoryLines,
		eventType:       event.EventType,
		text:            text,
		sourceEntityId:  player.PlayerID,
		questStep:       currentQuestStep,
		completionRate:  completionRate,
	}, nil
}

// callAI is the unary path: gather context, call Think, return the response and the RPC's
// own duration. Kept for the benchmark harness and as the non-streaming fallback.
func (h *WebSocketHandler) callAI(ctx context.Context, reqID string, event EventMessage, npc *ent.NPC, player *ent.Player) (*pb.ActionResponse, int64, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "req-id", reqID)
	a, err := h.buildAIArgs(ctx, event, npc, player)
	if err != nil {
		return nil, 0, err
	}
	grpcStart := time.Now()
	resp, err := h.aiClient.CallAIThink(ctx, a.personalityPath, a.backstoryPath, a.lorePath,
		a.emotions, a.generalMood, a.memoryLines, a.eventType, a.text, a.sourceEntityId, a.questStep, a.completionRate)
	return resp, time.Since(grpcStart).Milliseconds(), err
}

// streamAI is the streaming path (C3): gather context, open ThinkStream, forward each text
// delta as a SPEAK_PARTIAL frame, and close with a SPEAK frame carrying the full text.
// It returns a non-nil error only when the stream never produced anything (so the caller
// can fall back); a mid-stream failure is finalized best-effort with the partial text.
func (h *WebSocketHandler) streamAI(conn *websocket.Conn, ctx context.Context, reqID string, event EventMessage, npc *ent.NPC, player *ent.Player) (int64, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "req-id", reqID)
	a, err := h.buildAIArgs(ctx, event, npc, player)
	if err != nil {
		return 0, err
	}

	sent := 0
	grpcStart := time.Now()
	full, err := h.aiClient.CallAIThinkStream(ctx, a.personalityPath, a.backstoryPath, a.lorePath,
		a.emotions, a.generalMood, a.memoryLines, a.eventType, a.text, a.sourceEntityId, a.questStep, a.completionRate,
		func(tok string) {
			sent++
			h.sendSimpleResponse(conn, "SPEAK_PARTIAL", tok)
		})
	grpcMs := time.Since(grpcStart).Milliseconds()

	if ctx.Err() != nil {
		return grpcMs, ctx.Err()
	}

	if err != nil && sent == 0 {
		// Nothing streamed — signal the caller to fall back to the in-character line.
		return grpcMs, err
	}
	if err != nil {
		log.Printf("[stream] errored after %d partial(s); finalizing with partial text: %v", sent, err)
	}
	// Final frame carries the whole content (non-streaming clients can ignore partials).
	h.sendSimpleResponse(conn, "SPEAK", full)
	return grpcMs, nil
}

// getPlayerQuestState is a helper to find the active quest state for the AI context
func (h *WebSocketHandler) getPlayerQuestState(ctx context.Context, player *ent.Player) (int, float32) {
	if player == nil {
		return 0, 0.0
	}

	activeQuest, err := player.QueryQuestStates().Where(entplayerqueststate.IsCompletedEQ(false)).Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		log.Printf("Error fetching active quest state: %v", err)
	} else if activeQuest != nil {
		return activeQuest.CurrentStep, activeQuest.CompletionRate
	}

	return 0, 0.0 // Default if no active quest
}
