# Agentic NPC Framework - Architecture Diagram

## High-Level System Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                          GAME CLIENT                                 │
│                      (Unreal Engine 5)                               │
│                                                                       │
│  ┌────────────┐  ┌─────────────┐  ┌───────────────┐                │
│  │   Player   │  │  NPC Actors │  │  UI Widgets   │                │
│  │  Character │  │  (Blueprint)│  │  (Dialogue)   │                │
│  └────────────┘  └─────────────┘  └───────────────┘                │
└───────────────────────────┬─────────────────────────────────────────┘
                            │
                            │ WebSocket
                            │ (JSON Events)
                            │ Port 8080
                            ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      GO BACKEND SERVICE                              │
│                    (Orchestrator + Game Logic)                       │
│                                                                       │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │                    API LAYER                                  │  │
│  │  ┌───────────────┐  ┌──────────────┐  ┌─────────────────┐  │  │
│  │  │  WebSocket    │  │   REST API   │  │  Health Check   │  │  │
│  │  │   Handler     │  │   Handler    │  │     Handler     │  │  │
│  │  └───────────────┘  └──────────────┘  └─────────────────┘  │  │
│  └──────────────┬───────────────────────────────────────────────┘  │
│                 │                                                    │
│  ┌──────────────▼───────────────────────────────────────────────┐  │
│  │                   DOMAIN LAYER                                │  │
│  │  ┌────────────────┐  ┌─────────────────┐                     │  │
│  │  │ Quest Manager  │  │ Emotion Manager │                     │  │
│  │  │  (Game Rules)  │  │ (State Updates) │                     │  │
│  │  └────────────────┘  └─────────────────┘                     │  │
│  └──────────────┬───────────────────────────────────────────────┘  │
│                 │                                                    │
│  ┌──────────────▼───────────────────────────────────────────────┐  │
│  │                 INFRASTRUCTURE LAYER                          │  │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────────┐               │  │
│  │  │   Ent    │  │  Redis   │  │  gRPC Client │               │  │
│  │  │  (ORM)   │  │  Client  │  │   (AI Comm)  │               │  │
│  │  └──────────┘  └──────────┘  └──────────────┘               │  │
│  └──────┬──────────────┬───────────────┬────────────────────────┘  │
└─────────┼──────────────┼───────────────┼───────────────────────────┘
          │              │               │
          │              │               │ gRPC (Port 50051)
          │              │               │ Protocol Buffers
          │              │               ▼
          │              │      ┌─────────────────────────────────────┐
          │              │      │   PYTHON AI SERVICE                 │
          │              │      │   (LLM Inference + Agent Logic)     │
          │              │      │                                     │
          │              │      │  ┌────────────────────────────┐    │
          │              │      │  │   gRPC Servicer            │    │
          │              │      │  │   (Request Router)         │    │
          │              │      │  └──────────┬─────────────────┘    │
          │              │      │             │                       │
          │              │      │  ┌──────────▼──────────────────┐   │
          │              │      │  │   Agent Manager             │   │
          │              │      │  │   (Agent Registry)          │   │
          │              │      │  └──────────┬──────────────────┘   │
          │              │      │             │                       │
          │              │      │  ┌──────────▼──────────────────┐   │
          │              │      │  │   NPC Agent Instances       │   │
          │              │      │  │  ┌──────────────────────┐   │   │
          │              │      │  │  │  RAG Chain           │   │   │
          │              │      │  │  │  (Simple Questions)  │   │   │
          │              │      │  │  └──────────────────────┘   │   │
          │              │      │  │  ┌──────────────────────┐   │   │
          │              │      │  │  │  LangGraph Agent     │   │   │
          │              │      │  │  │  (Complex Quests)    │   │   │
          │              │      │  │  └──────────────────────┘   │   │
          │              │      │  └─────────────────────────────┘   │
          │              │      │             │                       │
          │              │      │  ┌──────────▼──────────────────┐   │
          │              │      │  │   LLM Providers             │   │
          │              │      │  │  • Gemini 3.5 Flash (Cloud) │   │
          │              │      │  │  • Llama 3 (Local/Ollama)   │   │
          │              │      │  └─────────────────────────────┘   │
          │              │      │             │                       │
          │              │      │  ┌──────────▼──────────────────┐   │
          │              │      │  │   Vector Store (FAISS)      │   │
          │              │      │  │   (Lore Embeddings)         │   │
          │              │      │  └─────────────────────────────┘   │
          │              │      └─────────────────────────────────────┘
          │              │
          │              │ Port 6379
          │              ▼
          │      ┌────────────────┐
          │      │     REDIS      │
          │      │  (Caching)     │
          │      │                │
          │      │ • Player State │
          │      │ • NPC Cache    │
          │      │ • Session Data │
          │      └────────────────┘
          │
          │ Port 5432
          ▼
