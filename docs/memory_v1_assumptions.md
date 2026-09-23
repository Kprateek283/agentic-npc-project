# Memory v1 — assumptions

Decisions taken without asking, so the work could run unattended. Change anything here and
the code will be adjusted to match. Written 2026-09-23; appended to as further choices come up.

## Data and migration

1. **The memory table becomes an episode log.** New columns: `actor`, `subject` (item id for
   gifts), `delta` (JSON emotion map), `intensity`, `count`, `harmful`, `forgiven`, `betrayal`,
   `text`, `first_at`, `last_at`, `covers` (episode ids an apology forgave). The existing
   `description`, `event_type`, `participants` and `importance` columns stay.
2. **Old rows keep working.** A memory written before this change has no `delta`, so it adds
   nothing to any emotion; it can still appear as a remembered line via its `description`.
3. **Removed schema fields leave their columns behind.** `NPC.emotions`,
   `PlayerNPCRelationship.trust_level` and `gift_count` are dropped from the ent schema, but ent
   only adds columns, it never drops them, so existing databases keep the unused columns. No
   manual migration is needed and no data is destroyed.

## Trust and rewards

4. **Trust is computed, in the range −1..1**: the `trust` value of the emotions toward that
   player. Quest `RELATIONSHIP_TRUST` preconditions compare against it, and the existing
   thresholds in the quest files (0.3–0.5) stay as they are.
5. **A gift becomes an episode** whose trust delta is the item's `base_trust_value` clamped to
   −1..1, with intensity = |delta| (so a rare gift is remembered far longer than an apple).
   Gifts are not harmful, so repeats have diminishing returns — this replaces the old
   `gift_count` rule.
6. **A quest trust reward becomes a `QUEST_REWARD` episode** with the quest's trust value
   clamped to −1..1 and intensity 0.3.

## Behaviour

7. **State first, narration second.** The episode, quest progress and all other state changes
   are recorded before the rate-limit check; only the LLM call is skipped when a player is over
   the limit. (Review finding: today quests advance but emotions and memories do not.)
8. **Prompt contents:** emotions toward the speaker and the general mood as two separate labeled
   lines; the speaker's own episodes ranked by current weight (top 5, with counts and text); and
   other players' episodes above the notability threshold (top 3), so a bystander hears that
   *someone* has been throwing stones.
9. **New events reach the LLM:** `PLAYER_THREW_STONE` and `PLAYER_APOLOGIZED` are added to the
   agent path in `router.py`, otherwise they would fall through to a canned line.

## Contract and client

10. **The gRPC request carries two emotion maps** (`speaker_emotions`, `general_mood`) and
    `memory_lines` (already ranked and worded by Go), replacing the fixed five-field emotion
    message and the raw memory list. Go stubs are regenerated with the `protoc` bundled in
    `grpcio-tools` plus `protoc-gen-go` / `protoc-gen-go-grpc` installed into `~/go/bin`.
11. **The server sends an `EMOTIONS` frame** after each event, whose `content` is a JSON string
    holding both maps, so the existing `{action_type, content}` envelope is unchanged.
12. **The browser demo** gets "Throw stone" and "Apologize" buttons and a panel showing both maps.

## Demo and verification

13. **The demo runs on local `llama3.1:8b`** (`LLM_PROVIDER=ollama`), because the Gemini key in
    `.env` is invalid.
14. **Each step is verified before it is committed:** `agy` implements, its full transcript and
    diff are reviewed, both test suites run, and the behavioural steps are checked against a
    throwaway stack (never the running compose stack's database).
15. **Scope discipline:** anything not needed for the two demo moments (beliefs, knowledge
    memories, LLM severity scoring, LLM sincerity judging, per-NPC personality settings) stays
    out of v1, as the scope document says.
16. **ADMIN_SET_TRUST rewrites history:** it forgets (deletes) all existing memory rows this NPC
    holds about that player and writes one `QUEST_REWARD` episode worth exactly the requested
    value (clamped to −1..1, intensity 0.3), because trust is computed rather than stored.
17. **Episode merging rules:** gifts merge into an existing episode for the same actor+item when
    `memory.CanMerge` allows; quest rewards and admin writes never merge.
18. **Temporary bridging in gatherAIContext:** `gatherAIContext` is only bridged in this step: the
    old `EmotionManager` still runs but from an empty base each event, so non-trust emotions are
    per-event deltas and do not accumulate. Step 4 replaces this with emotions computed from
    episodes. In-game non-trust emotions are therefore temporarily wrong until step 4.
19. **Gifting and quest reward authoring:** `PLAYER_GAVE_GIFT` and `QUEST_REWARD` episodes are
    authored and merged authoritatively by `quest_logic` (`handleGifting` and `applyRewards`) during
    `questManager.ProcessEvent`; `recordEpisode` ignores these event types to avoid duplicate rows.
20. **Episode salience sorting tie-breaker:** In `buildAIArgs`, when sorting speaker and bystander
    episodes by `memory.Weight`, ties in salience weight maintain stable order (newest first from
    `npcstate.Load`).
21. **Deterministic memory line wording:** Memory line attribution replaces leading actor ID with
    `"You"` for the speaker and `"Someone"` for other actors when the description begins with the
    actor ID.

