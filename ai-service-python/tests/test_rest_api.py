"""The REST surface (api/app.py) beyond /health, with fake agents and no model.

Agents are registered in agent_manager.live_agents, the one registry both the REST layer and
the router read, so requests go through the real FastAPI app, the real router and the real
agent lookup. TestClient calls the app in-process; nothing binds a port.
"""

import importlib

import pytest
from fastapi.testclient import TestClient

import agent_manager

app_module = importlib.import_module("api.app")
FALLBACK_MARKERS = ("my mind wandered", "my thoughts wandered")


class FakeAgent:
    def __init__(self, name, occupation, reply="Marcus guards the gate.", error=None):
        self.npc_name, self.npc_occupation = name, occupation
        self.reply, self.error = reply, error
        self.rag_calls, self.quest_calls = [], []

    def run_rag_agent(self, dynamic_context, question):
        self.rag_calls.append((dynamic_context, question))
        if self.error:
            raise self.error
        return self.reply

    def run_quest_agent(self, dynamic_context, description):
        self.quest_calls.append((dynamic_context, description))
        if self.error:
            raise self.error
        return self.reply


@pytest.fixture
def agents(monkeypatch):
    """Returns register(key, agent); the registry starts empty and is restored afterwards."""
    for key in list(agent_manager.live_agents):
        monkeypatch.delitem(agent_manager.live_agents, key)

    def register(key, agent):
        monkeypatch.setitem(agent_manager.live_agents, key, agent)
        return agent

    return register


@pytest.fixture
def client():
    return TestClient(app_module.app)


def test_npcs_lists_what_is_loaded(agents, client):
    agents("elara", FakeAgent("Elara", "Herbalist"))
    agents("kaelen", FakeAgent("Kaelen", "Magistrate, Village Leader"))
    response = client.get("/v1/npcs")
    assert response.status_code == 200
    assert sorted(response.json(), key=lambda n: n["key"]) == [
        {"key": "elara", "npc": "elara", "name": "Elara", "occupation": "Herbalist"},
        {"key": "kaelen", "npc": "kaelen", "name": "Kaelen", "occupation": "Magistrate, Village Leader"},
    ]


def test_npcs_is_empty_when_nothing_is_loaded(agents, client):
    response = client.get("/v1/npcs")
    assert (response.status_code, response.json()) == (200, [])


@pytest.mark.parametrize(
    "path, body",
    [
        pytest.param("/v1/chat", {"npc": "nobody", "question": "Hello?"}, id="chat"),
        pytest.param("/v1/event", {"npc": "nobody", "event_type": "PLAYER_INTERACT"}, id="event"),
    ],
)
def test_an_unknown_npc_is_404(agents, client, path, body):
    agents("elara", FakeAgent("Elara", "Herbalist"))
    response = client.post(path, json=body)
    assert response.status_code == 404
    assert response.json() == {"detail": "No agent loaded for key 'nobody'"}


def test_a_bad_event_type_is_422_and_never_reaches_the_agent(agents, client):
    elara = agents("elara", FakeAgent("Elara", "Herbalist"))
    response = client.post("/v1/event", json={"npc": "elara", "event_type": "PLAYER_DANCED"})
    assert response.status_code == 422
    detail = response.json()["detail"]
    assert detail.startswith("Unknown event_type 'PLAYER_DANCED'. Known: [")
    assert "'PLAYER_THREW_STONE'" in detail and "'PLAYER_ASKED_QUESTION'" in detail
    assert elara.rag_calls == elara.quest_calls == []


def test_the_event_type_is_checked_before_the_npc(agents, client):
    response = client.post("/v1/event", json={"npc": "nobody", "event_type": "PLAYER_DANCED"})
    assert response.status_code == 422


@pytest.mark.parametrize(
    "path, body",
    [
        pytest.param("/v1/event", {"npc": "elara"}, id="event without event_type"),
        pytest.param("/v1/chat", {"npc": "elara"}, id="chat without question"),
        pytest.param("/v1/chat", {"npc": "elara", "question": "Hi", "context": {"quest_step": "three"}},
                     id="context with a non-numeric quest step"),
    ],
)
def test_a_malformed_body_is_422(agents, client, path, body):
    elara = agents("elara", FakeAgent("Elara", "Herbalist"))
    assert client.post(path, json=body).status_code == 422
    assert elara.rag_calls == elara.quest_calls == []


