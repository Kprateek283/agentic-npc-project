import agent_manager
import pytest


@pytest.mark.parametrize("key", [
    "elara",
    "gamedata/npcs/elara/personality.json",
    "/app/gamedata/npcs/elara/personality.json",
    "../gamedata/npcs/elara/personality.json",
])
def test_get_agent_resolves_key_forms(monkeypatch, key):
    stand_in = object()
    monkeypatch.setattr(agent_manager, "live_agents", {"elara": stand_in})
    assert agent_manager.get_agent(key) is stand_in


def test_get_agent_unknown_returns_none(monkeypatch):
    stand_in = object()
    monkeypatch.setattr(agent_manager, "live_agents", {"elara": stand_in})
    assert agent_manager.get_agent("unknown") is None
