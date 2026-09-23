# Memory v1 — working plan and progress

NPC emotions are being replaced by values computed from remembered "episodes". Read
`docs/memory_v1_assumptions.md` first: it holds every decision already taken. Add any new
decision there rather than asking.

## Progress

- [x] Step 1 — `backend-go/internal/domain/memory`: the pure maths (episodes, escalation, decay,
      separate positive/negative caps, forgiveness, betrayal) with 10 table-driven tests.
- [x] Step 2 — `gamedata/events.json` + `backend-go/internal/domain/rules`: the rules file and a
      validating loader producing a `memory.Config`, 11 tests.
- [x] Step 3 — schema and episode plumbing (memory rows are episodes; trust computed; gifts and quest rewards recorded as episodes)
- [ ] Step 4 — rewrite `gatherAIContext`
- [ ] Step 5 — gRPC contract, Python formatting and prompts
- [ ] Step 6 — `EMOTIONS` frame and browser panel
- [ ] Step 7 — demo verification (needs a local machine: Ollama + Docker), then PR

## Working rules

- One commit per step, on branch `memory-v1`, after its checks pass. Plain commit messages
  describing the change and how it was verified. **Never add Claude or Anthropic attribution to a
  commit or PR** (no `Co-Authored-By`, no "Generated with", no "reviewed by").
- Tick the step here in the same commit and keep this file honest.
- Run before every commit: `cd backend-go && GOWORK=off go build ./... && GOWORK=off go vet ./... &&
  GOWORK=off go test -count=1 ./...` and, from `ai-service-python`, `PYTHONPATH=. python -m pytest tests/ -q`
  plus `ruff check .` (create a venv from `requirements.txt` + `requirements-dev.txt` if none exists;
  Python 3.12 is pinned in `.python-version`).
- Generated code (`backend-go/internal/db/ent/`, `ai-service-python/ai_pb2*.py`,
  `backend-go/internal/proto/`) is produced by generators, never hand-edited.
- If a step cannot be completed as written, commit what is finished, write what blocked it in the
  step's entry here, and move on to the next step rather than inventing a workaround.

## Step 3 — schema and episode plumbing

1. `backend-go/internal/db/ent/schema/memory.go`: add `actor` (string, default ""), `subject`
   (string, default ""), `delta` (JSON `map[string]float64`, optional), `intensity` (float64, 0),
   `count` (float64, 1), `harmful` (bool, false), `forgiven` (float64, 0), `betrayal` (bool, false),
   `text` (string, ""), `first_at` / `last_at` (time, default now), `covers` (JSON `[]int`, optional).
   Keep every existing field.
2. `npc.go`: remove `emotions`. `playernpcrelationship.go`: remove `trust_level` and `gift_count`
   (keep the edges). Regenerate: `cd backend-go/internal/db/ent && GOWORK=off go generate ./...`.
   ent only adds columns, so existing databases keep the unused ones — that is intended.
3. New `backend-go/internal/domain/npcstate/npcstate.go`: `Load(ctx, db, npc)` returns that NPC's
   memory rows as `[]memory.Episode` plus the matching `[]*ent.Memory` in the same order (actor falls
   back to `participants[0]`, `last_at` falls back to `created_at`, a row without `delta` contributes
   nothing); `TrustToward(eps, actor, now, cfg)` returns the computed `trust`.
4. `quest_logic`: `QuestManager` gains `Rules *rules.Rules` (set like `AdminEnabled`). `handleGifting`
   writes a gift episode (delta `{"trust": clamp(item.base_trust_value, -1, 1)}`, intensity `|delta|`,
   not harmful), merging into an existing episode for the same actor+event+subject when
   `memory.CanMerge` allows. `applyRewards` writes a `QUEST_REWARD` episode (trust clamped, intensity
   0.3). `RELATIONSHIP_TRUST` preconditions compare against `npcstate.TrustToward` using
   `qm.Rules.Config`, falling back to `memory.DefaultConfig()` when `Rules` is nil.