┌─────────────────────────────────────────────────────────────────────┐
│                          POSTGRESQL                                  │
│                     (Persistent Storage)                             │
│                                                                       │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌──────────────┐ │
│  │   Player   │  │    NPC     │  │   Memory   │  │    Quest     │ │
│  │   Table    │  │   Table    │  │   Table    │  │    Table     │ │
│  └────────────┘  └────────────┘  └────────────┘  └──────────────┘ │
│                                                                       │
│  ┌────────────────────────┐  ┌──────────────────────────────────┐  │
│  │ PlayerNPCRelationship  │  │   PlayerQuestState               │  │
│  │        Table           │  │        Table                     │  │
│  └────────────────────────┘  └──────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Data Flow Diagram: Player Asks Question

```
┌──────────┐
│  Player  │ "Who is the mayor?"
└────┬─────┘
     │
     │ 1. WebSocket Message
     │    { event_type: "PLAYER_ASKED_QUESTION",
     │      question_text: "Who is the mayor?",
     │      target_npc_name: "Elara" }
     │
     ▼
┌─────────────────────────────────────────────────────────────────┐
│                    GO BACKEND                                    │
│                                                                   │
│  2. WebSocket Handler                                            │
│     ├─ Validate authentication ✓                                │
│     ├─ Parse event JSON                                          │
│     └─ Route to HandleGameEvent()                                │
│                                                                   │
│  3. Game Handler                                                 │
│     ├─ Fetch NPC "Elara" from Redis/PostgreSQL                  │
│     │  (Cache Hit: 70% chance, ~5ms)                             │
│     │  (Cache Miss: Query PostgreSQL, ~20ms)                     │
│     │                                                             │
│     ├─ Update NPC Emotions                                       │
│     │  • Emotion Manager: Apply event deltas                     │
│     │  • Save to PostgreSQL                                      │
│     │                                                             │
│     ├─ Create Memory Record                                      │
│     │  "PlayerX asked question to Elara"                         │
│     │  • Insert into Memory table                                │
│     │                                                             │
│     ├─ Fetch Recent Memories (LIMIT 5)                           │
│     │  • Query NPC's memories, ORDER BY created_at DESC          │
│     │                                                             │
│     ├─ Get Player-NPC Relationship                               │
│     │  • Fetch trust_level from PlayerNPCRelationship            │
│     │                                                             │
│     └─ Prepare gRPC Request                                      │
│        ┌────────────────────────────────────────────────┐        │
│        │ EventRequest Proto:                            │        │
│        │  • personality_path: "gamedata/npcs/elara/..." │        │
│        │  • current_emotions: { joy: 0.7, anger: 0.2 } │        │
│        │  • recent_memories: [Memory1, Memory2, ...]    │        │
│        │  • event_type: "PLAYER_ASKED_QUESTION"         │        │
│        │  • question_text: "Who is the mayor?"          │        │
│        └────────────────┬───────────────────────────────┘        │
└─────────────────────────┼────────────────────────────────────────┘
                          │
                          │ 4. gRPC Call (Binary, HTTP/2)
                          │    Serialized Protocol Buffer
                          │    ~1-2KB payload
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│                    PYTHON AI SERVICE                             │
│                                                                   │
│  5. AIBrainServicer.Think()                                      │
│     ├─ Receive EventRequest                                      │
│     ├─ Extract personality_path as agent_key                     │
│     └─ Route based on event_type → "PLAYER_ASKED_QUESTION"      │
│                                                                   │
│  6. Agent Manager                                                │
│     ├─ Lookup agent in live_agents registry                      │
│     │  agent = live_agents["gamedata/npcs/elara/personality.json"]│
│     └─ Return NpcAgent instance (pre-loaded)                     │
│                                                                   │
│  7. NpcAgent.run_rag_agent()                                     │
│     ├─ Prepare input dict: { "question": "Who is the mayor?" }  │
│     └─ Invoke RAG chain                                          │
│        ┌──────────────────────────────────────────────┐         │
│        │         RAG CHAIN                             │         │
│        │                                               │         │
│        │  8a. Embed Question                           │         │
│        │      └─ OllamaEmbeddings (nomic-embed-text)  │         │
│        │         Returns: [0.23, -0.45, 0.12, ...]    │         │
│        │                                               │         │
│        │  8b. Vector Search                            │         │
│        │      └─ FAISS.similarity_search()            │         │
│        │         Top-1 result from lore database       │         │
│        │         "Elara is the herbalist, mayor is..." │         │
│        │                                               │         │
│        │  8c. Build Prompt                             │         │
│        │      System: "You are Elara, herbalist..."   │         │
│        │      Context: [Retrieved lore facts]         │         │
│        │      Human: "Who is the mayor?"               │         │
│        │                                               │         │
│        │  8d. Call LLM                                 │         │
│        │      └─ ChatGoogleGenerativeAI (Gemini)      │         │
│        │         Temperature: 0.7                      │         │
│        │         Model: gemini-3.5-flash               │         │
│        │                                               │         │
│        │  8e. Parse Output                             │         │
│        │      └─ StrOutputParser()                     │         │
│        │         "The mayor is Marcus, he leads..."    │         │
│        └──────────────────────────────────────────────┘         │
│                          │                                        │
│  9. Return Response                                              │
│     └─ ActionResponse Proto:                                     │
│        { action_type: "SPEAK",                                   │
│          content: "The mayor is Marcus, he leads..." }           │
└─────────────────────────┬────────────────────────────────────────┘
                          │
                          │ 10. gRPC Response (Binary)
                          │     Serialized ActionResponse
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│                    GO BACKEND                                    │
│                                                                   │
│  11. Receive gRPC Response                                       │
│      ├─ Deserialize ActionResponse                              │
│      └─ Extract content string                                   │
│                                                                   │
│  12. Send WebSocket Response                                     │
│      └─ JSON: { "action_type": "SPEAK",                          │
│                 "content": "The mayor is Marcus..." }            │
└─────────────────────────┬────────────────────────────────────────┘
                          │
                          │ 13. WebSocket Message (JSON)
                          ▼
                    ┌──────────┐
                    │  Player  │ Sees NPC dialogue
                    └──────────┘

Total Latency: ~300-500ms
  • Go handler: 50ms (DB + memory creation)
  • gRPC call: 5ms (network)
  • Python RAG: 200-300ms (embedding + LLM)
  • gRPC response: 5ms
  • Go response: 10ms (WebSocket send)
```

