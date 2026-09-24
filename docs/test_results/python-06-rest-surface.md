# Python 6 — REST surface: results and findings

- **Date:** 2026-09-24
- **Brief item:** Python list, item 6 (`api/app.py` beyond `/health`)
- **Test file:** `ai-service-python/tests/test_rest_api.py`
- **Commit:** `0f8db2c` (tests) on `tests/full-suite`
- **Toolchain:** Python 3.12 venv, pytest 9.1.1, ruff 0.15.22; FastAPI `TestClient` in-process, no port

## Result

`PYTHONPATH=. .venv/bin/python -m pytest tests/test_rest_api.py -v`

| Case | Result |
| --- | --- |
| test_npcs_lists_what_is_loaded | PASS |
| test_npcs_is_empty_when_nothing_is_loaded | PASS |
| test_an_unknown_npc_is_404: chat | PASS |
| test_an_unknown_npc_is_404: event | PASS |
| test_a_bad_event_type_is_422_and_never_reaches_the_agent | PASS |
| test_the_event_type_is_checked_before_the_npc | PASS |
| test_a_malformed_body_is_422: event without event_type | PASS |
| test_a_malformed_body_is_422: chat without question | PASS |
| test_a_malformed_body_is_422: context with a non-numeric quest step | PASS |
| test_a_provider_failure_is_502_with_only_the_error_type: chat | PASS |
| test_a_provider_failure_is_502_with_only_the_error_type: event | PASS |
| test_chat_answers_through_the_rag_path_with_a_neutral_anonymous_context | PASS |
| test_event_goes_to_the_quest_agent_with_the_supplied_context | PASS |
| test_a_legacy_personality_path_key_still_finds_the_agent | PASS |
| test_a_speaker_sent_over_rest_is_dropped | PASS |

15/15 pass in 1.2s, and `ruff check .` is clean. The full Python run is 180 passed, 1 failed:
the pre-existing `tests/test_agy_chat.py::test_config_with_agy_provider` (Python 1, finding 1).
The Go checks were re-run and green.

Fake agents are registered with `monkeypatch.setitem` in `agent_manager.live_agents`, the dict
that `api.app` imports and that `router.route_event` reads through `agent_manager.get_agent`, so
each request exercises the real app, validation, router and lookup. (The existing health test
replaces `api.app.live_agents` with a new dict, which the router would not see; that is fine for
`/health` but would not work for these routes.) The brief's four points: unknown NPC → 404 with
`{"detail": "No agent loaded for key 'nobody'"}`; bad event type → 422 listing the known types;
provider failure → 502 whose whole body is `{"detail": "AI provider call failed: ConnectionError"}`
(the exception's message carried a fake path and key, and neither reached the body);
`/v1/npcs` → exactly the loaded agents.

## Production change

None.

## Mutation check

Each break was applied on its own, the named cases went red, and the code was restored.

| Deliberate break | Case(s) that failed |
| --- | --- |
| unknown agent answered 500 | unknown NPC is 404 (chat, event) |
| the exception message put in the 502 detail | provider failure (chat, event) |
| the traceback put in the 502 detail | provider failure (chat, event) |
| provider failure answered 500 | provider failure (chat, event) |
| unknown event types accepted | bad event type is 422; event type checked before the NPC |
| unknown event types answered 400 | bad event type is 422; event type checked before the NPC |
| `/v1/npcs` reporting occupation as the name | npcs lists what is loaded |
| `/v1/npcs` always empty | npcs lists what is loaded |
| `/v1/chat` routed as `PLAYER_INTERACT` | chat through the RAG path; speaker dropped |
| supplied context not merged over the defaults | event with the supplied context |
| event text dropped | event with the supplied context |
| legacy `personality.json` key reduction removed (`agent_manager`) | legacy key still finds the agent |
| quest event description reworded (`router`) | event with the supplied context |

No break survived.

## Findings (reported, not fixed)

1. **REST cannot say who is speaking.** `DynamicContext` has no `speaker` field and pydantic
   ignores unknown keys, so `"speaker": "player1"` is silently dropped and every REST request is
   anonymous. With no memory lines such a request is cacheable, so a REST caller that sends
   feelings rounding to zero shares answers with every other anonymous caller. Consistent with the
   module docstring (REST serves evals, benchmarks and demos), but the silent drop is worth a
   decision. Pinned by "a speaker sent over REST is dropped".
2. **Every exception is reported as a provider failure.** `_dispatch` turns any exception into
   502 "AI provider call failed: <Type>", so a programming error, such as the `KeyError` from a
   brace in persona text (Python 4, finding 1), looks like an upstream outage to the caller. The
   full traceback still goes to the server's stderr.
3. **Observation:** the event type is validated before the NPC, so an unknown NPC with an unknown
   event type gets 422, not 404. Pinned.
4. **Observation:** the 422 detail for an unknown event type lists all known event types, which is
   helpful for a REST client and harmless (they are not secret).

## Next

Python 7: the lore tool (`tools/lore_retriever_tool.py`).
