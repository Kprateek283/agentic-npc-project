"""The semantic response cache (C4), replacing the `__main__` self-check CI never ran.

Vectors are chosen so cosine similarities are exact: [3, 4] vs [4, 3] is 24/25 = 0.96,
[1, 0] vs [3, 4] is 0.6, [5, 12] vs [12, 5] is 120/169 = 0.710. The cache reads the clock
through its module's `time`, which the tests replace with a clock that moves only when told.
"""

import pytest

import semantic_cache
from semantic_cache import SemanticCache, cacheable_context

T0 = 1_780_000_000.0  # a fixed instant; the value itself does not matter


class FakeClock:
    def __init__(self, now):
        self.now = now

    def time(self):
        return self.now


@pytest.fixture
def clock(monkeypatch):
    c = FakeClock(T0)
    monkeypatch.setattr(semantic_cache, "time", c)
    return c


@pytest.fixture(autouse=True)
def default_env(monkeypatch):
    for name in (
        "SEMANTIC_CACHE_ENABLED",
        "SEMANTIC_CACHE_THRESHOLD",
        "SEMANTIC_CACHE_TTL",
        "SEMANTIC_CACHE_MAX_PER_NPC",
    ):
        monkeypatch.delenv(name, raising=False)


@pytest.mark.parametrize(
    "stored, query, threshold, want",
    [
        pytest.param([1.0, 0.0, 0.0], [1.0, 0.0, 0.0], 0.9, "hit", id="the same question hits"),
        pytest.param([1.0, 0.0], [2.5, 0.0], 0.9, "hit", id="length does not matter, only direction"),
        pytest.param([3.0, 4.0], [4.0, 3.0], 0.9, "hit", id="a paraphrase above the threshold hits"),
        pytest.param([3.0, 4.0], [4.0, 3.0], 0.96, "hit", id="similarity exactly at the threshold hits"),
        pytest.param([3.0, 4.0], [4.0, 3.0], 0.9601, None, id="similarity just under the threshold misses"),
        pytest.param([5.0, 12.0], [12.0, 5.0], 0.9, None, id="a same-topic question at 0.71 misses"),
        pytest.param([1.0, 0.0], [0.0, 1.0], 0.9, None, id="an unrelated question misses"),
        pytest.param([1.0, 0.0], [-1.0, 0.0], 0.0, None, id="an opposite vector never hits even at threshold 0"),
        pytest.param([1.0, 0.0], [0.0, 0.0], 0.0, None, id="a zero vector never hits"),
    ],
)
def test_hit_or_miss_by_similarity(clock, stored, query, threshold, want):
    c = SemanticCache(threshold=threshold, ttl_s=100, max_entries=10)
    c.put(stored, "hit")
    assert c.get(query) == want


def test_empty_cache_misses(clock):
    assert SemanticCache(threshold=0.9, ttl_s=100, max_entries=10).get([1.0, 0.0]) is None


def test_the_most_similar_entry_wins(clock):
    c = SemanticCache(threshold=0.5, ttl_s=100, max_entries=10)
    c.put([0.0, 1.0], "farther")  # 0.6 to the query
    c.put([1.0, 0.0], "far")  # 0.8 to the query, above the threshold and stored earlier
    c.put([3.0, 4.0], "close")  # 0.96 to the query
    assert c.get([4.0, 3.0]) == "close"


@pytest.mark.parametrize(
    "age, want",
    [
        pytest.param(0, "stored", id="a fresh entry is served"),
        pytest.param(100, "stored", id="an entry exactly ttl seconds old is still served"),
        pytest.param(100.5, None, id="an entry older than the ttl has expired"),
    ],
)
def test_entries_expire_by_age(clock, age, want):
    c = SemanticCache(threshold=0.9, ttl_s=100, max_entries=10)
    c.put([1.0, 0.0], "stored")
    clock.now = T0 + age
    assert c.get([1.0, 0.0]) == want


def test_expired_entries_are_dropped_and_fresh_ones_kept(clock):
    c = SemanticCache(threshold=0.9, ttl_s=100, max_entries=10)
    c.put([1.0, 0.0], "old")
    clock.now = T0 + 60
    c.put([0.0, 1.0], "new")
    clock.now = T0 + 130  # old is 130s old, new is 70s old
    assert c.get([1.0, 0.0]) is None
    assert c.get([0.0, 1.0]) == "new"
    assert [e["answer"] for e in c._entries] == ["new"]


def test_reads_do_not_refresh_an_entry(clock):
    c = SemanticCache(threshold=0.9, ttl_s=100, max_entries=10)
    c.put([1.0, 0.0], "stored")
    clock.now = T0 + 90
    assert c.get([1.0, 0.0]) == "stored"
    clock.now = T0 + 101
    assert c.get([1.0, 0.0]) is None


