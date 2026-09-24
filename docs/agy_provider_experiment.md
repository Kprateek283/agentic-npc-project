# Antigravity CLI (`agy`) Provider Experiment

## Overview
This experiment introduces a local-only chat model provider (`ChatAgy`) that shells out to the locally installed Antigravity CLI (`agy`) for the lore path (`llm_light`).

## Motivation
The experiment measures whether routing lore queries to a stronger cloud-backed model via the `agy` CLI corrects in-character and factual nuances that local `llama3.1:8b` gets wrong, without exposing Google Gemini API keys directly in the environment.

## How to Run
Run the AI service on the host machine (not in Docker, since the container environment lacks the `agy` binary and credentials):

1. Verify that `agy` is installed and available on `PATH`.
2. Configure provider settings:
   ```bash
   export LLM_PROVIDER=agy
   # Optional overrides:
   # export AGY_BIN=agy
   # export AGY_TIMEOUT_S=120
   # export AGY_MODEL=
   ```
3. Start background dependencies (Ollama for embeddings and event-path tool calling):
   ```bash
   docker compose up -d ollama
   ```
4. Run the AI service directly on the host:
   ```bash
   cd ai-service-python
   source .venv/bin/activate
   python main.py
   ```

## Limitations
- **No Tool Calling**: The `agy` CLI returns plain text. The event path (`llm_heavy`) still routes to Ollama for LangGraph tool-calling tasks.
- **No Token Streaming**: CLI output is captured as a batch and returned in a single chunk.
- **Quota & Host Dependency**: Requires host execution and consumes the active user's Antigravity quota.

## Results

Measured 2026-09-24 on this machine, with the same context the orchestrator sends: anger 1.00
and trust -1.00 toward the speaker, general mood 0.25, and one remembered line, "You threw
stones at me (5 times)". The question was "Do you have a remedy for fever?".

| | llama3.1:8b (local) | agy |
|---|---|---|
| Reacts to the grievance | 0 of 3 runs | **3 of 3 runs** |
| Time per reply (REST, lore path) | 100-130 s | **15-17 s** |
| Keeps the lore grounding | yes | yes |

llama3.1:8b answered helpfully every time, explaining fever-root to a player who had just
stoned the NPC. The CLI refused while naming the grievance in all three runs, and still gave
the grounded fact: "You dare ask me for help after throwing stones at me five times?
Fever-root cures a fever, but it must be boiled twice ... Now get away from me."

This settles the question the memory work left open: the memory layer was supplying everything
needed, and the weak reaction was the small model's instruction-following, not the computed
state. Reordering the prompt had not changed it.

Caveats: the raw CLI answers a bare prompt in 3-4 s, so most of the 15-17 s here is retrieval,
embedding and process start-up. The event path still runs on Ollama, because the CLI cannot
make structured tool calls. Nothing here is deployable: the container has neither the binary
nor its credentials, and every call spends the account's quota.

## Note found while running this

The gRPC port is hardcoded as 50051 in `main.py`, while the REST port comes from `API_PORT`,
so a host instance collides with the container and one must be stopped. Making it an env var
(say `GRPC_PORT`) would let both run side by side.
