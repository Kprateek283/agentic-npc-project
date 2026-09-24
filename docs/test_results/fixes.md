# Fix ledger

The record of every finding reported under `docs/test_results/` and what was
decided about it. One line per finding.

Key: `[x]` fixed · `[ ]` open, needs a decision · `[-]` not a defect

## go-01-episode-recording.md

- [ ] go-01 #1 dead gift config — OPEN: needs a decision on whether `events.json` `PLAYER_GAVE_GIFT` (joy 0.15, intensity 0.2) should be removed or wired up. The quest layer writes the gift episode itself from `items.json` `base_trust_value`, so honouring the events.json values would change the trust and joy a gift produces, which the quest tests pin down.
- [-] go-01 #2 masked over-revocation — NOT A DEFECT: an observation about how far the handler tests reach, not wrong behaviour; `memory.RevokeForgiveness` is correct today and `memory_test.go` is the right place to guard it.
- [x] go-01 #3 gofmt not clean — FIXED: ran gofmt on the two unformatted files. `internal/db/ent/schema/npc.go` had a stray blank line after the import block (the file is hand-written generator input, not generated output); `tools/tools.go` was also unformatted, missing its trailing newline. `gofmt -l .` is now empty.
- [-] go-01 #4 brief vs code on gifts — NOT A DEFECT: the brief and the code agree that gifts are not recorded in `recordEpisode`, and the full-path test confirms one row per gift. Nothing to change.

## go-02-memory-line-ranking.md

- [ ] go-02 #1 unattributed quest reward / admin trust lines — OPEN: needs a decision on whether the descriptions written by `quest_logic.go` ("Quest reward: trust changed by 0.50") and `event_handlers.go` ("Admin set trust with Elara to 0.80") should start with the actor so `formatMemoryLine` can say "You"/"Someone". Both strings are pinned by `memory_lines_test.go` (the bystander case wants the raw line), so changing them is a deliberate behaviour change, not a repair.
- [ ] go-02 #2 "counts and text" — OPEN: `memory_v1_assumptions.md` item 8 says the speaker's lines carry text, but `formatMemoryLine` never reads the stored `text`. Wiring it in changes what every prompt sees and gives conversations a non-zero weight; that is a design decision about the prompt, not a defect to repair quietly.
- [ ] go-02 #3 raw event identifiers in lines — OPEN: the plan wants "You threw stones at me 5 times", the code sends "You triggered PLAYER_THREW_STONE on Elara (5 times)". Needs a decision on whether to add an event-name renderer; the doc itself notes this is not a ranking bug.
- [ ] go-02 #4 "You's gift" in the no-space branch — OPEN: the branch is unreachable today (no description written takes it), and the right rendering for a possessive ("Your gift"? "Someone's gift"?) is a wording decision rather than a repair of observed behaviour.
- [-] go-02 #5 decayed repeat count — NOT A DEFECT: an observation that the shown count is the decayed count rounded, which is what the design asks for; recorded so nobody reads it as a literal tally.

## go-03-trust-computed-value.md

- [ ] go-03 #1 `target_npc_name` and `emotion` ignored in preconditions — OPEN: needs a decision on whether `RELATIONSHIP_TRUST` should check trust toward the named NPC rather than the NPC being spoken to. Adding the field changes precondition semantics for every quest and reaches across the quest schema, parser and evaluator; today's data never differs, so nothing is observably wrong yet.
- [x] go-03 #2 unclamped admin trust description — FIXED: `HandleAdminCommand` clamped the trust delta to [-1, 1] but built the row's description from the raw value, so setting trust to 1.5 stored 1.00 and described it as "Admin set trust with Elara to 1.50". The description now uses the same clamped `v` the delta uses. The redundant clamp itself was left alone: it is what makes the stored value correct.
- [ ] go-03 #3 a "set" trust fades — OPEN: the admin row is an ordinary `QUEST_REWARD` episode with intensity 0.3, so an admin-set trust decays over about a day. It lands exactly when set, which is what the brief asks, and `trust_test.go` pins the fading. Whether an admin set should be permanent is a design decision.
- [ ] go-03 #4 negative gifts are not harmful — OPEN: trash items are written with `harmful=false`, so they get diminishing rather than escalating returns and can never be a betrayal. Needs a decision on whether a negative `base_trust_value` should imply harm.
- [ ] go-03 #5 `created_at` from the column default — OPEN: no memory writer sets `created_at`; every row takes the wall clock rather than the caller's `now`, so episode order can disagree with `first_at`. Making it caller-supplied touches all seven write sites across `quest_logic` and `game_handler`, and no trust value depends on it today.
- [-] go-03 #6 clock-dependent existing tests — NOT A DEFECT: an observation about how `quest_pipeline_test.go` seeds and reads at `time.Now()`. It concerns test construction, not product behaviour, and tests are not mine to change.
- [ ] go-03 #7 `unlocks_quest_id` never acted on — OPEN: the reward is parsed but nothing starts the unlocked quest. Wiring it up is a feature in the quest engine, well beyond the file this was found in.

