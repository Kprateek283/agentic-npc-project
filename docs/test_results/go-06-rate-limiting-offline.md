# Go 6 — Rate limiting without Redis: results and findings

- **Date:** 2026-09-24
- **Brief item:** Go list, item 6 (rate limiting without Redis, `internal/ratelimit`)
- **Test file:** `backend-go/internal/ratelimit/ratelimit_fake_test.go`
- **Commit:** `a541a48` (tests) on `tests/full-suite`
- **Toolchain:** go1.24.7 linux/amd64; no Redis, no port, no network

## Result

`GOWORK=off go test -count=1 -v ./internal/ratelimit/`

| Test | Case | Result |
| --- | --- | --- |
| TestAllowWindowOffline | calls up to the limit are allowed and the next one is refused | PASS |
| TestAllowWindowOffline | refused calls keep counting inside the window | PASS |
| TestAllowWindowOffline | a limit of one allows exactly one call | PASS |
| TestAllowWindowOffline | one second before the window ends the player is still limited | PASS |
| TestAllowWindowOffline | when the window ends the count starts again at one | PASS |
| TestAllowWindowOffline | the window is fixed from the first call and later calls do not extend it | PASS |
| TestAllowWindowOffline | each player has their own window | PASS |
| TestAllowWireCommands | a one-minute window is sent as 60 seconds in a transaction | PASS |
| TestAllowWireCommands | a sub-second window is rounded up to one second by the client | PASS |
| TestAllowWireCommands | the expiry set by the first call is not moved by later calls | PASS |
| TestAllowFailureSignals | a limit of zero allows without touching Redis | PASS |
| TestAllowFailureSignals | a negative limit allows without touching Redis | PASS |
| TestAllowFailureSignals | a nil client with a limit is an error, not a silent allow | PASS |
| TestAllowFailureSignals | an unreachable Redis returns an error so the caller can fail open | PASS |
| TestAllowFailureSignals | an error reply from INCR is returned, not counted | PASS |

15/15 pass in under 0.01s. The existing `TestAllowDisabled` still passes and
`TestAllowAgainstRedis` still skips without `REDIS_ADDR`, as the optional extra. The full Go
build, vet and test run was green.

**How it runs offline.** `Allow` takes a concrete `*redis.Client`, so rather than change the
signature to an interface, the test builds a real go-redis client whose `Options.Dialer`
returns one end of a `net.Pipe`; the other end is served by a ~100-line fake that speaks RESP
and understands MULTI, INCR, EXPIRE [NX] and EXEC. The real client code (transaction pipelining,
argument formatting, reply parsing) runs unchanged, nothing listens on a port, and no
production code changed. The fake's clock moves only when a test advances it, so the window
boundaries are exact (limited at 59s, reset at 60s). The wire case asserts the literal command
sequence `MULTI`, `INCR ratelimit:llm:p1`, `EXPIRE ratelimit:llm:p1 60 NX`, `EXEC`.

## Production change

None.

## Mutation check

Each break was applied to `ratelimit.go` on its own, the named cases went red, and the code was
restored.

| Deliberate break | Case(s) that failed |
| --- | --- |
| allowed only below the limit (`<` instead of `<=`) | all 7 window cases |
| one extra call allowed (`<= limit+1`) | 6 window cases (all but "window fixed from the first call", which never exceeds the limit) |
| `EXPIRE` without `NX` | window fixed from the first call; both wire-command cases; expiry not moved |
| no `EXPIRE` at all | count restarts at one; window fixed from the first call; both wire-command cases; expiry not moved |
| a plain pipeline instead of MULTI/EXEC | both wire-command cases |
| window doubled | count restarts at one; window fixed from the first call; one-minute wire case; expiry not moved |
| a zero limit enforced (`< 0`) | zero limit never touches Redis |
| nil-client check removed | nil client is an error (panics) |
| a Redis error swallowed as an allow | unreachable Redis; INCR error reply |
| a Redis error returned with `allowed=true` | unreachable Redis; INCR error reply |

No break survived.

## Findings (reported, not fixed)

1. **The fail-open branch itself is still untested.** `Allow` now has offline coverage for the
   signal it gives (error with `allowed=false`), but the decision to proceed on that error lives
   in `HandleGameEvent` (`game_handler.go`, step 5), which needs a live websocket connection and
   a concrete `*grpc_client.AIClient` whose inner client cannot be set from another package.
   Covering it offline needs a seam in the handler (for example, the rate-limit decision
   extracted into a function that takes `allowed, err`). Not done here; flagged for whoever
   takes the handler.
2. **Observation, not a bug:** go-redis's `formatSec` rounds a window under one second up to 1s
   with only a log line. Configuration cannot produce one today (`LLM_RATE_WINDOW_SECONDS` is
   parsed as whole seconds), so this only matters to a future caller passing a raw duration.
   Pinned by "a sub-second window is rounded up".
3. **The optional real-Redis test sleeps.** `TestAllowAgainstRedis` uses `time.Sleep` (100 ms and
   2.1 s) against the brief's "never add a sleep" rule. It only runs with `REDIS_ADDR`, so CI
   is unaffected; the new file covers the same window behaviour without sleeping.
4. **Observation:** refused calls keep incrementing the counter, so a player who keeps trying
   stays at a growing count, but the window still ends 60s after the first call; hammering does
   not extend the lockout. This matches the fixed-window design and is pinned.

## Next

Go 7: seeding (`internal/infra/database`).
