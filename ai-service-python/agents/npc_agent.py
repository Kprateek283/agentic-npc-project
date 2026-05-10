import time

# --- Local Imports ---
from tools.lore_retriever_tool import create_lore_tool_from_file
# --- THIS IS THE FIX: Use relative imports ('.') ---
from .agent_state import LangGraphAgentState
from .context_formatter import format_dynamic_context
from .prompt_loader import load_static_prompt
from .rag_builder import build_rag_chain
from .graph_builder import build_langgraph_agent
# ---------------------------------------------------

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
        self.tools = [self.lore_tool]
        self.tool_names = ", ".join([t.name for t in self.tools if t is not None])

        # 3. Build Brains
        # Pass the prompt and retriever to the builders
        self.rag_chain = build_rag_chain(self.static_system_prompt, self.lore_retriever)
        self.langgraph_chain = build_langgraph_agent(self.static_system_prompt)

        print(f"Successfully initialized agent: {self.npc_name} ({self.npc_occupation})")


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
        """Runs the complex LangGraph agent for quest events."""
        start_time = time.time()

        # Get the standard dynamic context
        context_dict = format_dynamic_context(dynamic_context)

        # Add the other inputs the LangGraph chain needs
        context_dict["input"] = player_event_description
        context_dict["agent_scratchpad"] = []  # Start with an empty scratchpad
        context_dict["tools"] = self.tools
        context_dict["tool_names"] = self.tool_names

        response = self.langgraph_chain.invoke(context_dict)

        end_time = time.time()
        print(f"--- LangGraph Agent execution took: {end_time - start_time:.2f} seconds ---")
        return response["agent_outcome"]