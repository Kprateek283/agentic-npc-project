# Go 1 — Episode recording: results and findings

- **Date:** 2026-09-24
- **Brief item:** Go list, item 1 (`internal/api/handlers`, `recordEpisode`)
- **Test file:** `backend-go/internal/api/handlers/episode_recording_test.go`
- **Commit:** `fcdf215` (tests) on `tests/full-suite`
- **Toolchain:** go1.26.5 linux/amd64, cgo, in-memory SQLite via `enttest`

## Result

`GOWORK=off go test -count=1 -v -run TestRecordEpisode ./internal/api/handlers/`

| Case | Result |
| --- | --- |
| a plain event creates one row carrying its rule | PASS |
| a repeat at the same instant merges to a count of two | PASS |
| a repeat six hours later merges with the old count decayed (1.508) | PASS |
| a repeat after the memory has faded past the merge threshold starts a new row | PASS |
| the same event from two players never merges across actors | PASS |
| submitting two different quest items keeps one row per item | PASS |
| a conversation never merges and keeps each question's text | PASS |
| an apology forgives only that actor's harmful episodes and records what it covered (0.8) | PASS |
| an apology cannot forgive a severe attack past its cap (0.37) | PASS |
| a second apology is worth half the first (0.4) | PASS |
| harm after forgiveness is a new betrayal episode with the extra trust penalty (−0.3) | PASS |
| a harmless event after forgiveness is not a betrayal and keeps the forgiveness | PASS |
| an unknown event type records nothing | PASS |
| gifts and quest rewards are left to the quest layer | PASS |
| a gift through the full path is written once, by the quest layer | PASS |

15/15 pass in 0.03s. The full Go build, vet and test run was green.

All times are fixed (`t0 = 2026-01-01 12:00 UTC` plus offsets). Expected numbers were worked out by hand from the design constants in `gamedata/events.json` (for example, the stone half-life is 1h × 8760^0.2 = 6.14h) and written into the tests as literals, not recomputed with the implementation's code.

## Production change

`recordEpisode` read `time.Now()` itself. I added `recordEpisodeAt(ctx, event, npc, player, now)`, and `recordEpisode` now calls it with `time.Now()`. Behaviour is unchanged.

## Mutation check

Each break was applied on its own, the named case went red, and the code was restored.

| Deliberate break | Case(s) that failed |
| --- | --- |
| never merge (`if mergeIndex >= 0` → `if false`) | same-instant merge, six-hour merge, quest items |
| merge without decay (`count + 1`) | six-hour merge |
| conversation branch skipped | conversation |
| question text not stored | conversation |
| apology halving removed | second apology |
| `covers` not stored | apology scope |
| apology forgives every player (`memory.Forgive`) | apology scope, betrayal |
| betrayal trust penalty removed | betrayal |
| penalty written into the shared rule map | betrayal |
| harmless event treated as betrayal | harmless after forgiveness |
| gifts/rewards recorded by the handler | gifts and rewards, gift full path |
| unknown event types recorded | unknown event |
| handler clears other players' forgiveness | betrayal |

## Findings (reported, not fixed)

1. **Dead gift config.** `events.json` defines `PLAYER_GAVE_GIFT` (joy 0.15, intensity 0.2), but gifts are written only by the quest layer (`handleGifting`), which uses `items.json` `base_trust_value` as a trust delta. `recordEpisode` returns early for gifts, so its gift branches (subject and description for gifts) cannot be reached.
2. **Masked over-revocation.** If `memory.RevokeForgiveness` cleared every player's forgiveness, these tests would still pass: the handler only writes back rows whose actor matches. Only `memory_test.go` can catch that regression.
3. **gofmt not clean on the base branch.** `internal/db/ent/schema/npc.go` has one stray blank line and fails `gofmt -l`. The brief requires gofmt to be clean.
4. **Brief vs code:** the brief says gifts are "not recorded here". The code agrees, and the test confirms a gift through the full path produces exactly one row (count 1, trust 0.01 for an apple).

## Next

Go 2: memory line ranking (`buildAIArgs`). It also calls `time.Now()` and will need the same seam.
