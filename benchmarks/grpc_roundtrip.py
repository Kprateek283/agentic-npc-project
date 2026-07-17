"""gRPC round-trip latency, with no LLM in the path.

Drives the Python AI service with an event type the router does not recognise, so it falls
through to the non-LLM emotion rules ("Greetings.") — see router.py. What this measures:
client -> protobuf serialisation -> HTTP/2 -> servicer -> routing -> response. What it does
NOT measure: inference. That is the point — it isolates the transport from the brain.

Usage (AI service must be running):
    python benchmarks/grpc_roundtrip.py [--n 200]
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

WARMUP = 20


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--n", type=int, default=200, help="iterations after warm-up (plan: >=200)")
    ap.add_argument("--target", default="localhost:50051")
    args = ap.parse_args()

    channel = grpc.insecure_channel(args.target)
    stub = ai_pb2_grpc.AIBrainStub(channel)
    request = ai_pb2.EventRequest(
        personality_path="gamedata/npcs/elara/personality.json",
        backstory_path="gamedata/npcs/elara/backstory.json",
        lore_path="gamedata/npcs/elara/lore.json",
        current_emotions=ai_pb2.EmotionStateMessage(joy=0.5, sadness=0.1, anger=0.1, fear=0.1, trust=0.5),
        recent_memories=[ai_pb2.MemoryMessage(description="Player walked past.", importance=0.1)],
        event_type="PLAYER_LOOKED_AT_NPC",  # unknown to the router -> non-LLM branch
        question_text="",
        source_entity_id="bench_player",
        current_quest_step=0,
        completion_rate=0.0,
    )

    first = stub.Think(request, timeout=30)
    if first.content != "Greetings.":
        raise SystemExit(f"Expected the non-LLM branch ('Greetings.'), got: {first.content!r}. "
                         f"An LLM in this path would invalidate the measurement.")

    for _ in range(WARMUP):
        stub.Think(request, timeout=30)

    samples = []
    for _ in range(args.n):
        t0 = time.perf_counter()
        stub.Think(request, timeout=30)
        samples.append((time.perf_counter() - t0) * 1000)

    summary = stats(samples)
    print(f"gRPC round-trip, no LLM (n={summary['n']}): "
          f"median {summary['median_ms']}ms, p95 {summary['p95_ms']}ms")
    write_result("grpc_roundtrip", {
        "description": "Go-equivalent gRPC client -> Python AIBrain.Think -> response, "
                       "non-LLM router branch. Includes protobuf serialisation, HTTP/2 "
                       "transport and routing; excludes inference.",
        "target": args.target,
        "warmup_iterations": WARMUP,
        "summary": summary,
        "samples_ms": [round(s, 3) for s in samples],
    })


if __name__ == "__main__":
    main()
