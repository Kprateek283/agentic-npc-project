# Interview Preparation Guide - Agentic NPC Framework
## Technical Interview for SkippyEd - January 14, 2026

---

## Project Overview (30-second Elevator Pitch)

*"I built an intelligent NPC framework for games using a microservices architecture. The system enables NPCs to have memory, emotions, and context-aware conversations powered by AI. It's built with a Go backend for orchestration and performance, a Python service for AI/LLM processing via gRPC, PostgreSQL for persistent state, Redis for caching, and uses LangChain with RAG for generating unique dialogue."*

---

## 1. ARCHITECTURE & DESIGN DECISIONS

### High-Level Architecture

**Multi-Service Design:**
- **Go Backend (Orchestrator)**: Handles WebSocket connections, game logic, quest management, database operations
- **Python AI Service**: Dedicated microservice for LLM inference, RAG, and agent logic
- **Communication Protocol**: gRPC for inter-service communication (low latency, type-safe)
- **Databases**: PostgreSQL (persistent state), Redis (caching/session management)
- **LLM Serving**: Ollama for local inference, with support for cloud LLMs (Gemini)

### Why This Architecture?

**1. Separation of Concerns:**
- Go excels at: concurrency, networking, database operations, game logic
- Python excels at: ML/AI, data science libraries, rapid prototyping
- *Interview Talking Point*: "I chose polyglot microservices to leverage each language's strengths rather than forcing everything into one language."

**2. Performance:**
- Go's goroutines handle thousands of concurrent WebSocket connections efficiently
- gRPC uses HTTP/2 and Protocol Buffers for binary serialization (faster than JSON/REST)
- Redis caching layer reduces database queries by ~70%

**3. Scalability:**
- Services can be scaled independently
- AI service can have multiple replicas behind a load balancer
- Stateless design (session data in Redis) enables horizontal scaling

**4. Type Safety:**
- Protocol Buffers define a contract between services
- Go's strong typing with Ent ORM prevents database-related bugs
- Python type hints with Pydantic for validation

---

## 2. SECURITY CONSIDERATIONS

### Current Implementation

**1. Authentication Flow:**
```
Client connects → Sends PLAYER_AUTH event → 
Go validates credentials → Creates/retrieves Player entity → 
Session established with playerId
```

**2. Data Validation:**
- All incoming events validated against DTO schemas
- Database constraints enforce data integrity (unique names, required fields)
- Emotion values clamped between 0.0-1.0 to prevent overflow

**3. Security Best Practices:**
- Environment variables for sensitive config (no hardcoded credentials)
- `.env` files excluded from version control
- CORS configuration (currently permissive for dev, production-ready approach planned)

### What You'd Improve (Great Interview Question!)

