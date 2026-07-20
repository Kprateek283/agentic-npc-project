# LangSmith tracing (C5)

[LangSmith](https://smith.langchain.com) captures a step-by-step trace of every LLM call —
the RAG chain (prompt → retriever → model → parser) and the multi-step agent loop (each
agent turn, tool call, and tool result). It is the debugging and demo lens for the two
brains. It is **off by default** and requires no code changes — it is driven entirely by
environment variables.

## Enable

Set these (in `ai-service-python/.env` for a direct run, or the root `.env` for Docker):

```bash
LANGCHAIN_TRACING_V2=true
LANGCHAIN_API_KEY=<your-langsmith-api-key>     # from smith.langchain.com → Settings
LANGCHAIN_PROJECT=agentic-npc                  # any project name
# LANGCHAIN_ENDPOINT=https://api.smith.langchain.com   # default; override for self-hosted
```

Restart the AI service. Ask an NPC a question and give one a gift; the runs appear in your
LangSmith project under **agentic-npc**.

## The two traces worth capturing

1. **RAG (Fast Brain)** — send `PLAYER_ASKED_QUESTION`. The trace shows the
   `RunnableSequence`: `ChatPromptTemplate` → `VectorStoreRetriever` (the retrieved lore
   facts) → `ChatOllama`/Gemini → `StrOutputParser`.
2. **Multi-step agent (Complex Brain)** — send `PLAYER_GAVE_GIFT`. The `LangGraph` trace
   shows the ReAct loop: `call_model` → `should_continue` → `tools`
   (`lore_book_search`, `quest_status`) → back to `call_model`, until the final answer.

> **Screenshots for the README are a manual step** — they need a logged-in LangSmith
> account, so they are not committed here. Capture the two traces above and drop them in
> `docs/img/` as `langsmith_rag.png` and `langsmith_agent.png`, then link them from the
> README.

## Verified trace contents

Tracing was verified by pointing the LangSmith SDK at a local capture server
(`LANGCHAIN_ENDPOINT=http://127.0.0.1:...`, dummy key) and running one RAG call and one
gift (agent) call. The exported runs contained, across 5 batched POSTs:

- **run types:** `chain`, `llm`, `prompt`, `retriever`, `parser`, `tool`
- **RAG chain:** `RunnableSequence`, `ChatPromptTemplate`, `VectorStoreRetriever`,
  `_format_docs`, `ChatOllama`, `StrOutputParser`
- **Agent loop:** `LangGraph`, `agent`/`call_model`, `should_continue`, `tools`,
  **`lore_book_search`**, **`quest_status`** (tool executions are captured)

This confirms the traces capture the RAG chain and the full multi-step agent loop with tool
executions — the substance the screenshots illustrate.