---

## Component Interaction: Quest Completion

```
┌────────┐
│ Player │ Submits "Ancient Herb"
└───┬────┘
    │
    ▼
┌──────────────────────────────────────────────────────────┐
│              GO BACKEND: QUEST MANAGER                    │
│                                                            │
│  1. Process Event                                          │
│     ├─ Get Player from DB                                 │
│     ├─ Get NPC "Elara" from DB                            │
│     ├─ Get PlayerQuestState (current_step = 2)            │
│     │                                                      │
│     └─ checkQuestCompletion()                             │
│        ├─ Load quest definition from memory cache         │
│        │  (Loaded at startup from gamedata/quests/)       │
│        │                                                   │
│        ├─ Get current step definition                     │
│        │  Step 2: "requires": ["herb_ancient"]            │
│        │                                                   │
│        ├─ Validate item in player's inventory             │
│        │  ✓ Player has "herb_ancient"                     │
│        │                                                   │
│        ├─ Remove item from inventory                      │
│        │  DELETE FROM InventoryItem WHERE ...             │
│        │                                                   │
│        ├─ Update quest state                              │
│        │  current_step: 2 → 3                             │
│        │  completion_rate: 0.66 → 1.0                     │
│        │  is_completed: true                               │
│        │                                                   │
│        └─ Return nil (no fail response)                   │
│                                                            │
│  2. Route to AI                                            │
│     └─ callAI() with quest context                        │
│        • current_quest_step: 3                             │
│        • completion_rate: 1.0                              │
└────────────────────────────┬───────────────────────────────┘
                             │
                             ▼
┌──────────────────────────────────────────────────────────┐
│              PYTHON AI SERVICE                            │
│                                                            │
│  3. Route to LangGraph Agent                              │
│     (Complex quest logic, not simple RAG)                 │
│                                                            │
│  4. NpcAgent.run_quest_agent()                            │
│     ├─ Format dynamic context (emotions, memories, quest)│
│     ├─ Add tools: [lore_book_search]                     │
│     └─ Invoke LangGraph chain                             │
│        ┌────────────────────────────────────────┐        │
│        │      LANGGRAPH AGENT                    │        │
│        │                                          │        │
│        │  Input: "Player submitted Ancient Herb" │        │
│        │  Context: "Quest step 3 (complete!)"    │        │
│        │           "Completion: 100%"            │        │
│        │                                          │        │
│        │  Node: "thinker"                        │        │
│        │    └─ Prompt with quest context         │        │
│        │    └─ LLM generates response            │        │
│        │    "Ah! The herb! You did it! [reward]"│        │
│        │                                          │        │
│        │  Output: agent_outcome string           │        │
│        └────────────────────────────────────────┘        │
│                                                            │
│  5. Return ActionResponse                                 │
│     { action_type: "SPEAK",                               │
│       content: "Ah! The herb!..." }                       │
└────────────────────────────┬───────────────────────────────┘
                             │
                             ▼
                      ┌────────────┐
                      │   PLAYER   │ Quest Completed!
                      └────────────┘
```

