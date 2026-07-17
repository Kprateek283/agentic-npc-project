"""Inference latency: cloud vs local, per brain (RAG vs multi-step agent).

Timing only — no judge, no scoring. That halves the API calls per sample, which is what
makes a cloud run possible at all on a free-tier key (20 requests/day/project). Answer
quality is the eval harness's job (ai-service-python/evals), not this script's.

Drives the FastAPI endpoints, so it measures the same code path the game uses (both
transports share router.route_event).

Run it once per provider, against a service started with that provider:

    LLM_PROVIDER=ollama python -u main.py            # then:
    python benchmarks/cloud_vs_local.py --provider ollama --reps 3

    LLM_PROVIDER=gemini python -u main.py            # then (mind the 20/day cap):
    python benchmarks/cloud_vs_local.py --provider gemini --reps 1 --questions 5 --events 2

--questions/--events cap the sample so a cloud run fits the free-tier budget. The sample
size lands in the results file and MUST be quoted wherever the number appears.
"""

import argparse
import json
import time
import urllib.error
import urllib.request

from common import stats, write_result

LORE_QUESTIONS = [
    ("elara", "What happens to the Mindbloom Petal if it touches ordinary metal?"),
    ("baelor", "What materials do you work with?"),
    ("marcus", "When do the village gates close?"),
    ("kaelen", "Who leads the bandits?"),
    ("elian", "What is the Mindbloom Petal a cure for?"),
    ("silas", "How much does a healing potion cost?"),
    ("rook", "Where does your crew camp?"),
    ("elara", "Where does silverleaf grow?"),
    ("marcus", "Where do the bandits usually strike?"),
    ("baelor", "What does cold-forging require?"),
]

QUEST_EVENTS = [
    ("elara", "PLAYER_SUBMITTED_QUEST_ITEM", "mindbloom_petal"),
    ("baelor", "PLAYER_GAVE_GIFT", "apple"),
    ("elara", "PLAYER_GAVE_GIFT", "rotten_fish"),
    ("marcus", "PLAYER_INTERACT", "asks about the bandits on the east road"),
    ("kaelen", "PLAYER_INTERACT_QUEST", "asks about the bandit threat"),
]

CONTEXT = {"emotions": {"joy": 0.5, "sadness": 0.1, "anger": 0.1, "fear": 0.1, "trust": 0.5},
           "memories": ["The player greeted them earlier."], "quest_step": 2, "completion_rate": 0.66}


def post(url, body, timeout=600):
    req = urllib.request.Request(url, data=json.dumps(body).encode(),
                                 headers={"Content-Type": "application/json"})
    t0 = time.perf_counter()
    try:
        with urllib.request.urlopen(req, timeout=timeout) as r:
            r.read()
        return (time.perf_counter() - t0) * 1000, None
    except urllib.error.HTTPError as e:
        return None, f"HTTP {e.code}: {e.read().decode()[:120]}"
    except Exception as e:
        return None, f"{type(e).__name__}: {str(e)[:120]}"


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--provider", required=True, choices=["gemini", "ollama"],
                    help="label only — start the service with the matching LLM_PROVIDER")
    ap.add_argument("--base", default="http://localhost:8000")
    ap.add_argument("--reps", type=int, default=3)
    ap.add_argument("--questions", type=int, default=len(LORE_QUESTIONS))
    ap.add_argument("--events", type=int, default=len(QUEST_EVENTS))
    args = ap.parse_args()

    with urllib.request.urlopen(f"{args.base}/health", timeout=30) as r:
        health = json.load(r)
    if health["llm_provider"] != args.provider:
        raise SystemExit(f"--provider={args.provider} but the service reports "
                         f"{health['llm_provider']!r}. Refusing to mislabel results.")
    print(f"health: {health}")

    questions = LORE_QUESTIONS[:args.questions]
    events = QUEST_EVENTS[:args.events]

    rag_ms, agent_ms, errors = [], [], []
    for rep in range(args.reps):
        for npc, q in questions:
            ms, err = post(f"{args.base}/v1/chat", {"npc": npc, "question": q, "context": CONTEXT})
            (rag_ms.append(ms) if ms else errors.append(f"chat/{npc}: {err}"))
            print(f"  [rep {rep+1}] RAG {npc}: {round(ms) if ms else err} ms")
        for npc, ev, text in events:
            ms, err = post(f"{args.base}/v1/event",
                           {"npc": npc, "event_type": ev, "text": text, "context": CONTEXT})
            (agent_ms.append(ms) if ms else errors.append(f"event/{npc}: {err}"))
            print(f"  [rep {rep+1}] AGENT {npc}: {round(ms) if ms else err} ms")

    rag, agent = stats(rag_ms), stats(agent_ms)
    print(f"\n{args.provider} RAG   : median {rag.get('median_ms')}ms p95 {rag.get('p95_ms')}ms (n={rag['n']})")
    print(f"{args.provider} AGENT : median {agent.get('median_ms')}ms p95 {agent.get('p95_ms')}ms (n={agent['n']})")
    if errors:
        print(f"errors: {len(errors)} -> {errors[:3]}")

    write_result(f"cloud_vs_local_{args.provider}", {
        "description": "Inference latency through the FastAPI endpoints, per brain. "
                       "Timing only; no answer scoring.",
        "provider": args.provider,
        "service_health": health,
        "sample": {"questions": len(questions), "events": len(events), "reps": args.reps,
                   "rag_n": rag["n"], "agent_n": agent["n"]},
        "rag": rag,
        "agent": agent,
        "errors": errors,
        "rag_samples_ms": [round(s, 2) for s in rag_ms],
        "agent_samples_ms": [round(s, 2) for s in agent_ms],
    })


if __name__ == "__main__":
    main()
