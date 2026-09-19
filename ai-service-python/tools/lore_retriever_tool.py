import json
import logging
import os

from langchain.tools import tool
from langchain_community.vectorstores import FAISS
from langchain_core.documents import Document
from langchain_text_splitters import RecursiveCharacterTextSplitter

from config import QDRANT_URL, RETRIEVER_K, VECTOR_STORE, embeddings

logger = logging.getLogger(__name__)


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
    logger.info("Building RAG retriever for: %s", lore_file_path)

    # Load the JSON file
    try:
        with open(lore_file_path, 'r') as f:
            data = json.load(f)
    except Exception as e:
        logger.exception("Failed to read or parse JSON from %s: %s", lore_file_path, e)
        return None, None

    lore_facts = data.get("known_facts", [])
    if not lore_facts:
        logger.warning("No 'known_facts' found in %s.", lore_file_path)
        return None, None

    documents = [Document(page_content=fact) for fact in lore_facts]
    if not documents:
        logger.warning("No documents to load for %s.", lore_file_path)
        return None, None

    text_splitter = RecursiveCharacterTextSplitter(chunk_size=500, chunk_overlap=50)
    texts = text_splitter.split_documents(documents)

    if not texts:
        logger.warning("Text splitting resulted in no texts for %s.", lore_file_path)
        return None, None

    vectorstore = _build_vector_store(texts, lore_file_path)
    retriever = vectorstore.as_retriever(search_kwargs={"k": RETRIEVER_K})
    logger.info("Successfully built retriever for: %s (backend=%s, k=%s)", lore_file_path, VECTOR_STORE, RETRIEVER_K)

    # This function "closes over" the retriever variable.
    @tool
    def lore_book_search(query: str) -> str:
        """Search for information about game lore, items, people, and places."""
        logger.info("--- Tool Called: lore_book_search, Query: %s ---", query)
        docs = retriever.invoke(query)
        # Format the results into a single string for the LLM
        return "\n".join([doc.page_content for doc in docs])

    # Return the raw retriever (for our simple RAG agent)
    # and the fully-formed tool (for our complex LangGraph agent)
    return retriever, lore_book_search