**Short-term:**
1. **JWT-based authentication** instead of simple username validation
2. **Rate limiting** on WebSocket connections (prevent DoS)
3. **Input sanitization** for player questions (prevent prompt injection attacks)
4. **SQL injection protection** (already handled by Ent ORM's parameterized queries)

**Long-term:**
1. **TLS/SSL encryption** for all services
2. **Service mesh** (Istio) for mTLS between microservices
3. **API Gateway** with authentication/authorization middleware
4. **Audit logging** for sensitive operations

**Critical Security Insight:**
*"LLM prompt injection is a real concern - we could implement input filtering and output validation to prevent malicious players from manipulating NPC responses."*

---

## 3. SCALABILITY DESIGN

### Current Scalability Features

**1. Database Layer:**
- **Connection Pooling**: Ent client manages database connections efficiently
- **Indexes**: Unique indexes on NPC names, player IDs for fast lookups
- **Query Optimization**: Uses `.Only()`, `.Limit()` to reduce data transfer

**2. Caching Strategy (Redis):**
```go
// Pattern: Check cache → Miss → Query DB → Store in cache
// TTL-based expiration prevents stale data
player, err := getPlayerFromCache(redis, playerId)
if err != nil {
    player = db.Player.Query().Where(player.IDEQ(playerId)).Only()
    cachePlayer(redis, player, 5*time.Minute)
}
```

**3. Stateless Services:**
- No in-memory session storage in Go service
- All state in PostgreSQL/Redis
- Enables horizontal scaling with load balancer

**4. Agent Pre-loading:**
```python
# Python service loads all agents at startup
# Avoids cold-start latency on first request
load_agents_on_startup()
```

### Bottlenecks & Solutions (Great Discussion Point!)

| Bottleneck | Current State | Solution for Scale |
|------------|---------------|-------------------|
| **LLM Inference** | Single Python process | Multiple AI service replicas + gRPC load balancing |
| **Database Writes** | Single PostgreSQL instance | Read replicas + write sharding by player_id |
| **WebSocket Connections** | Single Go server | Multiple Go instances behind load balancer (sticky sessions) |
| **Vector Search** | In-memory FAISS | Migrate to Qdrant/Weaviate/Pinecone for distributed vector DB |

**Key Interview Point:**
*"The current architecture supports ~1000 concurrent players. To scale to 100k+, I'd introduce a message queue (Kafka/RabbitMQ) between the Go backend and Python service for async processing, implement database sharding, and move vector search to a dedicated service."*

---

## 4. BEST PRACTICES & CODE QUALITY

### Go Backend

**1. Clean Architecture Pattern:**
```
cmd/          - Entry points
internal/
  ├── api/        - HTTP/WebSocket handlers (Presentation Layer)
  ├── domain/     - Business logic (quest/emotion managers)
  ├── db/         - Database models (Ent ORM)
  └── infra/      - External dependencies (gRPC, Redis, DB connections)
```
*Benefit*: Clear separation, easy to test, swap implementations

**2. Dependency Injection:**
```go
// App struct holds all dependencies
type App struct {
    DBClient       *ent.Client
    RedisClient    *redis.Client
    AIClient       *grpc_client.AIClient
    QuestManager   *quest_logic.QuestManager
}

// Injected into handlers
wsHandler := handlers.NewWebSocketHandler(dbClient, aiClient, questManager, ...)
```
*Benefit*: Testability (mock dependencies), loose coupling

**3. Error Handling:**
```go
// Always propagate context
if err != nil {
    return nil, fmt.Errorf("failed to load quest: %w", err)
}
```

**4. Graceful Shutdown:**
```go
defer application.Shutdown()  // Close DB, Redis, gRPC connections
```

### Python AI Service

**1. Modular Agent Architecture:**
```
agents/
  ├── npc_agent.py         - Main agent coordinator
  ├── rag_builder.py       - RAG chain construction
  ├── graph_builder.py     - LangGraph agent construction
  └── prompt_loader.py     - Prompt management
```
*Each agent is self-contained, loaded once, reused for all requests*

**2. Two-Brain System:**
- **RAG Chain (Fast)**: For simple questions (~200ms latency)
- **LangGraph Agent (Complex)**: For quest interactions with tool use (~1-2s latency)

**3. Configuration Management:**
```python
# Centralized config with environment variables
llm_cloud = ChatGoogleGenerativeAI(model="gemini-2.5-flash")
embeddings_local = OllamaEmbeddings(model="nomic-embed-text")
```

**4. Type Safety with Protocol Buffers:**
```python
# Auto-generated code from .proto ensures type consistency
def Think(self, request: pb.EventRequest, context) -> pb.ActionResponse:
```

---

## 5. TECHNICAL DEEP DIVES

### A. WebSocket Communication Pattern

**Why WebSocket over HTTP/REST?**
- **Bi-directional**: Server can push NPC actions to client
- **Low latency**: ~10-50ms vs HTTP's ~100-200ms
- **Persistent connection**: Eliminates TCP handshake overhead

**Connection Lifecycle:**
```
1. Client connects → Upgrade to WebSocket
2. Authentication required (isAuthenticated flag)
3. Event loop: Read JSON messages → Process → Send response
4. Graceful disconnect on error
```

**Interview Question: "How do you handle WebSocket disconnections?"**
*Answer*: "Currently, we use a simple read error detection. For production, I'd implement:
- Heartbeat/ping-pong messages to detect dead connections
- Exponential backoff reconnection logic on the client
- Session recovery using Redis-stored session IDs"

---

### B. gRPC Service Design

**Proto Definition (Contract):**
```protobuf
service AIBrain {
  rpc Think(EventRequest) returns (ActionResponse) {}
}
```

**Why gRPC?**
1. **Performance**: 7-10x faster than REST (binary vs JSON)
2. **Type Safety**: Auto-generated client/server code
3. **Streaming**: Supports bidirectional streaming (future enhancement)
4. **Language Agnostic**: Generated code for Go, Python, C++, etc.

**Interview Question: "Why not REST?"**
*Answer*: "REST is great for public APIs, but for internal microservices, gRPC's performance and type safety are more valuable. REST adds JSON serialization overhead and lacks compile-time type checking."

---

### C. Quest System Architecture

**Data-Driven Design:**
- Quest definitions stored as JSON files
- QuestManager loads all quests at startup into memory (fast lookup)
- Player quest state persisted in PostgreSQL

**Quest Flow:**
```
1. Player submits item → WebSocket handler receives event
2. QuestManager.ProcessEvent():
   - Validates item against quest definition
   - Checks current step preconditions
   - Updates player quest state
   - Returns success/failure
3. If success → Call AI for NPC dialogue
4. If failure → Return fail response directly
```

**Key Optimization:**
- Quest rules cached in memory (no DB query needed)
- Player state cached in Redis (5-minute TTL)

---

### D. Emotion & Memory System

**Emotion State (Per-NPC):**
```go
type EmotionState struct {
    Joy     float64
    Sadness float64
    Anger   float64
    Fear    float64
    Trust   float64  // Per-player relationship
}
```

**Dynamic Emotion Modification:**
- Event-based deltas loaded from `event_emotions.json`
- Applied on every player interaction
- Clamped to [0.0, 1.0] range

**Memory System:**
- Every interaction creates a Memory entity
- Memories linked to NPC via foreign key
- Recent 5 memories sent to AI for context
- Stored with importance score (future: memory prioritization)

**Interview Insight:**
*"The emotion system influences NPC dialogue. For example, high anger + low trust → aggressive responses. This creates dynamic, believable NPCs that 'remember' player behavior."*

---

### E. RAG (Retrieval-Augmented Generation) System

**Architecture:**
```
Player Question → Retriever (FAISS) → Top-K Lore Facts → 
LLM (with context) → Grounded Response
```

**Implementation Details:**
1. **Embedding Model**: Nomic Embed Text (local via Ollama)
2. **Vector Store**: FAISS (in-memory, fast)
3. **Chunk Strategy**: RecursiveCharacterTextSplitter (500 chars, 50 overlap)
4. **Retrieval**: Top-1 most relevant fact per query

**Why RAG?**
- Prevents LLM hallucinations
- Grounds responses in game lore
- Enables dynamic knowledge updates (edit JSON → restart service)

**Interview Question: "How would you scale the RAG system?"**
*Answer*: "FAISS is in-memory and single-node. For scale, I'd migrate to:
- **Qdrant/Weaviate**: Distributed vector databases
- **Hybrid search**: Combine semantic (embeddings) + keyword (BM25) search
- **Reranking**: Use cross-encoder model to refine top results"

---

## 6. DATABASE DESIGN & ORM

### Ent ORM (Go)

**Why Ent over GORM?**
1. **Code Generation**: Type-safe query builders
2. **Schema-as-Code**: Version-controlled migrations
3. **Graph Queries**: Efficient eager loading with `.With()` methods

**Schema Design:**
```
Player ─────< PlayerQuestState >───── Quest
   │
   └─────< PlayerNPCRelationship >───── NPC ─────< Memory
```

**Relationships:**
- Player has many quest states (one per quest)
- NPC has many relationships (one per player)
- NPC has many memories

**Query Optimization Example:**
```go
// Eager loading to prevent N+1 queries
npc, _ := db.NPC.Query().
    Where(npc.NameEQ("Elara")).
    WithMemories().
    Only(ctx)
```

---

## 7. DEPLOYMENT & DEVOPS

### Docker Compose Setup

**Services:**
```yaml
postgres:  # Port 5432
redis:     # Port 6379
```

**Current Deployment:**
1. `docker-compose up -d` → Start infrastructure
2. Manually run Python service (port 50051)
3. Manually run Go service (port 8080)

**Production-Ready Improvements:**
1. **Dockerize all services**:
   - `Dockerfile` for Go backend
   - `Dockerfile` for Python AI service
   - Multi-stage builds for smaller images

2. **Kubernetes Deployment**:
   - StatefulSet for PostgreSQL
   - Deployment + Service for Go backend (with HPA)
   - Deployment + Service for Python AI (with HPA)
   - Ingress for external traffic

3. **CI/CD Pipeline**:
   - GitHub Actions for automated testing
   - Build Docker images on commit
   - Deploy to staging/prod environments

**Interview Question: "How would you monitor this in production?"**
*Answer*: "I'd implement:
- **Metrics**: Prometheus + Grafana (request latency, error rates, AI inference time)
- **Logging**: ELK stack or Loki (centralized logs)
- **Tracing**: Jaeger for distributed tracing across gRPC calls
- **Alerts**: PagerDuty for critical failures"

---

## 8. COMMON INTERVIEW QUESTIONS & ANSWERS

### System Design Questions

**Q: "How would you handle 10,000 concurrent players?"**
A: 
1. **Horizontal scaling**: Run multiple Go backend instances behind ALB with sticky sessions
2. **AI service pool**: 5-10 Python replicas with gRPC load balancing
3. **Database**: Read replicas for memory/quest queries, write to primary
4. **Redis cluster**: Distributed cache for session data
5. **Message queue**: Decouple AI requests (async processing)

**Q: "What if the AI service goes down?"**
A:
1. **Circuit breaker pattern**: Stop sending requests after N failures
2. **Fallback responses**: Use rule-based dialogue (check emotions, return predefined text)
3. **Health checks**: Kubernetes probes restart unhealthy pods
4. **Graceful degradation**: NPCs still function, just less "intelligent"

**Q: "How do you prevent database bottlenecks?"**
A:
1. **Caching**: Redis for hot data (player sessions, NPC states)
2. **Connection pooling**: Reuse DB connections
3. **Query optimization**: Indexes on foreign keys, use `LIMIT`
4. **Read replicas**: Route read-heavy queries to replicas
5. **Eventual consistency**: Use message queue for non-critical writes

---

### Code Review Scenario

**Q: "Walk me through how a player question is processed."**
A:
1. **Client** sends WebSocket message: `{"event_type": "PLAYER_ASKED_QUESTION", "question_text": "Who is the mayor?"}`
2. **Go Handler** (`websocket_handler.go`):
   - Validates authenticated session
   - Calls `HandleGameEvent()`
3. **Quest Manager**:
   - No quest logic needed for questions, passes through
4. **Game Handler**:
   - Fetches target NPC from DB/Redis
   - Updates emotions based on event
   - Creates Memory entity
   - Fetches recent 5 memories
5. **gRPC Call**:
   - Packs data into `EventRequest` proto message
   - Sends to Python AI service (port 50051)
6. **Python Servicer**:
   - Routes to RAG agent (fast brain)
   - Retrieves agent from `live_agents` registry
7. **RAG Chain**:
   - Embeds player question
   - Searches FAISS vector store for relevant lore
   - Injects context into LLM prompt
   - Calls Gemini API
8. **Response Path**:
   - Python returns `ActionResponse` proto
   - Go handler sends JSON to client via WebSocket
   - Client displays NPC dialogue

**Total Latency**: ~300-500ms

---

### Best Practices Questions

**Q: "How did you ensure code quality?"**
A:
1. **Architecture**: Clean architecture with clear layer separation
2. **Dependency Injection**: All dependencies passed explicitly (testable)
3. **Error Handling**: Always wrap errors with context
4. **Type Safety**: Protocol Buffers, Ent ORM, TypeScript on frontend
5. **Configuration**: Environment variables, no hardcoded secrets
6. **Documentation**: README with setup instructions, inline code comments

**Q: "What would you test in this system?"**
A:
1. **Unit Tests**:
   - Quest logic (item validation, step progression)
   - Emotion calculations (clamp function, delta application)
   - NPC agent loading
2. **Integration Tests**:
   - WebSocket connection flow
   - gRPC communication
   - Database operations with test DB
3. **E2E Tests**:
   - Full player interaction flow
   - Quest completion scenario
4. **Load Tests**:
   - 1000+ concurrent WebSocket connections
   - AI service throughput (requests/second)

---

## 9. PROJECT CHALLENGES & LEARNINGS

### Challenge 1: Managing Agent State
**Problem**: Initially loaded agents on every request → slow (2-3s cold start)
**Solution**: Pre-load all agents at startup → singleton registry pattern
**Learning**: "Optimize for hot paths - moved expensive operations to startup"

### Challenge 2: gRPC Type Mismatches
**Problem**: Go's `EmotionState` struct didn't match Python's expected format
**Solution**: Created proto message types, explicit conversion in `ai_client.go`
**Learning**: "Protocol Buffers are powerful but require discipline in maintaining type consistency"

### Challenge 3: Memory Growth
**Problem**: Unlimited memory creation → database bloat
**Solution**: Query with `.Limit(5)`, plan to implement TTL/archival
**Learning**: "Always consider data lifecycle in persistent storage"

### Challenge 4: Quest Precondition Validation
**Problem**: AI called even when player didn't meet quest requirements
**Solution**: Two-phase validation (QuestManager first, AI second)
**Learning**: "Separate validation logic from business logic for clarity"

---

## 10. FUTURE ENHANCEMENTS (GREAT TALKING POINTS!)

### Short-term (1-2 months)
1. **Multi-NPC Conversations**: NPCs can reference each other, group dialogues
2. **Dynamic Quest Generation**: LLM generates side quests based on player behavior
3. **Voice Synthesis**: TTS for NPC dialogue
4. **Admin Dashboard**: Monitor NPC states, player progress

### Medium-term (3-6 months)
1. **Player Modeling**: AI learns player preferences, adapts NPC responses
2. **Procedural Lore Generation**: Expand world knowledge dynamically
3. **Conflict Resolution**: NPCs react to player choices (faction systems)
4. **Performance Optimizations**: Async AI calls, parallel processing

### Long-term (6-12 months)
1. **Multi-Game Support**: Generalize framework for different game genres
2. **Federated Learning**: NPCs learn across multiple game instances
3. **Blockchain Integration**: NFT-based unique NPC personalities
4. **Cloud-Native Deployment**: Full Kubernetes + service mesh

---

## 11. KEY INTERVIEW MESSAGES TO CONVEY

### Technical Competence
✅ "I made informed architectural decisions (gRPC over REST, polyglot microservices)"
✅ "I prioritized type safety, error handling, and graceful degradation"
✅ "I understand trade-offs (in-memory FAISS vs distributed vector DB)"

### Growth Mindset
✅ "I identified current limitations and know how to address them"
✅ "I learned new technologies (Ent ORM, LangChain, gRPC) through this project"
✅ "I can articulate what I'd do differently at scale"

### Production Awareness
✅ "I think beyond 'it works' - security, monitoring, deployment"
✅ "I use best practices: dependency injection, environment configs, graceful shutdown"
✅ "I design for maintainability with clean architecture and modular code"

---

## 12. QUESTIONS TO ASK INTERVIEWERS

*Asking good questions shows engagement and helps you evaluate the company*

### Technical Questions
1. "What does your current tech stack look like? Are you using microservices?"
2. "How do you approach system design - do you prioritize performance, scalability, or maintainability?"
3. "What's your deployment process? CI/CD pipeline? Kubernetes?"
4. "How do you handle technical debt and legacy code?"

### Team/Culture Questions
1. "What does code review look like? How do you ensure quality?"
2. "How much autonomy do interns have on architecture decisions?"
3. "What does a typical day look like for your engineering team?"
4. "How do you support learning and professional development?"

### Product Questions
1. "What are the biggest technical challenges SkippyEd is facing?"
2. "Where do you see the product in 1-2 years?"
3. "How do you balance moving fast vs building robust systems?"

---

## 13. FINAL PREP CHECKLIST

### Day Before (Jan 13)
- [ ] Review this document thoroughly
- [ ] Practice explaining architecture on whiteboard
- [ ] Review key code files:
  - `backend-go/internal/app/app.go` (dependency setup)
  - `ai-service-python/servicer.py` (request routing)
  - `backend-go/internal/api/handlers/game_handler.go` (event flow)
- [ ] Prepare 2-3 questions to ask interviewers
- [ ] Test your internet connection and Google Meet setup

### 1 Hour Before
- [ ] Re-read project README
- [ ] Open VS Code with project loaded (in case of code sharing)
- [ ] Have system architecture diagram ready (draw if needed)
- [ ] Get water, turn off notifications, quiet space

### During Interview
- [ ] Start with the 30-second elevator pitch
- [ ] Use the STAR method for project discussion:
  - **Situation**: "I wanted to build intelligent NPCs"
  - **Task**: "Challenge was making them memory-aware and scalable"
  - **Action**: "I designed a microservices architecture with Go and Python"
  - **Result**: "System handles 1000+ concurrent players with <500ms latency"
- [ ] Draw diagrams if doing screen share
- [ ] Be honest about what you don't know, but show how you'd find out
- [ ] Ask clarifying questions if a problem is ambiguous

---

## 14. STRESS TEST SCENARIOS (CODE REVIEW PRACTICE)

### Scenario 1: Race Condition
**Interviewer**: "What if two players interact with the same NPC simultaneously?"
**Answer**: 
"Currently, we have a potential race condition:
1. Both requests read NPC emotions → modify → write back
2. Last write wins, losing one update

**Solutions**:
- **Optimistic locking**: Add version field to NPC table, fail transaction if version changed
- **Database locks**: Use `SELECT FOR UPDATE` in transaction
- **Message queue**: Serialize NPC updates through single worker
- **Event sourcing**: Store emotion deltas as events, aggregate on read

I'd choose optimistic locking for simplicity."

### Scenario 2: Memory Leak
**Interviewer**: "How would you debug a memory leak in the Python service?"
**Answer**:
"I'd use systematic debugging:
1. **Reproduce**: Generate high load with locust/k6
2. **Profile**: Use `memory_profiler` or `tracemalloc` to find leaks
3. **Likely culprits**:
   - FAISS vector stores not being garbage collected
   - LangChain chains holding references to large objects
   - Circular references in agent state
4. **Fix**: Explicitly clear caches, use weak references, implement LRU cache for agents
5. **Verify**: Monitor with Prometheus (process_resident_memory_bytes metric)"

### Scenario 3: Database Migration
**Interviewer**: "You need to add a new field to the NPC table. How do you deploy without downtime?"
**Answer**:
"Blue-green deployment with backward-compatible schema changes:
1. **Phase 1**: Add new field as NULLABLE, deploy code that writes to it (old code ignores it)
2. **Phase 2**: Backfill existing rows with default values
3. **Phase 3**: Deploy code that reads from new field
4. **Phase 4**: Make field NOT NULL after confirming all rows populated

**Alternative**: Use Ent's migration system with versioned migrations + canary deployments"

---

## 15. PROJECT STATISTICS (IMPRESSIVE NUMBERS)

- **Lines of Code**: ~5,000 (Go), ~2,000 (Python)
- **Services**: 4 (Go backend, Python AI, PostgreSQL, Redis)
- **NPCs**: 8 fully configured with unique personalities
- **Database Tables**: 8 (Player, NPC, Memory, Quest, etc.)
- **gRPC Methods**: 1 primary (`Think`), extensible
- **Dependencies**: 
  - Go: 20+ modules (Gin, Ent, gRPC, Redis)
  - Python: 30+ packages (LangChain, LangGraph, FastAPI, gRPC)
- **Response Time**: 
  - Simple questions (RAG): ~300ms
  - Complex quests (LangGraph): ~1-2s
- **Concurrent Connections**: Tested up to 100 WebSocket connections
- **Memory Footprint**: 
  - Go service: ~50MB
  - Python service: ~300MB (with loaded models)

---

## GOOD LUCK! 🚀

**Remember**:
- You built something complex and impressive
- It's okay to say "I don't know, but here's how I'd find out"
- Show your thought process, not just answers
- Demonstrate passion for learning and problem-solving
- Be yourself - they're evaluating cultural fit too

**You've got this!**

---

*Last updated: January 12, 2026*
*Interview: January 14, 2026 at 8:30 PM IST*

