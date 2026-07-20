# Benchmarks

Every number here is produced by a committed script in [`benchmarks/`](../benchmarks/) and
its committed result JSON in [`benchmarks/results/`](../benchmarks/results/). Re-running the
scripts regenerates them. Where a measured value contradicts an older README claim, the
measured value wins and the README is corrected to match.

## Hardware & software

| | |
|---|---|
| CPU | 12th Gen Intel Core i5-1240P (16 threads) |
| RAM | ~15.3 GB |
| GPU | NVIDIA GeForce RTX 2050, 4 GB VRAM |
| OS / Python | Linux 6.14, Python 3.12.3 |
| Postgres / Redis | Postgres 17 (host), Redis 7 (container) |
| Local models | llama3.1:8b (chat), nomic-embed-text (embeddings) via Ollama |
| Cloud model | gemini-3.5-flash |

The 4 GB GPU cannot hold an 8B model, so local inference runs largely on CPU — local
latency here describes a laptop, not a served GPU deployment. State that caveat with any
local latency figure.

## Infrastructure latency (no LLM in the path)

These isolate the orchestration layer from inference by driving an event type that reaches
the non-LLM branch of the router.

| Measurement | Median | p95 | n | Script |
|---|---|---|---|---|
| gRPC round-trip (client → Python → response) | **0.14 ms** | 0.25 ms | 200 | `grpc_roundtrip.py` |
| End-to-end infra (WebSocket → Go → gRPC → Python → back) | **7.37 ms** | 9.8 ms | 100 | `ws_e2e.py` |
| Cache hit (Redis GET + Postgres PK fetch) | **0.15 ms** | 0.29 ms | 200 | `cmd/benchcache` |
| Cache miss (Postgres indexed lookup + cache write) | **0.20 ms** | 0.35 ms | 200 | `cmd/benchcache` |

**Corrections to earlier README claims** (measurement wins):

| Old README claim | Measured | Note |
|---|---|---|
| gRPC round-trip ~14 ms | **0.14 ms** | ~100× lower; the old figure was never measured |
| Redis cache hit ~8 ms | **0.15 ms** | ~50× lower |
| PostgreSQL query ~42 ms | **0.20 ms** (indexed lookup) | ~200× lower on localhost |
| Infra overhead < 50 ms | **7.37 ms** | true, and well under |

**On the cache.** The measured cache "speedup" is 1.38× (0.20 ms → 0.15 ms) — a 0.05 ms
saving, not the implied 34 ms. This is because `GetPlayer` on a cache *hit* still fetches the
row from Postgres by primary key; the cache replaces an indexed `WHERE player_id=?` lookup
with a PK fetch, it does not remove the database round-trip. At localhost latencies the whole
read path is sub-millisecond, so the cache earns its keep only when Postgres is remote or under
load — worth stating honestly rather than claiming a large local win.

## Inference latency: cloud vs local

