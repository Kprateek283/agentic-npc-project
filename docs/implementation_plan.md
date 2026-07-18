# Implementation Plan — Resume Alignment Checklist

Companion to `resume_alignment_checklist.md`. Each item below is a self-contained plan an
engineer can execute with only this document, the architecture docs, and the codebase.
No code here — only what to build, where, and how to know it's done.

**Global rules**

1. Every metric that ends up on the resume must be produced by a script committed in this
   repo, with results committed alongside it. A fresh run that disagrees with the resume
   means the resume changes.
2. All new configuration is environment-variable driven with sane defaults, documented in
   a committed `.env.example`. No new hardcoded hosts, ports, models, or credentials.
3. Existing behavior of the Go↔Python gRPC path must not regress; it is the production
   path and everything new (REST, evals, benchmarks) is layered beside it, not on top of it.

**Dependency order:** M3 → M1 → M2 → M7 → M5 → I1 → M4 → I2 → I3 → I4 → I5 → I6 → M6,
then C-items in any order, then S/N items. Rationale: provider switching (M3) underpins
benchmarks; FastAPI (M1) is the entry point evals and benchmarks call; the retriever
refactor (M2) sets the configurable `k` that evals need; the real agent loop (M7) must
exist before grounding/eval claims are measured against it.

---

## MANDATORY

### M1. Add a real FastAPI layer to the Python service

**Current state.** `ai-service-python/main.py` starts only a gRPC server (port 50051).
FastAPI and uvicorn are installed but unused. All request routing logic lives inline in
`servicer.py` (`AIBrainServicer.Think`), which inspects `event_type` and dispatches to
`agent.run_rag_agent` or `agent.run_quest_agent`.

**Plan.**
1. Extract the routing logic out of `servicer.py` into a transport-agnostic function in a
   new module (e.g. `router.py` at the service root): input = agent key, event type,
   question text, dynamic context dict; output = action type + content. `AIBrainServicer.Think`
   becomes a thin adapter that unpacks the protobuf request, calls this function, and packs
   the protobuf response. This avoids duplicating the RAG-vs-LangGraph dispatch in two places.
2. Create an `api/` package containing a FastAPI application with Pydantic request/response
   models and these endpoints:
   - `GET /health` — returns service status, number of loaded agents, active LLM provider,
     active vector-store backend. Used by Docker healthchecks (I4) and CI smoke tests.
   - `GET /v1/npcs` — lists loaded agents from `agent_manager` (key, name, occupation).
   - `POST /v1/chat` — body: NPC key, question, optional dynamic context (emotions,
     memories, quest step, completion rate; all defaulted when absent). Calls the shared
     router with event type `PLAYER_ASKED_QUESTION`. This is the endpoint evals (I1) and
     benchmarks (M4) will drive.
   - `POST /v1/event` — body: NPC key, event type (validated against the known set),
     payload text, optional dynamic context. Exercises the LangGraph path.
   - Error handling: unknown NPC key → 404; unknown event type → 422; LLM/provider
     failure → 502 with a structured error body. Never leak stack traces in responses.
3. Modify `main.py` so both servers run in one process: start the gRPC server (it is
   already non-blocking until `wait_for_termination`), then run uvicorn in the main thread
   on a new `API_PORT` env var (default 8000). Agent loading happens once, before either
   server starts, so both transports share the same in-memory `agent_manager` registry.
4. Document both transports (who uses which, ports) in the README architecture section.

**Done when.** `curl` against `/health`, `/v1/npcs`, `/v1/chat` works while the existing
Go→gRPC→Python flow is verified unchanged; unknown-NPC and bad-event cases return the
documented status codes.

**Notes from execution (2026-07-16).**
- `/health` reports provider, models and agent count; the **vector-store backend field is
  added by M2**, which is what introduces `VECTOR_STORE` config.
- The transport-agnostic dynamic context is plain data — `{"emotions": {5 floats},
  "memories": [str], "quest_step": int, "completion_rate": float}`. Only `.description`
  of memories and the five emotion floats were ever consumed, so the protobuf-shaped
  objects stopped at the servicer and `context_formatter` was updated to the plain shape.
- `agent_manager.get_agent` now also resolves a bare NPC directory name ("elara") after an
  exact-key miss, so REST/eval callers need not spell out personality paths. Exact matches
  still win, so the Go path is untouched.
- **Data-shape bug found and fixed here:** 3 of 8 NPCs author `occupation` as a *list*
  (e.g. `["Villager", "Dumb Brother"]`). Go already normalised this (`occupationToString`
  in `seeder.go`); Python did not, so the raw list was being rendered into every static
  system prompt as `['Villager', 'Dumb Brother']`. Normalised in `prompt_loader.py`
  (joined with ", "). This changes prompt text, so it had to land before I1/M4 measure
  anything.

---

### M2. Pluggable vector store: FAISS (default) + Qdrant

**Current state.** `tools/lore_retriever_tool.py` builds an in-memory FAISS index per NPC
at startup from `lore.json` `known_facts`, with retriever hardcoded to `k=1`. Qdrant
appears nowhere.