---

## Deployment Architecture (Docker Compose)

```
┌─────────────────────────────────────────────────────────────┐
│                       HOST MACHINE                           │
│                                                               │
│  ┌────────────────────────────────────────────────────────┐ │
│  │              DOCKER COMPOSE NETWORK                     │ │
│  │              (bridge mode)                              │ │
│  │                                                          │ │
│  │  ┌────────────────────┐       ┌────────────────────┐  │ │
│  │  │   PostgreSQL:15    │       │    Redis:7         │  │ │
│  │  │   Container        │       │    Container       │  │ │
│  │  │                    │       │                    │  │ │
│  │  │  Port: 5432        │       │  Port: 6379        │  │ │
│  │  │  Volume: postgres_ │       │  In-memory         │  │ │
│  │  │          data      │       │                    │  │ │
│  │  └────────────────────┘       └────────────────────┘  │ │
│  │                                                          │ │
│  └──────────────────────────────────────────────────────────┘ │
│                                                               │
│  ┌────────────────────────────────────────────────────────┐ │
│  │        Python AI Service (Manual)                       │ │
│  │        Process ID: [running on venv]                    │ │
│  │        Port: 50051 (gRPC)                               │ │
│  │        Dependencies: requirements.txt (77 packages)     │ │
│  └────────────────────────────────────────────────────────┘ │
│                                                               │
│  ┌────────────────────────────────────────────────────────┐ │
│  │        Go Backend Service (Manual)                      │ │
│  │        Binary: ./cmd/server/main.go                     │ │
│  │        Port: 8080 (HTTP + WebSocket)                    │ │
│  │        Env: .env file (POSTGRES_DSN, REDIS_ADDR)        │ │
│  └────────────────────────────────────────────────────────┘ │
│                                                               │
│  ┌────────────────────────────────────────────────────────┐ │
│  │        Ollama (Optional, for local LLM)                 │ │
│  │        Port: 11434                                       │ │
│  │        Models: llama3:8b, nomic-embed-text              │ │
│  └────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘

External Connections:
  • Gemini API (Cloud): api.google.com (HTTPS)
  • Unreal Client: localhost:8080 (WebSocket)
```

---

## Scaling Architecture (Production Vision)

```
                           ┌──────────────┐
                           │  Cloud CDN   │
                           └──────┬───────┘
                                  │
                     ┌────────────▼────────────┐
                     │   API Gateway (Kong)    │
                     │  • Rate Limiting        │
                     │  • JWT Validation       │
                     │  • TLS Termination      │
                     └────────────┬────────────┘
                                  │
            ┌─────────────────────┼─────────────────────┐
            │                     │                     │
    ┌───────▼────────┐   ┌───────▼────────┐   ┌───────▼────────┐
    │ Go Backend     │   │ Go Backend     │   │ Go Backend     │
    │ Pod #1         │   │ Pod #2         │   │ Pod #3         │
    │ (Auto-scaling) │   │ (Auto-scaling) │   │ (Auto-scaling) │
    └───────┬────────┘   └───────┬────────┘   └───────┬────────┘
            │                     │                     │
            └─────────────────────┼─────────────────────┘
                                  │
                                  │ gRPC Load Balancer
                                  │
            ┌─────────────────────┼─────────────────────┐
            │                     │                     │
    ┌───────▼────────┐   ┌───────▼────────┐   ┌───────▼────────┐
    │ Python AI      │   │ Python AI      │   │ Python AI      │
    │ Pod #1         │   │ Pod #2         │   │ Pod #3-N       │
    │                │   │                │   │ (GPU nodes)    │
    └───────┬────────┘   └───────┬────────┘   └───────┬────────┘
            │                     │                     │
            └─────────────────────┼─────────────────────┘
                                  │
                 ┌────────────────┼────────────────┐
                 │                │                │
        ┌────────▼───────┐  ┌────▼──────┐  ┌─────▼──────────┐
        │ PostgreSQL     │  │  Redis    │  │  Qdrant        │
        │ (Primary)      │  │  Cluster  │  │  (Vector DB)   │
        │    +           │  │  (Sharded)│  │  (Distributed) │
        │ Read Replicas  │  └───────────┘  └────────────────┘
        └────────────────┘

Observability Stack:
  • Prometheus → Metrics collection
  • Grafana → Dashboards
  • Loki → Log aggregation
  • Jaeger → Distributed tracing
  • AlertManager → On-call alerts
```

