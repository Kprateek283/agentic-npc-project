# Test brief — agentic-npc-project

A copy of the brief handed to whoever writes the test suite. Mirrors the shared document; if the
two disagree, the code is the source of truth and the disagreement is itself worth reporting.

## What you are writing

A test suite for the whole repository, working on branch `experiment/agy-provider`, which
contains everything: the earlier fixes, the memory redesign and the experimental CLI-backed
chat provider.

The project has tests already, inventoried below, but they were written step by step alongside
features. The job is to make the suite trustworthy as a whole: cover what is untested, replace
checks that cannot fail, and leave something a stranger can run offline in seconds and believe.

**The hard rule: the suite runs with no network, no Docker, no database server, no Ollama and no
API keys.** Everything external is faked. Someone who has just cloned the repository must get
the same green result as CI.

## The system in one page

Two services talk over gRPC.

**Go orchestrator (`backend-go/`)** owns the world and decides everything: WebSocket sessions
with login, quest triggers and preconditions, a per-player rate limit kept in Redis, and NPC
memory. Each event becomes an "episode" row — who did it, its emotion deltas, how serious it
was, how often it has happened, whether it was forgiven or was a betrayal. Feelings toward a
player and the NPC's general mood are **computed from those rows on every request**, never
stored.

**Python AI service (`ai-service-python/`)** turns that state into speech. A router picks one of
two paths by event type: a retrieval path for lore questions, and a LangGraph tool-calling path
for events such as gifts, attacks and apologies. It never changes game state.

The consequence for testing: **nearly all behaviour worth testing is deterministic and needs no
model.** The maths, the rules file, episode recording, memory ranking, routing and prompt
formatting can be tested exactly. Only the final wording needs an LLM, and that is out of scope.

## What exists today

**Go** — 10 files, 31 test functions, all passing:

| File | Tests | Covers |
| --- | --- | --- |
| `internal/domain/memory/memory_test.go` | 10 | The memory maths: half-lives, escalation, decay, separate caps, forgiveness, betrayal |
| `internal/domain/quest_logic/quest_pipeline_test.go` | 6 | Quest completion against real SQLite: triggers, preconditions, gifts, admin trust |
| `internal/domain/quest_logic/quest_logic_test.go` | 5 | Keyword matching, trust operators, item lookup, admin gate |
| `internal/domain/rules/rules_test.go` | 2 | Loading `gamedata/events.json` and 10 validation failures |
| `internal/api/handlers/game_handler_test.go` | 2 | The whole demo sequence against a database, plus the emotions frame payload |
| `internal/api/handlers/websocket_handler_test.go` | 1 | Origin allow-listing (15 sub-cases) |
| `internal/ratelimit/ratelimit_test.go` | 2 | Fixed-window limiting; the Redis one skips unless `REDIS_ADDR` is set |
| `internal/config/config_test.go` | 1 | Defaults, overrides, invalid values |
| `internal/domain/npcstate/npcstate_test.go` | 1 | Old-style rows loading with no emotional effect |
| `internal/dto/event_dto_test.go` | 1 | JSON round-trip |

**Go packages with no tests at all:** `internal/app`, `internal/infra/grpc_client`,
`internal/infra/database`, `internal/infra/httpsapi`, `internal/api/router`,
`internal/domain/user_logic`, `cmd/server`.

**Python** — 8 files, 31 test functions, all passing, plus `ruff`:

| File | Tests | Covers |
| --- | --- | --- |
| `tests/test_agy_chat.py` | 8 | The CLI-backed provider with a faked subprocess |
| `tests/test_context_formatter.py` | 5 | The two labelled emotion lines and memory lines |
| `tests/test_router.py` | 5 | Event-type routing and the non-LLM fallback |
| `tests/test_servicer.py` | 3 | Streaming cancellation when the client disconnects |
| `tests/test_prompt_loader.py` | 3 | Persona prompt building from game data |
| `tests/test_retriever.py` | 3 | Lore retrieval with faked embeddings |
| `tests/test_agent_manager.py` | 2 | NPC key resolution, including legacy path keys |
| `tests/test_health.py` | 2 | `/health` returning 503 with no agents |

**Python modules with no test file:** `semantic_cache.py` (it has a `__main__` self-check
instead, which never runs in CI), `config.py`, `main.py`, `api/app.py` beyond health,
`agents/npc_agent.py`, `agents/rag_builder.py`, `agents/graph_builder.py`,
`agents/agent_state.py`, `tools/lore_retriever_tool.py`, `tools/quest_status_tool.py`, and the
eval harness under `evals/`.