**Plan.**
1. Add config in `config.py`: `VECTOR_STORE` (`faiss` | `qdrant`, default `faiss`),
   `QDRANT_URL`, and `RETRIEVER_K` (default 3 — needed for hit@3 in I1; verify answer
   quality doesn't regress with the larger k before committing the default).
2. In `lore_retriever_tool.py`, split index construction from the rest of the function:
   document loading, splitting, tool wrapping stay identical; the "build a vector store
   from these documents" step delegates to a small factory keyed on `VECTOR_STORE`.
   - FAISS branch: current behavior, unchanged.
   - Qdrant branch: use the `langchain-qdrant` integration (add to `requirements.txt`)
     against `QDRANT_URL`. One collection per NPC, named deterministically from the NPC
     directory name. On startup, recreate the collection idempotently (drop-and-rebuild is
     acceptable — lore is static and small; note this ceiling in a comment). Embedding
     dimension comes from the embedding model; don't hardcode it.
3. Add a `qdrant` service to `docker-compose.yml` (official image, persistent volume,
   default ports).
4. Failure mode: if `VECTOR_STORE=qdrant` and Qdrant is unreachable at startup, fail fast
   with a clear error — do not silently fall back to FAISS (a silent fallback would make
   benchmark/eval results lie about which backend they measured).
5. Document backend selection in README; state explicitly that FAISS is the default and
   Qdrant is the scale-out path.

**Done when.** Full flow (REST and gRPC) works with each backend selected via env var;
Qdrant dashboard shows one populated collection per NPC; evals (I1) can run against both.

**Notes from execution (2026-07-16).**
- Qdrant pinned to `v1.18.2` (not `latest`) so benchmark runs stay reproducible; host ports
  are `${QDRANT_HTTP_PORT:-6333}` / `${QDRANT_GRPC_PORT:-6334}` per global rule 2.
- `k=3` verified against `k=1` before adopting the default: rank-1 documents are identical
  (k widens the result set, it does not reorder), and answers were equal or better — e.g.
  elara's "who has the pure heart" went from "I don't know anything about forging" at k=1
  to a grounded answer at k=3. No regression.
- Fail-fast verified by pointing `QDRANT_URL` at a dead port: startup raises with the
  remediation command, never falls back to FAISS.

**!! Metric caveat for I1 — the lore corpus is tiny.** Facts per NPC: baelor 5, elara 5,
elian 5, kaelen 5, marcus 4, rook 3, silas 3, **lian 1** — 31 total. With `k=3`, a query
returns 60–100% of an NPC's entire lore, so **hit@3 is close to meaningless here** (for
lian, k=3 returns the only fact, making hit@3 unconditionally 100%). Reporting "hit@3 = 9x%"
on a resume would be technically true and materially misleading. I1 must therefore either:
(a) report hit@1 as the headline retrieval metric and state the corpus size honestly beside
hit@3, and/or (b) expand the lore corpus first so retrieval is a real discrimination task.
Decide before recording any retrieval number.

---

### M3. Restore local Ollama inference as a first-class provider

**Current state.** `config.py` hardcodes `ChatGoogleGenerativeAI(model="gemini-2.5-flash")`
for both `llm_light` and `llm_heavy`; the Ollama chat path exists only as commented-out
code at the top of the file; the startup banner wrongly says "Gemini 1.5 Flash".

**Plan.**
1. Rewrite `config.py` around env vars: `LLM_PROVIDER` (`gemini` | `ollama`, default
   `gemini`), `GEMINI_MODEL` (default `gemini-2.5-flash`), `OLLAMA_MODEL_HEAVY` (default
   `llama3:8b`), `OLLAMA_MODEL_LIGHT` (default same as heavy; the phi3 split is optional),
   `EMBEDDING_MODEL` (default `nomic-embed-text`), `OLLAMA_HOST` (default localhost).
2. Construct `llm_light` / `llm_heavy` from the selected provider. Validate
   `GEMINI_API_KEY` only when the provider is `gemini` — the current unconditional check
   would break pure-local runs.
3. Embeddings stay on Ollama regardless of chat provider (this is what the resume claims);
   make the model name and host configurable per above.
4. Delete the commented-out block; fix the startup banner to print the *actual* provider,
   chat model, embedding model, and vector-store backend from config.
5. Verify both providers end-to-end through the RAG path and the LangGraph path (Ollama
   models can be weaker at instruction-following; confirm the ReAct-style prompt still
   produces usable output with llama3:8b, and note any prompt adjustments needed).
6. Document local setup in README: Ollama install, `ollama pull` for the chat and
   embedding models, env var examples for both modes.

**Done when.** The same conversation flow produces sensible responses with
`LLM_PROVIDER=gemini` and `LLM_PROVIDER=ollama`, with no code edits — env vars only.

