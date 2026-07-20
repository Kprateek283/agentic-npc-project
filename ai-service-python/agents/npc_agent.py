import logging
import os
import time

from langchain_core.messages import HumanMessage
from langgraph.errors import GraphRecursionError

# --- Local Imports ---
from config import embeddings
from semantic_cache import SemanticCache, cacheable_context
from tools.lore_retriever_tool import create_lore_tool_from_file
from tools.quest_status_tool import quest_status
# --- THIS IS THE FIX: Use relative imports ('.') ---
from .context_formatter import format_dynamic_context
from .prompt_loader import load_static_prompt
from .rag_builder import build_rag_chain
from .graph_builder import build_langgraph_agent
# ---------------------------------------------------

# Hard cap on agent<->tool loops, so a confused model cannot spin forever or burn quota.
# One loop costs two graph super-steps (agent, then tools), plus the final agent turn.
AGENT_MAX_ITERATIONS = int(os.getenv("AGENT_MAX_ITERATIONS", "5"))
_RECURSION_LIMIT = 2 * AGENT_MAX_ITERATIONS + 1

logger = logging.getLogger(__name__)


def _message_text(message) -> str:
    """The spoken text of a message, as a plain string.

    Gemini 3.x returns `content` as a list of content blocks (text plus a thought
    signature), where Ollama returns a plain string. Both transports need a string —
    the protobuf `content` field will not accept a list.
    """
    return str(message.text)


class NpcAgent:
    """
    A single, "live" stateful agent instance.
    This class now acts as a coordinator, loading and holding the
    specialized brains and tools built by other modules.
    """

    def __init__(self, personality_path: str, backstory_path: str, lore_path: str):
        print(f"Initializing new agent from: {personality_path}")

        # 1. Load Static Prompt Data
        (
            self.static_system_prompt,
            self.npc_name,
            self.npc_occupation,
            _
        ) = load_static_prompt(personality_path, backstory_path)

        # 2. Build Tools
        self.lore_retriever, self.lore_tool = create_lore_tool_from_file(lore_path)
        # lore_tool is None when an NPC has no usable lore; the agent keeps its other tools.
        self.tools = [t for t in (self.lore_tool, quest_status) if t is not None]
        self.tool_names = ", ".join(t.name for t in self.tools)

        # 3. Build Brains
        # Pass the prompt and retriever to the builders
        self.rag_chain = build_rag_chain(self.static_system_prompt, self.lore_retriever)
        self.langgraph_chain = build_langgraph_agent(self.static_system_prompt, self.tools)

        # 4. Per-NPC semantic response cache for repeated lore questions (C4).
        self._cache = SemanticCache()

        print(f"Successfully initialized agent: {self.npc_name} ({self.npc_occupation}) "
              f"[tools: {self.tool_names}]")


    def _cache_lookup(self, dynamic_context: dict, question: str):
        """Return (cached_answer_or_None, question_embedding_or_None). The embedding is
        computed only when the context is cacheable, and is reused to store the answer."""
        if not cacheable_context(dynamic_context):
            return None, None
        q_emb = embeddings.embed_query(question)
        return self._cache.get(q_emb), q_emb

    def run_rag_agent(self, dynamic_context: dict, player_question: str) -> str:
        """Runs the fast RAG chain for simple questions (semantic-cached, C4)."""
        start_time = time.time()

        cached, q_emb = self._cache_lookup(dynamic_context, player_question)
        if cached is not None:
            logger.info("rag_cache hit dur_ms=%d", (time.time() - start_time) * 1000)
            return cached

        # The question drives retrieval; the dynamic context conditions tone. The static
        # persona prompt and grounding rules are already built into the chain.
        input_dict = {
            "question": player_question,
            **format_dynamic_context(dynamic_context),
        }
        response = self.rag_chain.invoke(input_dict)

        if q_emb is not None:
            self._cache.put(q_emb, response)
        logger.info("rag_brain dur_ms=%d", (time.time() - start_time) * 1000)
        return response

    def stream_rag_agent(self, dynamic_context: dict, player_question: str):
        """Streaming variant of run_rag_agent (C3): yields answer text deltas as the LLM
        produces them. A semantic-cache hit (C4) yields the whole cached answer as one
        delta — same wire shape, no LLM call."""
        start_time = time.time()

        cached, q_emb = self._cache_lookup(dynamic_context, player_question)
        if cached is not None:
            logger.info("rag_cache_stream hit dur_ms=%d", (time.time() - start_time) * 1000)
            yield cached
            return

        input_dict = {
            "question": player_question,
            **format_dynamic_context(dynamic_context),
        }
        # The chain ends in StrOutputParser, so .stream() yields incremental strings.
        parts = []
        for delta in self.rag_chain.stream(input_dict):
            parts.append(delta)
            yield delta

        if q_emb is not None:
            self._cache.put(q_emb, "".join(parts))
        logger.info("rag_brain_stream dur_ms=%d", (time.time() - start_time) * 1000)

    def run_quest_agent(self, dynamic_context: dict, player_event_description: str) -> str:
        """Runs the multi-step ReAct agent for quest events."""
        start_time = time.time()

        # The scratchpad is the message list; the game context rides alongside it in state,
        # where the prompt renders it and quest_status reads it.
        state = {
            "messages": [HumanMessage(content=player_event_description)],
            **format_dynamic_context(dynamic_context),
        }

        try:
            result = self.langgraph_chain.invoke(state, config={"recursion_limit": _RECURSION_LIMIT})
        except GraphRecursionError:
            logger.warning("agent_brain hit the %d-iteration cap; using fallback line", AGENT_MAX_ITERATIONS)
            return "Forgive me, my thoughts wandered for a moment. What was it you needed?"

        tool_calls = sum(len(getattr(m, "tool_calls", []) or []) for m in result["messages"])
        logger.info("agent_brain dur_ms=%d tool_calls=%d messages=%d",
                    (time.time() - start_time) * 1000, tool_calls, len(result["messages"]))
        return _message_text(result["messages"][-1])
