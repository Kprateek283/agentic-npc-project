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

- [x] go-02 #1 unattributed quest reward / admin trust lines — FIXED: both writers now start the description with the actor ("p1 completed a quest for me: trust changed by 0.40", "p1 had trust with Elara set to 1.00 by an admin"), so `formatMemoryLine` renders a bystander's reward as "Someone ..." and the speaker's as "You ...". The reward text also now shows the clamped value it stores (the go-03 #2 bug at its sibling site). New `TestTrustRowDescriptionsStartWithTheActor` drives the real writers; verified red with the old ones. Rows written before this change keep their old text.
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

- [x] go-05 #1 usernames can be enumerated — FIXED: `LoginPlayer` returns one `ErrInvalidCredentials` ("invalid username or password") for an unknown name and a wrong password alike, and runs a bcrypt compare against a dummy hash on the unknown-name path so timing doesn't reveal it either. A real database failure is reported separately. New `TestLoginFailuresLookTheSame`.
- [x] go-05 #2 empty passwords are accepted — FIXED: `RegisterPlayer` refuses an empty password ("password must not be empty") before touching the database.
- [x] go-05 #3 bytes past 72 are ignored at login — FIXED: `checkPasswordHash` fails any password over 72 bytes, matching registration's refusal, instead of letting bcrypt ignore the tail.
- [-] go-05 #4 non-atomic exists-then-insert — NOT A DEFECT: the unique index still prevents a duplicate; the author notes only that the error text would differ under a race.
- [-] go-05 #5 case-sensitive usernames — NOT A DEFECT: an observation, pinned so a change would be deliberate.

## go-06-rate-limiting-offline.md

- [x] go-06 #1 the fail-open branch is untested — FIXED: the rate-limit decision moved into `WebSocketHandler.allowLLM`, which returns true on any limiter error. New `TestAllowLLMFailsOpen` covers no client and an unreachable Redis offline; verified red with the error path returning false.
- [-] go-06 #2 sub-second windows rounded up by go-redis — NOT A DEFECT: configuration cannot produce one today (`LLM_RATE_WINDOW_SECONDS` is whole seconds); recorded as a note for a future caller.
- [x] go-06 #3 the optional real-Redis test sleeps — FIXED: both sleeps replaced: the TTL is shortened by hand so an extension shows as a jump back to the window, and the key is deleted to end the window. The old check could not catch a missing NX (TTL has 1 s resolution); the new one does, verified by swapping ExpireNX for Expire. Runs in 0.00 s instead of ~2.2 s.
- [-] go-06 #4 refused calls keep incrementing — NOT A DEFECT: this is the fixed-window design working as intended; hammering does not extend the lockout, and the behaviour is pinned.

## go-07-seeding.md

- [x] go-07 #1 a seed failure's cause is lost at startup — FIXED: in `internal/app/app.go` steps 5 and 5b, `err := dbClient.Close()` shadowed the seed error, so when the close succeeded the function returned `fmt.Errorf("failed to seed NPCs: %w", nil)` — printed as "failed to seed NPCs: %!w(<nil>)" — and the real cause never reached the log. The close error now uses its own `closeErr`, leaving the seed error intact; verified with a standalone reproduction where the message goes from `%!w(<nil>)` to the validator's text and `errors.Is` finds the cause.
- [x] go-07 #2 one bad file blocks all seeding — FIXED: an NPC file with a missing or empty `name` is skipped with a warning, like malformed JSON already was, so one bad file no longer fails the bulk insert for every NPC.
- [x] go-07 #3 the quest name default never applies — FIXED: the seeder only calls `SetName` when the file has a name, so a nameless quest seeds as "Untitled Quest", the default the schema already declares.
- [x] go-07 #4 a wrong gamedata path seeds nothing silently — FIXED: both seeders start with `requireDir`: a missing gamedata directory (or a file in its place) is an error naming the path, and seed errors already stop startup. A directory that exists but is empty is still allowed.
- [ ] go-07 #5 seeding never updates — OPEN: both seeders use `ON CONFLICT DO NOTHING`, so gamedata edits never reach an existing database despite the "Synchronizing with database" log. `seeder_test.go` pins the leave-it-alone behaviour, so update-on-conflict is a deliberate change.
- [-] go-07 #6 `main_quest_1.json` is never seeded — NOT A DEFECT: only `definitions/sq_*.json` is read and nothing in the Go code refers to that file; recorded as an observation about unused data.
- [-] go-07 #7 duplicate personality names collapse — NOT A DEFECT: an observation about glob order with no wrong behaviour reported in today's data.

## python-01-semantic-cache.md

- [x] python-01 #1 the Python suite is red — FIXED: the teardown now resets `LLM_PROVIDER` to `ollama` (the value conftest.py sets) instead of deleting it, so the final reload no longer falls back to gemini. A clean clone with no `.env` goes from 207 passed / 1 failed to 208 passed.
- [x] python-01 #2 an empty answer is cached and replayed — FIXED: `SemanticCache.put` ignores empty or whitespace-only answers, so a failed generation is never replayed. One guard covers both callers in `npc_agent.py`.
- [x] python-01 #3 only the word "true" enables the cache — FIXED: flags go through `env.env_bool`, which accepts 1/true/yes/on in any case; `SEMANTIC_CACHE_ENABLED=1` now enables the cache.
- [-] python-01 #4 FIFO rather than LRU eviction — NOT A DEFECT: consistent with the code's own "evict oldest" comment and pinned so a change would be deliberate.
- [-] python-01 #5 an entry is served at exactly `ttl` — NOT A DEFECT: an off-by-one-second boundary note on a `>=` cutoff, recorded as an observation.

