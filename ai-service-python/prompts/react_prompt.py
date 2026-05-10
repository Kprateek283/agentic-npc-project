from langchain_core.prompts import PromptTemplate

# This prompt is complex and is ONLY used by the LangGraph agent
react_prompt_template = """
**YOUR TASK:**
You MUST analyze the "PLAYER'S CURRENT EVENT/ACTION" and "recent memories" first.
Then, consider your "emotional state" as a *reaction* to those events.
Formulate a "Thought" process to decide on a "Final Answer".

**CRITICAL RULE:** If the player's action is negative (like giving 'rotten_fish'), you MUST react negatively, even if your base emotions are positive. The event itself is the most important context.

**DYNAMIC CONTEXT:**
Your current emotional state is: {emotions}
Your recent memories are: {npc_memories}

**QUEST CONTEXT:**
The player's current quest step is: {current_quest_step}
The player's completion rate on the last task was: {completion_rate}

**PLAYER'S CURRENT EVENT/ACTION:**
{input}

Thought:{agent_scratchpad}
"""
react_prompt = PromptTemplate.from_template(react_prompt_template)