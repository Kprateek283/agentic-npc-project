from langchain_core.output_parsers import StrOutputParser
from langchain_core.prompts import ChatPromptTemplate
from langchain_core.runnables import RunnablePassthrough
from config import llm_light

def build_rag_chain(static_system_prompt: str, lore_retriever):
    """Builds the simple, fast RAG chain for direct questions."""

    # We combine our static prompt with the dynamic RAG prompt
    full_rag_prompt = ChatPromptTemplate.from_messages([
        ("system", static_system_prompt +
         "\n**Task:** Use the following context from your lore book to answer the player's question concisely.\n**CONTEXT:** {context}"),
        ("human", "{question}"),
    ])

    rag_chain = (
            {
                "context": (lambda x: x["question"]) | lore_retriever,
                "question": lambda x: x["question"],
            }
            | full_rag_prompt
            | llm_light
            | StrOutputParser()
    )
    return rag_chain