# Python 2 — CLI-backed provider: results and findings

- **Date:** 2026-09-24
- **Brief item:** Python list, item 2 (`agy_chat.py`, beyond what exists)
- **Test file:** `ai-service-python/tests/test_agy_chat_contract.py` (the existing `tests/test_agy_chat.py` is unchanged)
- **Commit:** `3b42fbc` (tests) on `tests/full-suite`
- **Toolchain:** Python 3.12 venv, pytest 9.1.1, ruff 0.15.22; `subprocess.run` faked, the `agy` binary never runs

## Result

`PYTHONPATH=. .venv/bin/python -m pytest tests/test_agy_chat_contract.py -v`

| Case | Result |
| --- | --- |
| test_a_prompt_full_of_shell_metacharacters_travels_as_one_argument | PASS |
| test_a_prompt_that_looks_like_a_flag_stays_the_value_of_p | PASS |
| test_model_flag_only_when_configured: no model configured adds no --model | PASS |
| test_model_flag_only_when_configured: an empty model name adds no --model | PASS |
| test_model_flag_only_when_configured: a configured model is passed last as --model | PASS |
| test_subprocess_timeout_is_cli_timeout_plus_30s: one second gets a 31-second subprocess limit | PASS |
| test_subprocess_timeout_is_cli_timeout_plus_30s: the default 120 seconds gets 150 | PASS |
| test_subprocess_timeout_is_cli_timeout_plus_30s: ten minutes gets ten and a half | PASS |
| test_failures_raise_instead_of_returning_an_empty_reply: a non-zero exit raises even when stdout holds a reply | PASS |
| test_failures_raise_instead_of_returning_an_empty_reply: empty output raises | PASS |
| test_failures_raise_instead_of_returning_an_empty_reply: JSON without a response field raises | PASS |
| test_failures_raise_instead_of_returning_an_empty_reply: a JSON list instead of an object raises | PASS |
| test_failures_raise_instead_of_returning_an_empty_reply: a timeout with no stderr raises | PASS |
| test_failures_raise_instead_of_returning_an_empty_reply: a missing binary raises | PASS |
| test_failures_raise_instead_of_returning_an_empty_reply: a null response raises rather than replying with nothing | PASS |
| test_stderr_in_the_error_is_cut_to_500_characters | PASS |
| test_an_empty_response_field_comes_back_as_an_empty_reply | PASS |
| test_other_message_types_are_labelled_by_their_type | PASS |
| test_bind_tools_fails_when_the_quest_agent_is_built | PASS |
| test_streaming_yields_the_whole_reply_in_one_piece | PASS |

20/20 pass in 0.85s, and `ruff check .` is clean. The full Python run is 97 passed, 1 failed:
the failure is the pre-existing `tests/test_agy_chat.py::test_config_with_agy_provider`
(see Python 1, finding 1), unchanged. The Go checks were re-run and green.

Each brief point maps to a case: shell metacharacters (backticks, `$(...)`, `;`, `&&`, `|`, `>`,
quotes, globs, backslash, tab and newline) arrive as exactly one argv element and the whole
command list is asserted literally, with `shell` never set; `--model` appears only for a
non-empty model and always last; the subprocess timeout is the CLI timeout plus 30 seconds at
three sizes; a non-zero exit, unparsable output and a timeout each raise; `bind_tools` fails when
`graph_builder.build_langgraph_agent` is called with a tool, i.e. at wiring time; streaming
yields exactly one chunk holding the whole reply, after checking it is not a canned fallback line.

## Production change

None.

## Mutation check

Each break was applied to `agy_chat.py` on its own, the named cases went red, and the code was
restored.

| Deliberate break | Case(s) that failed |
| --- | --- |
| the command joined into a string and run with `shell=True` | metacharacters; flag-like prompt; all 3 model cases; all 3 timeout cases; other message types |
| the prompt split on spaces into several arguments | metacharacters; flag-like prompt; other message types |
| `--model` added whenever the model is not `None` | an empty model name adds no `--model` |
| `--model` placed before the prompt | a configured model is passed last |
| a 60-second margin instead of 30 | all 3 timeout cases |
| no subprocess timeout | all 3 timeout cases |
| `--print-timeout` without its `s` unit | metacharacters; all 3 model cases; all 3 timeout cases |
| a non-zero exit ignored | non-zero exit raises; stderr cut to 500 |
| a parse failure returning an empty reply | empty output; no response field; a JSON list |
| a timeout returning an empty reply | a timeout with no stderr raises |
| stderr cut at 50 characters | stderr cut to 500 |
| `bind_tools` returning the model instead of raising | bind_tools fails when the quest agent is built |
| unknown message roles labelled "Human" | other message types labelled by their type |
| messages joined with one newline | other message types labelled by their type |

No break survived.

## Findings (reported, not fixed)

1. **An empty response is returned, not raised.** If `agy` exits 0 with `{"response": ""}`,
   `ChatAgy` returns an empty reply. Every other failure raises, and the brief's intent is
   "raise rather than returning an empty reply". On the RAG path an empty reply would also be
   stored by the semantic cache (Python 1, finding 2). Pinned by "an empty response field comes
   back as an empty reply".
2. **A null or non-string response escapes as a different exception.** `{"response": null}` (or
   a number) passes the `try` that wraps parsing and fails later inside `AIMessage(...)` with a
   pydantic `ValidationError`, not the `RuntimeError` every other failure uses, so a caller
   catching `RuntimeError` would miss it. It does raise, so no empty reply leaks.
3. **Observation:** stderr in error messages is truncated to its first 500 characters, then
   stripped; a CLI that prints a long banner before the real error would hide the error. Pinned.
4. **Observation:** `subprocess.TimeoutExpired.stderr` is bytes even with `text=True` on some
   Python versions, so the timeout message can show `b'...'`. Harmless; not asserted.

## Next

Python 3: provider selection (`config.py`).
