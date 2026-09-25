# Benchmarks

Every number here is produced by a committed script in [`benchmarks/`](../benchmarks/) and
its committed result JSON in [`benchmarks/results/`](../benchmarks/results/). Re-running the
scripts regenerates them. Where a measured value contradicts an older README claim, the
measured value wins and the README is corrected to match.

## Hardware & software

Two measurement dates are in this file. Every local and agy number was re-run on
**2026-09-25** against commit `7656cff` (branch `test-suite-review-fixes`), with memory
freed before the run; the Gemini
figures are from **2026-07-18** and were not re-run (no key). Numbers from different dates
are not directly comparable — the OS, GPU driver and code all changed in between.

| | 2026-09-25 (local, agy, infra) | 2026-07-18 (Gemini only) |
|---|---|---|
| CPU | 12th Gen Intel Core i5-1240P (16 threads) | same |
| RAM | ~15.3 GB; 11–12 GB available at the start, no disk swap (1.2 GB compressed zram) | ~15.3 GB |
| GPU | RTX 2050 4 GB via **Vulkan (open-source NVK driver)**; Ollama puts 16/33 llama3.1 layers on it | RTX 2050 4 GB, proprietary driver |
| OS / Python | Fedora 44, Linux 6.19, Python 3.12.14 | Linux 6.14, Python 3.12.3 |
| Ollama | 0.34.2 — llama3.1:8b (chat), nomic-embed-text (embeddings) | llama3.1:8b, nomic-embed-text |
| Postgres / Redis | Postgres 15, Redis 7 (fresh containers) | Postgres 17 (host), Redis 7 |
| Code | Memory v1 + test-suite fixes, `AGENT_MAX_ITERATIONS=5` (shipped default) | commit `095bf7e`, before Memory v1 |

The 4 GB GPU holds only half of an 8B model, so local inference is largely CPU-bound — local
latency here describes a laptop, not a served GPU deployment, and it is noisy: an earlier
run the same morning with 5.4 GB swapped out gave 12.9 s / 24.5 s for the two local paths
against 13.6 s / 27.4 s here, while single calls varied up to 2× between runs. State both
caveats with any local figure.

## Infrastructure latency (no LLM in the path)

These isolate the orchestration layer from inference by driving an event type that reaches
the non-LLM branch of the router. Measured 2026-09-25.

| Measurement | Median | p95 | n | Script |
|---|---|---|---|---|
| gRPC round-trip (client → Python → response) | **0.32 ms** | 0.76 ms | 200 | `grpc_roundtrip.py` |
| End-to-end infra (WebSocket → Go → gRPC → Python → back) | **3.76 ms** | 4.89 ms | 100 | `ws_e2e.py` |

The end-to-end path now includes the Memory v1 work per turn (episode write, emotion and
memory-line computation, the EMOTIONS frame) and the Redis rate-limit check, and is still
under 4 ms. (July, before those existed: 0.14 ms and 7.37 ms. Sub-millisecond gRPC times move
with CPU clock scaling: an earlier run the same morning measured 0.16 ms.) `ws_e2e.py` reads each turn through
to its final SPEAK frame; before that fix it would have timed only the first frame.

**Corrections to earlier README claims** (measurement wins):

| Old README claim | Measured | Note |
|---|---|---|
| gRPC round-trip ~14 ms | **0.32 ms** | ~40× lower; the old figure was never measured |
| Redis cache hit ~8 ms | **0.15 ms** | ~50× lower (July measurement) |
| PostgreSQL query ~42 ms | **0.20 ms** (indexed lookup) | ~200× lower on localhost (July measurement) |
| Infra overhead < 50 ms | **3.76 ms** | true, and well under |

## Inference latency: cloud vs local