def test_capacity_evicts_the_oldest_entry(clock):
    c = SemanticCache(threshold=0.9, ttl_s=100, max_entries=2)
    c.put([1.0, 0.0, 0.0], "first")
    c.put([0.0, 1.0, 0.0], "second")
    assert c.get([1.0, 0.0, 0.0]) == "first"  # a read does not protect it: eviction is by age
    c.put([0.0, 0.0, 1.0], "third")
    assert c.get([1.0, 0.0, 0.0]) is None
    assert c.get([0.0, 1.0, 0.0]) == "second"
    assert c.get([0.0, 0.0, 1.0]) == "third"
    assert len(c._entries) == 2


def test_an_empty_answer_is_stored_and_served(clock):
    # Pinned, not endorsed: see the results file. A stream that produced no text stores "".
    c = SemanticCache(threshold=0.9, ttl_s=100, max_entries=10)
    c.put([1.0, 0.0], "")
    assert c.get([1.0, 0.0]) == ""


def test_defaults_come_from_the_documented_values(clock):
    c = SemanticCache()
    assert (c.enabled, c.threshold, c.ttl_s, c.max_entries) == (True, 0.90, 3600, 128)


def test_environment_overrides_the_defaults(clock, monkeypatch):
    monkeypatch.setenv("SEMANTIC_CACHE_THRESHOLD", "0.95")
    monkeypatch.setenv("SEMANTIC_CACHE_TTL", "60")
    monkeypatch.setenv("SEMANTIC_CACHE_MAX_PER_NPC", "4")
    c = SemanticCache()
    assert (c.threshold, c.ttl_s, c.max_entries) == (0.95, 60, 4)


def test_explicit_arguments_beat_the_environment(clock, monkeypatch):
    monkeypatch.setenv("SEMANTIC_CACHE_THRESHOLD", "0.95")
    c = SemanticCache(threshold=0.5, ttl_s=10, max_entries=1)
    assert (c.threshold, c.ttl_s, c.max_entries) == (0.5, 10, 1)


# Only the word "true" (any case) enables the cache, so "1" and "yes" disable it too.
@pytest.mark.parametrize("value", ["false", "FALSE", "no", "0", "1", "yes"])
def test_disabled_cache_stores_nothing_and_never_hits(clock, monkeypatch, value):
    monkeypatch.setenv("SEMANTIC_CACHE_ENABLED", value)
    c = SemanticCache(threshold=0.9, ttl_s=100, max_entries=10)
    c.put([1.0, 0.0], "stored")
    assert c.get([1.0, 0.0]) is None
    assert c._entries == []


@pytest.mark.parametrize(
    "ctx, want",
    [
        pytest.param({}, True, id="an empty context is cacheable"),
        pytest.param(
            {"speaker_emotions": {}, "general_mood": {}, "speaker": "", "memory_lines": []},
            True,
            id="an anonymous context with no memories is cacheable",
        ),
        pytest.param(
            {"speaker_emotions": {"joy": 0.0, "sadness": 0.004, "fear": -0.004}, "general_mood": {"anger": 0.0}},
            True,
            id="feelings that round to 0.00 count as neutral",
        ),
        pytest.param(
            {"speaker": "p1"},
            False,
            id="a named speaker is never cacheable",
        ),
        pytest.param(
            {"speaker": "p1", "speaker_emotions": {}, "general_mood": {}, "memory_lines": []},
            False,
            id="a named speaker with a neutral, memory-free context is still never cacheable",
        ),
        pytest.param(
            {"memory_lines": ["Someone gave apple to Elara"]},
            False,
            id="any memory line makes a context uncacheable",
        ),
        pytest.param(
            {"memory_lines": [""]},
            False,
            id="even a blank memory line makes a context uncacheable",
        ),
        pytest.param(
            {"speaker_emotions": {"anger": 0.9}},
            False,
            id="a strong feeling toward the speaker is not cacheable",
        ),
        pytest.param(
            {"speaker_emotions": {"awe": 0.005}},
            False,
            id="a feeling that rounds to 0.01, even an unfamiliar one, is not neutral",
        ),
        pytest.param(
            {"general_mood": {"anger": 0.3}},
            False,
            id="a non-neutral general mood is not cacheable",
        ),
        pytest.param(
            {"general_mood": {"trust": -0.25}},
            False,
            id="a negative mood is not neutral",
        ),
    ],
)
def test_cacheable_context(ctx, want):
    assert cacheable_context(ctx) is want