## What to write — Go

In priority order. Higher items protect behaviour a player would notice.

1. **Episode recording (`internal/api/handlers`, `recordEpisode`).** The richest untested logic.
   Each branch deserves its own test against SQLite: a plain event creating a row; a repeat
   merging into it with a decayed count rather than a second row; a conversation event never
   merging and keeping its text; an apology raising forgiveness on that actor's harmful
   episodes, recording what it covered, and being worth half as much the second time; a harmful
   event from a forgiven actor creating a *new* episode flagged as betrayal with the extra trust
   penalty; an unknown event type recording nothing without erroring; gifts and quest rewards
   **not** being recorded here (the quest layer writes those — confirm no duplicate row appears).
2. **Memory line ranking (`buildAIArgs`).** Which memories reach the prompt and how they read:
   the speaker's own top five by weight; other players' episodes only above the notability
   threshold, capped at three; "You ..." for the speaker and "Someone ..." for others; a repeat
   count appended; the betrayal marker. Include an NPC with more than a dozen episodes across
   several players, which nothing currently covers.
3. **Trust as a computed value (`internal/domain/npcstate`, `quest_logic`).** Trust rising from a
   gift and decaying with time; a quest precondition passing then failing as the memory fades;
   `ADMIN_SET_TRUST` replacing history so the value lands exactly; legacy rows with no delta
   contributing nothing.
4. **The gRPC client mapping (`internal/infra/grpc_client`).** Currently untested. Build a
   request from known maps and lines and assert the wire fields, including an empty map and an
   emotion name the protocol never saw before.
5. **Password handling (`internal/domain/user_logic`).** Untested: a registered password
   verifies, a wrong one fails, a duplicate registration is refused, and hashes made with the
   older cost still verify.
6. **Rate limiting without Redis.** The existing Redis test skips when `REDIS_ADDR` is unset, so
   CI never runs it. Add a fake client or an interface so the window, the limit boundary and the
   fail-open path are covered offline; keep the real-Redis test as the optional extra.
7. **Seeding (`internal/infra/database`).** Seeding a fresh SQLite database loads all NPCs and
   quests, and running it twice does not duplicate them.

Not worth testing: `cmd/server`, `internal/app` wiring, `internal/api/router` and
`internal/infra/httpsapi` are thin wiring whose failure is immediate and obvious. Say so in the
pull request rather than writing hollow tests for them.

## What to write — Python

1. **The semantic cache (`semantic_cache.py`).** It has a `__main__` self-check that CI never
   runs, so in practice it is untested. Convert it to real tests and extend: a hit above the
   threshold, a miss below it, expiry by age, eviction at capacity, and the rule that matters
   most — a context carrying a speaker or any memory line is never cacheable, because one
   player's answer must never be replayed to another.
2. **The CLI-backed provider (`agy_chat.py`), beyond what exists.** Faking `subprocess.run`
   throughout: a prompt containing shell metacharacters travels as one argument with no shell
   involved; `--model` appears only when configured; the subprocess timeout is the CLI timeout
   plus its margin; a non-zero exit, unparsable output and a timeout each raise rather than
   returning an empty reply; `bind_tools` raises so the tool-calling path fails at wiring time
   rather than mid-conversation; streaming yields the whole reply in one piece.
3. **Provider selection (`config.py`).** Each `LLM_PROVIDER` value builds the expected pair of
   models, the reported model names match, an unknown value still fails at import, and embeddings
   stay on Ollama in every mode. Import-time side effects make this awkward — use
   `importlib.reload` with patched environment, and say in the pull request if the module needs a
   small change to be testable.
4. **The two prompt templates (`prompts/`).** Cheap and valuable: every placeholder the chains
   supply is present and renders, no stray braces break formatting, and the rules that were added
   deliberately are still there — the grounding rules, the 0.5 threshold for showing strong
   feelings, the "general mood is never the speaker's fault" separation, and the instruction not
   to search the lore book for an event name. These are the sentences a future edit is most
   likely to drop by accident.
5. **The agent wrapper (`agents/npc_agent.py`).** With a stubbed chat model: a cached answer
   short-circuits without calling the model; the streaming path yields the pieces then stores the
   whole answer; a cancelled stream does **not** write to the cache; hitting the iteration cap
   returns the in-character line instead of raising.
6. **The REST surface (`api/app.py`).** Only `/health` is covered. Add: an unknown NPC gives 404,
   a bad event type 422, a provider failure 502 with no stack trace in the body, and `/v1/npcs`
   listing what is loaded.
