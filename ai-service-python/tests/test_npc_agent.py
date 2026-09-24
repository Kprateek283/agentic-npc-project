"""The agent wrapper (agents/npc_agent.py) with a stubbed chat model.

A real NpcAgent is built from Elara's persona in gamedata. Only the outside world is replaced:
the lore retriever (no FAISS index, no embeddings at build time), the chat model (a recording
fake shared by the RAG chain, the streaming path and the quest agent) and the question
embeddings (fixed vectors, so cache hits are exact: [3, 4] vs [4, 3] is 0.96, above the
default 0.90 threshold; [0, 1] vs [3, 4] is 0.8, below it).
"""

import os

import pytest
from langchain_core.documents import Document
from langchain_core.language_models.fake_chat_models import GenericFakeChatModel
from langchain_core.messages import AIMessage, AIMessageChunk
from langchain_core.outputs import ChatGenerationChunk
from langchain_core.runnables import RunnableLambda
from pydantic import Field

import agents.graph_builder as graph_builder
import agents.npc_agent as npc_agent
import agents.rag_builder as rag_builder

GAMEDATA = os.path.join(os.path.dirname(__file__), "..", "..", "gamedata", "npcs", "elara")
FALLBACK = "Forgive me, my thoughts wandered for a moment. What was it you needed?"

ANON = {"speaker_emotions": {}, "general_mood": {}, "memory_lines": [], "speaker": ""}
PLAYER = {"speaker_emotions": {}, "general_mood": {}, "memory_lines": [], "speaker": "player1"}

VECTORS = {
    "Who guards the gate?": [3.0, 4.0],
    "Who watches the gate?": [4.0, 3.0],  # 0.96 to the first: a paraphrase
    "Where does silverleaf grow?": [0.0, 1.0],  # 0.8 to the first: a different question
}


class RecordingModel(GenericFakeChatModel):
    """A fake chat model that counts calls and can pretend to support tool binding."""

    calls: list = Field(default_factory=list)

    def _generate(self, messages, stop=None, run_manager=None, **kwargs):
        self.calls.append(messages)
        return super()._generate(messages, stop=stop, run_manager=run_manager, **kwargs)

    def bind_tools(self, tools, **kwargs):
        return self


class FakeEmbeddings:
    def __init__(self):
        self.queries = []

    def embed_query(self, text):
        self.queries.append(text)
        return VECTORS[text]


def not_fallback(text):
    assert "my mind wandered" not in text.lower() and "my thoughts wandered" not in text.lower(), text
    return text


@pytest.fixture
def build(monkeypatch):
    """Returns build(replies) -> (agent, model, embeddings)."""
    for name in ("SEMANTIC_CACHE_ENABLED", "SEMANTIC_CACHE_THRESHOLD", "SEMANTIC_CACHE_TTL", "SEMANTIC_CACHE_MAX_PER_NPC"):
        monkeypatch.delenv(name, raising=False)

    def _build(replies):
        model = RecordingModel(messages=iter(replies))
        emb = FakeEmbeddings()
        retriever = RunnableLambda(lambda q: [Document(page_content="Marcus guards the gate.")])
        monkeypatch.setattr(npc_agent, "create_lore_tool_from_file", lambda path: (retriever, None))
        monkeypatch.setattr(rag_builder, "llm_light", model)
        monkeypatch.setattr(npc_agent, "llm_light", model)
        monkeypatch.setattr(graph_builder, "llm_heavy", model)
        monkeypatch.setattr(npc_agent, "embeddings", emb)
        agent = npc_agent.NpcAgent(
            os.path.join(GAMEDATA, "personality.json"),
            os.path.join(GAMEDATA, "backstory.json"),
            os.path.join(GAMEDATA, "lore.json"),
        )
        return agent, model, emb

    return _build


def test_the_agent_is_built_from_the_persona_with_the_quest_tool_only(build):
    agent, _, _ = build([])
    assert (agent.npc_name, agent.npc_occupation) == ("Elara", "Herbalist")
    assert agent.tool_names == "quest_status"


def test_a_cached_answer_short_circuits_the_model(build):
    agent, model, emb = build([AIMessage(content="Marcus guards the gate.")])
    first = not_fallback(agent.run_rag_agent(ANON, "Who guards the gate?"))
    second = agent.run_rag_agent(ANON, "Who watches the gate?")
    assert first == second == "Marcus guards the gate."
    assert len(model.calls) == 1
    assert emb.queries == ["Who guards the gate?", "Who watches the gate?"]


def test_a_different_question_is_not_served_from_the_cache(build):
    agent, model, _ = build([AIMessage(content="Marcus guards the gate."), AIMessage(content="By the river.")])
    agent.run_rag_agent(ANON, "Who guards the gate?")
    assert agent.run_rag_agent(ANON, "Where does silverleaf grow?") == "By the river."
    assert len(model.calls) == 2


