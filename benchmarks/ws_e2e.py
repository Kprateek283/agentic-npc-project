"""End-to-end infrastructure overhead: WebSocket client -> Go -> gRPC -> Python -> back.

Sends an event type that reaches the non-LLM branch, so the measured time is the whole
orchestration path with no inference in it: WebSocket read, quest-manager validation
(player + NPC lookups through the Redis/Postgres cache-aside), emotion update, memory
write, gRPC call, and the WebSocket response.

This is the "<50ms total infrastructure overhead" claim in the README — measured, not asserted.

Usage (full stack must be running):
    python benchmarks/ws_e2e.py [--n 100] [--port 8080]
"""

import argparse
import json
import time

from common import stats, write_result
from websocket import create_connection

WARMUP = 5
USER = "bench_player"
PASSWORD = "bench_pass_123"


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--n", type=int, default=100)
    ap.add_argument("--port", default="8080", help="Go SERVER_PORT")
    ap.add_argument("--npc", default="Elara")
    args = ap.parse_args()

    ws = create_connection(f"ws://localhost:{args.port}/api/v1/ws", timeout=60)

    # Register (first run) or log in (subsequent runs) — either yields an authed session.
    ws.send(json.dumps({"event_type": "REGISTER_PLAYER", "username": USER, "password": PASSWORD}))
    reply = json.loads(ws.recv())
    if reply.get("action_type") != "LOGIN_SUCCESS":
        ws.send(json.dumps({"event_type": "LOGIN_PLAYER", "username": USER, "password": PASSWORD}))
        reply = json.loads(ws.recv())
        if reply.get("action_type") != "LOGIN_SUCCESS":
            raise SystemExit(f"Could not authenticate: {reply}")

    event = json.dumps({"event_type": "PLAYER_LOOKED_AT_NPC", "target_npc_name": args.npc})

    ws.send(event)
    first = json.loads(ws.recv())
    if first.get("content") != "Greetings.":
        raise SystemExit(f"Expected the non-LLM branch ('Greetings.'), got: {first}. "
                         f"An LLM in this path would invalidate the measurement.")

    for _ in range(WARMUP):
        ws.send(event)
        ws.recv()

    samples = []
    for _ in range(args.n):
        t0 = time.perf_counter()
        ws.send(event)
        ws.recv()
        samples.append((time.perf_counter() - t0) * 1000)
    ws.close()

    summary = stats(samples)
    print(f"end-to-end infra, no LLM (n={summary['n']}): "
          f"median {summary['median_ms']}ms, p95 {summary['p95_ms']}ms")
    write_result("ws_e2e", {
        "description": "WebSocket client -> Go orchestrator (quest validation, cache-aside "
                       "player/NPC lookups, emotion update, memory write) -> gRPC -> Python "
                       "router (non-LLM branch) -> response. Excludes inference.",
        "event_type": "PLAYER_LOOKED_AT_NPC",
        "warmup_iterations": WARMUP,
        "summary": summary,
        "samples_ms": [round(s, 3) for s in samples],
    })


if __name__ == "__main__":
    main()