## go-04-grpc-client-mapping.md

- [ ] go-04 #1 the stream's action type is discarded — OPEN: `CallAIThinkStream` returns only text and `streamAI` always emits `SPEAK`, so a non-SPEAK action would be lost on the streaming path. Carrying it through changes the function's signature and the handler that consumes it; every path answers `SPEAK` today, so nothing is visibly broken.
- [ ] go-04 #2 text on the done frame is dropped — OPEN: the loop breaks on `Done` before appending. The Python servicer sends `text=""` on done frames, and `ai_client_test.go` pins the dropping, so changing it is a contract decision between the two services rather than a repair.
- [-] go-04 #3 two copies of the contract — NOT A DEFECT: the two `ai.proto` files are identical and `TestProtoContractCopiesMatch` now fails if they drift. Consolidating them is a build-layout change, not a defect.
- [-] go-04 #4 unknown agent replies as speech — NOT A DEFECT: an observation about behaviour in the handler/servicer, outside this package, and reported as a note rather than wrong behaviour here.
- [-] go-04 #5 unclamped float32 narrowing — NOT A DEFECT: the memory layer clamps before this point, so the narrowing cannot see an out-of-range value; recorded by the author as a note.

## go-05-password-handling.md

- [ ] go-05 #1 usernames can be enumerated — OPEN: `LoginPlayer` distinguishes "player 'x' not found" from "invalid password" and the websocket handler forwards the text verbatim. Collapsing them to one message is a security/UX decision that also changes what the handler shows, and `user_manager_test.go` pins both strings.
- [ ] go-05 #2 empty passwords are accepted — OPEN: registration with "" succeeds because the column's `NotEmpty` check sees the hash. `user_manager_test.go` pins this as "an empty password is accepted and then required", so adding a minimum-length rule is a deliberate policy change, not a repair.
- [ ] go-05 #3 bytes past 72 are ignored at login — OPEN: bcrypt's own limit, pinned by "only the first 72 bytes of a password count at login". Pre-hashing or refusing long passwords at login is a credential-policy decision.
- [-] go-05 #4 non-atomic exists-then-insert — NOT A DEFECT: the unique index still prevents a duplicate; the author notes only that the error text would differ under a race.
- [-] go-05 #5 case-sensitive usernames — NOT A DEFECT: an observation, pinned so a change would be deliberate.

## go-06-rate-limiting-offline.md

- [ ] go-06 #1 the fail-open branch is untested — OPEN: covering it needs a seam in `HandleGameEvent` (extracting the rate-limit decision). That is a refactor of handler structure to serve testability, and the shape of the seam is a decision for whoever takes the handler.
- [-] go-06 #2 sub-second windows rounded up by go-redis — NOT A DEFECT: configuration cannot produce one today (`LLM_RATE_WINDOW_SECONDS` is whole seconds); recorded as a note for a future caller.
- [ ] go-06 #3 the optional real-Redis test sleeps — OPEN: `TestAllowAgainstRedis` uses `time.Sleep` against the brief's "never add a sleep" rule. It is a defect in a test, and tests are not mine to rewrite; it only runs with `REDIS_ADDR` set, so CI is unaffected.
- [-] go-06 #4 refused calls keep incrementing — NOT A DEFECT: this is the fixed-window design working as intended; hammering does not extend the lockout, and the behaviour is pinned.

## go-07-seeding.md

- [x] go-07 #1 a seed failure's cause is lost at startup — FIXED: in `internal/app/app.go` steps 5 and 5b, `err := dbClient.Close()` shadowed the seed error, so when the close succeeded the function returned `fmt.Errorf("failed to seed NPCs: %w", nil)` — printed as "failed to seed NPCs: %!w(<nil>)" — and the real cause never reached the log. The close error now uses its own `closeErr`, leaving the seed error intact; verified with a standalone reproduction where the message goes from `%!w(<nil>)` to the validator's text and `errors.Is` finds the cause.
- [ ] go-07 #2 one bad file blocks all seeding — OPEN: all NPCs go in a single bulk insert, so one file failing the `NotEmpty` validator seeds none. Switching to per-file inserts (or skipping invalid entries with a warning, as malformed JSON already is) changes the all-or-nothing guarantee and needs a decision.
- [ ] go-07 #3 the quest name default never applies — OPEN: the schema defaults `name` to "Untitled Quest" but the seeder always calls `SetName(q.Name)`, so a missing name is "" and fails validation instead. Whether a nameless quest should seed under the default or fail loudly is exactly the decision, and it interacts with #2.
- [ ] go-07 #4 a wrong gamedata path seeds nothing silently — OPEN: a missing directory or empty glob is not an error, so a mistyped `GAMEDATA_DIR` starts a server with no NPCs. Making it fatal is a startup-policy decision (an empty gamedata dir is legitimate for some tests).
- [ ] go-07 #5 seeding never updates — OPEN: both seeders use `ON CONFLICT DO NOTHING`, so gamedata edits never reach an existing database despite the "Synchronizing with database" log. `seeder_test.go` pins the leave-it-alone behaviour, so update-on-conflict is a deliberate change.
- [-] go-07 #6 `main_quest_1.json` is never seeded — NOT A DEFECT: only `definitions/sq_*.json` is read and nothing in the Go code refers to that file; recorded as an observation about unused data.
- [-] go-07 #7 duplicate personality names collapse — NOT A DEFECT: an observation about glob order with no wrong behaviour reported in today's data.

