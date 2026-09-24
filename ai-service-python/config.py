import logging

from dotenv import load_dotenv
from langchain_ollama import OllamaEmbeddings
from env import env_int, env_str

logger = logging.getLogger(__name__)

load_dotenv()

# --- Provider selection (env-driven; see .env.example) ---
LLM_PROVIDER = env_str("LLM_PROVIDER", "gemini").lower()
# Pinned to a concrete model (not the moving `gemini-flash-latest` alias) so a committed
# benchmark number always names the model that produced it. Note: gemini-2.5-flash, which
# this project used to hardcode, now 404s for newly-created Google Cloud projects.
GEMINI_MODEL = env_str("GEMINI_MODEL", "gemini-3.5-flash")
OLLAMA_HOST = env_str("OLLAMA_HOST", "http://localhost:11434")
# llama3.1:8b, not llama3:8b: the LangGraph agent needs native tool-calling, which
# llama3:8b does not have (`ollama show llama3:8b` reports capability "completion" only).
OLLAMA_MODEL_HEAVY = env_str("OLLAMA_MODEL_HEAVY", "llama3.1:8b")
OLLAMA_MODEL_LIGHT = env_str("OLLAMA_MODEL_LIGHT", OLLAMA_MODEL_HEAVY)
EMBEDDING_MODEL = env_str("EMBEDDING_MODEL", "nomic-embed-text")

# Configuration for local agy experiment provider (see docs/agy_provider_experiment.md).
AGY_BIN = env_str("AGY_BIN", "agy")
AGY_TIMEOUT_S = env_int("AGY_TIMEOUT_S", 120)
AGY_MODEL = env_str("AGY_MODEL")

# The library default is 6 retries. On a free-tier key that is actively harmful: the first
# 429 triggers a retry storm that burns the rest of the daily quota (it cost a whole eval
# sample once). Retrying a per-day quota error cannot succeed anyway. Read at module level
# because the eval judge may be Gemini even when the answering provider is Ollama.
GEMINI_MAX_RETRIES = env_int("GEMINI_MAX_RETRIES", 1)

# --- Chat models: llm_light (fast RAG) and llm_heavy (LangGraph agent) ---
if LLM_PROVIDER == "gemini":
    from langchain_google_genai import ChatGoogleGenerativeAI

    GEMINI_API_KEY = env_str("GEMINI_API_KEY")
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

elif LLM_PROVIDER == "agy":
    # Local experiment only: uses host-installed agy CLI for the lore path (light LLM).
    # Event path (heavy LLM) uses Ollama because agy does not support tool-calling.
    from agy_chat import ChatAgy
    from langchain_ollama.chat_models import ChatOllama

    llm_light = ChatAgy(binary=AGY_BIN, timeout_s=AGY_TIMEOUT_S, model=AGY_MODEL)
    llm_heavy = ChatOllama(model=OLLAMA_MODEL_HEAVY, base_url=OLLAMA_HOST, temperature=0.7)
    CHAT_MODEL_LIGHT = f"agy ({AGY_MODEL})" if AGY_MODEL else "agy"
    CHAT_MODEL_HEAVY = OLLAMA_MODEL_HEAVY

else:
    raise ValueError(
        f"Unsupported LLM_PROVIDER={LLM_PROVIDER!r} (expected 'gemini', 'ollama', or 'agy')"
    )

# --- Embeddings always run locally on Ollama, regardless of chat provider ---
embeddings = OllamaEmbeddings(model=EMBEDDING_MODEL, base_url=OLLAMA_HOST)

# --- Vector store: faiss (default, in-process) | qdrant (scale-out) ---
VECTOR_STORE = env_str("VECTOR_STORE", "faiss").lower()
QDRANT_URL = env_str("QDRANT_URL", "http://localhost:6333")
RETRIEVER_K = env_int("RETRIEVER_K", 3)

# Path to shared gamedata directory (defaults to ../gamedata).
GAMEDATA_DIR = env_str("GAMEDATA_DIR", "../gamedata")

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

logger.info("--- AI Config Loaded ---")
logger.info("Chat Provider: %s", LLM_PROVIDER)
logger.info("Chat Model (light/heavy): %s / %s", CHAT_MODEL_LIGHT, CHAT_MODEL_HEAVY)
logger.info("Embedding Model: %s (local via Ollama at %s)", EMBEDDING_MODEL, OLLAMA_HOST)
logger.info(
    "Vector Store: %s%s, retriever k=%d",
    VECTOR_STORE,
    " (" + QDRANT_URL + ")" if VECTOR_STORE == "qdrant" else "",
    RETRIEVER_K,
)
logger.info("------------------------")
