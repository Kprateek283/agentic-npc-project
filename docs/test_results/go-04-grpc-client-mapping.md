# Go 4 — gRPC client mapping: results and findings

- **Date:** 2026-09-24
- **Brief item:** Go list, item 4 (`internal/infra/grpc_client`)
- **Test file:** `backend-go/internal/infra/grpc_client/ai_client_test.go` (the package's first tests)
- **Commit:** `59a8bb0` (tests) on `tests/full-suite`
- **Toolchain:** go1.24.7 linux/amd64; no server, no port, no network

## Result

`GOWORK=off go test -count=1 -v ./internal/infra/grpc_client/`

| Test | Case | Result |
| --- | --- | --- |
| TestEventRequestWireFields | every field reaches its own wire field | PASS |
| TestEventRequestWireFields | the speaker's feelings and the general mood stay in separate fields | PASS |
| TestEventRequestWireFields | memory lines keep their order, duplicates included | PASS |
| TestEventRequestWireFields | an emotion name the protocol never saw travels like any other | PASS |
| TestEventRequestWireFields | empty maps and no memory lines put nothing on the wire | PASS |
| TestEventRequestWireFields | nil maps are sent as empty, not as an error | PASS |
| TestEventRequestWireFields | a zero emotion is still sent as an entry, so the other side sees the name | PASS |
| TestEventRequestWireFields | float64 feelings are narrowed to float32 without clamping | PASS |
| TestEventRequestWireFields | the request does not alias the caller's maps | PASS |
| TestCallAIThink | a successful call is made once and returns the response | PASS |
| TestCallAIThink | one Unavailable is retried and the retry's answer is returned | PASS |
| TestCallAIThink | a second Unavailable is returned, with no third attempt | PASS |
| TestCallAIThink | a deadline exceeded is never retried | PASS |
| TestCallAIThink | an internal error is not retried | PASS |
| TestCallAIThink | every attempt carries the configured deadline | PASS |
| TestCallAIThinkStream | tokens are forwarded in order and joined into the full reply | PASS |
| TestCallAIThinkStream | a whole answer in one chunk arrives as one token | PASS |
| TestCallAIThinkStream | empty chunks are not forwarded | PASS |
| TestCallAIThinkStream | the done frame ends the stream and anything after it is not read | PASS |
| TestCallAIThinkStream | text carried on the done frame itself is dropped | PASS |
| TestCallAIThinkStream | a stream that closes without a done frame returns what arrived | PASS |
| TestCallAIThinkStream | a mid-stream failure returns the partial text and the error | PASS |
| TestCallAIThinkStream | a stream that cannot open returns no text and is not retried | PASS |
| TestCallAIThinkStream | a non-status error from Recv is passed through unchanged | PASS |
| TestProtoContractCopiesMatch | (single test) | PASS |

25/25 pass in 0.61s; 0.6s of that is the client's own 200 ms retry pause in the three cases
that retry. The full Go build, vet and test run was green.

`buildEventRequest` is checked on the encoded bytes: the request is marshalled, the field
numbers present are read with `protowire`, and the bytes are unmarshalled as the Python side
would. Expected field numbers are written out from `proto/ai.proto` (1–11), and emotion values
are exact in float32 (0.75, −0.5, 0.0625, ...) so they are compared with `==`. The unary and
streaming calls use an `AIClient` built directly around a fake `AIBrainClient`, so nothing dials.
The deadline case can only bound the remaining time (just under the configured hour), because
the client derives its deadline from the wall clock; it also checks the retry does not get more
time than the first attempt.

## Production change

None.

## Mutation check

Each break was applied on its own, the named cases went red, and the code was restored.

| Deliberate break | Case(s) that failed |
| --- | --- |
| general mood filled from the speaker map | every field; separate fields; unfamiliar emotion; zero emotion; float32; all 5 unary cases (request check) |
| speaker and mood fields swapped | 6 wire cases, all 5 unary cases, 8 stream cases |
| memory lines dropped | every field; separate fields; order and duplicates; all 5 unary cases |
| zero-valued emotions skipped | zero emotion still sent |
| only the five known emotion names sent | unfamiliar emotion |
| values clamped to [0, 1] | separate fields; float32 without clamping |
| quest step not sent | every field; separate fields |
| question text and source swapped | every field; all 5 unary cases; all 8 opened-stream cases |
| no retry on Unavailable | retried once; no third attempt; deadline on every attempt |
| retry on any error code | deadline exceeded never retried; internal not retried |
| a third attempt after a second failure | no third attempt |
| unary call without a deadline | deadline on every attempt |
| retry given a fresh, longer deadline | deadline on every attempt |
| stream without a deadline | all 8 opened-stream cases |
| done frame ignored | in order; one chunk; empty chunks; done ends stream; done text dropped |
| empty chunks forwarded | empty chunks not forwarded |
| partial text discarded on error | mid-stream failure |
| mid-stream error swallowed | mid-stream failure; non-status error |
| `onToken` never called | 7 stream cases that carry text |
| `memory_lines = 11` changed to 12 in the Python copy of `ai.proto` | TestProtoContractCopiesMatch |

No break survived.

## Findings (reported, not fixed)

1. **The stream's action type is thrown away.** The Python `ThinkStream` ends with a done frame
   carrying `action_type` ("both end with a done frame carrying the action_type"), but
   `CallAIThinkStream` returns only the text and `streamAI` always sends the final frame as
   `SPEAK`. The unary path returns `ActionResponse.ActionType`. Today every path answers
   `SPEAK`, so nothing visible breaks; a non-SPEAK action would be lost only when streaming.
2. **Text on the done frame is dropped.** The loop breaks on `Done` before appending, so a
   server that sent its last words on the done frame would lose them. The Python servicer sends
   `text=""` on done frames today. Pinned by "text carried on the done frame itself is dropped".
3. **Two copies of the contract.** `backend-go/proto/ai.proto` and
   `ai-service-python/proto/ai.proto` are separate files; they are identical today, and
   `TestProtoContractCopiesMatch` now fails if they drift. Whether the generated Go code matches
   its `.proto` is not checked (generated code is out of scope).
4. **Observation:** an unknown agent comes back as a normal `SPEAK` reply whose content is
   "Error: Agent not found." (both paths), so the Go side forwards it to the player as speech.
   Not in this package; noted for whoever tests the handler or servicer.
5. **Observation:** values are narrowed to float32 without clamping, so an out-of-range feeling
   would travel as-is; the memory layer clamps before this point, so this is only a note.

## Next

Go 5: password handling (`internal/domain/user_logic`).