## python-02-agy-provider.md

- [x] python-02 #1 an empty response is returned, not raised — FIXED: `ChatAgy` raises `RuntimeError("agy returned no text ...")` when `response` is empty or whitespace.
- [x] python-02 #2 a null response escapes as `ValidationError` — FIXED: the same check rejects a non-string `response`, so `null` surfaces as `RuntimeError` like every other agy failure, not a pydantic `ValidationError`.
- [-] python-02 #3 stderr truncated to 500 characters — NOT A DEFECT: a deliberate cap, pinned; a CLI printing a long banner first is a hypothetical the author recorded as a note.
- [-] python-02 #4 `TimeoutExpired.stderr` may be bytes — NOT A DEFECT: called out as harmless and not asserted.

## python-03-provider-selection.md

- [x] python-03 #1 the existing reload test is the red one — FIXED: same fix as python-01 #1.
- [-] python-03 #2 import-time side effects — NOT A DEFECT: `from config import llm_light` binds a name at import, so a reload updates `config.llm_light` and not the consumer's copy. The author says outright this is not a bug in production (config loads once); it is a note about how tests must patch the consuming module.
- [x] python-03 #3 surrounding whitespace is not stripped — FIXED: every variable is read through `env.env_str`, which strips surrounding whitespace, so `LLM_PROVIDER=" ollama"` selects ollama.
- [x] python-03 #4 a bad number crashes with Python's own message — FIXED: numbers go through `env.env_int`/`env_float`, which fail with "AGY_TIMEOUT_S must be an integer, got '2m'".
- [-] python-03 #5 the default provider needs a key — NOT A DEFECT: the author records it as intended and pinned; it is the reason `conftest.py` forces `ollama`.

## python-04-prompt-templates.md

- [x] python-04 #1 a brace in persona text breaks the lore path — FIXED: `build_rag_chain` escapes the persona's braces before joining it to the RAG template, so authored text reaches the model as written. The agent path sends the persona as a literal SystemMessage and was never affected, so the escape belongs at the template boundary, not in the loader. The pinning test now asserts the literal text arrives; removing the escape turns it red.
- [x] python-04 #2 the RAG template never explains the "after apologising" marker — FIXED: the RAG template now carries the same sentence as the react template ("A memory line ending 'after apologising' means that person broke an apology, which you may hold against them"); the rule-presence test requires it in both.
- [-] python-04 #3 both templates repeat the "Toward you" paragraph — NOT A DEFECT: an observation about duplication, and the tests check both copies so they cannot drift apart silently.

## python-05-agent-wrapper.md

- [x] python-05 #1 a stream with no text caches an empty answer — FIXED: covered by the python-01 #2 guard in `put`; the pinning test now asserts a textless stream reaches the model again on the next paraphrase.
- [-] python-05 #2 the iteration-cap line is the brief's trap — NOT A DEFECT: an observation about test construction. The cap's fallback begins "Forgive me, my thoughts wandered", which would satisfy a loose apology check, so tests must exclude it first; these do.
- [-] python-05 #3 the cache check precedes the retriever and the model — NOT A DEFECT: an observation that in-game requests pay no embedding cost for the cache.

## python-06-rest-surface.md

- [ ] python-06 #1 REST cannot say who is speaking — OPEN: `DynamicContext` has no `speaker` field and pydantic drops unknown keys, so every REST request is anonymous and cacheable, and REST callers with near-zero feelings share answers. Pinned by "a speaker sent over REST is dropped". Consistent with the module docstring (REST serves evals, benchmarks and demos), so whether to add the field or reject the key is a decision about what REST is for.
- [x] python-06 #2 every exception is reported as a provider failure — FIXED: `_dispatch` reports programming-error types (KeyError, IndexError, TypeError, AttributeError, NameError, AssertionError) as 500 "Internal error: <Type>"; everything else stays 502 "AI provider call failed: <Type>". Neither leaks the message or a traceback.
- [-] python-06 #3 event type validated before the NPC — NOT A DEFECT: an observation about ordering, pinned; an unknown NPC with an unknown event type gets 422 rather than 404.
- [-] python-06 #4 the 422 detail lists all known event types — NOT A DEFECT: recorded by the author as helpful and harmless.

## python-07-lore-tool.md

- [x] python-07 #1 valid JSON of the wrong shape raises instead of returning no tool — FIXED: `create_lore_tool_from_file` checks that the JSON is an object and `known_facts` a list of strings; anything else logs an error and returns `(None, None)`, so a malformed file costs the NPC only its lore tool, exactly like a missing one.
- [x] python-07 #2 a string `known_facts` is indexed character by character — FIXED: the same shape check rejects a string (and an object) `known_facts` instead of indexing it character by character.
- [-] python-07 #3 a lore file outside an NPC directory gets the collection `lore_` — NOT A DEFECT: `_collection_name` takes the parent directory, and the bare-filename case is pinned as `lore_`. Only reachable with a layout no gamedata uses; recorded so an unusual layout is a known risk rather than a surprise.
- [-] python-07 #4 facts under 500 characters are indexed whole — NOT A DEFECT: an observation that a longer fact would be split into overlapping chunks and could return as a fragment; no current fact is long enough.
