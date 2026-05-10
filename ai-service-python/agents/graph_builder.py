from langchain_core.output_parsers import StrOutputParser
from langgraph.graph import StateGraph, END
from langchain_core.prompts import ChatPromptTemplate, MessagesPlaceholder

from config import llm_heavy
from prompts.react_prompt import react_prompt  # Our new, DYNAMIC quest prompt
from agents.agent_state import LangGraphAgentState

def build_langgraph_agent(static_system_prompt: str):
    """Builds the complex, stateful LangGraph agent for quests."""

    # We combine our STATIC prompt and the DYNAMIC react_prompt
    quest_prompt = ChatPromptTemplate.from_messages([
        ("system", static_system_prompt +
         "\n" + react_prompt.template),  # react_prompt is our DYNAMIC prompt
        ("human", "{input}"),
        MessagesPlaceholder(variable_name="agent_scratchpad"),
    ])

    # This is an inner function, defined *inside* build_langgraph_agent
    def simple_quest_node(state: LangGraphAgentState):
        print("---NODE: LangGraph (Simple) is processing quest step...---")
        # It closes over the 'quest_prompt' variable from the outer scope
        chain = quest_prompt | llm_heavy | StrOutputParser()
        response = chain.invoke(state)
        return {"agent_outcome": response}

    workflow = StateGraph(LangGraphAgentState)
    workflow.add_node("thinker", simple_quest_node)
    workflow.set_entry_point("thinker")
    workflow.add_edge("thinker", END)

    return workflow.compile()