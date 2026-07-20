"""Streaming time-to-first-token (TTFT) vs full-response latency (C3).

Drives the gRPC ThinkStream RPC on RAG questions — the same brain path the game streams —
and records, per question: TTFT (send -> first non-empty token) and total (send -> done).
The gap between them is the perceived-latency win streaming buys: the player sees dialogue
start at TTFT instead of waiting the full total in silence.

Run once per provider, against a service started with that provider:

    LLM_PROVIDER=ollama python -u main.py            # then:
    python benchmarks/streaming_ttft.py --provider ollama --reps 3

    LLM_PROVIDER=gemini python -u main.py            # then (mind the 20/day cap):
    python benchmarks/streaming_ttft.py --provider gemini --reps 1 --questions 3

--questions/--reps cap the sample so a cloud run fits the free-tier budget. The sample size
lands in the results file and MUST be quoted wherever the number is used.
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

# (npc, question) — a subset of the eval lore questions, one per NPC.
QUESTIONS = [
    ("baelor", "Who guards the gate?"),
    ("marcus", "When do the village gates close?"),
    ("elara", "Where does silverleaf grow?"),
    ("silas", "How much does a healing potion cost?"),
    ("kaelen", "Who leads the bandits?"),
]


def _request(npc: str, question: str) -> ai_pb2.EventRequest:
    return ai_pb2.EventRequest(
        personality_path=f"gamedata/npcs/{npc}/personality.json",
        backstory_path=f"gamedata/npcs/{npc}/backstory.json",
        lore_path=f"gamedata/npcs/{npc}/lore.json",
        current_emotions=ai_pb2.EmotionStateMessage(joy=0.5, sadness=0.1, anger=0.1, fear=0.1, trust=0.5),
        recent_memories=[],
        event_type="PLAYER_ASKED_QUESTION",  # RAG path -> real token streaming
        question_text=question,
        source_entity_id="bench_player",
        current_quest_step=0,
        completion_rate=0.0,
    )


def _measure(stub, req):
    """One streamed call. Returns (ttft_ms, total_ms, n_tokens)."""
    t0 = time.perf_counter()
    ttft = None
    tokens = 0
    for chunk in stub.ThinkStream(req):
        if chunk.done:
            break
        if chunk.text:
            tokens += 1
            if ttft is None:
                ttft = (time.perf_counter() - t0) * 1000
    total = (time.perf_counter() - t0) * 1000
    return ttft, total, tokens


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--provider", required=True, help="label only (gemini|ollama) — start the service with it")
    ap.add_argument("--target", default="localhost:50051")
    ap.add_argument("--reps", type=int, default=3, help="repeats per question")
    ap.add_argument("--questions", type=int, default=len(QUESTIONS), help="cap number of questions")
    args = ap.parse_args()

    stub = ai_pb2_grpc.AIBrainStub(grpc.insecure_channel(args.target))
    questions = QUESTIONS[: args.questions]

    # One warm-up so model load / first-call cost doesn't skew the sample.
    print("warm-up...")
    _measure(stub, _request(*questions[0]))

    ttfts, totals = [], []
    for npc, q in questions:
        for _ in range(args.reps):
            ttft, total, n = _measure(stub, _request(npc, q))
            if ttft is None:
                print(f"  {npc}: no tokens streamed (empty answer?) — skipped")
                continue
            ttfts.append(ttft)
            totals.append(total)
            print(f"  {npc:8s} ttft={ttft:8.0f}ms total={total:8.0f}ms tokens={n}")

    payload = {
        "provider": args.provider,
        "questions": len(questions),
        "reps": args.reps,
        "ttft_ms": stats(ttfts),
        "total_ms": stats(totals),
    }
    write_result(f"streaming_ttft_{args.provider}", payload)


if __name__ == "__main__":
    main()
