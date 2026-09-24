# Python 1 — Semantic cache: results and findings

- **Date:** 2026-09-24
- **Brief item:** Python list, item 1 (`semantic_cache.py`)
- **Test file:** `ai-service-python/tests/test_semantic_cache.py`
- **Commit:** `9b960cf` (tests) on `tests/full-suite`
- **Toolchain:** Python 3.12 venv from `requirements.txt` + `requirements-dev.txt`, pytest 9.1.1, ruff 0.15.22

## Result

`PYTHONPATH=. .venv/bin/python -m pytest tests/test_semantic_cache.py -v`

| Case | Result |
| --- | --- |
| test_hit_or_miss_by_similarity: the same question hits | PASS |
| test_hit_or_miss_by_similarity: length does not matter, only direction | PASS |
| test_hit_or_miss_by_similarity: a paraphrase above the threshold hits | PASS |
| test_hit_or_miss_by_similarity: similarity exactly at the threshold hits | PASS |
| test_hit_or_miss_by_similarity: similarity just under the threshold misses | PASS |
| test_hit_or_miss_by_similarity: a same-topic question at 0.71 misses | PASS |
| test_hit_or_miss_by_similarity: an unrelated question misses | PASS |
| test_hit_or_miss_by_similarity: an opposite vector never hits even at threshold 0 | PASS |
| test_hit_or_miss_by_similarity: a zero vector never hits | PASS |
| test_empty_cache_misses | PASS |
| test_the_most_similar_entry_wins | PASS |
| test_entries_expire_by_age: a fresh entry is served | PASS |
| test_entries_expire_by_age: an entry exactly ttl seconds old is still served | PASS |
| test_entries_expire_by_age: an entry older than the ttl has expired | PASS |
| test_expired_entries_are_dropped_and_fresh_ones_kept | PASS |
| test_reads_do_not_refresh_an_entry | PASS |
| test_capacity_evicts_the_oldest_entry | PASS |
| test_an_empty_answer_is_stored_and_served | PASS |
| test_defaults_come_from_the_documented_values | PASS |
| test_environment_overrides_the_defaults | PASS |
| test_explicit_arguments_beat_the_environment | PASS |
| test_disabled_cache_stores_nothing_and_never_hits: false | PASS |
| test_disabled_cache_stores_nothing_and_never_hits: FALSE | PASS |
| test_disabled_cache_stores_nothing_and_never_hits: no | PASS |
| test_disabled_cache_stores_nothing_and_never_hits: 0 | PASS |
| test_disabled_cache_stores_nothing_and_never_hits: 1 | PASS |
| test_disabled_cache_stores_nothing_and_never_hits: yes | PASS |
| test_cacheable_context: an empty context is cacheable | PASS |
| test_cacheable_context: an anonymous context with no memories is cacheable | PASS |
| test_cacheable_context: feelings that round to 0.00 count as neutral | PASS |
| test_cacheable_context: a named speaker is never cacheable | PASS |
| test_cacheable_context: a named speaker with a neutral, memory-free context is still never cacheable | PASS |
| test_cacheable_context: any memory line makes a context uncacheable | PASS |
| test_cacheable_context: even a blank memory line makes a context uncacheable | PASS |
| test_cacheable_context: a strong feeling toward the speaker is not cacheable | PASS |
| test_cacheable_context: a feeling that rounds to 0.01, even an unfamiliar one, is not neutral | PASS |
| test_cacheable_context: a non-neutral general mood is not cacheable | PASS |
| test_cacheable_context: a negative mood is not neutral | PASS |

38/38 pass in 0.06s, and `ruff check .` is clean. The full Python run is 77 passed, 1 failed:
the failure is the pre-existing `tests/test_agy_chat.py::test_config_with_agy_provider`
(finding 1), which fails the same way before this change. The Go checks were re-run and green.