## python-01-semantic-cache.md

- [x] python-01 #1 the Python suite is red — FIXED: the teardown now resets `LLM_PROVIDER` to `ollama` (the value conftest.py sets) instead of deleting it, so the final reload no longer falls back to gemini. A clean clone with no `.env` goes from 207 passed / 1 failed to 208 passed.
- [ ] python-01 #2 an empty answer is cached and replayed — OPEN: `put` stores any string and `get` treats "" as a hit, so a stream that produced no text serves an empty reply to every paraphrase for an hour. Pinned by "an empty answer is stored and served", so refusing empty values is a deliberate change; it also interacts with python-02 #1.
- [ ] python-01 #3 only the word "true" enables the cache — OPEN: `SEMANTIC_CACHE_ENABLED` is compared with "true", so "1" and "yes" silently disable it. Pinned. Whether to accept the usual truthy spellings is a config-convention decision that should apply to every flag, not just this one.
- [-] python-01 #4 FIFO rather than LRU eviction — NOT A DEFECT: consistent with the code's own "evict oldest" comment and pinned so a change would be deliberate.
- [-] python-01 #5 an entry is served at exactly `ttl` — NOT A DEFECT: an off-by-one-second boundary note on a `>=` cutoff, recorded as an observation.

## python-02-agy-provider.md

- [ ] python-02 #1 an empty response is returned, not raised — OPEN: `{"response": ""}` with exit 0 yields an empty reply where the brief's intent is to raise. Pinned by "an empty response field comes back as an empty reply", so changing it is a contract decision, and it should be settled together with python-01 #2.
- [ ] python-02 #2 a null response escapes as `ValidationError` — OPEN: `data["response"]` of `None` passes the parsing `try` and fails later inside `AIMessage(...)`, so it does not surface as the `RuntimeError` every other failure uses. `test_agy_chat_contract.py` pins exactly this (`ValidationError`, matching "AIMessage"), so making the type check raise `RuntimeError` is a deliberate contract change rather than a repair.
- [-] python-02 #3 stderr truncated to 500 characters — NOT A DEFECT: a deliberate cap, pinned; a CLI printing a long banner first is a hypothetical the author recorded as a note.
- [-] python-02 #4 `TimeoutExpired.stderr` may be bytes — NOT A DEFECT: called out as harmless and not asserted.

## python-03-provider-selection.md

- [x] python-03 #1 the existing reload test is the red one — FIXED: same fix as python-01 #1.
- [-] python-03 #2 import-time side effects — NOT A DEFECT: `from config import llm_light` binds a name at import, so a reload updates `config.llm_light` and not the consumer's copy. The author says outright this is not a bug in production (config loads once); it is a note about how tests must patch the consuming module.
- [ ] python-03 #3 surrounding whitespace is not stripped — OPEN: `LLM_PROVIDER=" ollama"` is lower-cased but not stripped, so it is rejected as unsupported, and `test_config.py` lists `" ollama"` among the unknown-provider cases. Whether config values should be stripped is the same convention decision as python-01 #3 (truthy spellings) and should be settled once for every variable, not here.
- [ ] python-03 #4 a bad number crashes with Python's own message — OPEN: `AGY_TIMEOUT_S=2m` fails at import with "invalid literal for int() with base 10: '2m'", which never names the variable. `test_config.py` pins that message (`match=r"invalid literal for int"`), and naming the variable means wrapping every `int(os.getenv(...))` in the module, so it is a decision about config error reporting rather than a repair.
- [-] python-03 #5 the default provider needs a key — NOT A DEFECT: the author records it as intended and pinned; it is the reason `conftest.py` forces `ollama`.

## python-04-prompt-templates.md

