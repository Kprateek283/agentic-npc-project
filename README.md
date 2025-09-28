# Agentic NPC Framework

> A multiservice backend in Go and Python to power autonomous, memory-aware NPCs for game environments.

![Status](https://img.shields.io/badge/status-under%20development-yellow)

This project is the backend engine for creating intelligent, non-player characters (NPCs) for video games. It moves beyond simple scripted behavior by giving NPCs a persistent memory, a dynamic emotional state, and the ability to have unscripted, context-aware conversations using a local Large Language Model.

---

## Architecture Overview

The system is built on a distributed, microservice-based architecture designed for real-time communication and state management.

### Core Features
* **Persistent Memory:** NPCs remember past interactions, which are stored in a PostgreSQL database.
* **Dynamic Emotions:** An NPC's emotional state (anger, fear, trust, etc.) changes based on events and influences their behavior.
* **Generative Dialogue:** Uses LangChain and a local LLM (Llama 3 via Ollama) with a RAG (Retrieval-Augmented Generation) system to produce unique, in-character dialogue.
* **Real-time Communication:** Utilizes WebSockets for low-latency communication with the game client.
* **Multi-Service Design:** A robust Go backend orchestrates data and state, while a dedicated Python service handles all complex AI/LLM logic via gRPC.

---

## Tech Stack

-   **Backend (Orchestrator):** Go, Gin (for WebSockets/REST)
-   **AI Service:** Python, FastAPI, LangChain, gRPC
-   **Database:** PostgreSQL (for persistent state), Redis (for caching)
-   **ORM:** ent (for type-safe Go database access)
-   **LLM Serving:** Ollama (running Llama 3 8B)
-   **DevOps:** Docker, Docker Compose

---

## Getting Started

### Prerequisites
- Go (1.22+)
- Python (3.11+)
- Docker & Docker Compose
- Ollama installed and running

### Running the Application
1.  **Clone the repository:**
    ```bash
    git clone [https://github.com/Kprateek283/agentic-npc-project.git](https://github.com/Kprateek283/agentic-npc-project.git)
    cd agentic-npc-project
    ```
2.  **Set up environment files:**
    * Create a `.env` file inside the `backend-go` directory.
    * Add your `POSTGRES_DSN` and `REDIS_ADDR` to this file. See `backend-go/internal/config/config.go` for details.
3.  **Launch infrastructure:**
    ```bash
    docker-compose up -d
    ```
4.  **Run the Python AI Service:**
    ```bash
    cd ai-service-python
    source .venv/bin/activate.fish # or your venv activation script
    pip install -r requirements.txt
    python main.py
    ```
5.  **Run the Go Backend:**
    (In a new terminal)
    ```bash
    cd backend-go
    go run ./cmd/server/main.go
    ```

