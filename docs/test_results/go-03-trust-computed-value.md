# Go 3 — Trust as a computed value: results and findings

- **Date:** 2026-09-24
- **Brief item:** Go list, item 3 (trust as a computed value, `internal/domain/npcstate`, `quest_logic`)
- **Test file:** `backend-go/internal/domain/quest_logic/trust_test.go`
- **Commit:** `a642adf` (tests and seam) on `tests/full-suite`
- **Toolchain:** go1.24.7 linux/amd64, cgo, in-memory SQLite via `enttest`

## Result

`GOWORK=off go test -count=1 -v -run 'TestTrustFromGifts|TestTrustPrecondition|TestQuestReward|TestAdminSetTrust$' ./internal/domain/quest_logic/`

| Test | Case | Result |
| --- | --- | --- |
| TestTrustFromGifts | a gift raises trust by the item's value | PASS |
| TestTrustFromGifts | trust from a gift halves with each half-life of its intensity | PASS |
| TestTrustFromGifts | a full-value gift is remembered for a year | PASS |
| TestTrustFromGifts | an item valued above one counts as exactly full trust | PASS |
| TestTrustFromGifts | the same gift again a half-life later merges into a faded count of 1.5 | PASS |
| TestTrustFromGifts | different gifts are separate memories that add up | PASS |
| TestTrustFromGifts | positive trust from gifts is capped at one | PASS |
| TestTrustFromGifts | a trash gift lowers trust | PASS |
| TestTrustFromGifts | a gift from one player does not raise trust toward another | PASS |
| TestTrustFromGifts | each player's trust counts only their own gifts | PASS |
| TestTrustPreconditionFollowsMemory | a year-long gift passes the Elara check a day before its half-life | PASS |
| TestTrustPreconditionFollowsMemory | the same gift fails the Elara check once it has faded to exactly 0.5 | PASS |
| TestTrustPreconditionFollowsMemory | a petal passes Baelor's check a day later | PASS |
| TestTrustPreconditionFollowsMemory | the same petal fails Baelor's check two days later | PASS |
| TestTrustPreconditionFollowsMemory | another player's gift does not unlock the quest | PASS |
| TestTrustPreconditionFollowsMemory | an attack cancels enough trust to lock the quest | PASS |
| TestTrustPreconditionFollowsMemory | gifts beyond full trust are not banked against a later attack | PASS |
| TestTrustPreconditionFollowsMemory | legacy rows with no delta contribute nothing to the check | PASS |
| TestQuestRewardTrust | the herb reward raises trust by 0.4 | PASS |
| TestQuestRewardTrust | the herb reward halves after its 15-hour half-life | PASS |
| TestQuestRewardTrust | the reward alone does not open the cure question, the reward plus a petal does | PASS |
| TestAdminSetTrust | setting trust with no history lands exactly | PASS |
| TestAdminSetTrust | setting trust replaces gifts, an attack and legacy rows rather than adding to them | PASS |
| TestAdminSetTrust | setting twice keeps only the second value | PASS |
| TestAdminSetTrust | a negative value lands exactly | PASS |
| TestAdminSetTrust | a value above one is clamped to one | PASS |
| TestAdminSetTrust | setting to zero clears trust | PASS |
| TestAdminSetTrust | other players' memories and other NPCs' memories are left alone | PASS |
| TestAdminSetTrust | a set trust is a memory like any other and fades with a quest reward's half-life | PASS |
| TestAdminSetTrust | with admin commands disabled nothing is replaced | PASS |

30/30 pass in 0.08s. The full Go build, vet and test run was green.

Everything goes through the real code paths: gifts through `handleGiftingAt` with the real
`items.json`, quests through `checkQuestCompletionAt` with the real quest definitions, admin
through `HandleAdminCommand`, and trust is read with `npcstate.Load` + `TrustToward` using the
rules loaded from `gamedata/events.json`. Rows are written at the fixed instant
`t0 = 2026-01-01 12:00 UTC` and read at fixed offsets. The expected values are hand-derived
literals from the half-lives: a 0.5 gift has half-life 8760^0.5 h = 93h35m42s (so 0.25 after
one, 0.125 after two), a 1.0 gift one year (exactly 0.5 after 8760 h, which sits exactly on the
Elara `GREATER_THAN 0.5` boundary), a reward or admin row 8760^0.3 h = 15h13m54s. Baelor's
`> 0.4` against a 0.5 petal: 0.419 after 24 h (pass), 0.350 after 48 h (fail). Tolerance: 1%
relative, stated once in the file.

`HandleAdminCommand` still stamps its row with the wall clock (no seam was added there), so the
admin cases read trust at the row's own `last_at` rather than a guessed time.

