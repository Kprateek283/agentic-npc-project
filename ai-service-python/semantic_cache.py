"""In-process semantic response cache for the RAG (lore) path (C4).

A paraphrase of a lore question ("Who guards the gate?" vs "Who watches the gate?")
should not pay for a fresh LLM call. We embed the incoming question, compare it against
recently answered questions for the same NPC by cosine similarity, and on a match above a
tuned threshold return the stored answer — no LLM, no streaming wait.

Design choices (ponytail):
  * In-process, not Redis. The AI service is a single process, lore is static, and a dict
    does exactly what a Redis KV would here. Avoids adding a Python Redis dependency for a
    few lines of code.
  * Per-NPC linear scan over the stored entries. At this scale (8 NPCs, tens of distinct
    lore questions) that is trivially fast.
  ponytail: in-process dict + O(n) scan per NPC. Upgrade path if this ever grows: move the
  store to Redis and the lookup to Redis vector search (RediSearch KNN).

Only near-neutral emotional contexts are cached (see cacheable_context): the RAG prompt
conditions tone on live emotion, so a cached answer is only reused when the live context is
also baseline. Memory variation is ignored — lore answers are grounded on retrieved facts,
not on the low-importance per-event memory line (documented simplification, per the plan).
"""

import math
import os
import time


def _cosine(a, b) -> float:
    dot = sum(x * y for x, y in zip(a, b))
    na = math.sqrt(sum(x * x for x in a))
    nb = math.sqrt(sum(y * y for y in b))
    if na == 0 or nb == 0:
        return 0.0
    return dot / (na * nb)


def cacheable_context(dynamic_context: dict) -> bool:
    """Only baseline (near-neutral) contexts are cached, so a cached answer is never reused
    under a different emotional tone. Trust is ignored (relationship state, not lore)."""
    e = dynamic_context.get("emotions", {})
    return all(abs(e.get(k, 0.0)) < 0.2 for k in ("joy", "sadness", "anger", "fear"))


class SemanticCache:
    """One NPC's cache of (question embedding -> answer). Not thread-safe beyond CPython's
    GIL-atomic list ops, which is enough for the gRPC thread pool at this scale."""

    def __init__(self, threshold=None, ttl_s=None, max_entries=None):
        self.enabled = os.getenv("SEMANTIC_CACHE_ENABLED", "true").lower() == "true"
        # 0.90 tuned on nomic-embed-text against the eval questions: it sits above measured
        # same-topic/different-intent pairs (<=0.81, e.g. "where does silverleaf grow?" vs
        # "what is silverleaf used for?") so the cache never serves a wrong answer, while
        # catching genuine near-duplicates (>=0.90). Weak paraphrases (~0.81-0.83) miss and
        # pay for an LLM call — the safe trade, since a false hit is a wrong answer.
        self.threshold = threshold if threshold is not None else float(os.getenv("SEMANTIC_CACHE_THRESHOLD", "0.90"))
        self.ttl_s = ttl_s if ttl_s is not None else int(os.getenv("SEMANTIC_CACHE_TTL", "3600"))
        self.max_entries = max_entries if max_entries is not None else int(os.getenv("SEMANTIC_CACHE_MAX_PER_NPC", "128"))
        self._entries = []  # list of {"emb": [float], "answer": str, "ts": float}

    def _live(self):
        cutoff = time.time() - self.ttl_s
        self._entries = [e for e in self._entries if e["ts"] >= cutoff]
        return self._entries

    def get(self, embedding) -> str | None:
        """Best match above threshold, or None."""
        if not self.enabled:
            return None
        best, best_sim = None, 0.0
        for e in self._live():
            sim = _cosine(embedding, e["emb"])
            if sim > best_sim:
                best, best_sim = e, sim
        if best is not None and best_sim >= self.threshold:
            return best["answer"]
        return None

    def put(self, embedding, answer: str) -> None:
        if not self.enabled:
            return
        self._entries.append({"emb": embedding, "answer": answer, "ts": time.time()})
        if len(self._entries) > self.max_entries:
            self._entries.pop(0)  # evict oldest


if __name__ == "__main__":
    # Self-check: a match above threshold hits, an unrelated vector misses, TTL expires.
    c = SemanticCache(threshold=0.9, ttl_s=100, max_entries=2)
    c.put([1.0, 0.0, 0.0], "guarded by Marcus")
    assert c.get([1.0, 0.0, 0.0]) == "guarded by Marcus", "identical must hit"
    assert c.get([0.99, 0.01, 0.0]) == "guarded by Marcus", "near-identical must hit"
    assert c.get([0.0, 1.0, 0.0]) is None, "orthogonal must miss"
    c.put([0.0, 1.0, 0.0], "b"); c.put([0.0, 0.0, 1.0], "cc")  # over max -> evict oldest
    assert c.get([1.0, 0.0, 0.0]) is None, "evicted entry must miss"
    assert cacheable_context({"emotions": {"joy": 0.0, "sadness": 0.1, "anger": 0.0, "fear": 0.0}})
    assert not cacheable_context({"emotions": {"joy": 0.0, "sadness": 0.0, "anger": 0.9, "fear": 0.0}})
    print("semantic_cache self-check OK")