Timing only — no answer scoring (that is the eval harness's job). Measured through the FastAPI
endpoints, the same router the game uses (`cloud_vs_local.py`).

| Path | Local llama3.1:8b (2026-09-25) | agy CLI (2026-09-25) | Gemini 3.5 Flash (2026-07-18, **stale**) |
|---|---|---|---|
| Lore path (RAG) | **13.6 s** median, p95 28.8 s, n=30 | **14.4 s** median, p95 16.9 s, n=30 | 2.87 s median, n=3 |
| Event path (agent) | **27.4 s** median, p95 44.0 s, n=15 | runs on local llama3.1 — see note | 5.43 s, n=1 |

**agy.** With `LLM_PROVIDER=agy` only the light model (lore answers) runs on the Antigravity
CLI, headless, one process per call (`AGY_MODEL` unset, so the CLI's default model). The
agent path needs tool calling, which the CLI does not offer, so it stays on local llama3.1.
agy is not faster than local here — every call spawns the CLI — but it is far steadier
(p95 16.9 s vs 28.8 s) and answers lore questions the 8B model gets wrong (see
`docs/agy_provider_experiment.md`).

**The agent path is slower in agy mode, but not because of agy.** In the agy run the
agent path measured 83.2 s median (n=15) against 27.4 s in the local run — reproduced across
two runs, the same local model, code and tool calls, no model reloads. A direct A/B of one
agent event (Silas, `PLAYER_INTERACT`) with no lore traffic in between gave overlapping
times in both modes (local 44–72 s, median ~55 s, n=6; agy 47–126 s, n=3, the slowest being
the first call after a restart). So an agent call costs about the same on its own; what the
local benchmark has and agy mode lacks is lore traffic on the *same* local model just before
each agent call, which keeps Ollama warm. With lore answers on agy, the local agent model
runs cold. That is a real cost of the mixed setup, not agy's latency, so it is not reported
in the agy column.

**Gemini (stale).** Measured 2026-07-18 on commit `095bf7e`, before Memory v1, with
`gemini-3.5-flash` on the free tier (20 requests/day/project): 3 questions and 2 events at
1 rep, so n=3 and n=1 — the second agent call failed with a 502 when the quota ran out.
Hardware as in the right-hand column above. It has not been re-run since, so it describes
older code on an older OS. Kept for reference, not for ratios: no local÷cloud ratio is given,
because the two sides were measured months apart.

## Streaming: time-to-first-token (C3)

RAG replies stream token-by-token over gRPC → WebSocket, so the player sees dialogue begin
at the first token instead of waiting for the whole answer. Measured directly on the
`ThinkStream` RPC (`streaming_ttft.py`, 3 questions × 2 reps, reps as the outer loop).

| Provider | TTFT median | TTFT range | Full response median | n |
|---|---|---|---|---|
| Local (llama3.1:8b) | **0.54 s** | 0.31 s – 13.8 s | 15.8 s | 6 |
| agy CLI | **15.4 s** | 13.7 s – 19.2 s | 15.4 s | 6 |
| Gemini | not measured† | — | — | — |

**Local TTFT is bimodal.** The first question to an NPC waits ~14 s for its first token (the
whole persona prompt is processed cold); every later question to that NPC gets its first
token in ~0.5 s, because Ollama keeps that NPC's prompt prefix cached. In game, every player
talking to the same NPC shares that prefix, so ~0.5 s is the steady state and ~14 s is a
once-per-NPC cold start (~36 s in the earlier, memory-starved run). An earlier run asked each question twice back to back, which made
the repeat look like a 0.2 s TTFT; the script now interleaves reps.

**agy does not stream**: the CLI returns the whole answer at once, so TTFT equals the full
response time.

† *Gemini returned persistent `503 UNAVAILABLE` during the July C3 window, and there is no
key now.*

## Semantic response cache (C4)

Repeated lore questions in a neutral context are served from an in-process per-NPC semantic
cache (cosine match on the question embedding, threshold 0.90) instead of the LLM. Measured
on the gRPC RAG path (`semantic_cache.py`): ask N questions cold, then re-ask the same N.

| | Latency | Notes |
|---|---|---|
| Cache miss (LLM) | **16.3 s** (median, n=4) | full RAG generation, local llama3.1:8b |
| Cache hit | **39 ms** (median, n=4; p95 43 ms) | question embedding + cosine scan, no LLM |
| Round-2 hit rate | **100%** (4/4) | exact repeats of the same question |

A hit is **~400× faster** than the miss and makes no model call. Threshold 0.90 is tuned to sit above measured same-topic /
different-intent question pairs (≤0.81), so the cache never serves a wrong answer; the trade
is that loosely-worded paraphrases (~0.82) miss and pay for the LLM. Only neutral contexts
are cached — since Memory v1 that means every emotion rounds to 0.00, trust included (it is
signed, and 0 is its baseline). The benchmark still sent July's neutral `trust: 0.5` and so
measured a 0% hit rate until it was corrected. The cache only serves anonymous, memory-free
requests (REST/evals/benchmarks); in-game requests carry a speaker and bypass it.

## The headline finding

Infrastructure is **under 4 ms end-to-end**; inference is **seconds**. The orchestration layer
(Go validation, lookups, episode writes, emotion and memory-line computation, rate limiting,
gRPC) is ~0.03% of a 13–14 s lore answer on local or agy. **Latency is inference-dominated;
the Go/gRPC/Redis pipeline is not the bottleneck.**

## Reproduce

The AI service runs on the host (the agy provider needs the host's `agy` login); gRPC is
fixed at :50051 and REST defaults to :8000.

```bash
# Infrastructure (AI service on :50051; Go server, Postgres, Redis running — set
# LLM_RATE_LIMIT high so the limiter runs but never refuses the 100+ events):
python benchmarks/grpc_roundtrip.py --n 200
python benchmarks/ws_e2e.py --n 100 --port <go port>

# Inference timing (start the service with the matching LLM_PROVIDER first):
python benchmarks/cloud_vs_local.py --provider ollama --reps 3
python benchmarks/cloud_vs_local.py --provider agy --reps 3
python benchmarks/cloud_vs_local.py --provider gemini --reps 1 --questions 3 --events 2

# Streaming TTFT (same):
python benchmarks/streaming_ttft.py --provider ollama --reps 2 --questions 3
python benchmarks/streaming_ttft.py --provider agy --reps 2 --questions 3

# Semantic response cache (restart the service first for a cold cache):
python benchmarks/semantic_cache.py --questions 4
```