## Production change

`handleGifting`, `checkQuestCompletion`, `checkPreconditions` and `applyRewards` each read
`time.Now()`. Added `handleGiftingAt(..., now)` and `checkQuestCompletionAt(..., now)`; the
existing functions call them with `time.Now()`. `checkPreconditions` (single caller) was renamed
`checkPreconditionsAt` and takes `now`; `applyRewards` takes `now` from its single caller.
Behaviour is unchanged.

## Mutation check

Each break was applied on its own, the named cases went red, and the code was restored.

| Deliberate break | Case(s) that failed |
| --- | --- |
| gift handler ignores `now` (uses `time.Now()`) | halves per half-life; year; above one; merge 1.5; Elara fails at 0.5; Baelor fails at 48 h |
| precondition ignores `now` | Elara fails at 0.5; Baelor passes at 24 h; reward plus petal |
| quest reward ignores `now` | herb reward halves |
| gift intensity fixed at 0.2 (the `events.json` value) instead of \|value\| | halves; year; above one; merge 1.5; Elara passes at 364 d; Baelor passes at 24 h |
| gift value not clamped to 1 | above one |
| repeat gifts never merge | merge 1.5 |
| `ADMIN_SET_TRUST` appends instead of replacing | replaces history; set twice; set to zero; others left alone |
| `ADMIN_SET_TRUST` deletes every actor's rows | others left alone |
| `ADMIN_SET_TRUST` ignores the legacy `participants[0]` actor | replaces history (legacy row survives) |
| admin row intensity 0 | set trust fades with reward half-life |
| `GREATER_THAN` treated as `>=` | Elara fails at 0.5; attack cancels trust |
| `EmotionsToward` sums every actor | each player's own gifts; others left alone |
| positive trust not capped at 1 before subtracting negatives | gifts not banked against an attack |
| `npcstate.Load` gives legacy rows a trust delta | legacy rows contribute nothing; others left alone |
| **admin value not clamped** | **none — survived** (see finding 2) |

Two breaks initially survived and led to new cases: "trust sums every actor" was only caught by
the admin case until "each player's trust counts only their own gifts" was added (a player with
no episodes gets an empty map, so the plain "another player's gift" case cannot see it), and
"positive trust not capped" was invisible while the final signed clamp masked it, until "gifts
beyond full trust are not banked against a later attack" was added.

## Findings (reported, not fixed)

1. **Precondition `target_npc_name` and `emotion` are ignored.** Every quest file gives
   `RELATIONSHIP_TRUST` a `target_npc_name`, but the `Precondition` struct has no field for it,
   and `emotion` is parsed but never read: the check is always trust toward the NPC being spoken
   to. In today's data the named NPC is always the step's trigger NPC, so nothing breaks yet; a
   quest that gates talking to one NPC on trust with another would silently check the wrong one.
2. **The admin clamp is redundant, and the description is unclamped.** `memory.Contribution`
   clamps every scaled delta to [-1, 1], so removing the clamp in `ADMIN_SET_TRUST` changes no
   computed trust (the one surviving mutation). The row's description uses the raw value:
   "Admin set trust with Elara to 1.50" while trust is 1.0.
3. **A "set" trust fades.** The admin row is an ordinary `QUEST_REWARD` episode with intensity
   0.3, so a trust set to 0.8 is 0.4 fifteen hours later. It lands exactly when set, which is
   what the brief asks, but a tester setting trust to unlock a quest has about a day before it
   stops working. Pinned by "a set trust ... fades", so a design change will be noticed.
4. **Negative gifts are not harmful.** Trash items (`rotten_fish`, `old_boot`, −0.1) are written
   with `harmful=false`, so repeats use diminishing rather than escalating returns, an apology
   never covers them and they can never be a betrayal. Possibly intended; worth a decision.
5. **Minor:** gift rows take `created_at` from the column default (wall clock) rather than the
   gift's `now`, so episode order in `npcstate.Load` can disagree with `first_at`. No trust value
   depends on it.
6. **Existing tests are clock-dependent.** `quest_pipeline_test.go` seeds with `time.Now()` and
   reads trust at a second `time.Now()` (for example `seedTrustMemory`, and 1e-6 tolerances on
   gifts). They pass because little time elapses; the new file does not rely on that.
7. **Observation:** `unlocks_quest_id` rewards are parsed but nothing acts on them; completing
   `sq_2_elara_research` does not start `sq_3_baelor_gate`. Outside this item's scope.

## Next

Go 4: the gRPC client mapping (`internal/infra/grpc_client`).
