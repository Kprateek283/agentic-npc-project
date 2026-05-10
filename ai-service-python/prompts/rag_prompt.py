from langchain_core.prompts import PromptTemplate

# This prompt is simple and is ONLY used by the fast RAG agent
# It only contains placeholders for DYNAMIC data.
rag_prompt_template = """
**CONTEXT FROM LORE BOOK:**
{context}

**PLAYER'S QUESTION:**
{question}

**YOUR TASK:**
Based *only* on the context above, answer the player's question.
"""
rag_prompt = PromptTemplate.from_template(rag_prompt_template)
