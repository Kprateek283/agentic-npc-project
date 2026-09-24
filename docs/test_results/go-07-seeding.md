# Go 7 — Seeding: results and findings

- **Date:** 2026-09-24
- **Brief item:** Go list, item 7 (`internal/infra/database`, `SeedNPCs` and `SeedQuests`)
- **Test file:** `backend-go/internal/infra/database/seeder_test.go` (the package's first tests)
- **Commit:** `55360b2` (tests) on `tests/full-suite`
- **Toolchain:** go1.24.7 linux/amd64, cgo, in-memory SQLite via `enttest`; temporary directories for synthetic gamedata

## Result

`GOWORK=off go test -count=1 -v ./internal/infra/database/`

| Test | Case | Result |
| --- | --- | --- |
| TestSeedRealGamedata | a fresh database gets all eight NPCs with their occupations | PASS |
| TestSeedRealGamedata | each NPC points at the three files in its own directory | PASS |
| TestSeedRealGamedata | a fresh database gets all seven side quests and nothing else | PASS |
| TestSeedRealGamedata | seeding twice adds nothing and keeps the same rows | PASS |
| TestSeedRealGamedata | an NPC that already exists is left exactly as it was | PASS |
| TestSeedNPCsFromFiles | an empty directory seeds nothing and is not an error | PASS |
| TestSeedNPCsFromFiles | a malformed personality file is skipped and the rest are seeded | PASS |
| TestSeedNPCsFromFiles | occupations that are missing, empty, numeric or non-string lists become Unknown | PASS |
| TestSeedNPCsFromFiles | one personality file with no name fails the whole seed and nothing is written | PASS |
| TestSeedNPCsFromFiles | two files claiming the same name seed one NPC | PASS |
| TestSeedNPCsFromFiles | a directory that does not exist seeds nothing and reports no error | PASS |
| TestSeedQuestsFromFiles | only definitions/sq_*.json files are seeded | PASS |
| TestSeedQuestsFromFiles | a quest with no quest_id or malformed JSON is skipped | PASS |
| TestSeedQuestsFromFiles | the quest_id inside the file is the key, not the file name | PASS |
| TestSeedQuestsFromFiles | one quest with no name fails the whole seed and nothing is written | PASS |

15/15 pass in 0.05s. The full Go build, vet and test run was green.

The real-gamedata cases write out the expected eight NPCs (Baelor Blacksmith, Elara Herbalist,
Elian Villager, Kaelen Magistrate, Lian Villager, Marcus Guard, Rook Bandit, Silas Merchant) and
seven quests (`sq_0` … `sq_e1` with their names) as literals, and check that every stored path
exists on disk. Idempotence is checked on NPC ids, not just counts, so a delete-and-reinsert
would also fail. The seeder is called with the same `npcs` and `quests` sub-directories that
`app.go` passes.

## Production change

None.

## Mutation check

Each break was applied to `seeder.go` on its own, the named cases went red, and the code was
restored.

| Deliberate break | Case(s) that failed |
| --- | --- |
| NPC name conflict updates the row instead of ignoring it | existing NPC left as it was; two files, one name |
| NPC upsert removed (plain bulk insert) | seeding twice; existing NPC left as it was; two files, one name |
| quest upsert removed (plain bulk insert) | seeding twice |
| occupation list takes its last element | eight NPCs with occupations; unusual occupations |
| occupation fallback changed from "Unknown" | unusual occupations |
| a malformed personality file aborts the seed | malformed file skipped |
| backstory path pointing at `lore.json` | each NPC's three files |
| quest glob widened from `sq_*.json` to `*.json` | only `definitions/sq_*.json` seeded |
| quest id taken from the file name | the quest_id inside the file is the key |
| quests without a `quest_id` kept | no quest_id or malformed JSON skipped |
| quest definition path not stored | seven side quests |

No break survived.

## Findings (reported, not fixed)

1. **A seed failure's cause is lost at startup (bug).** In `internal/app/app.go` (steps 5 and
   5b) the seed error is shadowed: `err := dbClient.Close()` declares a new `err` inside the
   `if`, so when closing succeeds the function returns
   `fmt.Errorf("failed to seed NPCs: %w", nil)`, i.e. "failed to seed NPCs: %!w(<nil>)", and the
   real reason (for example the validator error below) never reaches the log. Found by reading
   the caller; `internal/app` is on the brief's not-worth-testing list, so no test was written.
2. **One bad file blocks all seeding.** All NPCs go in one bulk insert, so a single
   `personality.json` without a `name` fails the `NotEmpty` validator and no NPC is seeded; the
   same holds for one quest without a `name`. With finding 1 the server then exits with a
   message that does not say why. Malformed JSON, by contrast, is skipped with a warning.
3. **The quest name default never applies.** The schema gives `name` the default
   "Untitled Quest", but the seeder always calls `SetName(q.Name)`, so a missing name becomes ""
   and fails validation (finding 2) instead of falling back to the default.
4. **A wrong gamedata path seeds nothing silently.** A directory that does not exist (or has no
   `*/personality.json`) is not an error; with a mistyped `GAMEDATA_DIR` the server starts with
   no NPCs and every event fails later at NPC lookup.
5. **Seeding never updates.** Both seeders use `ON CONFLICT DO NOTHING`, so a changed
   occupation or moved file in gamedata never reaches an existing database, although the log
   says "Synchronizing with database". Pinned by "an NPC that already exists is left exactly as
   it was", so a switch to update-on-conflict will be deliberate.
6. **Observation:** `gamedata/quests/main_quest_1.json` is never seeded (only
   `definitions/sq_*.json` is read). Nothing in the Go code refers to it either.
7. **Observation:** two personality files with the same `name` silently collapse into one NPC
   (the first in glob order wins).

## Next

The Go list is complete. Next: Python 1, the semantic cache (`semantic_cache.py`).
