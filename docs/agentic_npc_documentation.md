# Agentic NPC Project Documentation

## 1. Project Architecture Overview
The Agentic NPC Framework is a high-performance distributed system for generative game AI. It powers autonomous, stateful, and memory-aware NPCs using Large Language Models (LLMs). The architecture is divided into two primary services working in tandem to balance real-time responsiveness and cognitive depth (the Two-Brain model).

### Core Components
1. **Go Orchestrator (The Dungeon Master)**:
   - **Role**: The central hub and authoritative source of truth for the game world.
   - **Communication**: Manages bidirectional communication with game clients (e.g., Unreal Engine) via WebSockets.
   - **State Machine & Logic**: Handles quest lifecycles, player inventories, NPC relationships, and emotions.
   - **Persistence**: Uses PostgreSQL (via Ent ORM) for relational data and Redis for cache-aside fast retrieval.
   - **Orchestration**: Validates game events and routes them to the Python AI service via gRPC.

2. **Python AI Service (The Brain)**:
   - **Role**: Encapsulates all LLM logic, inference, and cognitive processes.
   - **Two-Brain Model**:
     - *Fast Brain (RAG)*: Uses FAISS and LangChain for rapid, context-grounded responses to factual player queries (Lore).
     - *Complex Brain (LangGraph)*: Uses LangGraph to perform stateful multi-step reasoning, tool execution, and dynamic quest processing.
   - **Communication**: Exposes a gRPC server (`AIBrain`) to receive state context (emotions, memories, quest steps) and requests from the Go backend.

### Data Flow Example
1. Player asks a question in-game. Client sends a JSON event over WebSocket to the Go backend.
2. Go backend validates the player and target NPC, updates the NPC's emotions based on the event using `EmotionManager`, and fetches recent memories from PostgreSQL.
3. Go backend calls the Python AI service via gRPC (`Think` method), passing the question, emotions, and memories.
4. Python service routes the request using `AIBrainServicer` to the NPC's specific RAG agent.
5. RAG agent queries the FAISS vector store for lore and generates a response via the LLM.
6. Python service returns the response over gRPC to Go, which sends it back via WebSocket to the client.

---

## 2. Codebase Documentation: File by File

### Python AI Service (`ai-service-python/`)
This microservice is responsible for LLM inference, LangChain/LangGraph orchestration, and vector search.

#### Root Configuration and Entry
- **`main.py`**: The entry point for the Python service. It triggers the `agent_manager` to load all NPC agents into memory and starts the gRPC server listening on port 50051.
- **`servicer.py`**: Implements the gRPC endpoint `Think`. It acts as the **AI Router**, receiving the context from the Go backend and routing the event either to the RAG Agent (for simple questions) or the LangGraph Agent (for complex quest interactions like gifting or submitting items).
- **`agent_manager.py`**: Acts as a singleton registry. On startup, it scans the `gamedata/npcs` directory, instantiates an `NpcAgent` for every valid NPC, and keeps them in memory for fast access during gRPC calls.
- **`config.py`**: Initializes configuration variables, such as the LLM instances (using Google Gemini or Ollama models) and the embedding models required for the vector store.
- **`ai_pb2.py`, `ai_pb2_grpc.py`, `ai_pb2.pyi`**: Auto-generated Protocol Buffer bindings used for gRPC communication.

#### Agents & Core Logic (`ai-service-python/agents/`)
- **`npc_agent.py`**: Represents the "live" state of an NPC. It acts as a coordinator that holds the initialized RAG chain, LangGraph chain, and tools for a specific NPC character. It exposes `run_rag_agent` and `run_quest_agent` methods called by the servicer.
- **`rag_builder.py`**: Constructs the LangChain pipeline for the "Fast Brain". It combines the NPC's static prompt with a query to the vector store (`lore_retriever`) to quickly answer lore questions.
- **`graph_builder.py`**: Constructs the LangGraph workflow for the "Complex Brain". It sets up a stateful graph that processes complex player interactions, allowing the LLM to use a scratchpad and toolchain for multi-step reasoning.
- **`agent_state.py`**: Defines `LangGraphAgentState`, a `TypedDict` structure that dictates the state format passed around the nodes in LangGraph.
- **`context_formatter.py`**: Helper script to format the dynamic context (like current emotions, completion rate, recent memories) into a string block injected into the LLM prompt.
- **`prompt_loader.py`**: Utility to read and load the static personality and backstory JSON files into the system prompts.

#### Tools and Prompts
- **`tools/lore_retriever_tool.py`**: Reads the NPC's `lore.json`, splits the text using `RecursiveCharacterTextSplitter`, embeds it into a `FAISS` vector store, and returns both a raw retriever and a LangChain `@tool` (`lore_book_search`) for the agent to use dynamically.
- **`prompts/rag_prompt.py` & `prompts/react_prompt.py`**: Define the system instructions and ReAct templates for guiding the LLM's thought process.

