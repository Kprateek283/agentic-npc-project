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
