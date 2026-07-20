"""Semantic response cache: hit rate and cached-response latency (C4).

Drives the gRPC ThinkStream RAG path with a neutral emotional context (the only context
that is cached — see semantic_cache.cacheable_context). Round 1 asks each question cold
(all miss, real LLM); round 2 re-asks the SAME questions (all should hit, no LLM). Reports
miss vs hit latency and the round-2 hit rate.

The AI service must be running (any provider — the cache path is provider-agnostic):
    LLM_PROVIDER=ollama python -u main.py     # then:
    python benchmarks/semantic_cache.py --questions 4
"""

import argparse
import sys
import time
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "ai-service-python"))

import grpc  # noqa: E402

import ai_pb2  # noqa: E402
import ai_pb2_grpc  # noqa: E402
from common import stats, write_result  # noqa: E402

QUESTIONS = [
    ("baelor", "Who guards the gate?"),
    ("marcus", "When do the village gates close?"),
    ("elara", "Where does silverleaf grow?"),
    ("silas", "How much does a healing potion cost?"),
    ("kaelen", "Who leads the bandits?"),
]


def _ask(stub, npc, question):
    """Returns (latency_ms, n_frames). A cache hit streams the whole answer as 1 frame."""
    req = ai_pb2.EventRequest(
        personality_path=f"gamedata/npcs/{npc}/personality.json",
        backstory_path=f"gamedata/npcs/{npc}/backstory.json",
        lore_path=f"gamedata/npcs/{npc}/lore.json",
        current_emotions=ai_pb2.EmotionStateMessage(joy=0.0, sadness=0.0, anger=0.0, fear=0.0, trust=0.5),
        recent_memories=[], event_type="PLAYER_ASKED_QUESTION",
        question_text=question, source_entity_id="bench", current_quest_step=0, completion_rate=0.0,
    )
    t0 = time.perf_counter()
    frames = 0
    for ch in stub.ThinkStream(req):
        if ch.done:
            break
        if ch.text:
            frames += 1
    return (time.perf_counter() - t0) * 1000, frames


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--target", default="localhost:50051")
    ap.add_argument("--questions", type=int, default=len(QUESTIONS))
    args = ap.parse_args()

    stub = ai_pb2_grpc.AIBrainStub(grpc.insecure_channel(args.target))
    qs = QUESTIONS[: args.questions]

    print("round 1 (cold, all miss):")
    miss_ms = []
    for npc, q in qs:
        ms, frames = _ask(stub, npc, q)
        miss_ms.append(ms)
        print(f"  {npc:8s} {ms:8.0f}ms frames={frames}")

    print("round 2 (repeat, all should hit):")
    hit_ms, hits = [], 0
    for npc, q in qs:
        ms, frames = _ask(stub, npc, q)
        hit_ms.append(ms)
        is_hit = frames == 1  # a hit returns the whole cached answer as a single frame
        hits += is_hit
        print(f"  {npc:8s} {ms:8.0f}ms frames={frames} {'HIT' if is_hit else 'MISS'}")

    payload = {
        "questions": len(qs),
        "hit_rate_round2": round(hits / len(qs), 3),
        "miss_ms": stats(miss_ms),
        "hit_ms": stats(hit_ms),
    }
    write_result("semantic_cache", payload)


if __name__ == "__main__":
    main()