Every case in the old `__main__` self-check is covered here (identical and near-identical hits,
orthogonal miss, eviction at capacity, and all seven `cacheable_context` assertions), plus the
threshold boundary, expiry, best-match, defaults and the speaker/memory rule. Similarities are
exact by construction: [3, 4] vs [4, 3] is 24/25 = 0.96 (so threshold 0.96 hits and 0.9601
misses), [1, 0] vs [3, 4] is 0.6, [5, 12] vs [12, 5] is 120/169 = 0.710. The cache reads
`time.time()` itself; the tests replace the module's `time` with a fake clock through
`monkeypatch`, so expiry is asserted to the second without a production change or a sleep.

## Production change

None. The `__main__` self-check is still in `semantic_cache.py`; it is now redundant and can be
deleted whenever someone touches the file.

## Mutation check

Each break was applied to `semantic_cache.py` on its own, the named cases went red, and the
code was restored.

| Deliberate break | Case(s) that failed |
| --- | --- |
| threshold strict (`>` instead of `>=`) | similarity exactly at the threshold hits |
| first match above zero kept instead of the best | the most similar entry wins |
| dot product instead of cosine | just under the threshold misses; same-topic at 0.71 misses |
| TTL boundary exclusive | an entry exactly ttl seconds old is still served |
| no expiry | older than the ttl expired; expired dropped and fresh kept; reads do not refresh |
| a hit refreshes the entry's timestamp | reads do not refresh an entry |
| eviction pops the newest | capacity evicts the oldest |
| no eviction | capacity evicts the oldest |
| capacity off by one (`>=`) | capacity evicts the oldest |
| default threshold 0.80 | documented defaults |
| a disabled cache still stores | all 6 disabled cases |
| speaker ignored in `cacheable_context` | both named-speaker cases |
| memory lines ignored | any memory line; a blank memory line |
| general mood ignored | non-neutral mood; negative mood |
| neutral means exactly 0 rather than rounding to 0.00 | feelings that round to 0.00 |
| "-0.00" not treated as neutral | feelings that round to 0.00 |
| only the five known emotion names checked | a feeling that rounds to 0.01, even an unfamiliar one |

The best-match break first survived, because the most similar entry had been stored first;
the case now stores it last, behind an earlier entry that is also above the threshold.

## Findings (reported, not fixed)

1. **The Python suite is already red on this branch (brief vs code).** The brief says 8 files and
   31 tests, all passing. On `experiment/agy-provider` there are 40 tests and
   `tests/test_agy_chat.py::test_config_with_agy_provider` fails with
   `ValueError: GEMINI_API_KEY is required when LLM_PROVIDER=gemini`, alone or in the full run.
   Its cleanup calls `monkeypatch.delenv("LLM_PROVIDER")`, which removes the `ollama` default
   `conftest.py` set, then reloads `config`, which falls back to `gemini` and raises. CI only runs
   on `main`, so it has never run there. Not fixed here (report-only rule); the owner was
   notified. The failed reload also leaves `config` half-re-executed for every later test
   (`LLM_PROVIDER` reads "gemini" while `llm_light` is still the agy object), which later
   config tests must not depend on.
2. **An empty answer is cached and replayed.** `put` stores any string, and `get` returns "" as a
   hit (it is not `None`). `stream_rag_agent` stores `"".join(parts)`, so a stream that produced
   no text caches an empty reply that is then served to every paraphrase for an hour. Pinned by
   "an empty answer is stored and served"; the agent-level consequence belongs to Python 5.
3. **Only the word "true" enables the cache.** `SEMANTIC_CACHE_ENABLED` is compared with
   "true", so "1" and "yes" silently disable it. Pinned.
4. **Observation:** eviction is first-in-first-out, not least-recently-used: a question answered
   from cache many times is still evicted first. Consistent with the code's comment ("evict
   oldest"); pinned so a change is deliberate.
5. **Observation:** an entry is still served at exactly `ttl` seconds old (`>=` cutoff).

## Next

Python 2: the CLI-backed provider (`agy_chat.py`), beyond what exists.
