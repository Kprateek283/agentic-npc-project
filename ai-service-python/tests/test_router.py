"""Router dispatch is the core routing contract: which event type reaches which brain.

Uses a stub agent whose brains are fakes, so no LLM is called.
"""

import agent_manager
import pytest
from router import UnknownAgentError, default_context, route_event


class StubAgent:
    def __init__(self):
        self.calls = []

    def run_rag_agent(self, ctx, text):
        self.calls.append(("rag", text))
        return "rag answer"

    def run_quest_agent(self, ctx, text):
        self.calls.append(("quest", text))
        return "quest answer"


@pytest.fixture
def stub(monkeypatch):
    agent = StubAgent()
    monkeypatch.setattr(agent_manager, "get_agent", lambda key: agent if key == "elara" else None)
    return agent


def test_question_routes_to_rag(stub):
    action, content = route_event("elara", "PLAYER_ASKED_QUESTION", "Who is the mayor?", default_context())
    assert action == "SPEAK"
    assert content == "rag answer"
    assert stub.calls == [("rag", "Who is the mayor?")]


@pytest.mark.parametrize("event_type", [
    "PLAYER_SUBMITTED_QUEST_ITEM",
    "PLAYER_INTERACT",
    "PLAYER_INTERACT_QUEST",
    "PLAYER_GAVE_GIFT",
    "PLAYER_ATTACKED",
])
def test_interaction_events_route_to_quest(stub, event_type):
    action, content = route_event("elara", event_type, "apple", default_context())
    assert action == "SPEAK"
    assert content == "quest answer"
    assert stub.calls[0][0] == "quest"


def test_unknown_event_falls_through_to_greeting(stub):
    action, content = route_event("elara", "PLAYER_SANG", "", default_context())
    assert (action, content) == ("SPEAK", "Greetings.")
    assert stub.calls == []  # no brain invoked on the fallback path


def test_unknown_event_high_anger_says_get_lost(stub):
    ctx = default_context()
    ctx["emotions"]["anger"] = 0.8
    _, content = route_event("elara", "PLAYER_SANG", "", ctx)
    assert content == "Get lost."


def test_unknown_agent_raises(stub):
    with pytest.raises(UnknownAgentError):
        route_event("nobody", "PLAYER_ASKED_QUESTION", "hi", default_context())
