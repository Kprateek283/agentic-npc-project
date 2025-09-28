import time
from langchain_community.document_loaders import TextLoader
from langchain_core.runnables import RunnableLambda
from langchain_text_splitters import RecursiveCharacterTextSplitter
from langchain_community.vectorstores import FAISS
from langchain.prompts import PromptTemplate
from langchain_ollama.chat_models import ChatOllama
from langchain_ollama import OllamaEmbeddings
from langchain_core.output_parsers import StrOutputParser

# --- RAG Setup (This part is correct and stays the same) ---
print("Building the Herbalist's knowledge base (RAG)...")
loader = TextLoader("lore.txt")
documents = loader.load()
text_splitter = RecursiveCharacterTextSplitter(chunk_size=500, chunk_overlap=50)
texts = text_splitter.split_documents(documents)
embeddings = OllamaEmbeddings(model="nomic-embed-text")
vectorstore = FAISS.from_documents(texts, embeddings)
retriever = vectorstore.as_retriever(search_kwargs={"k": 1})
print("Knowledge base built successfully.")

# --- LLM is the same ---
llm = ChatOllama(model="llama3:8b-instruct-q4_K_M")

# --- RAG Prompt Template is the same ---
rag_prompt_template = """
**Your Persona:** You are {npc_name}, a wise and ancient herbalist. Speak in a calm and knowing manner.
**Your Mood:** Your current emotional state is: Joy={joy}, Sadness={sadness}, Anger={anger}, Fear={fear}, Trust={trust}.
**Your Memories:** Here are your recent memories:
{npc_memories}

**Task:** Use the following retrieved context from your lore book to answer the player's question. If the context doesn't contain the answer, say you don't know. Keep your answer concise and focused on the question.

**CONTEXT FROM LORE BOOK:**
{context}

**PLAYER'S QUESTION:**
{question}

**Your Answer:**
"""
rag_prompt = PromptTemplate.from_template(rag_prompt_template)


# This simple function will just print the data that passes through it.
def log_prompt(prompt):
    print("\n--- FINAL PROMPT SENT TO LLM ---\n")
    print(prompt)
    print("\n---------------------------------\n")
    return prompt


# --- THIS IS THE NEW, CORRECT RAG CHAIN ---
# The chain is now designed to accept a dictionary as input.
# It uses lambda functions to correctly route the dictionary keys.
rag_chain = (
        {
            "context": lambda x: retriever.invoke(x["question"]),  # The retriever gets the "question"
            "question": lambda x: x["question"],  # The question is passed through
            "npc_name": lambda x: x["npc_name"],  # The npc_name is passed through
            "joy": lambda x: x["joy"],  # The emotions are passed through
            "sadness": lambda x: x["sadness"],
            "anger": lambda x: x["anger"],
            "fear": lambda x: x["fear"],
            "trust": lambda x: x["trust"],
            "npc_memories": lambda x: x["npc_memories"],
        }
        | rag_prompt
        | RunnableLambda(log_prompt)
        | llm
        | StrOutputParser()
)


# -------------------------------------------


# --- THIS IS THE NEW, CORRECT ask_herbalist FUNCTION ---
def ask_herbalist(npc_context, player_question):
    start_time = time.time()

    formatted_memories = "\n".join([f"- {mem.description}" for mem in npc_context['memories']])
    emotions = npc_context['emotions']

    # We create a single dictionary that contains ALL the variables our chain needs.
    input_dict = {
        "question": player_question,
        "npc_name": npc_context['name'],
        "joy": emotions.joy,
        "sadness": emotions.sadness,
        "anger": emotions.anger,
        "fear": emotions.fear,
        "trust": emotions.trust,
        "npc_memories": formatted_memories,
    }

    # We invoke the chain with this single dictionary.
    response = rag_chain.invoke(input_dict)

    end_time = time.time()
    print(f"--- RAG chain execution took: {end_time - start_time:.2f} seconds ---")
    return response
# ----------------------------------------------------