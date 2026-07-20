"""Transport-agnostic event routing.

Both transports (the gRPC servicer and the FastAPI layer) unpack their own request
format into the plain shapes below and call `route_event`. The dispatch rules live
here once so the two transports cannot drift apart.

dynamic_context shape (all keys required; transports supply defaults):
    {
        "emotions": {"joy": float, "sadness": float, "anger": float,
                     "fear": float, "trust": float},
        "memories": [str, ...],        # memory descriptions, newest first
        "quest_step": int,
        "completion_rate": float,
    }
"""

import logging

import agent_manager

logger = logging.getLogger(__name__)

RAG_EVENTS = frozenset({"PLAYER_ASKED_QUESTION"})
AGENT_EVENTS = frozenset({
    "PLAYER_SUBMITTED_QUEST_ITEM",
    "PLAYER_INTERACT",
    "PLAYER_INTERACT_QUEST",
    "PLAYER_GAVE_GIFT",
    "PLAYER_ATTACKED",
})
KNOWN_EVENTS = RAG_EVENTS | AGENT_EVENTS

NEUTRAL_EMOTIONS = {"joy": 0.0, "sadness": 0.0, "anger": 0.0, "fear": 0.0, "trust": 0.0}


class UnknownAgentError(LookupError):
    """Raised when no agent is loaded for the requested key."""


def default_context() -> dict:
    """A neutral dynamic context, for callers that don't supply game state."""
    return {
        "emotions": dict(NEUTRAL_EMOTIONS),
        "memories": [],
        "quest_step": 0,
        "completion_rate": 0.0,
        "speaker": "",
    }


def route_event(agent_key: str, event_type: str, text: str, dynamic_context: dict) -> tuple[str, str]:
    """Route one game event to the right brain. Returns (action_type, content).

    Unknown event types fall through to the non-LLM emotion rules, matching the
    original gRPC behaviour — the Go orchestrator relies on it, and M4 benchmarks
    time this branch as the no-inference path.
    """
    agent = agent_manager.get_agent(agent_key)
    if agent is None:
        raise UnknownAgentError(agent_key)

    if event_type in RAG_EVENTS:
        logger.debug("route rag agent=%s", agent_key)
        return "SPEAK", agent.run_rag_agent(dynamic_context, text)

    if event_type in AGENT_EVENTS:
        logger.debug("route quest agent=%s", agent_key)
        event_description = f"Player event: {event_type}, Item/Keyword: {text}"
        return "SPEAK", agent.run_quest_agent(dynamic_context, event_description)

    logger.debug("route emotion-rules event=%s", event_type)
    if dynamic_context["emotions"]["anger"] > 0.7:
        return "SPEAK", "Get lost."
    return "SPEAK", "Greetings."


def stream_event(agent_key: str, event_type: str, text: str, dynamic_context: dict):
    """Streaming counterpart of route_event. Yields (text_delta, done, action_type).

    RAG events stream real LLM tokens (partials with done=False), then a final empty
    frame with done=True. Every other event has no token stream, so its whole route_event
    answer is emitted as one partial frame followed by the done frame — the wire shape is
    identical, so the Go consumer handles both uniformly.
    """
    agent = agent_manager.get_agent(agent_key)
    if agent is None:
        raise UnknownAgentError(agent_key)

    if event_type in RAG_EVENTS:
        logger.debug("stream rag agent=%s", agent_key)
        for delta in agent.stream_rag_agent(dynamic_context, text):
            yield delta, False, ""
        yield "", True, "SPEAK"
        return

    action_type, content = route_event(agent_key, event_type, text, dynamic_context)
    yield content, False, action_type
    yield "", True, action_type
