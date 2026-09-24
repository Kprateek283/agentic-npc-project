# Go 5 — Password handling: results and findings

- **Date:** 2026-09-24
- **Brief item:** Go list, item 5 (`internal/domain/user_logic`)
- **Test file:** `backend-go/internal/domain/user_logic/user_manager_test.go` (the package's first tests)
- **Commit:** `92f0831` (tests) on `tests/full-suite`
- **Toolchain:** go1.24.7 linux/amd64, cgo, in-memory SQLite via `enttest`

## Result

`GOWORK=off go test -count=1 -v ./internal/domain/user_logic/`

| Test | Case | Result |
| --- | --- | --- |
| TestPasswords | a registered password logs in | PASS |
| TestPasswords | a wrong password is refused | PASS |
| TestPasswords | a second registration with the same name is refused and the first password still works | PASS |
| TestPasswords | usernames are case-sensitive, so two players can differ only by case | PASS |
| TestPasswords | logging in as an unknown player fails | PASS |
| TestPasswords | an empty username cannot be registered | PASS |
| TestPasswords | a password longer than bcrypt's 72 bytes is refused at registration | PASS |
| TestPasswords | an empty password is accepted and then required | PASS |
| TestPasswords | only the first 72 bytes of a password count at login | PASS |
| TestPasswords | a hash made at the older cost 14 still verifies | PASS |
| TestPasswords | any cost is read from the stored hash, and a wrong password fails against it | PASS |
| TestPasswords | a row holding a plain-text password cannot log in with it | PASS |
| TestRegisteredRow | the stored password is a cost-10 bcrypt hash, not the password | PASS |
| TestRegisteredRow | the display name defaults to the username | PASS |
| TestRegisteredRow | registering the same password twice gives different hashes | PASS |

15/15 pass in 2.6s. The full Go build, vet and test run was green.

"The older cost" in the brief is 14: commit `d783cf5` changed `hashPassword` from cost 14 to
`bcrypt.DefaultCost` (10). The legacy cases store literal hashes of "lantern-oath-1" made
outside the code under test (one at cost 14, one at cost 4) and log in through `LoginPlayer`.
Verifying a cost-14 hash is deliberately slow: that case alone takes about 1.1s, and it is the
only one; the wrong-password check against a stored hash uses the cost-4 literal instead.

## Production change

None.

## Mutation check

Each break was applied on its own, the named cases went red, and the code was restored.

| Deliberate break | Case(s) that failed |
| --- | --- |
| login accepts any password | wrong password; duplicate registration; case-sensitive; empty password; 72 bytes; wrong password against a stored hash; plain-text row |
| passwords stored and compared as plain text | 6 `TestPasswords` cases; cost-10 hash; fresh salt |
| plain string comparison added before bcrypt | plain-text row cannot log in |
| duplicate registration overwrites the existing player | duplicate registration refused |
| cost raised back to 14 | cost-10 hash |
| cost lowered to 4 | cost-10 hash |
| username matched case-insensitively at login | case-sensitive usernames |
| display name not set | display name defaults to the username |
| password silently truncated to 72 bytes before hashing | over 72 bytes refused at registration |
| one fixed hash shared by players with the same password | registered password logs in; fresh salt |

No break survived.

## Findings (reported, not fixed)

1. **Usernames can be enumerated.** `LoginPlayer` returns "player 'x' not found" for an unknown
   name and "invalid password" for a known one, and `websocket_handler.go` sends
   `authErr.Error()` straight to the client, so anyone can test which usernames exist.
   Registration also reveals it ("already exists"), which is harder to avoid.
2. **Empty passwords are accepted.** `RegisterPlayer("alice", "")` succeeds (the column's
   `NotEmpty` check sees the hash, not the password) and that account then logs in with an empty
   password.
3. **Bytes past 72 are ignored at login.** Registration refuses a password longer than 72 bytes
   (bcrypt's limit, surfacing as "failed to hash password: bcrypt: password length exceeds 72
   bytes"), but at login `CompareHashAndPassword` accepts any password whose first 72 bytes
   match, so a 72-byte password also accepts that password plus any suffix.
4. **Observation:** the existence check and the insert are separate queries; the unique index
   on `player_id` still stops a concurrent duplicate, which would then surface as "failed to
   create new player" rather than "already exists". Not tested (it needs a race).
5. **Observation:** usernames are case-sensitive, so "alice" and "Alice" are two players. That
   may be intended; the test pins it so a change will be deliberate.

## Next

Go 6: rate limiting without Redis (`internal/ratelimit`).
