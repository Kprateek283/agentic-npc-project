from langchain_core.messages import SystemMessage
from langgraph.prebuilt import create_react_agent

from agents.agent_state import LangGraphAgentState
from config import llm_heavy
from prompts.react_prompt import react_prompt


def build_langgraph_agent(static_system_prompt: str, tools: list):
    """Builds the stateful multi-step ReAct agent for quest events.

    Uses LangGraph's prebuilt ReAct constructor rather than a hand-rolled graph: it already
    provides the agent node, the tool node, the conditional edge between them and the loop
    back, and it accommodates both the custom persona prompt and the dynamic context (via
    the prompt callable and the custom state schema). The iteration cap is applied by the
    caller through the graph's recursion_limit.
    """

    def prompt(state: LangGraphAgentState) -> list:
        # Re-rendered on every LLM call, so the NPC keeps its persona and the current game
        # context in view across tool-calling turns.
        dynamic_block = react_prompt.format(
            speaker=state["speaker"],
            emotions=state["emotions"],
            npc_memories=state["npc_memories"],
            current_quest_step=state["current_quest_step"],
            completion_rate=state["completion_rate"],
        )
        return [SystemMessage(content=static_system_prompt + dynamic_block), *state["messages"]]

    return create_react_agent(
        llm_heavy,
        tools,
        prompt=prompt,
        state_schema=LangGraphAgentState,
    )
