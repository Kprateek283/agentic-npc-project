import os
import time

from langchain_core.messages import HumanMessage
from langgraph.errors import GraphRecursionError

# --- Local Imports ---
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

        print(f"Successfully initialized agent: {self.npc_name} ({self.npc_occupation}) "
              f"[tools: {self.tool_names}]")


    def run_rag_agent(self, dynamic_context: dict, player_question: str) -> str:
        """Runs the fast RAG chain for simple questions."""
        start_time = time.time()

        # The RAG chain now only needs the question.
        # The static prompt is already built into the chain.
        input_dict = {
            "question": player_question
        }
        response = self.rag_chain.invoke(input_dict)

        end_time = time.time()
        print(f"--- RAG Agent execution took: {end_time - start_time:.2f} seconds ---")
        return response

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
            print(f"--- Agent hit the {AGENT_MAX_ITERATIONS}-iteration cap; using fallback line ---")
            return "Forgive me, my thoughts wandered for a moment. What was it you needed?"

        end_time = time.time()
        tool_calls = sum(len(getattr(m, "tool_calls", []) or []) for m in result["messages"])
        print(f"--- LangGraph Agent execution took: {end_time - start_time:.2f} seconds "
              f"({tool_calls} tool call(s), {len(result['messages'])} messages) ---")
        return result["messages"][-1].content