- [ ] python-04 #1 a brace in persona text breaks the lore path — OPEN, and the one finding in this batch the author calls a bug. `load_static_prompt` does not escape braces, so a `{word}` in a personality `summary` or a backstory `fact` becomes a `ChatPromptTemplate` variable the RAG chain never supplies, and every lore question to that NPC raises `KeyError`. No gamedata contains a brace today. The repair (escaping braces before the persona is joined to the template) is blocked here: `test_prompt_templates.py::test_a_brace_in_persona_text_becomes_a_placeholder_the_chain_never_fills` asserts `pytest.raises(KeyError, match="weapon")`, so the fix turns an existing test red. The test's own comment says "Pinned, not endorsed" — it needs a decision to unpin it, and unpinning is not mine.
- [ ] python-04 #2 the RAG template never explains the "after apologising" marker — OPEN: Go appends " — after apologising" to betrayal lines on both paths, but only `react_prompt.py` tells the model what it means ("A memory line ending 'after apologising' means that person broke an apology..."). The wording to add already exists verbatim in the sibling template and no test pins its absence, but adding it changes what the model sees on every lore answer, which is a prompt-design decision to sanction rather than a repair to make quietly.
- [-] python-04 #3 both templates repeat the "Toward you" paragraph — NOT A DEFECT: an observation about duplication, and the tests check both copies so they cannot drift apart silently.

## python-05-agent-wrapper.md

- [ ] python-05 #1 a stream with no text caches an empty answer — OPEN: `stream_rag_agent` stores `""` when the model's chunks carry no text, and every anonymous paraphrase is then answered empty for an hour without calling the model. Pinned by "a stream with no text caches an empty answer and replays it". The same contract decision as python-01 #2 and python-02 #1 — whether an empty answer is a value or a failure — and all three should be settled together.
- [-] python-05 #2 the iteration-cap line is the brief's trap — NOT A DEFECT: an observation about test construction. The cap's fallback begins "Forgive me, my thoughts wandered", which would satisfy a loose apology check, so tests must exclude it first; these do.
- [-] python-05 #3 the cache check precedes the retriever and the model — NOT A DEFECT: an observation that in-game requests pay no embedding cost for the cache.

## python-06-rest-surface.md

- [ ] python-06 #1 REST cannot say who is speaking — OPEN: `DynamicContext` has no `speaker` field and pydantic drops unknown keys, so every REST request is anonymous and cacheable, and REST callers with near-zero feelings share answers. Pinned by "a speaker sent over REST is dropped". Consistent with the module docstring (REST serves evals, benchmarks and demos), so whether to add the field or reject the key is a decision about what REST is for.
- [ ] python-06 #2 every exception is reported as a provider failure — OPEN: `_dispatch` turns any non-`UnknownAgentError` exception into 502 "AI provider call failed: <Type>", so a programming error such as python-04 #1's `KeyError` looks like an upstream outage. The catch-all is deliberate (its comment: "a stack trace must never reach a game client") and `test_rest_api.py` pins the exact body, so separating client errors from provider errors is an API contract change.
- [-] python-06 #3 event type validated before the NPC — NOT A DEFECT: an observation about ordering, pinned; an unknown NPC with an unknown event type gets 422 rather than 404.
- [-] python-06 #4 the 422 detail lists all known event types — NOT A DEFECT: recorded by the author as helpful and harmless.

## python-07-lore-tool.md

- [ ] python-07 #1 valid JSON of the wrong shape raises instead of returning no tool — OPEN: only the file read and `json.load` sit inside the `try`, so a list-shaped `lore.json` raises `AttributeError` and a non-string fact raises a pydantic `ValidationError`. `load_agents_on_startup` catches per NPC, so that NPC is dropped entirely — no persona, no quest agent — where a missing or empty lore file costs it only the lore tool. Widening the `try` is blocked here: `test_lore_tool.py` asserts `pytest.raises(error)` for exactly these shapes. Unpinning it is a decision about whether a malformed lore file should cost the NPC its lore tool or its whole agent.
- [ ] python-07 #2 a string `known_facts` is indexed character by character — OPEN: `"known_facts": "gate"` builds the facts "g", "a", "t", "e" with no error, and `test_a_string_instead_of_a_list_is_split_into_single_characters` pins it ("Pinned, not endorsed"). The same decision as #1: whether the loader validates the shape of `known_facts` at all.
- [-] python-07 #3 a lore file outside an NPC directory gets the collection `lore_` — NOT A DEFECT: `_collection_name` takes the parent directory, and the bare-filename case is pinned as `lore_`. Only reachable with a layout no gamedata uses; recorded so an unusual layout is a known risk rather than a surprise.
- [-] python-07 #4 facts under 500 characters are indexed whole — NOT A DEFECT: an observation that a longer fact would be split into overlapping chunks and could return as a fragment; no current fact is long enough.
