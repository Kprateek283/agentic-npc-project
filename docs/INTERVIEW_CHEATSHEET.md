# Interview Quick Reference Cheatsheet

## 30-Second Pitch
"I built an intelligent NPC framework using microservices: Go backend for game logic and WebSocket handling, Python service for AI/LLM processing via gRPC, with PostgreSQL and Redis for data. NPCs have persistent memory, dynamic emotions, and use RAG to generate context-aware dialogue powered by LangChain."

---

## Architecture at a Glance

```
[Unreal Client]
      ↓ WebSocket (JSON)
[Go Backend :8080]
      ↓ PostgreSQL (player/NPC state)
      ↓ Redis (caching)
      ↓ gRPC (binary)
[Python AI :50051]
      ↓ Ollama/Gemini (LLM)
      ↓ FAISS (vector search)
```

---

## Tech Stack Quick List

**Go**: Gin, Ent ORM, gRPC, Gorilla WebSocket, Redis client
**Python**: LangChain, LangGraph, FastAPI, gRPC, FAISS, Gemini API
**Databases**: PostgreSQL 15, Redis 7
**DevOps**: Docker Compose, Protocol Buffers
**LLM**: Gemini 2.5 Flash (cloud), Llama 3 (local via Ollama)

---

## Key Design Decisions

| Decision | Why? | Trade-off |
|----------|------|-----------|
| **Go for backend** | Concurrency, performance | Steeper learning curve |
| **Python for AI** | ML ecosystem (LangChain) | Slower than compiled languages |
| **gRPC** | Low latency, type-safe | More complex than REST |
| **WebSocket** | Real-time, bidirectional | Stateful (harder to scale) |
| **Microservices** | Independent scaling | Network overhead |
| **Redis caching** | Reduces DB load 70% | Eventual consistency risk |
| **RAG system** | Prevents hallucinations | Requires up-to-date embeddings |

---

## Request Flow (Most Important!)

### Simple Question Flow (~300ms)
1. Client → WebSocket → Go Handler
2. Go → Fetch NPC/Memory → Update Emotions
3. Go → gRPC → Python Servicer
4. Python → Agent Registry → RAG Chain
5. RAG → FAISS Retriever → Top lore facts
6. RAG → LLM + Context → Response
7. Response → Go → WebSocket → Client

### Quest Interaction Flow (~1-2s)
Same as above, but:
- Step 4: Routes to LangGraph Agent (complex brain)
- LangGraph can use tools (lore_book_search)
- Multi-step reasoning with scratchpad

---

## Database Schema (Core Tables)

```
Player
  - id (UUID)
  - username
  - passwordHash
  
NPC
  - id (UUID)
  - name (unique)
  - personality_path, backstory_path, lore_path
  - emotions (JSON: joy, sadness, anger, fear, trust)
  
Memory
  - id (int)
  - npc_id (FK)
  - description
  - event_type
  - importance
  - created_at
  
PlayerNPCRelationship
  - player_id (FK)
  - npc_id (FK)
  - trust_level (per-player trust)
  
PlayerQuestState
  - player_id (FK)
  - quest_id (FK)
  - current_step
  - completion_rate
  - is_completed
```

---

## Security Highlights

✅ Environment variables for secrets
✅ Ent ORM (prevents SQL injection)
✅ Input validation (DTO schemas)
✅ Emotion value clamping (0.0-1.0)

🔧 **Would Improve**:
- JWT authentication
- Rate limiting
- TLS encryption
- Prompt injection protection

---

## Scalability Plan

| Current | At 10K Players | At 100K Players |
|---------|---------------|-----------------|
| 1 Go server | 5 Go replicas + LB | Auto-scaling (HPA) |
| 1 Python server | 3 Python replicas | 10+ replicas + queue |
| 1 PostgreSQL | 1 primary + 2 read replicas | Sharding by player_id |
| 1 Redis | Redis cluster | Distributed cache |
| FAISS (in-mem) | Qdrant/Weaviate | Cloud vector DB |

---

## Code Quality Practices

1. **Clean Architecture**: Layered design (API → Domain → Infra)
2. **Dependency Injection**: All deps passed explicitly
3. **Error Handling**: Always wrap with context
4. **Type Safety**: Proto + Ent + type hints
5. **Graceful Shutdown**: Defer cleanup in main()
6. **Caching Strategy**: Check cache → DB → Update cache

---

## Performance Numbers

- WebSocket latency: 10-50ms
- RAG query: ~300ms
- LangGraph query: ~1-2s
- gRPC overhead: <5ms
- Memory/NPC: ~5KB per NPC in DB
- Redis cache hit rate: ~70%

---

## Common Gotchas & Solutions

**Problem**: Race condition on NPC emotions
**Solution**: Optimistic locking with version field

**Problem**: Memory table grows unbounded
**Solution**: TTL-based archival, keep only recent 100 memories

**Problem**: FAISS is in-memory (not scalable)
**Solution**: Migrate to Qdrant/Pinecone for distributed vector DB

**Problem**: WebSocket doesn't auto-reconnect
**Solution**: Client-side exponential backoff + session recovery

**Problem**: AI service single point of failure
**Solution**: Circuit breaker + fallback to rule-based responses

---

## Best Interview Responses

**"How did you ensure this scales?"**
→ "Stateless services, caching layer, horizontal scaling design, and identified bottlenecks (LLM inference) with mitigation plans (replicas + queue)."

**"What was the hardest bug?"**
→ "Agent cold-start latency was 2-3s. Profiled with Python's cProfile, found file I/O in hot path. Solution: pre-load all agents at startup into singleton registry."

**"How do you prevent LLM hallucinations?"**
→ "RAG system retrieves grounded facts from lore database before LLM call. Also considering output validation with regex/classifier."

**"Why microservices?"**
→ "Leverage language strengths: Go for I/O, Python for ML. Independent scaling: AI service is bottleneck, needs more replicas than backend."

**"How would you test this?"**
→ "Unit: quest logic, emotion math. Integration: gRPC + DB with test containers. E2E: full player flow with mock LLM. Load: k6 for 1000+ concurrent WebSockets."

---

## Questions to Ask Them

1. "What's your approach to system design - scale-first or iterate-first?"
2. "How do you balance technical debt vs new features?"
3. "What does your deployment pipeline look like?"
4. "How much ownership do interns have over architectural decisions?"
5. "What are the biggest technical challenges you're facing?"

---

## If You Get Stuck

1. **Think aloud**: "Let me break this down..."
2. **Ask clarifying questions**: "Are we optimizing for latency or cost?"
3. **Admit unknowns**: "I haven't implemented X, but I'd research Y and Z approaches."
4. **Show learning**: "I learned gRPC for this project by reading the docs and experimenting."

---

## Confidence Boosters

✅ Built a **production-grade** architecture (not a toy project)
✅ Used **industry-standard** tools (gRPC, PostgreSQL, Redis, K8s-ready)
✅ Solved **real problems** (latency, state management, scaling)
✅ Demonstrated **polyglot** skills (Go + Python + SQL + Proto)
✅ Thought about **non-functionals** (security, monitoring, deployment)

---

## Final Reminders

- **Be honest**: "I don't know" + how you'd learn is better than guessing
- **Show passion**: Talk about what you learned and enjoyed
- **Ask questions**: Engagement shows interest
- **Relax**: You've built something impressive, they want you to succeed

**You're ready. Go ace it! 🚀**

