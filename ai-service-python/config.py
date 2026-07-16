import os

from dotenv import load_dotenv
from langchain_ollama import OllamaEmbeddings

load_dotenv()

# --- Provider selection (env-driven; see .env.example) ---
LLM_PROVIDER = os.getenv("LLM_PROVIDER", "gemini").lower()
GEMINI_MODEL = os.getenv("GEMINI_MODEL", "gemini-2.5-flash")
OLLAMA_HOST = os.getenv("OLLAMA_HOST", "http://localhost:11434")
OLLAMA_MODEL_HEAVY = os.getenv("OLLAMA_MODEL_HEAVY", "llama3:8b")
OLLAMA_MODEL_LIGHT = os.getenv("OLLAMA_MODEL_LIGHT", OLLAMA_MODEL_HEAVY)
EMBEDDING_MODEL = os.getenv("EMBEDDING_MODEL", "nomic-embed-text")

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

print("--- AI Config Loaded ---")
print(f"Chat Provider: {LLM_PROVIDER}")
print(f"Chat Model (light/heavy): {CHAT_MODEL_LIGHT} / {CHAT_MODEL_HEAVY}")
print(f"Embedding Model: {EMBEDDING_MODEL} (local via Ollama at {OLLAMA_HOST})")
print("------------------------")