SECRET = "boom at /home/deploy/.config/gemini key=AIza-secret"


@pytest.mark.parametrize(
    "path, body",
    [
        pytest.param("/v1/chat", {"npc": "elara", "question": "Who guards the gate?"}, id="chat"),
        pytest.param("/v1/event", {"npc": "elara", "event_type": "PLAYER_THREW_STONE"}, id="event"),
    ],
)
def test_a_provider_failure_is_502_with_only_the_error_type(agents, client, path, body):
    agents("elara", FakeAgent("Elara", "Herbalist", error=ConnectionError(SECRET)))
    response = client.post(path, json=body)
    assert response.status_code == 502
    assert response.json() == {"detail": "AI provider call failed: ConnectionError"}
    assert "Traceback" not in response.text and "secret" not in response.text and "/home/" not in response.text


@pytest.mark.parametrize("error", [KeyError(SECRET), TypeError(SECRET), AttributeError(SECRET)])
def test_a_bug_on_our_side_is_500_not_a_provider_failure(agents, client, error):
    agents("elara", FakeAgent("Elara", "Herbalist", error=error))
    response = client.post("/v1/chat", json={"npc": "elara", "question": "Who guards the gate?"})
    assert response.status_code == 500
    assert response.json() == {"detail": f"Internal error: {type(error).__name__}"}
    assert "Traceback" not in response.text and "secret" not in response.text and "/home/" not in response.text


def test_chat_answers_through_the_rag_path_with_a_neutral_anonymous_context(agents, client):
    elara = agents("elara", FakeAgent("Elara", "Herbalist"))
    response = client.post("/v1/chat", json={"npc": "elara", "question": "Who guards the gate?"})
    assert response.status_code == 200
    body = response.json()
    for marker in FALLBACK_MARKERS:
        assert marker not in body["content"].lower()
    assert body == {"action_type": "SPEAK", "content": "Marcus guards the gate."}
    assert elara.rag_calls == [(
        {"speaker_emotions": {}, "general_mood": {}, "memory_lines": [], "quest_step": 0,
         "completion_rate": 0.0, "speaker": ""},
        "Who guards the gate?",
    )]


def test_event_goes_to_the_quest_agent_with_the_supplied_context(agents, client):
    elara = agents("elara", FakeAgent("Elara", "Herbalist", reply="An apple? Thank you."))
    context = {"speaker_emotions": {"joy": 0.25, "awe": 0.5}, "general_mood": {"anger": 0.125},
               "memory_lines": ["You gave apple to Elara"], "quest_step": 2, "completion_rate": 0.5}
    response = client.post("/v1/event", json={"npc": "elara", "event_type": "PLAYER_GAVE_GIFT", "text": "apple",
                                              "context": context})
    assert response.json() == {"action_type": "SPEAK", "content": "An apple? Thank you."}
    assert elara.quest_calls == [({**context, "speaker": ""}, "Player event: PLAYER_GAVE_GIFT, Item/Keyword: apple")]


def test_a_legacy_personality_path_key_still_finds_the_agent(agents, client):
    agents("elara", FakeAgent("Elara", "Herbalist"))
    response = client.post("/v1/chat", json={"npc": "../gamedata/npcs/elara/personality.json", "question": "Hi"})
    assert response.status_code == 200


def test_a_speaker_sent_over_rest_is_dropped(agents, client):
    # Pinned, not endorsed (see the results file): REST has no speaker field, so every REST
    # request is anonymous, whatever the caller sends.
    elara = agents("elara", FakeAgent("Elara", "Herbalist"))
    client.post("/v1/chat", json={"npc": "elara", "question": "Hi", "context": {"speaker": "player1"}})
    assert elara.rag_calls[0][0]["speaker"] == ""
