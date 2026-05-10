# from langchain_ollama.chat_models import ChatOllama
# from langchain_ollama import OllamaEmbeddings
#
# # --- Our "Heavy" model for complex reasoning (LangGraph) ---
# llm_heavy = ChatOllama(model="llama3:8b-instruct-q4_K_M")
#
# # --- Our "Lightweight" model for fast RAG & simple chat ---
# llm_light = ChatOllama(model="phi3")
#
# # --- The embedding model  ---
# embeddings = OllamaEmbeddings(model="nomic-embed-text")
#

import os
from langchain_google_genai import ChatGoogleGenerativeAI
from langchain_ollama import OllamaEmbeddings # <-- NEW IMPORT
from dotenv import load_dotenv

load_dotenv()

# 1. Load the Gemini API Key
GEMINI_API_KEY = os.getenv("GEMINI_API_KEY")
if not GEMINI_API_KEY:
    raise ValueError("GEMINI_API_KEY not found in .env file")

# 2. Define our "Chat" model (the fast cloud model)
# We will use this for ALL chat, both simple and complex.
llm_cloud = ChatGoogleGenerativeAI(
    model="gemini-2.5-flash",
    google_api_key=GEMINI_API_KEY,
    temperature=0.7
)

# 3. Define our "Embedding" model (the fast local model)
# This will run on your local Ollama service.
embeddings_local = OllamaEmbeddings(model="nomic-embed-text")


# 4. Map these models to the variables our agents expect
llm_light = llm_cloud  # Use the fast cloud model for simple RAG
llm_heavy = llm_cloud  # Use the fast cloud model for complex LangGraph
embeddings = embeddings_local # Use the local model for embeddings

print("--- AI Config Loaded ---")
print("Chat Model: Gemini 1.5 Flash (Cloud)")
print("Embedding Model: Nomic Embed Text (Local via Ollama)")
print("------------------------")