7. **The lore tool (`tools/lore_retriever_tool.py`).** Missing or malformed lore returns no tool
   rather than raising, and the per-NPC collection name is derived as documented.

## Rules the suite follows

- **Nothing external.** No network, no Postgres, no Redis, no Ollama, no API key, no `agy`
  binary, no Docker. Go database tests use in-memory SQLite through the generated `enttest`
  helper, as `quest_pipeline_test.go` already does. Python fakes every model and embedding call.
- **No clock dependence.** The memory maths takes `now` as a parameter precisely so tests can be
  exact. Never call `time.Now()` and hope; pass fixed times and assert real numbers. Never add a
  sleep to "let something settle".
- **Assert literals, not the implementation.** Write the expected number the design predicts, not
  a value recomputed with the same formula the code uses. A test that recalculates the
  implementation cannot catch a wrong implementation.
- **Floating point:** a relative tolerance of about 1% is the house style; state it once per file
  rather than sprinkling magic constants.
- **Table-driven** in both languages, in the style already in the repository. Name subtests after
  the behaviour, not the function.
- **No new dependencies** beyond what is installed. Go test-only `mattn/go-sqlite3` is already
  there; Python has `pytest` and `ruff` in `requirements-dev.txt`.
- **Generated code is not tested:** `backend-go/internal/db/ent/`, `backend-go/internal/proto/`,
  `ai-service-python/ai_pb2*`.
- **Speed:** the whole suite should stay in seconds. If a test needs minutes it belongs in the
  manual list instead.
- `ruff check .` must stay clean; `gofmt` must leave no file unformatted.

## Traps this project has already sprung

Every one of these cost real time here. They are the failure modes most worth guarding against.

- **A check that passed on an error message.** A verification script looked for
  "apolog|sorry|forgiv" in the reply and went green against the server's fallback line,
  "*Forgiv*e me — my mind wandered". Any test asserting on generated text must first assert the
  text is not one of the canned fallback lines.
- **A check that could not fail.** Asserting a value recomputed with the implementation's own
  formula proves nothing. Write the number down.
- **Shared state between runs.** A demo harness reused one database, so an NPC accumulated angry
  players across runs and a general-mood assertion drifted. Every test starts from a clean
  database.
- **A "set" that quietly became an "add".** When trust moved from a stored column to a computed
  value, `ADMIN_SET_TRUST` started appending instead of replacing, and every existing test still
  passed. Test the *semantics* a command promises, not just that it wrote something.
- **Fixed-shape assumptions.** Emotions were once five fixed fields and are now a map. Tests
  should tolerate an unfamiliar emotion name rather than assuming the five.
- **Ports and binaries assumed present.** The gRPC port is hardcoded at 50051 while the REST port
  is configurable, so anything binding a real port collides with a running container. Tests
  should bind nothing.

## Out of scope

Do not attempt these, and do not fake them convincingly enough to look covered:

- **Model wording.** Whether an NPC sounds angry is not a unit test. That is checked by hand and
  recorded in `docs/memory_v1_plan.md` and `docs/agy_provider_experiment.md`.
- **The browser panel.** `client-demo/index.html` has never been opened in a browser; that needs
  a person.
- **Anything spending quota or needing a running stack**: real `agy` calls, real Ollama,
  `docker compose`, the eval harness (`evals/run.py`) against live models. Testing its dataset
  validation offline is fine and welcome.
- **Performance and load.** The committed benchmark scripts cover that separately.

## How to run it

Python needs 3.12; it is pinned in `ai-service-python/.python-version`.

```bash
# Go
cd backend-go
GOWORK=off go build ./... && GOWORK=off go vet ./... && GOWORK=off go test -count=1 ./...

# Python
cd ai-service-python
uv venv && uv pip install -r requirements.txt -r requirements-dev.txt   # first time only
PYTHONPATH=. .venv/bin/python -m pytest tests/ -q
.venv/bin/ruff check .
```

CI runs the same two jobs on every push and pull request (`.github/workflows/ci.yml`). It has no
Redis, so anything requiring one must skip cleanly rather than fail.

## Done means

- Both suites pass from a clean clone with no network, no Docker and no API keys, and are still
  fast.
- The Go packages listed as untested either have tests or are named in the pull request as
  deliberately skipped, with the reason.
- The semantic cache is covered by real tests rather than a self-check CI never runs.
- No test asserts on generated text without first excluding the fallback lines, and no test
  recomputes the implementation to check the implementation.
- Each new test fails when the behaviour it describes is broken. The quickest proof: break the
  line deliberately, watch the test go red, put it back. Record which ones were verified this way
  — it is the only real evidence a test works.
