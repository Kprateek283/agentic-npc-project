# Agentic NPC Framework: A High-Performance Distributed System for Generative Game AI

This repository contains a modular, production-grade backend framework designed to power autonomous, stateful, and memory-aware Non-Player Characters (NPCs) in modern game environments like Unreal Engine 5. The system moves beyond traditional deterministic behavior trees by leveraging Large Language Models (LLMs) for dynamic dialogue, emotional evolution, and complex quest reasoning.

## Core Philosophy: The Two-Brain Model

To balance the competing requirements of real-time responsiveness and cognitive depth, the architecture implements a novel Two-Brain model:

1. Fast Brain (Retrieval-Augmented Generation): A low-latency path for factual queries and lore-related interactions. It utilizes a FAISS-based vector store to ground LLM responses in game-specific context, ensuring factual accuracy while maintaining sub-second response times using cloud-based inference (Gemini Flash).
2. Complex Brain (Stateful Reasoning): A high-depth path for state-changing events and quest progression. Built on LangGraph, this "brain" manages multi-step reasoning, updates internal emotional states, and handles complex transitions in game logic, with a focus on narrative consistency while maintaining highly competitive latency (~3s).

## Architectural Overview

The framework is built as a decoupled microservice architecture, leveraging the strengths of Go for high-concurrency orchestration and Python for advanced AI/ML workflows.

### 1. Go Orchestrator (The Dungeon Master)
The Go service acts as the authoritative source of truth and the central hub for the game world.
- Connection Management: Handles persistent, bidirectional communication with game clients via WebSockets (Gin).
- State Machine: Manages quest lifecycles, player inventories, and NPC relationships.
- Orchestration: Routes player events to the appropriate AI brain via high-performance gRPC calls.
- Persistence Layer: Utilizes the Ent ORM for type-safe, graph-based interactions with PostgreSQL.
- Caching: Implements a cache-aside strategy with Redis to minimize database I/O for frequently accessed player and NPC states (e.g., trust levels, active session data).

### 2. Python AI Service (The Brain)
The Python service encapsulates all LLM logic and cognitive processes.
- gRPC Interface: Exposes specialized methods for RAG-based retrieval and LangGraph-driven reasoning.
- LangChain Integration: Orchestrates model prompts, output parsers, and tool-calling chains.
- Vector Store (FAISS): Provides efficient similarity search for NPC lore and background information.
- Dynamic Context Injection: Formats real-time game state (emotions, memories, quest progress) into the LLM context window to ensure situational awareness.

## Technical Specifications

### Tech Stack
- Backend: Go (Golang), Gin, Ent ORM, gRPC-Go, Go-Redis.
- AI Service: Python, LangChain, LangGraph, FAISS, gRPC-Python.
- Data Management: PostgreSQL, Redis.
- Inference: Google Gemini API (Cloud) and Ollama/Llama 3 (Local).
- Infrastructure: Docker, Docker Compose, Protocol Buffers.
- Client Target: Unreal Engine 5 (C++/Blueprints).

### Communication Protocols
- Client-to-Backend: JSON-based events over persistent WebSockets.
- Inter-Service: Binary Protocol Buffers over gRPC (HTTP/2), ensuring low-latency and strict type safety between the Go and Python layers.

### Inference Provider Configuration

The chat provider is selected at startup by environment variable — no code changes. See
`ai-service-python/.env.example` for the full set of variables and their defaults.

| Variable | Default | Purpose |
|---|---|---|
| `LLM_PROVIDER` | `gemini` | `gemini` (cloud) or `ollama` (local) |
| `GEMINI_API_KEY` | — | Required only when `LLM_PROVIDER=gemini` |
| `GEMINI_MODEL` | `gemini-2.5-flash` | Cloud chat model |
| `OLLAMA_MODEL_HEAVY` | `llama3:8b` | Local chat model for the LangGraph agent |
| `OLLAMA_MODEL_LIGHT` | = `OLLAMA_MODEL_HEAVY` | Local chat model for the RAG path |
| `EMBEDDING_MODEL` | `nomic-embed-text` | Embedding model (always local via Ollama) |
| `OLLAMA_HOST` | `http://localhost:11434` | Ollama endpoint |

Embeddings always run locally on Ollama regardless of the chat provider, so Ollama is a
dependency in both modes:

```bash
# Install Ollama (https://ollama.com/download), then:
ollama pull nomic-embed-text   # embeddings — required in both modes
ollama pull llama3:8b          # chat — only needed for LLM_PROVIDER=ollama
```

```bash
# Cloud mode (default)
LLM_PROVIDER=gemini
GEMINI_API_KEY=your-key-here

# Local mode — no API key needed
LLM_PROVIDER=ollama
OLLAMA_MODEL_HEAVY=llama3:8b
```

### Performance Benchmarks (Empirical Results)
- Go Logic & State Validation: < 50ms.
- Database Cache Hit (Redis): ~8ms.
- gRPC Round-trip (No LLM): ~14ms.
- Fast Brain Response (Cloud): ~1.0s (Inference-dominated).
- Complex Brain Response (Cloud): ~3.0s (Multi-step reasoning).

## Data Persistence & Schema Design

The system employs an 8-table relational schema designed for extensibility:
- Player & NPC Entities: Core state and identity.
- Relationships: Tracks dynamic variables like trust_level and emotional affinity.
- NPC_Memory: Stores persistent key-value memories for long-term NPC continuity.
- Quest & Inventory: Manages player progression and item-based triggers.

## Deployment & Scaling

The framework is designed for horizontal scalability:
- Stateless Orchestration: The Go service can be scaled behind a load balancer with Redis handling session state.
- GPU-Aware AI Routing: The Python service is structured to support multi-instance deployment on GPU-accelerated nodes for local inference or high-throughput cloud API routing.
- Containerization: Full Docker Compose support for standardized development and production environments.

## Future Vision

Planned enhancements focused on production-scale deployment include:
- Persistent Writable RAG: Enabling NPCs to dynamically update their own vector stores with new player-specific memories.
- Proactive NPCs: State-driven triggers that allow NPCs to initiate conversations or world actions without player input.
- Hardware-Aware Routing: Intelligent load balancing that toggles between local and cloud inference based on real-time latency and cost constraints.

---
This project was developed as a comprehensive exploration of distributed systems, real-time state management, and the integration of generative AI into high-performance gaming backends.
