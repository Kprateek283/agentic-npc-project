# Python 4 — Prompt templates: results and findings

- **Date:** 2026-09-24
- **Brief item:** Python list, item 4 (`prompts/rag_prompt.py`, `prompts/react_prompt.py`)
- **Test file:** `ai-service-python/tests/test_prompt_templates.py`
- **Commit:** `c2a4725` (tests) on `tests/full-suite`
- **Toolchain:** Python 3.12 venv, pytest 9.1.1, ruff 0.15.22; no model called

## Result

`PYTHONPATH=. .venv/bin/python -m pytest tests/test_prompt_templates.py -v`

| Case | Result |
| --- | --- |
| test_rag_template_placeholders_are_exactly_what_the_chain_supplies | PASS |
| test_react_template_placeholders_are_exactly_what_the_agent_state_supplies | PASS |
| test_no_stray_braces: rag | PASS |
| test_no_stray_braces: react | PASS |
| test_rag_prompt_renders_for_every_npc_with_every_value_in_place: baelor | PASS |
| test_rag_prompt_renders_for_every_npc_with_every_value_in_place: elara | PASS |
| test_rag_prompt_renders_for_every_npc_with_every_value_in_place: elian | PASS |
| test_rag_prompt_renders_for_every_npc_with_every_value_in_place: kaelen | PASS |
| test_rag_prompt_renders_for_every_npc_with_every_value_in_place: lian | PASS |
| test_rag_prompt_renders_for_every_npc_with_every_value_in_place: marcus | PASS |
| test_rag_prompt_renders_for_every_npc_with_every_value_in_place: rook | PASS |
| test_rag_prompt_renders_for_every_npc_with_every_value_in_place: silas | PASS |
| test_rag_prompt_with_no_lore_says_the_npc_knows_nothing | PASS |
| test_a_brace_in_persona_text_becomes_a_placeholder_the_chain_never_fills | PASS |
| test_react_prompt_reaches_the_model_through_the_agent_with_every_value_in_place | PASS |
| test_deliberate_rules_are_still_there: answer only from lore | PASS |
| test_deliberate_rules_are_still_there: grounding rules override | PASS |
| test_deliberate_rules_are_still_there: no embellishment | PASS |
| test_deliberate_rules_are_still_there: never invent a proper name | PASS |
| test_deliberate_rules_are_still_there: admit not knowing in character | PASS |
| test_deliberate_rules_are_still_there: never mention the machinery | PASS |
| test_deliberate_rules_are_still_there: 0.5 threshold, first sentence | PASS |
| test_deliberate_rules_are_still_there: 0.5 threshold, no neutral greeting | PASS |
| test_deliberate_rules_are_still_there: general mood never blamed on the speaker | PASS |
| test_deliberate_rules_are_still_there: rag: no blame for another's deed | PASS |
| test_deliberate_rules_are_still_there: react: no blame for another's deed | PASS |
| test_deliberate_rules_are_still_there: react: broken apology marker | PASS |
| test_deliberate_rules_are_still_there: never search the lore book for an event name | PASS |
| test_deliberate_rules_are_still_there: never mention tools | PASS |
| test_deliberate_rules_are_still_there: no Thought: narration | PASS |
| test_deliberate_rules_are_still_there: reply in character | PASS |
| test_the_labels_the_rules_name_match_what_the_formatter_writes: rag | PASS |
| test_the_labels_the_rules_name_match_what_the_formatter_writes: react | PASS |

33/33 pass in 0.97s, and `ruff check .` is clean. The full Python run is 153 passed, 1 failed:
the pre-existing `tests/test_agy_chat.py::test_config_with_agy_provider` (Python 1, finding 1).
The Go checks were re-run and green.

**Placeholders and rendering go through the real code.** The RAG template is rendered by
`rag_builder.build_rag_chain` (persona from `load_static_prompt` for each of the eight real NPCs,
retriever replaced by a `RunnableLambda`), and the event template reaches a
`GenericFakeChatModel` through the prompt callable `graph_builder` gives LangGraph; the test reads
the exact messages the model received. Sentinel values (`player_sentinel`, `lore_sentinel`,
`anger=0.75, trust=-0.50`, quest step 3) must all appear, no `{name}` may remain, and the
rendered text is checked against the canned fallback lines.

**Rules.** Sixteen deliberate sentences are asserted verbatim with whitespace collapsed, so
re-wrapping a line passes but deleting or rewording a rule fails: the grounding rules (answer only
from lore, rules override, no embellishment, never invent a proper name, admit not knowing in
character, never mention the machinery), the 0.5 threshold (first sentence, no neutral greeting),
the general mood never blamed on the speaker, the You/Someone attribution in each template, the
"after apologising" marker (the exact suffix Go writes in `game_handler.go`), never searching the
lore book for an event name, never mentioning tools, and no "Thought:" narration.

## Production change

None.

## Mutation check

Each break was applied on its own, the named cases went red, and the code was restored.

| Deliberate break | Case(s) that failed |
| --- | --- |
| RAG: "Never invent a proper name" rule dropped | rule: never invent a proper name |
| RAG: threshold 0.5 changed to 0.7 | rule: 0.5 threshold, first sentence |
| react: "must never be blamed on the current speaker" dropped | rule: general mood never blamed |
| react: "Never search for the event name" changed to "Always" | rule: never search for an event name |
| react: "after apologising" respelled "after apologizing" | rule: broken apology marker |
| RAG: `{speaker}` replaced by fixed text | placeholders; rendering for all 8 NPCs |
| RAG: a stray `{` added | the test module fails to import (collection error) |
| react: a new `{quest_total}` placeholder | react placeholders; agent rendering (KeyError) |
| react: `{current_quest_step}` removed | react placeholders; agent rendering |
| `rag_builder` feeds `general_mood` as `emotions` | rendering for all 8 NPCs |
| `rag_builder` no-lore line emptied | the no-lore line |
| `graph_builder` persona after the dynamic block | agent rendering |
| `graph_builder` speaker not rendered | agent rendering |
| `rag_builder` persona dropped | rendering for all 8 NPCs; no-lore line; brace pin |
| `rag_builder` persona after the RAG block | rendering for all 8 NPCs; no-lore line |

No break survived. The first attempt at the event-name break did not apply (the sentence is
wrapped across two lines in the file) and was redone against the wrapped text; the persona-order
breaks led to the "persona comes first" assertion being added to the RAG rendering case.

## Findings (reported, not fixed)

1. **A brace in persona text breaks the NPC's lore path (bug).** `rag_builder` builds
   `ChatPromptTemplate.from_messages([("system", static_system_prompt + rag_prompt.template), ...])`
   and `load_static_prompt` does not escape braces, so any `{word}` in a personality `summary` or a
   backstory `fact` becomes a template variable. The chain never supplies it, so every lore
   question to that NPC raises `KeyError`; an unmatched `{` fails when the chain is built. No
   current gamedata contains a brace, which is why nothing fails today. The event path is not
   affected (`graph_builder` formats the dynamic block first and concatenates the persona as plain
   text). Pinned by "a brace in persona text becomes a placeholder the chain never fills".
2. **The RAG template never explains the "after apologising" marker.** Go appends
   " — after apologising" to betrayal lines on both paths, but only the event template tells the
   model what it means. Wording, not a crash.
3. **Observation:** both templates repeat the same long "Toward you" paragraph verbatim; the tests
   check both copies so they cannot drift apart silently.

## Next

Python 5: the agent wrapper (`agents/npc_agent.py`).
