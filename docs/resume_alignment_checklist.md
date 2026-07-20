# Resume ↔ Project Alignment Checklist

Goal: make every resume claim verifiable in this repo, then add high-signal improvements
targeting the AI Engineer JD (Python, FastAPI, RAG, agents, evals, vector DBs, Docker/CI, cloud).

## Claim-vs-Reality Audit (as of 2026-07-16)

| Resume claim | Status in repo | Action |
|---|---|---|
| Two-service split: Python AI + Go orchestrator (gRPC, WebSockets) | ✅ Real | None |
| 8-table PostgreSQL schema (Ent ORM) | ✅ Real (player, npc, quest, item, inventoryitem, memory, playerqueststate, playernpcrelationship) | None |
| Redis cache-aside | ✅ Real (`cache_helpers.go`) | None |
| `nomic-embed-text` embeddings + Gemini Flash generation | ✅ Real (`config.py`) | None |
| LangGraph agent for **multi-step reasoning** with tools | ⚠️ LangGraph exists but graph is single-node; tools never executed | **M7** |
| **FastAPI** | ❌ Only in `requirements.txt`; `main.py` is gRPC-only | **M1** |
| **FAISS/Qdrant vector store** | ⚠️ FAISS only; Qdrant absent | **M2** |
| **Benchmarked cloud vs local (Gemini vs Ollama llama3:8b, 2.3x)** | ❌ Ollama chat path commented out; no benchmark scripts; numbers unbacked | **M3, M4** |
| **Latency attribution (gRPC 14ms, Redis 8ms vs PG 42ms, <50ms overhead)** | ❌ Numbers in README but no measurement script/results | **M4** |
| **Eliminated hallucination via grounding** | ⚠️ RAG exists but no eval proving it | **M5, I1** |
| **Integrated UE5 C++ WebSocket client** | ✅ Reframed to "exposes a WS API"; backed by `docs/client_protocol.md` + `client-demo/` reference client | **M6** (done) |
| CI/CD GitHub Actions (skills section) | ❌ No `.github/` | **I3** |
| "Full Docker Compose support" (README) | ✅ Dockerfiles for both services; compose boots postgres/redis/qdrant/ai-service/backend | **I4** (done) |
| Eval harness bullet (commented in resume) | ❌ Doesn't exist | **I1** |
| Testing & debugging (JD mandatory) | ❌ Zero tests in repo | **I2** |

---

## MANDATORY — resume is untruthful without these

- [x] **M1. Add a real FastAPI layer to the Python service.**
  Run FastAPI alongside the gRPC server: `/health`, `/v1/chat` (direct REST access to RAG/agent for testing and demos), `/v1/npcs` (list loaded agents), and later `/v1/eval` endpoints. Serve with uvicorn from `main.py`. This also gives you a REST surface to demo without the Go stack.
- [x] **M2. Add Qdrant as a supported vector store.**
  Make `lore_retriever_tool.py` backend-pluggable (env var: `VECTOR_STORE=faiss|qdrant`), add Qdrant to `docker-compose.yml`, use `langchain-qdrant`. FAISS stays the default; Qdrant path must actually work end-to-end.
- [x] **M3. Restore the local Ollama inference path.**
  Un-comment and gate behind env config (`LLM_PROVIDER=gemini|ollama`, model name configurable). Verify `llama3:8b` works through both RAG and LangGraph paths. Also fixes the stale "Gemini 1.5 Flash" print (code uses 2.5).
- [x] **M4. Build reproducible benchmark scripts and commit results.**
  - `benchmarks/latency_attribution.py|go`: measure gRPC round-trip (no LLM), Redis hit vs PostgreSQL query, end-to-end WebSocket→response. 
  - `benchmarks/cloud_vs_local.py`: same question set through Gemini vs Ollama for both RAG and agentic paths; report medians and p95.
  - Commit `docs/benchmarks.md` with methodology, hardware, and result tables.
  - **Re-run and put the *measured* numbers on the resume** — if they differ from 2.3x / 14ms / 8ms / 42ms, update the resume, don't keep the old figures.
