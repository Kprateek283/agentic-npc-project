import logging
from typing import Annotated

from langchain.tools import tool
from langgraph.prebuilt import InjectedState

logger = logging.getLogger(__name__)


@tool
def quest_status(state: Annotated[dict, InjectedState]) -> str:
    """Look up how far the player has progressed in their current quest.

    Use this when the player's progress is relevant to your reply — for example when they
    hand over a quest item, or ask how they are doing.
    """
    step = state.get("current_quest_step", 0)
    rate = state.get("completion_rate", 0.0)
    logger.info("--- Tool Called: quest_status -> step=%s, completion=%s ---", step, rate)
    if not step:
        return "The player has no active quest with you."
    return f"The player is on quest step {step}, with the last task {rate * 100:.0f}% complete."