Timing only — no answer scoring (that is the eval harness's job). Measured through the FastAPI
endpoints, the same router the game uses.

| Path | Cloud (gemini-3.5-flash) | Local (llama3.1:8b) | Local ÷ Cloud |
|---|---|---|---|
| RAG (fast brain) | **2.87 s** (median, n=3*) | **18.2 s** (median, n=30) | ~6.3× |
| Agent (complex brain) | **5.43 s** (n=1*) | **26.0 s** (median, n=15) | ~4.8× |

\* *Cloud sample is tiny: the free-tier key allows 20 requests/day/project, and the run hit
the cap. These medians are indicative, not robust — n is stated so the number is never
mistaken for a large sample. See `evals/results.md` for why the cloud side stays small.*

**On the cloud-vs-local ratio.** Local RAG is ~6× slower than cloud here, not the "2.3×" the
old resume claimed — and the honest reason is hardware, not the model: llama3.1:8b runs mostly
on CPU because it does not fit the 4 GB GPU, while gemini-3.5-flash is served on Google's
accelerators. The ratio is a laptop-vs-cloud measurement, and both caveats (tiny cloud n,
CPU-bound local) must travel with the number. The old 2.3× figure came from no committed
measurement and is replaced by these.

## Streaming: time-to-first-token (C3)

RAG replies stream token-by-token over gRPC → WebSocket, so the player sees dialogue begin
at the first token instead of waiting for the whole answer. Measured directly on the
`ThinkStream` RPC (`benchmarks/streaming_ttft.py`).

| Provider | TTFT (first token) | Full response | Player sees text sooner by |
|---|---|---|---|
| Local (llama3.1:8b) | **13.6 s** (median, n=6) | **21.1 s** (median, n=6) | **~7.4 s** |
| Cloud (gemini-3.5-flash) | not measured† | — | — |

† *Gemini returned persistent `503 UNAVAILABLE` ("high demand") throughout the C3 test
window, so a cloud TTFT could not be captured without inventing a number. The streaming
path is provider-agnostic (same LCEL `.stream()`); the cloud win is expected to be far
larger, since cloud full-response RAG is ~2.9 s (above) — re-run when the model is
available. On local, TTFT is CPU-bound (prompt processing on the 4 GB GPU spilling to CPU),
so even the first token takes seconds; the value shown is the ~7 s of silence removed, not a
sub-second TTFT.*

## Semantic response cache (C4)

Repeated lore questions in a near-neutral context are served from an in-process per-NPC
semantic cache (cosine match on the question embedding, threshold 0.90) instead of the LLM.
Measured on the gRPC RAG path (`benchmarks/semantic_cache.py`): ask N questions cold, then
re-ask the same N.

| | Latency | Notes |
|---|---|---|
| Cache miss (LLM) | **20.8 s** (median, n=4) | full RAG generation, local llama3.1:8b |
| Cache hit | **65 ms** (median, n=4) | question embedding + cosine scan, no LLM |
| Round-2 hit rate | **100%** (4/4) | exact repeats of the same question |

A cache hit is **~320× faster** than the LLM miss and makes no model call. The hit-latency
p95 (~3.2 s) is a one-off cold embedding-model call; steady-state hits are ~40–90 ms.
Threshold 0.90 is tuned to sit above measured same-topic/different-intent question pairs
(≤0.81) so the cache never serves a wrong answer — the trade is that loosely-worded
paraphrases (~0.82) miss and pay for the LLM. Only near-neutral emotional contexts are
cached, since the RAG prompt conditions tone on live emotion.

## The headline finding

Infrastructure is **sub-10 ms end-to-end**; inference is **seconds**. The orchestration layer
(Go validation, cache-aside reads, gRPC, emotion/memory writes) contributes ~7 ms to a turn
whose LLM call takes 3–5 s on cloud. **Latency is inference-dominated; the Go/gRPC/Redis
pipeline is not the bottleneck** — it is ~0.2% of a cloud RAG turn. That is the defensible,
measured version of the two-brain performance story.

## Reproduce

```bash
# Infrastructure (needs the AI service on :50051, Go server on :8081, Postgres, Redis):
python benchmarks/grpc_roundtrip.py --n 200
python benchmarks/ws_e2e.py --n 100 --port 8081
cd backend-go && go run ./cmd/benchcache -n 200 -player bench_player

# Inference timing (start the service with the matching LLM_PROVIDER first):
python benchmarks/cloud_vs_local.py --provider ollama --reps 3
python benchmarks/cloud_vs_local.py --provider gemini --reps 1 --questions 3 --events 2

# Streaming TTFT (start the service with the matching LLM_PROVIDER first):
python benchmarks/streaming_ttft.py --provider ollama --reps 2 --questions 3
python benchmarks/streaming_ttft.py --provider gemini --reps 1 --questions 3

# Semantic response cache (restart the service first for a cold cache):
python benchmarks/semantic_cache.py --questions 4
```
