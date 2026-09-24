# Python 7 — Lore tool: results and findings

- **Date:** 2026-09-24
- **Brief item:** Python list, item 7 (`tools/lore_retriever_tool.py`)
- **Test file:** `ai-service-python/tests/test_lore_tool.py` (the existing `tests/test_retriever.py` is unchanged)
- **Commit:** `a0417b9` (tests) on `tests/full-suite`
- **Toolchain:** Python 3.12 venv, pytest 9.1.1, ruff 0.15.22; fake embeddings, in-process FAISS, Qdrant faked

## Result

`PYTHONPATH=. .venv/bin/python -m pytest tests/test_lore_tool.py -v`

| Case | Result |
| --- | --- |
| test_missing_or_malformed_lore_gives_no_tool: truncated JSON | PASS |
| test_missing_or_malformed_lore_gives_no_tool: an empty file | PASS |
| test_missing_or_malformed_lore_gives_no_tool: no known_facts key | PASS |
| test_missing_or_malformed_lore_gives_no_tool: known_facts is null | PASS |
| test_missing_or_malformed_lore_gives_no_tool: known_facts is empty | PASS |
| test_missing_or_malformed_lore_gives_no_tool: only empty facts | PASS |
| test_lore_of_the_wrong_shape_raises: a JSON list instead of an object | PASS |
| test_lore_of_the_wrong_shape_raises: a fact that is not a string | PASS |
| test_a_string_instead_of_a_list_is_split_into_single_characters | PASS |
| test_collection_name_is_lore_underscore_directory: relative path | PASS |
| test_collection_name_is_lore_underscore_directory: absolute path | PASS |
| test_collection_name_is_lore_underscore_directory: parent-relative path, as GAMEDATA_DIR defaults | PASS |
| test_collection_name_is_lore_underscore_directory: a bare file name gives an empty NPC part | PASS |
| test_the_qdrant_branch_uses_the_per_npc_collection_and_recreates_it | PASS |
| test_the_retriever_returns_k_facts: 1 | PASS |
| test_the_retriever_returns_k_facts: 2 | PASS |
| test_the_retriever_returns_k_facts: 4 | PASS |
| test_the_tool_answers_with_the_best_facts_joined_by_newlines | PASS |
| test_a_fact_longer_than_500_characters_is_split_into_chunks | PASS |
| test_every_npcs_real_lore_builds_a_tool: baelor | PASS |
| test_every_npcs_real_lore_builds_a_tool: elara | PASS |
| test_every_npcs_real_lore_builds_a_tool: elian | PASS |
| test_every_npcs_real_lore_builds_a_tool: kaelen | PASS |
| test_every_npcs_real_lore_builds_a_tool: lian | PASS |
| test_every_npcs_real_lore_builds_a_tool: marcus | PASS |
| test_every_npcs_real_lore_builds_a_tool: rook | PASS |
| test_every_npcs_real_lore_builds_a_tool: silas | PASS |

27/27 pass in 1.8s, and `ruff check .` is clean. The full Python run is 207 passed, 1 failed:
the pre-existing `tests/test_agy_chat.py::test_config_with_agy_provider` (Python 1, finding 1).
The Go checks were re-run and green.

The embeddings are a deterministic word-slot fake (a stable byte checksum per word rather than
Python's salted `hash()`, so ranking is identical on every run), the FAISS index is real and
in-process, and the Qdrant branch is exercised by replacing `QdrantVectorStore.from_documents`
with a recorder, which asserts the literal call: the facts, `url="http://qdrant:6333"`,
`collection_name="lore_marcus"`, `force_recreate=True`, then `as_retriever(search_kwargs={"k": 3})`.
The brief's two points: missing or malformed lore returns `(None, None)` for unreadable JSON, a
missing or null or empty `known_facts`, and facts that split to nothing, but not for valid JSON
of the wrong shape (finding 1); the collection name is `lore_<npc directory>` as documented.

## Production change

None.

## Mutation check

Each break was applied to `tools/lore_retriever_tool.py` on its own, the named cases went red,
and the code was restored.

| Deliberate break | Case(s) that failed |
| --- | --- |
| JSON read errors no longer caught | truncated JSON; empty file |
| empty `known_facts` not rejected | known_facts is null |
| an empty split not rejected | only empty facts |
| collection named after the file (`lore_lore.json`) | all 4 collection-name cases; Qdrant branch |
| collection without the `lore_` prefix | all 4 collection-name cases; Qdrant branch |
| `force_recreate=False` | Qdrant branch |
| one shared collection for every NPC | Qdrant branch |
| `k` fixed at 4 instead of `RETRIEVER_K` | Qdrant branch; k = 1 and 2; best facts joined |
| chunk size 1000 | long fact split into chunks |
| tool answer joined with spaces | best facts joined by newlines |
| tool returns only the best fact | best facts joined by newlines |
| tool registered as `lore_search` | best facts joined by newlines (name check) |

No break survived. A first attempt at the rename break renamed the Python function, which only
broke the module (15 cases red for the wrong reason); it was redone as `@tool("lore_search")`,
which changes only the tool's name, and the name check caught it.

## Findings (reported, not fixed)

1. **Valid JSON of the wrong shape raises instead of returning no tool.** Only the file read and
   `json.load` are inside the `try`. A `lore.json` whose top level is a list raises
   `AttributeError` (`data.get`), and a fact that is not a string raises a pydantic
   `ValidationError` from `Document`. `agent_manager.load_agents_on_startup` catches exceptions per
   NPC, so that NPC is skipped entirely at startup (no persona, no quest agent), where a missing
   or empty lore file only costs it the lore tool. Pinned by "lore of the wrong shape raises".
2. **A string `known_facts` is indexed character by character.** `"known_facts": "gate"` builds an
   index of the facts "g", "a", "t", "e" with no error. Pinned.
3. **A lore file outside an NPC directory gets the collection `lore_`.** The name comes from the
   parent directory, so `lore.json` on its own maps to `lore_`, which every such file would share
   (and, with `force_recreate`, overwrite) on Qdrant. Only reachable with an unusual layout.
4. **Observation:** every real NPC's facts are under 500 characters, so each fact is indexed whole;
   a longer fact would be split into overlapping chunks and could come back as a fragment.

## Next

This was the last item on both lists. Every Go and Python area in the brief now has a results
file. What remains is the brief's "Not worth testing" list (`cmd/server`, `internal/app`,
`internal/api/router`, `internal/infra/httpsapi`), which gets no tests by design, and the
findings reported across the results files.
