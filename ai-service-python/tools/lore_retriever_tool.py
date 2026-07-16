from langchain_text_splitters import RecursiveCharacterTextSplitter
from langchain_community.vectorstores import FAISS
# --- THIS IS THE FIX ---
# We import the '@tool' decorator, as shown in the documentation you found
from langchain.tools import tool
# ---------------------
from langchain_core.documents import Document
from config import QDRANT_URL, RETRIEVER_K, VECTOR_STORE, embeddings
import json
import os


def _collection_name(lore_file_path: str) -> str:
    """Deterministic per-NPC collection name, e.g. gamedata/npcs/elara/lore.json -> lore_elara."""
    return f"lore_{os.path.basename(os.path.dirname(lore_file_path))}"


def _build_vector_store(texts, lore_file_path: str):
    """Builds the vector index for one NPC on the configured backend.

    Embedding dimension is inferred from the embedding model by the integration —
    never hardcoded, so swapping EMBEDDING_MODEL just works.
    """
    if VECTOR_STORE == "faiss":
        return FAISS.from_documents(texts, embeddings)

    # ponytail: drop-and-rebuild each startup. Lore is static and tiny (tens of facts per
    # NPC), so a full re-embed costs less than reconciling. If lore grows or becomes
    # player-writable (S1), switch to an upsert keyed on a content hash.
    from langchain_qdrant import QdrantVectorStore

    return QdrantVectorStore.from_documents(
        texts,
        embeddings,
        url=QDRANT_URL,
        collection_name=_collection_name(lore_file_path),
        force_recreate=True,
    )


def create_lore_tool_from_file(lore_file_path: str):
    """
    Creates a RAG retriever AND a RAG tool from a specific lore JSON file.
    This function is called once for each agent at startup.
    """
    print(f"Building RAG retriever for: {lore_file_path}")

    # Load the JSON file
    try:
        with open(lore_file_path, 'r') as f:
            data = json.load(f)
    except Exception as e:
        print(f"  ERROR: Failed to read or parse JSON from {lore_file_path}: {e}")
        return None, None

    lore_facts = data.get("known_facts", [])
    if not lore_facts:
        print(f"  WARNING: No 'known_facts' found in {lore_file_path}.")
        return None, None

    documents = [Document(page_content=fact) for fact in lore_facts]
    if not documents:
        print(f"  WARNING: No documents to load for {lore_file_path}.")
        return None, None

    text_splitter = RecursiveCharacterTextSplitter(chunk_size=500, chunk_overlap=50)
    texts = text_splitter.split_documents(documents)

    if not texts:
        print(f"  WARNING: Text splitting resulted in no texts for {lore_file_path}.")
        return None, None

    vectorstore = _build_vector_store(texts, lore_file_path)
    retriever = vectorstore.as_retriever(search_kwargs={"k": RETRIEVER_K})
    print(f"Successfully built retriever for: {lore_file_path} (backend={VECTOR_STORE}, k={RETRIEVER_K})")

    # --- THE NEW, CORRECT TOOL DEFINITION ---
    # We manually define our tool using the @tool decorator.
    # This function "closes over" the retriever variable.
    @tool
    def lore_book_search(query: str) -> str:
        """Search for information about game lore, items, people, and places."""
        print(f"--- Tool Called: lore_book_search, Query: {query} ---")
        docs = retriever.invoke(query)
        # Format the results into a single string for the LLM
        return "\n".join([doc.page_content for doc in docs])

    # --- END OF FIX ---

    # Return the raw retriever (for our simple RAG agent)
    # and the fully-formed tool (for our complex LangGraph agent)
    return retriever, lore_book_search

