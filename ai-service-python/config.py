import os

from dotenv import load_dotenv
from langchain_ollama import OllamaEmbeddings

load_dotenv()

# --- Provider selection (env-driven; see .env.example) ---
LLM_PROVIDER = os.getenv("LLM_PROVIDER", "gemini").lower()
# Pinned to a concrete model (not the moving `gemini-flash-latest` alias) so a committed
# benchmark number always names the model that produced it. Note: gemini-2.5-flash, which
# this project used to hardcode, now 404s for newly-created Google Cloud projects.
GEMINI_MODEL = os.getenv("GEMINI_MODEL", "gemini-3.5-flash")
OLLAMA_HOST = os.getenv("OLLAMA_HOST", "http://localhost:11434")
# llama3.1:8b, not llama3:8b: the LangGraph agent needs native tool-calling, which
# llama3:8b does not have (`ollama show llama3:8b` reports capability "completion" only).
OLLAMA_MODEL_HEAVY = os.getenv("OLLAMA_MODEL_HEAVY", "llama3.1:8b")
OLLAMA_MODEL_LIGHT = os.getenv("OLLAMA_MODEL_LIGHT", OLLAMA_MODEL_HEAVY)
EMBEDDING_MODEL = os.getenv("EMBEDDING_MODEL", "nomic-embed-text")

# The library default is 6 retries. On a free-tier key that is actively harmful: the first
# 429 triggers a retry storm that burns the rest of the daily quota (it cost a whole eval
# sample once). Retrying a per-day quota error cannot succeed anyway. Read at module level
# because the eval judge may be Gemini even when the answering provider is Ollama.
GEMINI_MAX_RETRIES = int(os.getenv("GEMINI_MAX_RETRIES", "1"))

# --- Chat models: llm_light (fast RAG) and llm_heavy (LangGraph agent) ---
if LLM_PROVIDER == "gemini":
    from langchain_google_genai import ChatGoogleGenerativeAI

    GEMINI_API_KEY = os.getenv("GEMINI_API_KEY")
    if not GEMINI_API_KEY:
        raise ValueError("GEMINI_API_KEY is required when LLM_PROVIDER=gemini")

    llm_light = llm_heavy = ChatGoogleGenerativeAI(
        model=GEMINI_MODEL,
        google_api_key=GEMINI_API_KEY,
        temperature=0.7,
        max_retries=GEMINI_MAX_RETRIES,
    )
    CHAT_MODEL_LIGHT = CHAT_MODEL_HEAVY = GEMINI_MODEL

elif LLM_PROVIDER == "ollama":
    from langchain_ollama.chat_models import ChatOllama

    llm_light = ChatOllama(model=OLLAMA_MODEL_LIGHT, base_url=OLLAMA_HOST, temperature=0.7)
    llm_heavy = ChatOllama(model=OLLAMA_MODEL_HEAVY, base_url=OLLAMA_HOST, temperature=0.7)
    CHAT_MODEL_LIGHT, CHAT_MODEL_HEAVY = OLLAMA_MODEL_LIGHT, OLLAMA_MODEL_HEAVY

else:
    raise ValueError(f"Unsupported LLM_PROVIDER={LLM_PROVIDER!r} (expected 'gemini' or 'ollama')")

# --- Embeddings always run locally on Ollama, regardless of chat provider ---
embeddings = OllamaEmbeddings(model=EMBEDDING_MODEL, base_url=OLLAMA_HOST)

# --- Vector store: faiss (default, in-process) | qdrant (scale-out) ---
VECTOR_STORE = os.getenv("VECTOR_STORE", "faiss").lower()
QDRANT_URL = os.getenv("QDRANT_URL", "http://localhost:6333")
RETRIEVER_K = int(os.getenv("RETRIEVER_K", "3"))

if VECTOR_STORE not in ("faiss", "qdrant"):
    raise ValueError(f"Unsupported VECTOR_STORE={VECTOR_STORE!r} (expected 'faiss' or 'qdrant')")

if VECTOR_STORE == "qdrant":
    # Fail fast and loudly: a silent fall back to FAISS would make benchmark and eval
    # results lie about which backend produced them.
    from qdrant_client import QdrantClient

    try:
        QdrantClient(url=QDRANT_URL, timeout=5).get_collections()
    except Exception as exc:
        raise RuntimeError(
            f"VECTOR_STORE=qdrant but Qdrant is unreachable at {QDRANT_URL}: {exc}. "
            f"Start it with `docker compose up -d qdrant`."
        ) from exc

print("--- AI Config Loaded ---")
print(f"Chat Provider: {LLM_PROVIDER}")
print(f"Chat Model (light/heavy): {CHAT_MODEL_LIGHT} / {CHAT_MODEL_HEAVY}")
print(f"Embedding Model: {EMBEDDING_MODEL} (local via Ollama at {OLLAMA_HOST})")
print(f"Vector Store: {VECTOR_STORE}" + (f" ({QDRANT_URL})" if VECTOR_STORE == "qdrant" else "") + f", retriever k={RETRIEVER_K}")
print("------------------------")