---

### Go Backend Orchestrator (`backend-go/`)
The Go application acts as the real-time server, state machine, and bridge between the database, the client, and the AI.

#### Application Core (`backend-go/cmd/` & `internal/app/`)
- **`cmd/server/main.go`**: The main executable file. It instantiates the app, registers a deferred shutdown for cleanup, and starts the server.
- **`internal/app/app.go`**: The Dependency Injection hub. It initializes environment variables, database connections (PostgreSQL and Redis), seeds the DB with initial gamedata, initializes the logic managers (`QuestManager`, `EmotionManager`), and wires up the HTTP server and gRPC client.
- **`internal/config/config.go`**: Parses system environment variables into a structured format for the application.

#### API Layer (`backend-go/internal/api/`)
- **`router/router.go`**: Defines the Gin framework routes, primarily setting up the `/api/v1/ws` WebSocket endpoint and a health check.
- **`handlers/websocket_handler.go`**: Upgrades HTTP connections to WebSockets. It runs an event loop that reads incoming JSON events (`EventMessage`), manages client authentication state, and delegates game logic to `HandleGameEvent`.
- **`handlers/game_handler.go`**: The primary logic flow for game events. It does the following:
  1. Calls `QuestManager` to validate if an action is allowed (preconditions).
  2. Modifies the NPC's emotions using `EmotionManager`.
  3. Records the event as a new memory in the PostgreSQL database.
  4. Collects relationship and quest state.
  5. Triggers a gRPC call to the Python service with the newly updated context.
  6. Sends the AI's response back over the WebSocket.
- **`handlers/auth_handler.go`**: Handles player login/authentication over WebSockets.
- **`handlers/health_handler.go`**: Provides a simple 200 OK endpoint to verify the server is running.

#### Domain Logic (`backend-go/internal/domain/`)
- **`quest_logic/quest_manager.go`**: Serves as the "rulebook". It loads static definitions (items, quests) from disk into memory. Its `ProcessEvent` method acts as the gatekeeper, validating player actions against quest preconditions before any AI gets involved.
- **`quest_logic/event_handlers.go`, `quest_logic.go`, `gamedata_defs.go`, `cache_helpers.go`**: Contain specific domain logic for checking quest progression, resolving gifting logic, and handling Redis cache lookups.
- **`npc_logic/emotion_manager.go`**: Loads static emotion deltas from `event_emotions.json`. Modifies an NPC's core emotional state (Joy, Sadness, Anger, Fear, Trust) when specific events occur.
- **`user_logic/user_manager.go`**: Contains domain logic for managing user creation and queries.

#### Infrastructure and Persistence (`backend-go/internal/infra/` & `internal/db/`)
- **`infra/database/postgres.go` & `redis.go`**: Manage connection pools and client initializations for the respective databases.
- **`infra/database/seeder.go`**: A utility that runs on startup to populate the PostgreSQL database with the default static NPC and Quest data from the `gamedata` directory.
- **`infra/grpc_client/ai_client.go`**: A wrapper around the generated gRPC client that creates and formats the `ThinkRequest` sent to the Python microservice.
- **`infra/httpsapi/server.go`**: Sets up the Gin HTTP server configuration and starts it.
- **`db/ent/*`**: Contains all auto-generated code produced by the Ent ORM framework.
  - **`db/ent/schema/*`**: Contains the developer-defined schema declarations for the database tables: `Player`, `NPC`, `Quest`, `InventoryItem`, `Memory`, `PlayerQuestState`, and `PlayerNpcRelationship`.
  - The rest of the `ent` directory contains generated CRUD methods and query builders used heavily in `game_handler.go` and `quest_manager.go`.

#### Shared Types (`backend-go/internal/dto/` & `internal/proto/`)
- **`dto/event_dto.go`**: Defines the JSON-serializable structs (like `EventMessage`) used for the WebSocket communication between the Go server and the game client.
- **`proto/ai.pb.go` & `proto/ai_grpc.pb.go`**: The Go bindings generated from `ai.proto`, used to communicate with the Python service.

### Shared Gamedata and Definitions
- **`gamedata/`**: A crucial directory containing the static JSON content that dictates game state and behavior. It is read by both the Go and Python components:
  - `npcs/<name>/`: Contains `backstory.json`, `lore.json`, and `personality.json` defining the persona of each NPC.
  - `quests/` & `items.json`: Define static game rules, item IDs, and quest logic constraints (read by Go).
  - `event_emotions.json`: Defines how specific game events numerically alter an NPC's emotion state (read by Go).
- **`proto/ai.proto`**: The Protocol Buffer schema file defining the contract between the Go Orchestrator and the Python AI Service. It specifies the `AIBrain` service and the structure of `ThinkRequest` and `ActionResponse`.