5. `internal/app/app.go`: load `rules.Load(filepath.Join(cfg.GamedataDir, "events.json"))`, fail
   startup with a clear error, assign `questManager.Rules`. Leave the old `EmotionManager` wiring alone.
6. Tests: keep the existing quest tests passing by seeding trust as a `QUEST_REWARD` episode instead of
   `SetTrustLevel`; add cases for a +0.5 gift (computed trust 0.5) and a merged second identical gift
   (count ~2, computed trust below 1.0 because non-harmful repeats diminish); add an `npcstate` test for
   an old-style row.

## Step 4 — rewrite `gatherAIContext` (`backend-go/internal/api/handlers/game_handler.go`)

- Record the event as an episode first (merge, new episode, apology, or betrayal), **before** the
  rate-limit check, so a rate-limited action still counts and only the narration is skipped.
- An apology (`PLAYER_APOLOGIZED`) calls `memory.Forgive` for that actor, counting prior apologies, and
  stores the ids it forgave in `covers`. A harmful event from an actor with forgiven episodes calls
  `memory.RevokeForgiveness`, starts a new episode flagged `betrayal`, and adds the config's betrayal
  trust penalty.
- Conversation events store the question in `text` and never merge.
- Then compute, with `npcstate.Load` + the memory package: emotions toward the speaker, the general
  mood, and the memory lines — the speaker's own episodes ranked by `memory.Weight` (top 5, with counts
  and text), plus other actors' episodes at or above `NotabilityThreshold` (top 3), worded so the NPC can
  tell who did what ("You threw stones at me 5 times" vs "Someone threw stones at me").
- Delete the now-unused `EmotionManager` (`internal/domain/npc_logic/`) and `gamedata/event_emotions.json`,
  and their wiring.
- Tests: extend the `enttest`-based tests with the full demo sequence — five stones from P1, then P2
  arrives (P2 sees low anger toward them but a raised general mood and a line about someone else), then an
  apology, then a sixth stone (betrayal).

## Step 5 — contract and prompts

- `ai-service-python/proto/ai.proto` and `backend-go/proto/ai.proto` (keep them identical): replace
  `current_emotions` and `recent_memories` with `map<string, float> speaker_emotions`,
  `map<string, float> general_mood` and `repeated string memory_lines`.
- Regenerate: Python with `python -m grpc_tools.protoc` (grpcio-tools is in requirements-dev), Go with the
  same bundled protoc plus `protoc-gen-go` and `protoc-gen-go-grpc` installed via
  `GOWORK=off go install google.golang.org/protobuf/cmd/protoc-gen-go@latest` and
  `google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest` (they land in `~/go/bin`).
- Update the Go client/servicer and `ai-service-python/servicer.py`, `router.py`,
  `agents/context_formatter.py` and both prompts: two labelled lines ("Toward you:" / "Your general
  mood:") and the ranked memory lines; add `PLAYER_THREW_STONE` and `PLAYER_APOLOGIZED` to the agent
  path in `router.py`; mention the betrayal flag in the event prompt.
- The rule-based `"Get lost."` fallback uses the speaker's anger, not the general mood.
- Update the Python tests that touch these shapes.

## Step 6 — `EMOTIONS` frame and browser panel

- After each event the Go handler sends `{"action_type": "EMOTIONS", "content": "<json>"}` where the JSON
  holds both maps; document it in `docs/client_protocol.md`.
- `client-demo/index.html`: "Throw stone" and "Apologize" buttons, and a panel showing both maps, updated
  from that frame.

## Step 7 — demo verification (local machine only)

Needs Ollama (`llama3.1:8b`) and Docker, so it cannot run in the cloud. Script both moments end to end
against a throwaway stack (never the running compose stack's database): five stones from P1 → P2 gets a
curt greeting mentioning someone else → P1 returns to fury → apology → P1 calm but distrusted → a sixth
stone triggering betrayal. Both moments must come out right in 3 of 3 runs before the step is ticked.
