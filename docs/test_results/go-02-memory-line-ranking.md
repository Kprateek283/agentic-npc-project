# Go 2 — Memory line ranking: results and findings

- **Date:** 2026-09-24
- **Brief item:** Go list, item 2 (`buildAIArgs`, memory line ranking)
- **Test file:** `backend-go/internal/api/handlers/memory_lines_test.go`
- **Commit:** `8e29249` (tests) on `tests/full-suite`
- **Toolchain:** go1.26.5 linux/amd64, cgo, in-memory SQLite via `enttest`

## Result

`GOWORK=off go test -count=1 -v -run TestMemoryLines ./internal/api/handlers/`

| Case | Result |
| --- | --- |
| an NPC with no memories sends no lines | PASS |
| the speaker's own top five by weight reach the prompt and the sixth is dropped | PASS |
| the speaker's faint episodes and conversations still reach the prompt | PASS |
| other players' episodes are notable only, capped at three, heaviest first | PASS |
| a bystander's episode at exactly the notability threshold is included, just under is not | PASS |
| the speaker's lines come before bystanders' even when the bystander's weigh more | PASS |
| memories fade: a bystander's drops out of the prompt and the speaker's old one sinks | PASS |
| forgiveness can pull a bystander's attack below notability | PASS |
| a repeat count is appended, rounded to the nearest whole time | PASS |
| a betrayal is marked after the count, for the speaker and for a bystander | PASS |
| equal weights keep the newer episode first | PASS |
| one stone from a bystander is not notable, five are | PASS |
| a bystander's quest reward line carries no actor and passes through unrewritten | PASS |
| many episodes: the first player hears their own top five and the three most notable others | PASS |
| many episodes: the second player hears all three of their own and the first player's episodes as someone's | PASS |

15/15 pass in 0.04s. The full Go build, vet and test run was green.

Rows are seeded straight into SQLite with explicit `created_at`, `last_at`, count, forgiveness
and delta, and the prompt is built at the fixed instant `t0 = 2026-01-01 12:00 UTC`. Most rows
use intensity 0 (half-life exactly 1h) and `last_at = t0` (decay exactly 1), so every expected
weight is a hand-computable literal (for example 0.8 one half-life old is 0.4, two is 0.2;
0.5 × 1.508^0.5 = 0.614). The stone case goes through `recordEpisodeAt` with the real rules:
one stone weighs 0.1, five weigh 5^1.5 × 0.1 = 1.12, capped at 1. The many-episodes fixture has
15 rows across four players, including an emotion name (`awe`) outside the usual five.

## Production change

`buildAIArgs` read `time.Now()` itself. I added `buildAIArgsAt(ctx, event, npc, player, now)`,
and `buildAIArgs` now calls it with `time.Now()`. Behaviour is unchanged.

## Mutation check

Each break was applied on its own, the named cases went red, and the code was restored.

| Deliberate break | Case(s) that failed |
| --- | --- |
| speaker limit 5 → 6 | top five; many episodes (first player) |
| bystander limit 3 → 4 | notable/cap three; many episodes (both) |
| notability `>=` → `>` | exactly at the threshold |
| notability filter removed | threshold; fade; forgiveness; one stone vs five |
| speaker sort ascending | top five; faint episodes; fade; repeat count; many episodes (both) |
| bystander sort ascending | notable/cap three; many episodes (both) |
| tie-break oldest first | equal weights |
| bystanders worded "You" | 10 cases, every one with a bystander line |
| count truncated instead of rounded | repeat count |
| count shown only above 2 | repeat count; betrayal |
| betrayal marker dropped | betrayal; many episodes (second player) |
| speaker lines omitted | 9 cases, every one with a speaker line |
| `now` replaced by `time.Now()` in `buildAIArgsAt` | 13 of 15 (not the empty NPC, and not the repeat count, whose seed order happens to match its weight order once every weight decays to near zero) |
| forgiveness ignored in `memory.Contribution` | forgiveness |
| betrayal flag not loaded in `npcstate.Load` | betrayal; many episodes (second player) |

My first attempt at the clock break patched the `time.Now()` in `HandleGameEvent` instead of
`buildAIArgsAt` (the same line appears twice) and stayed green; re-applied at the right line it
went red as shown.

## Findings (reported, not fixed)

1. **Quest rewards and admin trust reach other players unattributed.** `quest_logic.go` writes
   "Quest reward: trust changed by 0.50" and `event_handlers.go` writes "Admin set trust with
   Elara to 0.80". Neither begins with the actor, so `formatMemoryLine` cannot turn them into
   "You"/"Someone". A reward of 0.5 trust is well above notability (0.3), so it reaches every
   other player's prompt reading exactly like something the speaker did. The case "a
   bystander's quest reward line carries no actor" pins today's output so a fix will be noticed.
2. **Design vs code: "counts and text".** `docs/memory_v1_assumptions.md` item 8 and
   `docs/memory_v1_plan.md` say the speaker's lines carry "counts and text". `recordEpisode`
   stores a conversation's question in `text`, but `formatMemoryLine` never reads it, so the
   line is "You triggered PLAYER_ASKED_QUESTION on Elara". Conversations and apologies also have
   an empty delta, so their weight is always 0: they sort last for the speaker and can never be
   notable for a bystander.
3. **Plan vs code: wording.** The plan describes lines like "You threw stones at me 5 times";
   the code sends raw identifiers, "You triggered PLAYER_THREW_STONE on Elara (5 times)". Not a
   bug in ranking, but the prompt relies on the model to interpret event names.
4. **Minor:** the no-space branch of `formatMemoryLine` would turn "P1's gift" into "You's gift".
   No description written today takes that branch, so it is untested and unreachable.
5. **Observation:** the repeat count shown is the decayed count rounded, so a stone thrown once,
   then again six hours later (count 1.508) reads "(2 times)" — consistent with the design, noted
   so nobody reads the number as a literal tally.

## Next

Go 3: trust as a computed value (`internal/domain/npcstate`, `quest_logic`).
