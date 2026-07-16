from operator import itemgetter

from langchain_core.output_parsers import StrOutputParser
from langchain_core.prompts import ChatPromptTemplate

from config import llm_light
from prompts.rag_prompt import rag_prompt


def _format_docs(docs) -> str:
    """Retrieved documents as plain numbered facts, rather than repr'd Document objects."""
    return "\n".join(f"- {d.page_content}" for d in docs) or "(You know nothing about this.)"


def build_rag_chain(static_system_prompt: str, lore_retriever):
    """Builds the fast RAG chain for direct questions.

    The prompt is the shared one from prompts/rag_prompt.py — persona, grounding rules,
    retrieved lore and the live game state, in one place.
    """
    full_rag_prompt = ChatPromptTemplate.from_messages([
        ("system", static_system_prompt + rag_prompt.template),
        ("human", "{question}"),
    ])

    rag_chain = (
            {
                "context": itemgetter("question") | lore_retriever | _format_docs,
                "question": itemgetter("question"),
                "emotions": itemgetter("emotions"),
                "npc_memories": itemgetter("npc_memories"),
            }
            | full_rag_prompt
            | llm_light
            | StrOutputParser()
    )
    return rag_chain
