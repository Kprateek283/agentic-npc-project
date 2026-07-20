from langgraph.prebuilt.chat_agent_executor import AgentState


class LangGraphAgentState(AgentState):
    """State for the ReAct quest agent.

    `messages` (the real scratchpad: the player's event, the LLM's tool calls and the
    tool observations) and `remaining_steps` are inherited from AgentState. The fields
    below carry the per-request game context: they are rendered into the system prompt
    on every LLM call, and the quest_status tool reads them via InjectedState.
    """

    emotions: str
    npc_memories: str
    current_quest_step: int
    completion_rate: float
    speaker: str