- [x] **M5. Make the grounding claim defensible.** *(Verified by I1: llama3.1:8b, independent
  qwen2.5:14b judge — 95% grounded on-topic, 90% refusal on out-of-scope, 0% in-scope
  hallucination. See `evals/results.md`, including the judge's known blind spot for incidental
  persona contradictions. Cloud not measured, per free-tier cap.)*
  Ensure the RAG prompt explicitly forbids answering outside retrieved context ("if not in context, say you don't know"), and demonstrate it via the eval harness (I1) with an out-of-scope question set. Soften the resume wording to "eliminated hallucination on factual lore queries, verified by a N-question adversarial eval" once measured.
- [x] **M6. Reframe the Unreal claim + ship a reference client.**
  The UE5 client was deleted and is unrecoverable. Reword the resume bullet to "exposes a WebSocket API for real-time game-client integration (JSON event protocol, designed for Unreal Engine 5 clients)" — remove "integrated with" everywhere. Back it with a committed WebSocket protocol spec (`docs/client_protocol.md`) and a minimal browser-based reference client (`client-demo/`) that authenticates and converses — the demoable artifact for README GIF and interviews.
- [x] **M7. Make the LangGraph agent actually multi-step.**
  `graph_builder.py` is a single-node graph: one LLM call, no tool execution — `tools` and `agent_scratchpad` are put in state and never used. The resume's "multi-step reasoning" and the docs' "toolchain" are not true as written. Rebuild as a real ReAct loop (agent node ⇄ tool node with conditional edges, iteration cap), wire `lore_book_search` into it, and add a second small tool (quest-status) so "toolchain" is honest.

## IMMEDIATE — do before sending the resume out

