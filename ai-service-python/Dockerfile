FROM python:3.12-slim
WORKDIR /app

# curl is used by the compose healthcheck against /health.
RUN apt-get update && apt-get install -y --no-install-recommends curl \
    && rm -rf /var/lib/apt/lists/*

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Service code + its gamedata (agents build embeddings from lore.json at startup).
COPY . .

# gRPC (Go orchestrator) and REST (evals/benchmarks/healthcheck) — both in one process.
EXPOSE 50051 8000
CMD ["python", "-u", "main.py"]