- The pull request says what is covered, what is deliberately not, and anything found along the
  way that looks like a bug rather than a missing test. Report those; do not fix them in the same
  change.

Background: `docs/memory_v1_plan.md` (what each step did and how it was checked),
`docs/memory_v1_assumptions.md` (every design decision, numbered),
`docs/agy_provider_experiment.md` (the provider and its measured results), and `README.md`
(architecture and known limitations).

## Progress

- 2026-09-24 — Go 1, episode recording: done. `internal/api/handlers/episode_recording_test.go`, 15 table cases on SQLite, all branches of `recordEpisode`. Added a small seam, `recordEpisodeAt(..., now)`, so the tests can use fixed times; `recordEpisode` still calls it with `time.Now()`. Mutation-checked: never merging, merging without decay, conversation merging, conversation text dropped, no apology halving, covers dropped, forgiving every actor, no betrayal penalty, betrayal penalty written into the shared rule map, harmless event treated as betrayal, gift/reward recorded twice, unknown event recorded, handler zeroing other actors' forgiveness. Findings: (a) `events.json` `PLAYER_GAVE_GIFT` (joy 0.15, intensity 0.2) is dead config, because gifts are written by the quest layer with `items.json` `base_trust_value` as trust, and the gift-subject and gift-description branches inside `recordEpisode` are unreachable after its early return; (b) `memory.RevokeForgiveness` over-revoking is masked at this layer by the handler's own actor check, so only `memory_test.go` can catch it; (c) `internal/db/ent/schema/npc.go` fails `gofmt -l` on the base branch (one stray blank line), which the brief says must stay clean. Full results: `docs/test_results/go-01-episode-recording.md`. Next: Go 2, memory line ranking (`buildAIArgs` also calls `time.Now()` and will need the same seam).
- 2026-09-24 — Go 2, memory line ranking: done. `internal/api/handlers/memory_lines_test.go`, 13 table cases plus a 15-episode, four-player fixture read from two speakers, all on SQLite at a fixed instant. Added the seam `buildAIArgsAt(..., now)`; `buildAIArgs` still calls it with `time.Now()`. Mutation-checked 15 breaks (limits, threshold boundary, sort order, tie-break, You/Someone wording, count rounding, betrayal marker, clock, forgiveness, betrayal load); all went red. Findings: (a) quest-reward and admin-set-trust descriptions do not begin with the actor, so a bystander's reward (weight 0.5, above notability) reaches other players' prompts unattributed, reading as the speaker's own; (b) the design says speaker lines carry "counts and text", but a conversation's question text never reaches a line and conversations/apologies always weigh 0; (c) lines carry raw event-type names ("You triggered PLAYER_THREW_STONE on Elara (5 times)") rather than the prose the plan describes. Full results: `docs/test_results/go-02-memory-line-ranking.md`. Next: Go 3, trust as a computed value.
- 2026-09-24 — Go 3, trust as a computed value: done. `internal/domain/quest_logic/trust_test.go`, 30 table cases on SQLite through the real gift, quest and admin paths at fixed instants: gifts raising trust and halving per half-life, merged repeats, the positive cap, per-player trust, Elara/Baelor preconditions passing then failing as a gift fades, the sq_e1 reward, `ADMIN_SET_TRUST` replacing gifts, attacks and legacy rows so the value lands exactly, legacy rows contributing nothing. Added seams `handleGiftingAt`/`checkQuestCompletionAt` and `now` parameters on `checkPreconditionsAt`/`applyRewards`; behaviour unchanged. Mutation-checked 15 breaks; 14 went red, the admin clamp survived because `memory.Contribution` clamps anyway. Findings: (a) precondition `target_npc_name` and `emotion` are parsed but ignored; (b) admin clamp redundant and its description shows the unclamped value; (c) an admin-set trust fades with a 15-hour half-life; (d) negative-value gifts are recorded as not harmful; (e) existing `quest_pipeline_test.go` reads the wall clock twice. Full results: `docs/test_results/go-03-trust-computed-value.md`. Next: Go 4, the gRPC client mapping.
- 2026-09-24 — Go 4, gRPC client mapping: done. `internal/infra/grpc_client/ai_client_test.go`, the package's first tests, 25 cases: `buildEventRequest` checked on the encoded wire (field numbers, separate speaker/mood maps, unfamiliar emotion names, empty and nil maps, zero values, float32 narrowing, no aliasing), `CallAIThink` retry and deadline rules and `CallAIThinkStream` token handling against a fake client and stream, plus a check that the Go and Python copies of `ai.proto` are identical. No production change; nothing binds a port. Mutation-checked 20 breaks, all went red. Findings: (a) the stream's done-frame `action_type` is discarded and `streamAI` always sends SPEAK; (b) text on a done frame is dropped; (c) the contract exists in two copies, now guarded; (d) an unknown agent reaches the player as the speech "Error: Agent not found.". Full results: `docs/test_results/go-04-grpc-client-mapping.md`. Next: Go 5, password handling.
- 2026-09-24 — Go 5, password handling: done. `internal/domain/user_logic/user_manager_test.go`, the package's first tests, 15 cases on SQLite: register then log in, wrong passwords, duplicate registration refused without overwriting, case-sensitive usernames, unknown player, empty username, the 72-byte limit, empty passwords, literal cost-14 (the pre-`d783cf5` cost) and cost-4 hashes still verifying, a plain-text row refused, and a new row holding a fresh cost-10 hash. No production change; the cost-14 case takes about a second. Mutation-checked 10 breaks, all went red. Findings: (a) login errors distinguish an unknown player from a wrong password and reach the client, so usernames can be enumerated; (b) empty passwords are accepted; (c) bytes past 72 are ignored at login while registration refuses them. Full results: `docs/test_results/go-05-password-handling.md`. Next: Go 6, rate limiting without Redis.
- 2026-09-24 — Go 6, rate limiting without Redis: done. `internal/ratelimit/ratelimit_fake_test.go`, 15 cases: the real go-redis client talks RESP to an in-process fake over `net.Pipe` via `Options.Dialer`, with a manual clock, so no port, no Redis and no production change. Covers the limit boundary, refused calls still counted, the fixed 60s window (limited at 59s, reset at 60s, not extended by later calls), per-player keys, the literal MULTI/INCR/EXPIRE NX/EXEC transaction, disabled limits never touching Redis, and nil/unreachable/erroring Redis returning an error with allowed=false. The real-Redis test stays as the optional extra. Mutation-checked 10 breaks, all went red. Findings: (a) the fail-open decision in `HandleGameEvent` still has no offline test and needs a handler seam; (b) the optional real-Redis test sleeps; observation: go-redis rounds a sub-second window up to 1s, which configuration (whole seconds) cannot currently produce. Full results: `docs/test_results/go-06-rate-limiting-offline.md`. Next: Go 7, seeding.
- 2026-09-24 — Go 7, seeding: done. `internal/infra/database/seeder_test.go`, the package's first tests, 15 cases: the real gamedata seeds eight NPCs (occupations and file paths as literals) and seven side quests into fresh SQLite; a second seed keeps the same NPC ids; an existing NPC is left untouched; synthetic directories cover empty and missing paths, malformed files, occupation shapes, the `sq_*.json` glob, the in-file quest_id, and duplicate names. No production change. Mutation-checked 11 breaks, all went red. Findings: (a) BUG: `app.go` shadows the seed error (`err := dbClient.Close()`), so a failed seed is reported as "failed to seed NPCs: %!w(<nil>)"; (b) one NPC or quest file without a name fails the whole bulk insert; (c) the "Untitled Quest" default never applies; (d) a missing gamedata directory seeds nothing silently; (e) existing rows are never updated though the log says "Synchronizing"; (f) `quests/main_quest_1.json` is never seeded. The Go list is complete. Full results: `docs/test_results/go-07-seeding.md`. Next: Python 1, the semantic cache.
- 2026-09-24 — Python 1, semantic cache: done. `tests/test_semantic_cache.py`, 38 cases replacing the `__main__` self-check: exact-cosine hits and misses at the threshold boundary, best match, expiry at exactly ttl with a fake clock (the module's `time` monkeypatched, no production change), FIFO eviction, defaults and env overrides, the disabled cache, and `cacheable_context` never allowing a speaker or any memory line. Mutation-checked 17 breaks, all went red (one after reordering a case). Findings: (a) BASELINE RED: `test_agy_chat.py::test_config_with_agy_provider` fails without a Gemini key because its cleanup deletes the conftest's `LLM_PROVIDER=ollama`; the brief's "31 tests, all passing" is out of date (40 tests, 1 failing); not fixed, owner notified; (b) an empty answer is cached and replayed; (c) `SEMANTIC_CACHE_ENABLED=1` or `yes` disables the cache. Full results: `docs/test_results/python-01-semantic-cache.md`. Next: Python 2, the CLI-backed provider.
- 2026-09-24T02:35Z — IN PROGRESS: Python 2, CLI-backed provider
