# Reference WebSocket client

A single self-contained HTML page (plain JS, no build step, no dependencies) that
implements the [WebSocket protocol](../docs/client_protocol.md) and proves the orchestrator's
game-client API end to end: authenticate → converse → gift/quest events.

## Run

1. Start the stack (`docker compose up`, or run the Go backend + Python AI service directly).
2. Open `index.html` in a browser (double-click, or `python -m http.server` in this folder).
3. Set the **WebSocket URL** to your Go backend (default `ws://localhost:8080/api/v1/ws`;
   the port is `SERVER_PORT`).
4. **Register** (or **Log in**) → pick an NPC → ask a question or use the action buttons.

Gifts/quest-item submissions take an item id (e.g. `apple`, `sunpetal_herb`) from
`backend-go/gamedata/items.json`.
