# Python 3 — Provider selection: results and findings

- **Date:** 2026-09-24
- **Brief item:** Python list, item 3 (`config.py`)
- **Test file:** `ai-service-python/tests/test_config.py`
- **Commit:** `137a66c` (tests) on `tests/full-suite`
- **Toolchain:** Python 3.12 venv, pytest 9.1.1, ruff 0.15.22; no network, no API key, no Ollama

## Result

`PYTHONPATH=. .venv/bin/python -m pytest tests/test_config.py -v`

| Case | Result |
| --- | --- |
| test_gemini_builds_one_shared_model_with_the_pinned_name | PASS |
| test_gemini_model_and_retries_can_be_overridden | PASS |
| test_gemini_without_a_key_fails_at_import: gemini without a key | PASS |
| test_gemini_without_a_key_fails_at_import: gemini with an empty key | PASS |
| test_gemini_without_a_key_fails_at_import: no provider at all defaults to gemini and still needs a key | PASS |
| test_ollama_builds_two_models_that_default_to_the_same_name | PASS |
| test_ollama_light_and_heavy_models_and_host_can_differ | PASS |
| test_provider_name_is_case_insensitive | PASS |
| test_agy_uses_the_cli_for_lore_and_ollama_for_tools | PASS |
| test_agy_reports_its_model_when_one_is_set | PASS |
| test_an_unknown_provider_fails_at_import: openai | PASS |
| test_an_unknown_provider_fails_at_import: anthropic | PASS |
| test_an_unknown_provider_fails_at_import: gemini2 | PASS |
| test_an_unknown_provider_fails_at_import:  ollama | PASS |
| test_embeddings_stay_on_ollama_in_every_mode: gemini | PASS |
| test_embeddings_stay_on_ollama_in_every_mode: ollama | PASS |
| test_embeddings_stay_on_ollama_in_every_mode: agy | PASS |
| test_embedding_model_can_be_overridden | PASS |
| test_a_non_numeric_agy_timeout_fails_at_import | PASS |
| test_vector_store_defaults | PASS |
| test_an_unknown_vector_store_fails_at_import | PASS |
| test_qdrant_is_checked_at_import: reachable qdrant loads | PASS |
| test_qdrant_is_checked_at_import: unreachable qdrant fails loudly | PASS |

23/23 pass in about 7.4s, and `ruff check .` is clean. The full Python run is 120 passed,
1 failed: the pre-existing `tests/test_agy_chat.py::test_config_with_agy_provider` (Python 1,
finding 1), unchanged. The Go checks were re-run and green.

**How.** As the brief suggested, each case clears every variable `config.py` reads, sets the ones
it needs with `monkeypatch`, and calls `importlib.reload(config)`, then inspects the objects built.
`dotenv.load_dotenv` is replaced by a no-op so a developer's `.env` cannot change the outcome.
Building `ChatGoogleGenerativeAI` (with a dummy key), `ChatOllama` and `OllamaEmbeddings` opens no
connection. The one import-time connection, the Qdrant reachability check, is served by a fake
`QdrantClient` that records the URL and the 5-second timeout. When the file finishes, a
module-scoped fixture reloads `config` with `LLM_PROVIDER=ollama`, the state `conftest.py`
creates, so later test files see a normal module.

**Speed.** Each reload builds httpx clients, and creating their TLS contexts takes about 0.3s per
reload in this sandbox (the profile is dominated by `ssl.create_default_context`), so the file
takes ~7s here. Restoring once per file instead of once per case halved it from 14.8s. It will
likely be faster on a stock CI runner; if not, it is the slowest Python file.

## Production change

None. The brief allowed a small change if the module was untestable; reload was enough.

## Mutation check

Each break was applied to `config.py` on its own, the named cases went red, and the code was
restored.

| Deliberate break | Case(s) that failed |
| --- | --- |
| provider name not lower-cased | case-insensitive provider |
| default provider changed to ollama | no provider defaults to gemini and needs a key |
| gemini key not required | all 3 missing-key cases |
| gemini default moved to the `gemini-flash-latest` alias | gemini pinned name |
| gemini retries default 6 | gemini pinned name (asserts one retry) |
| ollama light model defaulting to a different model | ollama models default to the same name |
| ollama light and heavy swapped | light, heavy and host can differ |
| ollama reported names wrong | light, heavy and host can differ |
| agy heavy model also agy | agy uses the CLI for lore and Ollama for tools |
| agy model not passed to `ChatAgy` | agy reports its model |
| agy reported name ignoring the model | agy reports its model |
| unknown provider falling back to ollama | all 4 unknown-provider cases |
| embeddings ignoring `OLLAMA_HOST` | embeddings on Ollama, all 3 modes |
| embeddings dropped in gemini mode | embeddings on Ollama, gemini |
| unknown vector store accepted | unknown vector store fails |
| unreachable Qdrant swallowed (falling back to faiss) | unreachable Qdrant fails loudly |
| retriever k default 4 | vector store defaults |

No break survived.

## Findings (reported, not fixed)

1. **The existing reload test is the red one.** `test_agy_chat.py::test_config_with_agy_provider`
   (Python 1, finding 1) is a config-reload test whose cleanup deletes `LLM_PROVIDER` and reloads
   into the gemini default. The new file shows the working pattern (reset to `ollama` before the
   final reload); the old test is left as it is.
2. **Import-time side effects.** Every consumer does `from config import llm_light` (or
   `llm_heavy`, `embeddings`) at import, so a reload changes `config.llm_light` but not the names
   already bound in `rag_builder`, `graph_builder`, `npc_agent` or the lore tool. Tests that need a
   different model must patch those modules directly, as the Python 2 wiring test does with
   `graph_builder.llm_heavy`. Not a bug in production (config loads once), but a trap for tests.
3. **Surrounding whitespace is not stripped.** `LLM_PROVIDER=" ollama"` is rejected as unsupported
   (the value is lower-cased but not stripped). Pinned in the unknown-provider cases.
4. **A bad number crashes with Python's own message.** `AGY_TIMEOUT_S=2m` (likewise
   `GEMINI_MAX_RETRIES`, `RETRIEVER_K`, the cache variables) fails at import with
   "invalid literal for int() with base 10: '2m'", which does not name the variable.
5. **The default provider needs a key.** With nothing set, the service defaults to gemini and
   refuses to start without `GEMINI_API_KEY`; that is why `conftest.py` must force `ollama`.
   Intended, and pinned.

## Next

Python 4: the two prompt templates (`prompts/`).
