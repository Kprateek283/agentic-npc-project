# WebSocket Client Protocol

The Go orchestrator exposes a single bidirectional WebSocket endpoint that a game client
(designed for an Unreal Engine 5 client; any WebSocket client works) uses to authenticate a
player and drive NPC interactions. This document is the contract: it matches
`internal/dto/event_dto.go`, `internal/api/handlers/auth_handler.go`, and
`internal/api/handlers/game_handler.go` exactly. A minimal browser reference client that
implements it lives in [`client-demo/`](../client-demo/).

## Endpoint

```
ws://<host>:<SERVER_PORT>/api/v1/ws      # SERVER_PORT defaults to 8080
```

No subprotocol, no query params. All origins are accepted (dev setting).

## Message envelope

Every message the client sends is one JSON object (`EventMessage`). Only the fields
relevant to a given `event_type` need to be set; the rest are omitted.

| Field              | Type   | Used by                                             |
|--------------------|--------|-----------------------------------------------------|
| `event_type`       | string | every message — selects the handler                 |
| `username`         | string | `REGISTER_PLAYER`, `LOGIN_PLAYER`                    |
| `password`         | string | `REGISTER_PLAYER`, `LOGIN_PLAYER`                    |
| `target_npc_name`  | string | every in-game event — the NPC display name (e.g. `Elara`) |
| `question_text`    | string | `PLAYER_ASKED_QUESTION`                              |
| `keyword`          | string | `PLAYER_GAVE_GIFT`, `PLAYER_SUBMITTED_QUEST_ITEM` (item name) |

`source_entity_id` exists on the struct but is **server-controlled**: the orchestrator
overwrites it with the authenticated player's ID on every in-game event. Clients never set it.

Every message the server sends is one JSON object:

```json
{ "action_type": "SPEAK", "content": "..." }
```

`action_type` is one of `LOGIN_SUCCESS`, `SPEAK`, `SPEAK_PARTIAL`, `ADMIN_ACK`, `ERROR`,
or an action type carried by a quest fail-response (data-defined in the quest JSON,
commonly `SPEAK`). `content` is the human-readable payload — NPC dialogue, a welcome line,
or an error message.

### Streamed replies (`SPEAK_PARTIAL` → `SPEAK`)

NPC dialogue is streamed token-by-token. For each AI-backed event the server sends zero or
more `SPEAK_PARTIAL` frames whose `content` is an incremental text delta, followed by one
final `SPEAK` frame whose `content` is the **whole** reply:

```json
{ "action_type": "SPEAK_PARTIAL", "content": "Marcus's " }
{ "action_type": "SPEAK_PARTIAL", "content": "guard, that's who." }
{ "action_type": "SPEAK",         "content": "Marcus's guard, that's who." }
```

A streaming client appends each `SPEAK_PARTIAL` delta to the live bubble; on the closing
`SPEAK` it replaces the bubble with the authoritative full text. A client that does not
care about progressive rendering can **ignore `SPEAK_PARTIAL` entirely** and act only on
the final `SPEAK` — the full reply always arrives there. Non-AI replies (`ADMIN_ACK`,
quest fail-responses, the non-LLM `"Greetings."`) are sent as a single frame with no
preceding partials.

## Session flow

### 1. Authenticate (required first)

Until the connection is authenticated, the server accepts **only** `REGISTER_PLAYER` and
`LOGIN_PLAYER`; any other event returns an `ERROR` ("authentication required…").

```json
// Register (auto-logs-in on success — falls through to login)
{ "event_type": "REGISTER_PLAYER", "username": "aria", "password": "secret123" }

// Or log in an existing player
{ "event_type": "LOGIN_PLAYER", "username": "aria", "password": "secret123" }
```

Success response (the connection is now authenticated for its lifetime):

```json
{ "action_type": "LOGIN_SUCCESS", "content": "Welcome, aria!" }
```

Failure (bad credentials, duplicate registration, …):

```json
{ "action_type": "ERROR", "content": "<reason>" }
```

### 2. Converse and interact (after authentication)

| `event_type`                 | Extra fields                     | Brain / result |
|------------------------------|----------------------------------|----------------|
| `PLAYER_ASKED_QUESTION`      | `target_npc_name`, `question_text` | RAG (Fast Brain) → `SPEAK` grounded answer |
| `PLAYER_GAVE_GIFT`           | `target_npc_name`, `keyword`       | LangGraph agent → `SPEAK` (or quest fail-response) |
| `PLAYER_SUBMITTED_QUEST_ITEM`| `target_npc_name`, `keyword`       | LangGraph agent → `SPEAK` (or quest fail-response) |
| `PLAYER_INTERACT`            | `target_npc_name`                  | LangGraph agent → `SPEAK` |
| `PLAYER_INTERACT_QUEST`      | `target_npc_name`                  | LangGraph agent → `SPEAK` |
| `PLAYER_ATTACKED`            | `target_npc_name`                  | LangGraph agent → `SPEAK` |
| `PLAYER_LOOKED_AT_NPC`       | `target_npc_name`                  | Non-LLM emotion rule → `SPEAK` `"Greetings."` (`"Get lost."` if the NPC's anger > 0.7) |
| `ADMIN_SET_TRUST`            | `target_npc_name`, `keyword`       | State mutation → `ADMIN_ACK` |
| `ADMIN_SET_QUEST_STAGE`      | `target_npc_name`, `keyword`       | State mutation → `ADMIN_ACK` |

Example question → answer:

```json
// client →
{ "event_type": "PLAYER_ASKED_QUESTION", "target_npc_name": "Elara",
  "question_text": "What do you know about the old tower?" }
// server →
{ "action_type": "SPEAK", "content": "The old tower... I've heard whispers..." }
```

Every event first passes through the quest manager (precondition checks, emotion update,
memory write, cache-aside player/NPC lookups). If a quest precondition fails, the server
returns that quest's fail-response instead of calling the AI. Otherwise the orchestrator
gathers context (emotions, recent memories, player-specific trust, quest state) and calls
the Python AI service over gRPC, then relays its `action_type`/`content`.

### Errors

Any handler error (NPC not found, AI service unavailable, auth failure) returns:

```json
{ "action_type": "ERROR", "content": "<message>" }
```

An unrecognized `event_type` from an authenticated client is not an error — it falls
through to the non-LLM emotion rule and yields `"Greetings."`/`"Get lost."`.
