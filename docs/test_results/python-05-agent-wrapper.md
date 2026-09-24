# Python 5 — Agent wrapper: results and findings

- **Date:** 2026-09-24
- **Brief item:** Python list, item 5 (`agents/npc_agent.py`)
- **Test file:** `ai-service-python/tests/test_npc_agent.py`
- **Commit:** `a5a8984` (tests) on `tests/full-suite`
- **Toolchain:** Python 3.12 venv, pytest 9.1.1, ruff 0.15.22; no model, no embeddings service

## Result

`PYTHONPATH=. .venv/bin/python -m pytest tests/test_npc_agent.py -v`

| Case | Result |
| --- | --- |
| test_the_agent_is_built_from_the_persona_with_the_quest_tool_only | PASS |
| test_a_cached_answer_short_circuits_the_model | PASS |
| test_a_different_question_is_not_served_from_the_cache | PASS |
| test_a_named_speaker_is_never_cached_or_even_embedded | PASS |
| test_a_speakers_answer_is_not_replayed_to_an_anonymous_caller | PASS |
| test_streaming_yields_the_pieces_then_stores_the_whole_answer | PASS |
| test_a_cached_answer_streams_as_one_piece_without_the_model | PASS |
| test_a_cancelled_stream_does_not_write_to_the_cache | PASS |
| test_a_stream_with_no_text_caches_an_empty_answer_and_replays_it | PASS |
| test_the_quest_agent_returns_the_models_words | PASS |
| test_the_quest_agent_turns_content_blocks_into_plain_text | PASS |
| test_hitting_the_iteration_cap_returns_the_in_character_line | PASS |

12/12 pass in 1.1s, and `ruff check .` is clean. The full Python run is 165 passed, 1 failed:
the pre-existing `tests/test_agy_chat.py::test_config_with_agy_provider` (Python 1, finding 1).
The Go checks were re-run and green.

**Setup.** A real `NpcAgent` is constructed from `gamedata/npcs/elara`, so the persona loader, the
RAG chain builder, the LangGraph agent builder and the semantic cache are all the production ones.
Replaced: `create_lore_tool_from_file` (returns a fixed retriever and no lore tool, so no FAISS
index and no embedding calls at build time), the chat model (a `GenericFakeChatModel` subclass
that records every call, installed as `rag_builder.llm_light`, `npc_agent.llm_light` and
`graph_builder.llm_heavy`), and `npc_agent.embeddings` (fixed vectors). Replies are checked
against the canned fallback lines before their text is asserted.

The brief's four points: a cached answer short-circuits the model (one model call for two
paraphrased questions); the streaming path yields the pieces (more than one) then stores the
whole answer (a later paraphrase is served from the cache); a cancelled stream (`close()` after
the first piece) does not write to the cache; hitting the iteration cap returns
"Forgive me, my thoughts wandered for a moment. What was it you needed?" instead of raising,
after exactly six model calls (five agent/tool loops plus the final turn, from
`AGENT_MAX_ITERATIONS = 5` and a recursion limit of 11).

## Production change

None.

## Mutation check

Each break was applied to `agents/npc_agent.py` on its own, the named cases went red, and the
code was restored.

| Deliberate break | Case(s) that failed |
| --- | --- |
| cache hits ignored on the run path | cached answer short-circuits; stream then stores; textless stream |
| the run path never stores | cached answer short-circuits; cached answer streams as one piece |
| `cacheable_context` check skipped | named speaker never cached; speaker's answer not replayed |
| the stream stores in a `finally`, even on cancel | cancelled stream does not write |
| the stream stores only its last piece | stream then stores the whole answer |
| the stream never stores | stream then stores; textless stream |
| the stream buffers and yields one piece | stream yields pieces; cancelled stream |
| stream cache hits ignored | cached answer streams as one piece |
| `GraphRecursionError` not caught | iteration cap returns the in-character line |
| recursion limit doubled | iteration cap (11 calls instead of 6) |
| content blocks passed through raw (`message.content`) | content blocks become plain text |
| occupation overwritten after loading | agent built from the persona |

No break survived.

## Findings (reported, not fixed)

1. **A stream with no text caches an empty answer.** If the model's stream yields chunks that
   carry no text (thinking-only or metadata-only chunks), `stream_rag_agent` yields nothing and
   stores `""`; for the next hour every anonymous paraphrase is answered with an empty reply,
   without calling the model. A stream with no chunks at all raises inside langchain ("No
   generation chunks were returned") and caches nothing. Pinned by "a stream with no text caches
   an empty answer and replays it". Related: Python 1 finding 2, Python 2 finding 1.
2. **The iteration-cap line is the brief's trap.** The fallback returned at the cap begins
   "Forgive me, my thoughts wandered", exactly the kind of line the brief warns can satisfy a
   loose "apolog|sorry|forgiv" check. Any test of the quest agent must exclude it first, as these do.
3. **Observation:** the cache check happens before the lore retriever and the model, and an
   uncacheable context is never embedded, so in-game requests pay no embedding cost for the cache.

## Next

Python 6: the REST surface (`api/app.py`).