---

## Security Layers

```
┌────────────────────────────────────────────────────────────┐
│                     EXTERNAL CLIENT                         │
└────────────────────────────┬───────────────────────────────┘
                             │
                             │ ① TLS/SSL Encryption
                             ▼
┌────────────────────────────────────────────────────────────┐
│                      API GATEWAY                            │
│  • ② Rate Limiting (100 req/min per IP)                    │
│  • ③ JWT Validation (Bearer token)                         │
│  • ④ WAF Rules (SQL injection, XSS)                        │
└────────────────────────────┬───────────────────────────────┘
                             │
                             │ ⑤ Internal Network (VPC)
                             ▼
┌────────────────────────────────────────────────────────────┐
│                      GO BACKEND                             │
│  • ⑥ Input Validation (DTO schemas)                        │
│  • ⑦ Authorization (player can only modify their data)     │
│  • ⑧ Prepared Statements (Ent ORM - SQL injection proof)   │
└────────────────────────────┬───────────────────────────────┘
                             │
                             │ ⑨ mTLS (Service Mesh)
                             ▼
┌────────────────────────────────────────────────────────────┐
│                   PYTHON AI SERVICE                         │
│  • ⑩ Prompt Sanitization (remove injection attempts)       │
│  • ⑪ Output Validation (regex checks)                      │
│  • ⑫ LLM Guardrails (content filtering)                    │
└────────────────────────────┬───────────────────────────────┘
                             │
                             │ ⑬ API Key Rotation
                             ▼
┌────────────────────────────────────────────────────────────┐
│                    GEMINI API (External)                    │
│  • ⑭ Rate Limiting (API quota)                             │
│  • ⑮ Content Safety Filters                                │
└────────────────────────────────────────────────────────────┘

Database Security:
  • ⑯ Encryption at Rest (PostgreSQL AES-256)
  • ⑰ Encryption in Transit (TLS connections)
  • ⑱ Least Privilege (service accounts with minimal permissions)
  • ⑲ Audit Logging (all writes logged)
  • ⑳ Backup Encryption (automated daily backups)
```

---

## Key Metrics & Monitoring

```
┌───────────────────────────────────────────────────────────┐
│                    PROMETHEUS METRICS                      │
├───────────────────────────────────────────────────────────┤
│                                                            │
│  Go Backend Metrics:                                       │
│    • websocket_connections_total (gauge)                  │
│    • http_request_duration_seconds (histogram)            │
│    • db_query_duration_seconds (histogram)                │
│    • redis_cache_hit_rate (gauge)                         │
│    • grpc_client_requests_total (counter)                 │
│                                                            │
│  Python AI Metrics:                                        │
│    • ai_inference_duration_seconds (histogram)            │
│    • rag_retrieval_duration_seconds (histogram)           │
│    • llm_tokens_used_total (counter)                      │
│    • agent_errors_total (counter)                         │
│                                                            │
│  Database Metrics:                                         │
│    • postgres_connections_active (gauge)                  │
│    • postgres_query_duration_seconds (histogram)          │
│    • redis_memory_usage_bytes (gauge)                     │
│                                                            │
│  Business Metrics:                                         │
│    • npc_interactions_total (counter)                     │
│    • quests_completed_total (counter)                     │
│    • players_active_current (gauge)                       │
└───────────────────────────────────────────────────────────┘

SLOs (Service Level Objectives):
  • Availability: 99.9% uptime
  • Latency: P95 < 500ms for RAG queries
  • Error Rate: < 0.1% of requests
  • Cache Hit Rate: > 70%
```

---

This architecture is designed for:
✅ **High Performance**: gRPC, Redis caching, pre-loaded agents
✅ **Scalability**: Stateless services, horizontal scaling ready
✅ **Maintainability**: Clean architecture, dependency injection
✅ **Reliability**: Error handling, graceful degradation
✅ **Observability**: Metrics, logs, tracing (production-ready)