def test_a_named_speaker_is_never_cached_or_even_embedded(build):
    agent, model, emb = build([AIMessage(content="For you, Marcus."), AIMessage(content="Still Marcus.")])
    assert agent.run_rag_agent(PLAYER, "Who guards the gate?") == "For you, Marcus."
    assert agent.run_rag_agent(PLAYER, "Who guards the gate?") == "Still Marcus."
    assert len(model.calls) == 2
    assert emb.queries == []
    assert agent._cache._entries == []


def test_a_speakers_answer_is_not_replayed_to_an_anonymous_caller(build):
    agent, model, _ = build([AIMessage(content="You threw stones at me."), AIMessage(content="Marcus guards the gate.")])
    agent.run_rag_agent({**PLAYER, "memory_lines": ["You threw a stone"]}, "Who guards the gate?")
    assert agent.run_rag_agent(ANON, "Who guards the gate?") == "Marcus guards the gate."
    assert len(model.calls) == 2


def test_streaming_yields_the_pieces_then_stores_the_whole_answer(build):
    agent, model, _ = build([AIMessage(content="Marcus guards the gate.")])
    pieces = list(agent.stream_rag_agent(ANON, "Who guards the gate?"))
    assert len(pieces) > 1
    assert not_fallback("".join(pieces)) == "Marcus guards the gate."
    assert agent.run_rag_agent(ANON, "Who watches the gate?") == "Marcus guards the gate."
    assert len(model.calls) == 1


def test_a_cached_answer_streams_as_one_piece_without_the_model(build):
    agent, model, _ = build([AIMessage(content="Marcus guards the gate.")])
    agent.run_rag_agent(ANON, "Who guards the gate?")
    assert list(agent.stream_rag_agent(ANON, "Who watches the gate?")) == ["Marcus guards the gate."]
    assert len(model.calls) == 1


def test_a_cancelled_stream_does_not_write_to_the_cache(build):
    agent, model, _ = build([AIMessage(content="Marcus guards the gate."), AIMessage(content="Marcus, again.")])
    gen = agent.stream_rag_agent(ANON, "Who guards the gate?")
    assert next(gen) == "Marcus"
    gen.close()  # the client went away mid-answer
    assert agent._cache._entries == []
    assert agent.run_rag_agent(ANON, "Who guards the gate?") == "Marcus, again."
    assert len(model.calls) == 2


class TextlessStreamModel(RecordingModel):
    """Streams one chunk that carries no text, as a thinking-only or metadata-only chunk does."""

    def _stream(self, messages, stop=None, run_manager=None, **kwargs):
        self.calls.append(messages)
        yield ChatGenerationChunk(message=AIMessageChunk(content=""))


def test_a_stream_with_no_text_caches_an_empty_answer_and_replays_it(build, monkeypatch):
    # Pinned, not endorsed (see the results file). A stream with no chunks at all raises inside
    # langchain ("No generation chunks were returned") and caches nothing; chunks that carry no
    # text are what reach the cache as "".
    agent, _, _ = build([])
    model = TextlessStreamModel(messages=iter([]))
    monkeypatch.setattr(npc_agent, "llm_light", model)
    assert list(agent.stream_rag_agent(ANON, "Who guards the gate?")) == []
    assert agent.run_rag_agent(ANON, "Who watches the gate?") == ""
    assert len(model.calls) == 1


def test_the_quest_agent_returns_the_models_words(build):
    agent, model, _ = build([AIMessage(content="How dare you throw that at me!")])
    reply = agent.run_quest_agent(PLAYER, "PLAYER_THREW_STONE")
    assert not_fallback(reply) == "How dare you throw that at me!"
    assert len(model.calls) == 1


def test_the_quest_agent_turns_content_blocks_into_plain_text(build):
    blocks = [{"type": "text", "text": "Thank you, "}, {"type": "text", "text": "truly."}]
    agent, _, _ = build([AIMessage(content=blocks)])
    assert agent.run_quest_agent(PLAYER, "PLAYER_GAVE_GIFT") == "Thank you, truly."


def test_hitting_the_iteration_cap_returns_the_in_character_line(build):
    def endless_tool_calls():
        i = 0
        while True:
            i += 1
            yield AIMessage(content="", tool_calls=[{"name": "quest_status", "args": {}, "id": f"call_{i}"}])

    agent, model, _ = build(endless_tool_calls())
    assert agent.run_quest_agent(PLAYER, "PLAYER_INTERACT") == FALLBACK
    # AGENT_MAX_ITERATIONS is 5: five agent/tool loops, then the final agent turn hits the cap.
    assert len(model.calls) == 6
