from typing import TypedDict, Sequence, List, Any
from langchain_core.messages import BaseMessage

# --- LangGraph State Definition ---
# This state now correctly matches our DYNAMIC react_prompt
class LangGraphAgentState(TypedDict):
    input: str
    emotions: str
    npc_memories: str
    current_quest_step: int
    completion_rate: float
    agent_outcome: str
    agent_scratchpad: Sequence[BaseMessage]
    tools: List[Any]
    tool_names: str