- [x] **I1. Build the 50-question eval harness** (this unlocks the commented resume bullet and directly matches JD responsibility #5 "evaluation frameworks").
  - `evals/dataset.json`: ~50 questions across NPCs/lore domains, each with expected source chunk + reference answer; include 5–10 out-of-scope traps.
  - Score retrieval hit@1/hit@3 (does the gold chunk appear in top-k) and LLM-as-judge grounded-answer accuracy (Gemini judging faithfulness to retrieved context).
  - Output a results table to `evals/results.md`. Expose via `python -m evals.run` and optionally the FastAPI `/v1/eval` endpoint.
  - Then un-comment the resume bullet with real X/Y numbers.
- [x] **I2. Add tests** (JD lists testing as mandatory; repo currently has zero).
  - Python (pytest): servicer routing (RAG vs LangGraph event types), `context_formatter`, `prompt_loader`, retriever returns relevant chunk for a known query (can mock LLM).
  - Go: table-driven tests for `quest_manager.ProcessEvent` preconditions and `emotion_manager` delta application.
  - Target: enough to say "tested" honestly (~15–25 meaningful tests), not coverage theater.
- [x] **I3. GitHub Actions CI.**
  One workflow: Python lint (ruff) + pytest; Go vet + test + build. Badge in README. Backs the "CI/CD (GitHub Actions)" skills claim.
- [x] **I4. Dockerize both services.**
  `Dockerfile` for `ai-service-python` and `backend-go`; add both (plus Qdrant, optionally Ollama) to `docker-compose.yml` so `docker compose up` boots the whole stack. Backs the README claim.
- [x] **I5. Repo hygiene (recruiters will read this repo).**
  - **Remove `docs/INTERVIEW_*.md`, `PRACTICE_CODE_REVIEW.md`, `PREPARATION_SUMMARY.md` from the public repo** — interview-prep cheatsheets sitting next to the project undermine every claim in it.
  - Parametrize the hardcoded Postgres password in `docker-compose.yml` (`${POSTGRES_PASSWORD}` + `.env.example`).
  - Clean commented-out dead code in `config.py` (replace with the env-driven provider switch from M3).
  - Commit `docs/agentic_npc_documentation.md` (currently untracked).
  - Consider dropping the committed `vendor/` directory (Go modules make it unnecessary; it bloats the repo diff view).
- [x] **I6. README upgrade.**
  Quickstart (`docker compose up` → working demo), mermaid architecture diagram, benchmark table linking to `docs/benchmarks.md`, eval results table, demo GIF. The README is the interview's first 30 seconds.

## CAN BE DONE — strong signal, moderate effort

- [x] **C1. Observability** (JD responsibility #8): structured logging (stdlib `log/slog` in Go, stdlib `logging` in Python — no new deps), per-event timing that logs the latency breakdown on every request (this *continuously reproduces* the M4 numbers). A `req-id` generated at the WS boundary is propagated via gRPC metadata so each service emits one correlated summary line. *Verified live: Go `game_event req_id=… quest_ms/grpc_ms/total_ms` + Python `think req_id=… dur_ms` sharing the id; Go grpc_ms matched Python rag_brain dur_ms to the ms.* Prometheus `/metrics` deliberately skipped (YAGNI for a portfolio repo; add if a dashboard is ever needed).
- [x] **C2. Resilience on the gRPC boundary**: `grpc.NewClient` (replaces deprecated `grpc.Dial`), a per-call deadline (`AI_CALL_TIMEOUT`, default 40s > worst-case local inference), retry-once on `Unavailable` (never on `DeadlineExceeded` — no double-billing the LLM), reconnect backoff capped at 5s, and a graceful in-character fallback line when the AI is down instead of an error. *Verified live: killed the AI service mid-session → fallback in 213ms (within deadline); restarted it → next request normal in 12ms with **no Go restart**.*
- [x] **C3. Token streaming**: added a server-streaming `ThinkStream` RPC alongside unary `Think`; RAG replies stream real LLM tokens over gRPC → WebSocket as `SPEAK_PARTIAL` frames, closed by a `SPEAK` frame with the full text. Reference client renders progressively; `client_protocol.md` documents the partial/final shapes. *Verified live: 72 token deltas for one RAG answer; no regression on agent/non-LLM paths (single-frame). TTFT measured (`benchmarks/streaming_ttft.py`): local median 13.6s TTFT vs 21.1s full (n=6) → dialogue ~7.4s sooner; cloud TTFT not captured (Gemini 503 during the test window — documented, not invented).*
- [ ] **C4. Semantic response cache**: embed incoming lore questions, serve cached answers for near-duplicate queries from Redis. Yields a "% of queries served at ~Xms, cutting LLM cost" metric.
- [ ] **C5. LangSmith tracing** (already in requirements): enable it, screenshot traces of the LangGraph agent for README/interview.

## STRETCH GOALS — differentiators if time allows

- [ ] **S1. Writable RAG / persistent memory**: after conversations, summarize and embed player-specific memories back into the NPC's vector store (README already promises this as "future vision" — shipping it is a standout bullet).
- [ ] **S2. Cloud deployment**: run the stack on an AWS EC2 instance (resume claims EC2), with a public demo endpoint or at least a documented deploy script. Directly hits JD "Cloud & Infrastructure".
- [ ] **S3. Fine-tuning experiment** (JD responsibility #6): LoRA fine-tune a small model (e.g., llama3:8b or phi-3) on NPC persona dialogue; compare persona-consistency eval scores vs prompted baseline. Even a modest result is a rare, high-signal bullet.
- [ ] **S4. Multi-agent interactions**: NPC↔NPC conversations or a coordinator agent (JD mentions multi-agent systems).
- [ ] **S5. Load testing**: k6/locust against the WebSocket path — "sustained N concurrent players with p95 latency X" is a strong scalability metric.

## NICE TO HAVE

- [ ] N1. Demo video (2–3 min) linked in README — UE5 client talking to a live NPC.
- [ ] N2. Knowledge-graph experiment for quest/relationship state (JD mentions knowledge graphs).
- [ ] N3. Blog-style writeup of the two-brain architecture + benchmark findings (linkable from LinkedIn).
- [ ] N4. Cost analysis: Gemini API cost per 1k NPC interactions vs local inference — pairs with the cloud/local benchmark.
- [ ] N5. Pre-commit hooks (ruff, gofmt) to signal engineering discipline.

---

## Suggested order of execution

1. M3 → M1 → M2 (config/provider plumbing first, FastAPI, Qdrant)
2. M7 (real multi-step agent — must exist before evals/benchmarks measure it)
3. M5 → I1 → M4 (grounding, then evals and benchmarks — these produce the resume numbers)
4. I2 → I3 → I4 (tests, CI, Docker)
5. I5 + I6 (hygiene + README) — do before making the repo link live anywhere
6. M6 (protocol doc + reference client + resume rewording) — parallel/any time
7. C1–C5, then S/N items as time allows

Full per-item implementation plans: see `implementation_plan.md`.

**Rule for every metric:** the number on the resume must come from a committed, re-runnable script whose results are in the repo. If a fresh run disagrees with the current resume figure, the resume changes, not the data.