**Observed on verification (2026-07-16), for M5/M7 to fix — not fixed here.**
- llama3:8b *hallucinates* on out-of-scope lore questions where Gemini refuses (same
  question, same prompt: Gemini "I don't know anything about that" vs llama3:8b "I've
  heard whispers among the villagers..."). This is the missing refusal instruction in
  `rag_builder.py`'s inline prompt — M5's item, and it makes M5 provider-sensitive:
  measure refusal rate per provider, don't assume one number covers both.
- Both providers leak the ReAct scaffolding (`Thought:` / `Final Answer:`) from
  `prompts/react_prompt.py` into player-facing dialogue on the LangGraph path. M7 step 3
  (prompt rewrite for native tool-calling) should remove it; verify the leak is gone.

**Deviation (executed).** `SERVER_PORT` (default `8080`) was pulled forward from I4 into
this item's commit: `app.go` hardcoded `:8080`, which is occupied by an unrelated service
on the dev machine, and global rule 3 requires re-verifying the Go↔Python flow here.
I4 still owns the remaining address plumbing (`AI_SERVICE_ADDR`, container networking).
`backend-go/.env.example` was created alongside it per global rule 2.

---

### M4. Reproducible benchmarks with committed results

**Current state.** README asserts numbers (Redis ~8ms, gRPC ~14ms, <50ms overhead, ~1s /
~3s brain latency) and the resume asserts more (2.3x cloud vs local, PG 42ms), but there
is no measurement code anywhere in the repo.

**!! Constraint — Gemini free-tier quota (confirmed 2026-07-17, decided).** The key is free
tier: **20 `generateContent` requests per day**, quota id
`GenerateRequestsPerDayPerProjectPerModel-FreeTier`. Facts established the hard way:
- The counter is **per project**, not per key — issuing a new key in the same project
  inherits the exhausted counter (tested).
- It is **per model**, but `gemini-flash-latest` is an *alias* for `gemini-3.5-flash` and
  shares its counter (tested) — aliases buy no extra budget.
- **`gemini-2.5-flash` and `gemini-2.5-flash-lite` now 404 for newly-created projects**
  ("no longer available to new users"), so the model this repo claimed is unreachable on a
  fresh project. Default is now **`gemini-3.5-flash`**, pinned rather than `-latest` so a
  committed number always names the model that produced it.

**Measured feasibility (2026-07-17).** 20 calls/day ÷ 2 calls per eval entry (answer + judge)
= **10 entries/day maximum**, before any probe or retry. In practice a run scored **3 entries**
before 429, because availability probes and transient `503 UNAVAILABLE` retries consume the
same budget. The 50-entry dataset needs 100 calls; M4 needs several hundred. Three separate
projects were burned through establishing this.

**Decision (repo owner, 2026-07-17), revised:**
- **I1 quality evals: cloud is NOT MEASURED.** `evals/results.md` reports Ollama at the full
  n=50 and states plainly that the cloud configuration was not measured due to the free-tier
  cap. No cloud quality number, no cloud-vs-local *quality* claim, no fabricated partial.
- **M4 keeps a cloud slice, timing only.** Latency needs no judge — 1 call per sample instead
  of 2 — so RAG (~5 reps) plus a few agent events fits inside 20/day. Every cloud timing
  figure carries its sample size inline ("gemini-3.5-flash, n=5, free-tier cap").
- Do not pre-commit to the conclusion. The hypothesis worth testing is where end-to-end time
  actually goes; early observations (infra in ms, inference in seconds) point to
  inference-dominated latency with negligible orchestration overhead, but the report states
  what the clock measures either way.

**Plan.** Create a `benchmarks/` directory with three deliverables plus a results doc.

1. **Infrastructure latency attribution** (`benchmarks/` script, language per sub-measure):
   - *gRPC round-trip without LLM:* drive the Python gRPC server with an event type that
     falls through to the non-LLM default branch in the router (the "Greetings." path).
     Measure client-observed round-trip over N≥200 iterations after warm-up; report
     median and p95. Note in the methodology that this path includes serialization and
     routing but no inference.
   - *Redis vs PostgreSQL:* a small Go program (can live under `benchmarks/` as its own
     module or a subcommand) that exercises the cache-aside `GetPlayer` in
     `quest_logic/cache_helpers.go` twice per key: once cold (cache flushed → PG read)
     and once warm (Redis hit). Time each side separately; report medians.
   - *End-to-end infra overhead:* a WebSocket test client that authenticates, sends a
     non-LLM event, and measures full round-trip through Go (validation → emotion update
     → memory write → gRPC → response). This is the "<50ms total infrastructure overhead"
     claim; measure it rather than assert it.
2. **Cloud vs local inference** (`benchmarks/` Python script): a fixed set of ~10 lore
   questions and ~5 quest events, run through the FastAPI endpoints (M1) with N
   repetitions each. Run the whole suite twice — once with `LLM_PROVIDER=gemini`, once
   with `LLM_PROVIDER=ollama` — and emit per-path (RAG vs agent) medians and p95 to a
   JSON results file. The ratio between the two runs replaces the "2.3x" resume figure.
3. **Methodology + results doc** (`docs/benchmarks.md`): hardware spec (CPU, RAM, GPU if
   any), software versions, network conditions, warm-up policy, sample counts, and the
   result tables. Include the exact commands to reproduce. Copy the headline numbers into
   README's benchmark section, replacing the current unbacked table.
4. Update the resume bullets with the measured values.

**Done when.** A stranger can clone the repo, follow `docs/benchmarks.md`, and regenerate
every number that appears in the README and on the resume.

---

### M5. Make the grounding / hallucination claim defensible

**Current state.** Three problems found in code review:
- `rag_builder.py` builds its own inline prompt ("answer the player's question concisely")
  with **no** instruction to refuse when the context lacks the answer.
- `prompts/rag_prompt.py` contains a stricter template ("Based *only* on the context
  above") that is **never used** — `rag_builder` ignores it.
- `run_rag_agent` in `npc_agent.py` accepts `dynamic_context` but silently discards it:
  emotions/memories never reach the RAG prompt, despite the resume claiming responses
  reason over them.

**Plan.**
1. Consolidate to a single RAG prompt, defined in `prompts/rag_prompt.py` and consumed by
   `rag_builder.py` (delete the inline duplicate). The prompt must include: answer only
   from the provided lore context; if the context does not contain the answer, refuse
   **in character** (e.g. the NPC admits they don't know) rather than inventing facts;
   never mention "context", "lore book mechanics", or being an AI.
2. Thread `dynamic_context` into the RAG chain: pass the formatted block from
   `context_formatter.format_dynamic_context` into the prompt so emotions and recent
   memories condition the tone and content of answers. `run_rag_agent`'s signature already
   receives it — wire it through the chain inputs instead of dropping it.
3. Add ~10 adversarial out-of-scope questions to the eval dataset (I1): questions about
   entities that exist in *another* NPC's lore, and questions about entities in no lore at
   all. Metric: refusal rate on out-of-scope + grounded-accuracy on in-scope.
4. Rephrase the resume bullet from "eliminated hallucination" to the measured form, e.g.
   "grounded all factual responses in retrieved lore, verified by an adversarial eval
   (N% refusal on out-of-scope queries, M% faithfulness on in-scope)".

**Done when.** Eval run shows the refusal behavior working; the unused-prompt duplication
is gone; a RAG response demonstrably changes when emotions/memories in the request change.

**Notes from execution (2026-07-17).**
- Duplication gone: `prompts/rag_prompt.py` is the only RAG prompt; `rag_builder` imports it
  and the inline copy is deleted. Retrieved docs are now joined as plain facts rather than
  interpolated as repr'd `Document` objects.
- `dynamic_context` is threaded into the chain inputs via `format_dynamic_context`, and the
  prompt instructs that emotions/memories colour *tone*, never facts.
- Emotion/memory conditioning demonstrated (llama3.1:8b, identical question, identical
  retrieval): joy=0.9/trust=0.9 + "player saved his daughter" → "You know me well! I'm a
  blacksmith through and through…"; anger=0.95/trust=0.0 + "player stole from his forge" →
  "(scoffs) Ah, you think you can just waltz into my forge after stealing from me?"
- Refusal needed **two** iterations. The first draft (answer-only-from-context + refuse in
  character) still invented a mayor ("Thorne") and an NPC to ask ("Gorin") — neither string
  exists anywhere in gamedata. Adding an explicit *never invent a proper name* rule plus
  "never suggest asking someone else unless they are named in the lore facts" took manual
  out-of-scope probes from 3/4 to **4/4**, with no over-refusal on in-scope questions.
- **This is n=4 on one provider — not a measured refusal rate.** The headline number must
  come from I1's adversarial set, per provider (llama3.1:8b hallucinated where Gemini
  refused pre-M5, so one number will not cover both). Tick this item when I1 reports it.

---

### M6. Reframe the Unreal Engine claim + ship a reference client

**Current state.** The UE5 client was deleted and is not recoverable; the repo's git
history never contained it. The resume currently claims "integrated with an Unreal Engine
5 C++ WebSocket client", which cannot be evidenced.

**Plan.**
1. **Resume rewording (agreed):** change the bullet to expose-API framing, e.g.
   "Exposed a WebSocket API for real-time game-client integration (JSON event protocol,
   designed for Unreal Engine 5 clients), delivering NPC dialogue and persistent agent
   memory over a bidirectional session." Remove "integrated with an Unreal Engine 5 C++
   WebSocket client" everywhere it appears. Apply the same rewording to the README
   ("Client Target" section stays — designed-for is truthful; "integrated" is not).
2. **Protocol documentation** (`docs/client_protocol.md`): full specification of the
   WebSocket contract a game client implements — connection URL, authentication event and
   its flow (from `auth_handler.go`), every `EventMessage` type accepted
   (from `dto/event_dto.go` and the router in `game_handler.go`), field-by-field message
   schemas, response message shapes, and error events. This makes "exposes an API for
   game clients" a checkable claim.
3. **Reference client:** build the smallest client that proves the API — a single
   self-contained HTML page (plain JS WebSocket, no build step, no dependencies) served
   from disk: connect, authenticate, pick an NPC, send question/gift/interact events,
   render the dialogue stream. Lives in `client-demo/`. This is the demo artifact that
   replaces the lost UE5 video: it can be screen-recorded for the README GIF (N1) and
   demoed live in interviews.
4. Optional (only if time permits, after everything else): a `docs/unreal_integration.md`
   note describing *how* a UE5 client would connect (which UE WebSocket module, event
   mapping) — clearly labeled as an integration guide, not a shipped integration.

**Done when.** Resume and README contain no claim of a shipped UE5 client; the protocol
doc matches `event_dto.go` exactly; the HTML reference client completes a full
authenticate → converse → quest-event session against a locally running stack.

---

### M7. Make the LangGraph agent actually multi-step (discovered during planning)

**Current state.** The resume says "stateful LangGraph agent for multi-step reasoning"
and the docs mention a "scratchpad and toolchain" — but `graph_builder.py` compiles a
**single-node** graph: one LLM call, entry → `thinker` → END. The `tools` and
`agent_scratchpad` fields are placed into state by `run_quest_agent` and never used; the
`lore_book_search` tool is never executed by the complex brain. As written, the claim is
not true.

**Plan.**
1. Rebuild the graph in `graph_builder.py` as a genuine ReAct loop:
   - An *agent node* that invokes the LLM bound with the NPC's tools (tool-calling mode).
   - A *conditional edge*: if the LLM response contains tool calls → route to a tool
     node; otherwise → END with the final answer.
   - A *tool node* that executes the requested tool (start with `lore_book_search` from
     `lore_retriever_tool.py`), appends the observation to the message history in state,
     and loops back to the agent node.
   - A hard iteration cap (e.g. 5 loops) with a graceful in-character fallback response,
     so a confused model can't spin forever or burn API quota.
   - Consider LangGraph's prebuilt ReAct constructor first (already a dependency) — only
     hand-roll the graph if the prebuilt can't accommodate the custom persona prompt and
     dynamic context injection.
2. Update `agent_state.py` so state carries a message list (the scratchpad) rather than
   an unused placeholder; `run_quest_agent` in `npc_agent.py` populates it and reads the
   final answer from the terminal state.
3. Review `prompts/react_prompt.py` for compatibility with tool-calling (the template was
   written for text-based ReAct; with native tool-calling most of the "Thought/Action"
   scaffolding becomes unnecessary — keep persona + dynamic context + task instructions).
4. Add a second simple tool to make "toolchain" plural and honest — e.g. a
   *quest-status* tool that formats the current quest step and completion rate from the
   dynamic context already passed in state. Zero new infrastructure, real utility.
5. Verify with both providers (M3): Gemini and llama3:8b must both complete a
   gift-submission event that requires at least one tool call.
6. Add a checklist entry for this item in `resume_alignment_checklist.md` (done alongside
   this plan).

**Done when.** A quest event observably triggers ≥1 tool execution (visible in logs /
LangSmith trace), the loop terminates on the cap, and both providers complete the flow.

**Notes from execution (2026-07-16/17).**
- **Prebuilt chosen.** `langgraph.prebuilt.create_react_agent` accommodates both the custom
  persona prompt (via a `prompt` callable re-rendered each LLM call) and the dynamic context
  (via a custom `state_schema`), so the graph was not hand-rolled. It supplies the agent
  node, tool node, conditional edge and loop; the iteration cap is applied by the caller as
  `recursion_limit = 2 * AGENT_MAX_ITERATIONS + 1` (one loop = agent + tools super-steps),
  and `GraphRecursionError` is caught in `run_quest_agent` for the in-character fallback.
- **Step 5 of this plan was wrong about the model: `llama3:8b` cannot do tool-calling.**
  `ollama show llama3:8b` reports capability `completion` only — no `tools` — so it can
  never satisfy "completes an event requiring at least one tool call". `OLLAMA_MODEL_HEAVY`
  now defaults to **`llama3.1:8b`** (same 8B class, `tools` capability, already local).
  Consequence for M4 and the resume: the local model is llama3.1:8b, not llama3:8b — the
  cloud-vs-local figure must be labelled with the model that actually ran.
- `quest_status` reads game context through `InjectedState`, so the LLM never sees or
  fabricates the argument — it is filled from state by the graph.
- **Gemini leg verified 2026-07-17** on `gemini-3.5-flash`: a `PLAYER_SUBMITTED_QUEST_ITEM`
  event executed **both** tools (`lore_book_search`, then `quest_status` with step=2 /
  completion=0.66 injected from state) and returned a final in-character answer. Both
  providers therefore complete a tool-using flow.
- Two defects surfaced only on Gemini and are fixed here:
  1. **Gemini 3.x requires `thought_signature` round-tripping** on multi-turn tool calls.
     `langchain-google-genai==3.0.0` did not do it: the first tool call succeeded and the
     follow-up turn died with `400 Function call is missing a thought_signature in
     functionCall parts`. Fixed by upgrading to 4.2.7 (which pulls langchain-core 1.4.9,
     so langchain-ollama was upgraded to 1.1.0 to match; `pip check` is clean and the
     Ollama path was re-verified after).
  2. **Gemini 3.x returns `content` as a list of content blocks**, not a string, so
     `run_quest_agent` returned a Python list — which the protobuf `content` string field
     would reject. `_message_text` now normalises via the `.text` property; checked against
     both providers' real message shapes, including that the thought signature never leaks
     into dialogue.

---

## IMMEDIATE

### I1. 50-question evaluation harness

**Current state.** Nothing exists. This unlocks the commented-out resume bullet and is
the single strongest JD match ("Create evaluation frameworks and benchmarks").

**Plan.**
1. **Dataset** (`evals/dataset.json`): ~50 entries. Schema per entry: unique id; NPC key;
   question text; category (`in_scope`, `out_of_scope_other_npc`, `out_of_scope_global`);
   gold evidence — a distinctive substring of the lore fact that should be retrieved
   (used for hit@k matching); a reference answer (1–2 sentences) for the judge.
   Authoring approach: draft candidate questions per NPC from each `lore.json`'s
   `known_facts` (LLM-assisted drafting is fine), then **hand-review every entry** for
   answerability and unambiguous gold evidence. Include ~10 adversarial out-of-scope
   entries (see M5). Aim for coverage across all 8 NPCs.
2. **Runner** (`evals/run.py`, invokable as a module): for each entry —
   - Call the retriever directly (import the agent's retriever from `agent_manager`) to
     get top-k documents; score **hit@1** and **hit@3** by gold-substring containment.
   - Call the full RAG path (via the shared router from M1) to get the NPC's answer.
   - **LLM-as-judge**: send question + retrieved context + reference answer + NPC answer
     to the judge model with a rubric prompt returning a structured verdict:
     `grounded_correct` / `grounded_incorrect` / `ungrounded (hallucinated)` /
     `appropriate_refusal`. Use Gemini as judge; judge prompt asks for a one-line reason
     (kept in the raw results for auditing). Known limitation to note in the report:
     Gemini judging Gemini answers has self-preference bias; mitigate by judging against
     the reference answer and retrieved context, not free-form.
   - CLI flags/env passthrough for provider (M3) and vector store (M2) so the same
     dataset scores every configuration.
3. **Outputs**: raw per-question JSON (committed) and `evals/results.md` with the summary
   table: hit@1, hit@3, grounded-answer accuracy on in-scope, refusal rate on
   out-of-scope — per configuration. Date-stamp each run and record model versions.
4. Un-comment the resume eval bullet and fill in the measured numbers.

**Done when.** `python -m evals.run` completes against a running stack and regenerates
`evals/results.md`; numbers on the resume match the committed results.

---

### I2. Test suites for both services

**Current state.** Zero test files in the entire repo (`*_test.go`, `test_*.py` both absent).

**Plan.**
1. **Python (pytest, in `ai-service-python/tests/`):**
   - *Router/servicer dispatch*: with a stub agent whose brains are fakes, assert each
     `event_type` reaches the right brain (`PLAYER_ASKED_QUESTION` → RAG; the five
     interaction events → LangGraph; unknown → fallback branch, including the
     anger-threshold behavior). This pins the core routing contract.
   - *`context_formatter`*: given a known dynamic-context dict, assert the formatted
     block contains/omits the right fields (empty memories, zero completion rate, etc.).
   - *`prompt_loader`*: loads the real gamedata JSON for one NPC and asserts name/
     occupation/prompt extraction; plus a malformed-JSON case.
   - *Retriever construction*: build the lore tool from a small fixture lore file using a
     **deterministic fake embedding class** (no Ollama dependency in CI) and assert the
     known fact is retrieved for its obvious query; assert the documented behavior for
     missing/empty `known_facts`.
   - LLM calls are always faked in tests — no network, no API keys in CI. Anything that
     genuinely needs Ollama gets a pytest marker that CI skips.
2. **Go (standard `testing`, table-driven):**
   - `quest_logic`: precondition validation in `ProcessEvent` — allowed vs blocked events
     against fixture quest definitions (wrong item, wrong quest step, correct submission).
     Where functions currently require live DB/Redis handles, prefer extracting the pure
     decision logic into testable functions over spinning up containers; if DB access is
     unavoidable for a case, use Ent's SQLite in-memory driver.
   - `npc_logic`: `EmotionManager` delta application from a fixture `event_emotions.json`
     — correct arithmetic, clamping at bounds, unknown-event handling.
   - `dto`: JSON round-trip of `EventMessage` for each event type the client can send.
3. Target is honest confidence, not coverage numbers: ~15–25 meaningful tests total.
4. Document how to run both suites in README (single command each).

**Done when.** Both suites pass locally with no external services and no API keys.

---

### I3. GitHub Actions CI

**Current state.** No `.github/` directory; resume skills section claims GitHub Actions.

**Plan.**
1. One workflow file, triggered on push and pull_request to main, with two parallel jobs:
   - *Python job*: checkout, Python 3.12, install `requirements.txt` (plus dev deps —
     add a small dev requirements file with pytest and ruff), run ruff (lint only; don't
     retro-format the whole codebase in the same PR), run pytest excluding
     network-marked tests. `GEMINI_API_KEY` must not be required (I2 guarantees this).
   - *Go job*: checkout, Go toolchain from `go.mod` version, `go vet`, `go build ./...`,
     `go test ./...` for the backend module. Decide vendor strategy first (I5 removes
     `vendor/`) so the job uses module downloads with caching.
2. Add the status badge to README.
3. Keep it to lint + build + test. No deploy stage until S2 exists.

**Done when.** A trivial PR shows both jobs green; a deliberately broken test shows red.

---

### I4. Containerize both services; one-command bring-up

**Current state.** `docker-compose.yml` runs only Postgres and Redis (with a hardcoded
password — fixed in I5). No Dockerfile exists for either service, yet README claims
"Full Docker Compose support".

**Plan.**
1. **Go Dockerfile** (`backend-go/`): multi-stage — build stage compiles a static binary;
   runtime stage is a minimal base image carrying the binary plus the `gamedata/`
   directory it reads at startup (seeder + quest manager load from disk; confirm the
   paths it expects and set the working directory accordingly).
2. **Python Dockerfile** (`ai-service-python/`): slim Python 3.12 base, install
   requirements, copy service code + its `gamedata/`. Note: agents build embeddings at
   startup and need Ollama reachable — the embedding host env var from M3 is what makes
   this containerizable.
**Known environment conflict (found 2026-07-16).** On the dev machine a *native* Postgres 17
(systemd) owns host port 5432 and holds the real project database — the compose `postgres`
service has never started (`failed to bind host port 0.0.0.0:5432: address already in use`),
so the stack has always run against the native server. Compose must therefore publish
Postgres on a configurable host port (`${POSTGRES_PORT:-5432}`) rather than assume 5432 is
free, and the quickstart must say which server the DSN points at.

3. **Compose expansion**: services for postgres, redis, qdrant (M2), ai-service, and
   backend. Healthchecks on all of them (the FastAPI `/health` from M1 serves the Python
   service; Go's existing health endpoint serves the backend); `depends_on` with
   healthy conditions so startup ordering is deterministic. All inter-service addresses
   flow through env vars (verify `internal/config/config.go` and the Python config read
   hosts rather than assuming localhost — fix where they don't). Ollama runs as an
   optional compose profile OR is documented as a host-machine dependency reached via
   the Docker host gateway — pick one and document it; embeddings make this mandatory
   to resolve, not optional.
4. Ship `.env.example` covering every variable compose consumes (I5 overlaps here).
5. Update README quickstart to: clone → copy env file → `docker compose up` → open the
   reference client (M6) → talk to an NPC.

**Done when.** From a clean machine with Docker (plus documented Ollama step), the full
stack starts with one command and the reference client completes a conversation.

**Execution note (done 2026-07-18).** Both Dockerfiles built (backend 85MB, ai-service
790MB). Compose expanded to postgres/redis/qdrant/ai-service/backend with healthchecks +
`depends_on: service_healthy`. Ollama chosen as a documented **host dependency** reached
via the host gateway (not a compose profile), because the dev box already runs it and a
containerized Ollama would re-download models. **Verification:** the containerized full
flow was exercised end to end — WebSocket → containerized Go backend → gRPC →
containerized Python ai-service → host Ollama → grounded `SPEAK` response — with both app
containers on host networking against throwaway Postgres/Redis. Also confirmed standalone:
ai-service container serves `/health` (8 agents loaded), `/v1/npcs`, `/v1/chat` (real RAG),
and gRPC `:50051`; backend container seeds the DB and connects to the gRPC server.
**Deviation:** the real bridge-network `docker compose up` additionally requires the host
Ollama to listen on `0.0.0.0` (its default `127.0.0.1` refuses the host-gateway
connection). Rebinding the box's systemd-managed Ollama was out of scope, so the flow was
verified via host networking instead, which is wire-identical (same images, same gRPC/HTTP
paths — only the address Ollama is dialed at differs). The `0.0.0.0` requirement is
documented in `.env.example` and the compose comment.

---

### I5. Repository hygiene

**Current state.** Interview-prep docs are committed and public; compose has a hardcoded
DB password; `vendor/` (Go) is committed; the architecture doc is untracked; `config.py`
carries dead commented code (removed by M3).

**Plan.**
1. Delete from the repo: `docs/INTERVIEW_CHEATSHEET.md`, `docs/INTERVIEW_PREP.md`,
   `docs/INTERVIEW_DAY_CHECKLIST.md`, `docs/PRACTICE_CODE_REVIEW.md`,
   `docs/PREPARATION_SUMMARY.md`. Move them somewhere private outside the repo first.
   They remain in git history; that's acceptable (nobody archaeology-digs a candidate
   repo), but if desired, a history rewrite before the repo gets shared is the only
   moment to do it cheaply — decide once, now.
2. Replace the hardcoded Postgres credentials in `docker-compose.yml` with env-var
   substitution; add `.env.example` with placeholder values; ensure `.env` is
   git-ignored. Rotate the password locally since the old one is in history.
3. Remove the committed `vendor/` directory and rely on Go modules (`go.sum` already
   pins everything); confirm builds and CI still pass. This dramatically shrinks the
   repo a reviewer clones and browses.
4. Track `docs/agentic_npc_documentation.md` (currently untracked) after refreshing any
   sections these changes invalidate (e.g., it inherits M7's corrected agent description).
5. Sweep for other reviewer-facing rough edges: stray debug prints in `servicer.py`
   (replace with logging when C1 lands, or trim now), the deprecated `grpc.Dial` note in
   `ai_client.go` (fixed properly in C2).

**Done when.** A recruiter browsing the GitHub repo sees only project material; compose
contains no secrets; clone size is reasonable; CI is green after the vendor removal.

**Execution note (done 2026-07-18).** (1) Removed all five interview-prep docs from the
repo (kept in history — no rewrite). (2) Compose password parametrization and (3)
`config.py` dead-code removal were already done earlier (I4 and M3 respectively). (3)
Removed the committed `vendor/` tree (3157 files) and added `/vendor/` to `.gitignore`;
verified `go build ./...`, `go vet ./...`, and `go test ./...` all pass from modules with
vendor gone (Docker build already uses `go mod download`, not vendor). (4) Tracked
`docs/agentic_npc_documentation.md`. (5) **Deferred:** the `servicer.py` debug-print /
`ai_client.go` `grpc.Dial` cleanups belong to C1/C2 (out of the current Phase 0–4 scope).
`docs/agent_prompt.md` left untracked deliberately — it is internal build scaffolding
(the implementing-agent prompt), same class as the interview docs.

---

### I6. README rewrite

**Current state.** README is well-written but asserts unbacked benchmark numbers and an
"integrated" UE5 client, and has no quickstart, no diagram, no badges.

**Plan.** Restructure to, in order:
1. One-paragraph pitch + demo GIF (recorded against the M6 reference client) + CI badge.
2. Quickstart: the I4 one-command bring-up, then "open `client-demo/` and talk to Elara".
3. Architecture: a Mermaid diagram (renders natively on GitHub) showing client ↔ Go
   orchestrator ↔ Python AI service, the two brains, and the data stores; a short
   two-brain explanation; link to `docs/client_protocol.md` and the architecture doc.
4. Benchmarks: the measured tables from `docs/benchmarks.md` (M4) — delete every number
   that doesn't come from a committed script.
5. Evals: the summary table from `evals/results.md` (I1) with a one-line methodology.
6. Configuration reference: the env-var matrix (provider, models, vector store, ports).
7. Roadmap: the honest subset of "Future Vision" not yet built.

**Done when.** Every claim in the README is either visible in code or linked to a
committed result file; a stranger reaches a working demo in under ten minutes.

---

## CAN BE DONE

### C1. Observability: structured logging + latency attribution built in

**Plan.** Go: replace `log`/`fmt` prints with stdlib `log/slog` (no new dependency);
attach request-scoped fields (player id, NPC, event type, request id). In
`game_handler.go`, time each stage — quest validation, emotion update, memory write,
context collection, gRPC call, WebSocket send — and emit one structured summary line per
event with all stage durations; this makes the M4 latency-attribution numbers
continuously observable rather than a one-off. Generate a request id at the WebSocket
handler and propagate it to Python via gRPC metadata. Python: replace `print` with the
stdlib `logging` module, JSON-ish structured format, include the propagated request id
and per-brain timing (the timing already exists in `npc_agent.py` as prints — formalize
it). Optional last step: a Prometheus `/metrics` endpoint on the Go service
(request counts, stage-latency histograms) — add only if time remains after S-items.

**Done when.** A single conversation event produces one correlated log line per service
sharing a request id, with a full stage-latency breakdown in the Go line.

### C2. Resilience on the gRPC boundary

**Plan.** In `ai_client.go`: replace deprecated `grpc.Dial` with the current client
constructor; enforce a per-call deadline (env-configurable, default a few seconds beyond
worst-case local inference); retry once on transient codes (Unavailable) with brief
backoff — never retry deadline-exceeded (the LLM call is expensive; a hung call
shouldn't double-bill). In `game_handler.go`: on AI failure after retry, send a graceful
in-character fallback line over the WebSocket instead of an error or silence, and log
the failure with the request id. Test by killing the Python service mid-session (this
scenario becomes an I2-style test where feasible, otherwise a documented manual check).

**Done when.** Killing the AI service mid-conversation yields the fallback response
within the deadline, and restarting it restores normal service with no Go restart.

### C3. Token streaming end-to-end

**Plan.** Add a server-streaming RPC alongside the existing unary `Think` in `ai.proto`
(keep unary for compatibility; regenerate bindings for both languages). Python: use the
LangChain streaming interface on the RAG chain to yield chunks into the gRPC stream;
the LangGraph path can stream only its final node's tokens (intermediate tool loops stay
silent). Go: consume the stream and forward each chunk over the WebSocket as a `partial`
message followed by a `final` message; extend `dto/event_dto.go` and
`docs/client_protocol.md` with the partial/final message shapes; update the reference
client (M6) to render progressively. Measure time-to-first-token before/after in the M4
harness — this becomes a new resume metric ("cut perceived latency from Xs to Yms TTFT").

**Done when.** The reference client visibly renders dialogue word-by-word and the TTFT
delta is recorded in `docs/benchmarks.md`.

### C4. Semantic response cache

**Plan.** In the Python service, in front of the RAG path only (quest events mutate
state and must never be cached): embed the incoming question, compare against cached
question embeddings for the same NPC, and on similarity above a tuned threshold return
the cached answer. Storage: Redis (already in the stack) keyed per NPC, embeddings
stored alongside answers; a linear scan per NPC is fine at this scale — note the ceiling
and the upgrade path (Redis vector search) in a comment. TTL of hours; no invalidation
needed since lore is static — but the cache key must incorporate the emotional-state
bucket if M5's context injection makes answers emotion-dependent (decide: either bucket
emotions coarsely into the key, or only cache when emotions are near-neutral; the second
is simpler and honest). Tune the threshold empirically against the eval dataset:
paraphrase pairs must hit, different-question pairs must miss. Instrument hit rate and
cached-response latency; report both in `docs/benchmarks.md` ("N% of repeated lore
queries served in ~Xms without an LLM call").

**Done when.** A paraphrased repeat of a lore question returns in cache-latency time with
the hit logged, and the eval suite (I1) passes unchanged with the cache enabled.

### C5. LangSmith tracing

**Plan.** LangSmith is already a dependency. Enable via the standard env vars (documented
in `.env.example`, disabled by default), verify traces capture the RAG chain and the full
M7 agent loop with tool executions, and export two annotated screenshots (one RAG trace,
one multi-step agent trace with a tool call) into `docs/` for the README and interviews.

**Done when.** Screenshots are committed and the env vars are documented.

---

## STRETCH GOALS

### S1. Writable RAG — persistent player-specific memories

**Plan.** Depends on M2 (Qdrant) and ideally C1. After each conversation exchange, an
async summarization step distills a one-sentence durable memory ("Player gave Elara a
silver locket; she was moved") using the light model, and upserts it into a per-NPC
*episodic* collection in Qdrant, tagged with player id and timestamp — separate from the
static lore collection. At question time, the RAG context becomes a merge of top-k lore
plus top-k episodic memories filtered to the current player. Deduplicate near-identical
memories at write time (similarity check against existing entries). Define the boundary
with the existing PostgreSQL `Memory` table explicitly in the architecture doc: PG stays
the authoritative event log (source of truth), the vector collection is a derived,
rebuildable semantic index over it — rebuild tooling is a one-off script. Extend the eval
dataset with memory-recall questions ("do you remember what I gave you?") scored the
same way as I1.

**Done when.** In a two-session demo, an NPC correctly recalls a player-specific event
from a previous session via retrieval, and the memory-recall eval entries pass.

### S2. Cloud deployment (AWS EC2)

**Plan.** Single EC2 instance (CPU-only; cloud provider path only — local Ollama
inference stays a laptop story, though embeddings still need Ollama on the box for
nomic-embed-text, so size the instance for that). Install Docker + compose, deploy the
I4 stack via a documented script or make target; reverse proxy (Caddy for automatic TLS)
in front of the Go WebSocket endpoint; security group locked to 443; secrets via a
non-committed env file on the host. Optional: a GitHub Actions deploy job (SSH + compose
pull/up) gated on main — only after CI (I3) is stable. Document teardown and monthly
cost. Host the reference client page from the same box so the demo is one URL.

**Done when.** A public URL serves the reference client talking to a live NPC, and the
deploy is reproducible from the doc.

### S3. Persona fine-tuning experiment (LoRA)

**Plan.** Scope: one NPC. (1) Dataset: generate ~500–1000 persona dialogue pairs with
Gemini from the NPC's personality/backstory/lore files, spanning in-scope QA, refusals,
and emotional variations; hand-review a sample for quality; hold out a test split.
(2) Train: QLoRA on llama3:8b via a standard recipe (Unsloth or axolotl) on Colab or a
rented GPU; track the run. (3) Serve: merge/convert and load through an Ollama Modelfile
so the M3 provider switch can select it with zero code changes. (4) Evaluate: extend the
LLM-judge rubric (I1) with a persona-consistency score (voice, refusal style, knowledge
boundaries) and compare fine-tuned vs prompted baseline on the held-out set; also check
the I1 groundedness metrics don't regress. Write up methodology and results in
`docs/finetuning.md`. Even a null result ("prompting matched fine-tuning at this scale")
is an honest, strong interview story — report what's measured.

**Done when.** The comparison table exists with judge scores for both configurations and
the model is selectable via env var.

### S4. NPC↔NPC multi-agent interactions

**Plan.** An orchestrated exchange in the Go service: a trigger (admin REST endpoint or
timer) selects two NPCs and a topic seeded from their shared quest/lore context; the Go
service alternates turns, calling each NPC's brain with the other's last utterance plus
their own dynamic context, capped at N turns; both NPCs write the exchange summary to
their memories (and S1's episodic store if built). Surface to the player as overheard
dialogue via a new WebSocket broadcast message type (protocol doc update). Guard rails:
turn cap, cost logging, and a kill switch env var.

**Done when.** Triggering an exchange produces a coherent N-turn dialogue visible in the
reference client, and each NPC can later reference it from memory.

### S5. Load testing

**Plan.** Add a stub-LLM mode to the Python service (env flag: canned response with
configurable artificial delay) so infrastructure limits are measured separately from
provider rate limits. Script a k6 (WebSocket support) or Locust scenario: connect,
authenticate, send events at a realistic cadence; ramp virtual users until p95 degrades.
Report max concurrent sessions, p95/p99 at plateau, error rate, and the first bottleneck
identified (CPU, DB pool, gRPC thread pool — the Python server's 10-worker executor is a
likely early ceiling; note it). Findings go in `docs/benchmarks.md`; the headline number
("sustained N concurrent players at p95 < Xms with stubbed inference") goes on the resume
with the stub caveat stated honestly.

**Done when.** The scenario is committed, results are in the benchmarks doc, and the
bottleneck analysis names a specific limit with evidence.

---

## NICE TO HAVE (brief)

- **N1. Demo GIF/video:** screen-record the M6 reference client completing an
  authenticate → lore question → gift-quest flow; embed in README. Do after C3 if
  streaming lands (progressive rendering demos far better).
- **N2. Knowledge-graph experiment:** prototype quest/relationship state as a graph and
  a retrieval comparison vs vector-only on multi-hop questions; write up findings.
  Timebox hard; a doc with honest results beats a half-integrated feature.
- **N3. Architecture blog post:** the two-brain model + benchmark findings, published on
  a personal blog/LinkedIn; link from README. Source material: this repo's docs.
- **N4. Cost analysis:** from the M4 runs, compute Gemini cost per 1k interactions vs
  local-inference amortized cost; one table in `docs/benchmarks.md`.
- **N5. Pre-commit hooks:** ruff + gofmt/govet via the pre-commit framework; config
  committed; mention in README's contributing note.
