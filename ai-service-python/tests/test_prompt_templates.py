"""The two prompt templates in prompts/: placeholders, rendering through the real chains, and
the deliberate rules a future edit is most likely to drop.

Rendering goes through the same code the service uses: the RAG path through
rag_builder.build_rag_chain (with a fake retriever), the event path through the prompt callable
graph_builder hands to LangGraph (with a fake chat model that records what it was sent). No
model is called. Rule sentences are compared with whitespace collapsed, so re-wrapping a line
does not fail a test but deleting or rewording the rule does.
"""

import glob
import os
import re

import pytest
from langchain_core.documents import Document
from langchain_core.language_models.fake_chat_models import GenericFakeChatModel
from langchain_core.messages import AIMessage, HumanMessage
from langchain_core.runnables import RunnableLambda

import agents.graph_builder as graph_builder
from agents.context_formatter import format_dynamic_context
from agents.prompt_loader import load_static_prompt
from agents.rag_builder import build_rag_chain
from prompts.rag_prompt import rag_prompt, rag_prompt_template
from prompts.react_prompt import react_prompt, react_prompt_template

GAMEDATA = os.path.join(os.path.dirname(__file__), "..", "..", "gamedata")
NPC_DIRS = sorted(glob.glob(os.path.join(GAMEDATA, "npcs", "*")))
FALLBACK_MARKERS = ("my mind wandered", "my thoughts wandered")

CONTEXT = {
    "speaker_emotions": {"anger": 0.75, "trust": -0.5},
    "general_mood": {"fear": 0.25},
    "memory_lines": ["You triggered PLAYER_THREW_STONE on Elara (5 times)", "Someone gave apple to Elara"],
    "quest_step": 3,
    "completion_rate": 0.5,
    "speaker": "player_sentinel",
}


def flat(text: str) -> str:
    return " ".join(text.split())


def placeholders(template: str) -> set[str]:
    return set(re.findall(r"\{([^{}]*)\}", template))


def test_rag_template_placeholders_are_exactly_what_the_chain_supplies():
    assert set(rag_prompt.input_variables) == {"context", "speaker", "emotions", "general_mood", "npc_memories"}
    assert placeholders(rag_prompt_template) == set(rag_prompt.input_variables)


def test_react_template_placeholders_are_exactly_what_the_agent_state_supplies():
    want = {"speaker", "emotions", "general_mood", "npc_memories", "current_quest_step", "completion_rate"}
    assert set(react_prompt.input_variables) == want
    assert placeholders(react_prompt_template) == want
    # Every placeholder is a key the formatter produces, so the agent state can fill it.
    assert want <= set(format_dynamic_context(CONTEXT))


@pytest.mark.parametrize("template", [rag_prompt_template, react_prompt_template], ids=["rag", "react"])
def test_no_stray_braces(template):
    stripped = re.sub(r"\{[a-z_]+\}", "", template)
    assert "{" not in stripped and "}" not in stripped


def rendered_rag_messages(npc_dir: str, docs: list) -> list:
    static, name, *_ = load_static_prompt(
        os.path.join(npc_dir, "personality.json"), os.path.join(npc_dir, "backstory.json")
    )
    retriever = RunnableLambda(lambda question: docs)
    prompt_chain, _ = build_rag_chain(static, retriever)
    value = prompt_chain.invoke({"question": "Who guards the gate?", **format_dynamic_context(CONTEXT)})
    system, human = value.to_messages()
    # The persona comes first, then the shared RAG block.
    assert system.content.startswith(static + "\n**YOUR TASK:**")
    assert f"Your name is: {name}" in system.content
    return system, human


@pytest.mark.parametrize("npc_dir", NPC_DIRS, ids=[os.path.basename(d) for d in NPC_DIRS])
def test_rag_prompt_renders_for_every_npc_with_every_value_in_place(npc_dir):
    system, human = rendered_rag_messages(npc_dir, [Document(page_content="lore_sentinel: Marcus guards the gate.")])
    text = system.content
    assert human.content == "Who guards the gate?"
    assert "- lore_sentinel: Marcus guards the gate." in text
    assert "You are speaking with: player_sentinel" in text
    assert "Toward you: anger=0.75, trust=-0.50" in text
    assert "Your general mood: fear=0.25" in text
    assert "- You triggered PLAYER_THREW_STONE on Elara (5 times)\n- Someone gave apple to Elara" in text
    assert not re.search(r"\{[a-z_]+\}", text), "an unfilled placeholder reached the model"
    for marker in FALLBACK_MARKERS:
        assert marker not in text.lower()


def test_rag_prompt_with_no_lore_says_the_npc_knows_nothing():
    system, _ = rendered_rag_messages(os.path.join(GAMEDATA, "npcs", "elara"), [])
    assert "**LORE FACTS YOU KNOW:**\n(You know nothing about this.)" in system.content


def test_a_brace_in_persona_text_reaches_the_model_as_written(tmp_path):
    # A "{weapon}" in a backstory is authored text; it used to become a template variable the
    # chain never fills, so every lore question to that NPC raised KeyError.
    (tmp_path / "personality.json").write_text('{"name": "Tess", "occupation": "Smith", "summary": "Gruff {mostly}."}')
    (tmp_path / "backstory.json").write_text('{"core_facts": [{"fact": "Her favourite {weapon} is a hammer."}]}')
    static, *_ = load_static_prompt(str(tmp_path / "personality.json"), str(tmp_path / "backstory.json"))
    prompt_chain, _ = build_rag_chain(static, RunnableLambda(lambda q: []))
    system = prompt_chain.invoke({"question": "Hi", **format_dynamic_context(CONTEXT)}).to_messages()[0].content
    assert "Her favourite {weapon} is a hammer." in system
    assert "Gruff {mostly}." in system
    assert "{{" not in system


def test_react_prompt_reaches_the_model_through_the_agent_with_every_value_in_place(monkeypatch):
    model = GenericFakeChatModel(messages=iter([AIMessage(content="Leave me be.")]))
    seen = []
    original = model._generate

    def recording_generate(messages, *args, **kwargs):
        seen.append(messages)
        return original(messages, *args, **kwargs)

    monkeypatch.setattr(model, "_generate", recording_generate, raising=False)
    monkeypatch.setattr(graph_builder, "llm_heavy", model)
    agent = graph_builder.build_langgraph_agent("STATIC_PERSONA_SENTINEL", [])
    result = agent.invoke({"messages": [HumanMessage(content="PLAYER_THREW_STONE")], **format_dynamic_context(CONTEXT)})

    assert result["messages"][-1].content == "Leave me be."
    (messages,) = seen
    system, human = messages
    text = system.content
    assert text.startswith("STATIC_PERSONA_SENTINEL\n**YOUR TASK:**")
    assert "You are reacting to: player_sentinel" in text
    assert "Toward you: anger=0.75, trust=-0.50" in text
    assert "Your general mood: fear=0.25" in text
    assert "- You triggered PLAYER_THREW_STONE on Elara (5 times)\n- Someone gave apple to Elara" in text
    assert "The player's current quest step is: 3" in text
    assert "The player's completion rate on the last task was: 0.5" in text
    assert not re.search(r"\{[a-z_]+\}", text)
    assert human.content == "PLAYER_THREW_STONE"


BOTH = [rag_prompt_template, react_prompt_template]
RAG = [rag_prompt_template]
REACT = [react_prompt_template]


@pytest.mark.parametrize(
    "templates, sentence",
    [
        # Grounding rules (RAG).
        pytest.param(RAG, "Answer the player's question using ONLY the lore facts provided below.", id="answer only from lore"),
        pytest.param(RAG, "**GROUNDING RULES — these override everything else:**", id="grounding rules override"),
        pytest.param(RAG, "Do not add, infer or embellish names, places, events, or history that are not written there.",
                     id="no embellishment"),
        pytest.param(RAG, "**Never invent a proper name from lore.**", id="never invent a proper name"),
        pytest.param(RAG, "If the lore facts do not answer the question, say so **in character**", id="admit not knowing in character"),
        pytest.param(RAG, 'Never mention "context", "lore", "lore book", "facts provided", searching, retrieval, or '
                          "that you are an AI or language model.", id="never mention the machinery"),
        # The 0.5 threshold for strong feelings (both).
        pytest.param(BOTH, "when any value there is at or above 0.5 in size, your FIRST sentence must show that feeling",
                     id="0.5 threshold, first sentence"),
        pytest.param(BOTH, "Never open with a generic shopkeeper greeting when feelings toward them are at or above 0.5.",
                     id="0.5 threshold, no neutral greeting"),
        # General mood is never the speaker's fault (both).
        pytest.param(BOTH, 'The "Your general mood" line is your overall mood from everyone; it colours tone only and '
                           "must never be blamed on the current speaker.", id="general mood never blamed on the speaker"),
        # You/Someone attribution.
        pytest.param(RAG, "do not accuse the current speaker of another's deed", id="rag: no blame for another's deed"),
        pytest.param(REACT, "do not blame the current person for what another did", id="react: no blame for another's deed"),
        pytest.param(REACT, 'A memory line ending "after apologising" means that person broke an apology',
                     id="react: broken apology marker"),
        # Tool rules (event path).
        pytest.param(REACT, 'Never search for the event name itself (such as "PLAYER_THREW_STONE"); it is not lore.',
                     id="never search the lore book for an event name"),
        pytest.param(REACT, "Never mention your tools, the lore book, or that you looked anything up.", id="never mention tools"),
        pytest.param(REACT, 'Do not narrate your reasoning, do not write "Thought:", and never break character.',
                     id="no Thought: narration"),
        pytest.param(BOTH, "in character.", id="reply in character"),
    ],
)
def test_deliberate_rules_are_still_there(templates, sentence):
    for template in templates:
        assert flat(sentence) in flat(template)


@pytest.mark.parametrize("template", BOTH, ids=["rag", "react"])
def test_the_labels_the_rules_name_match_what_the_formatter_writes(template):
    out = format_dynamic_context(CONTEXT)
    assert out["emotions"].startswith("Toward you: ") and 'The "Toward you" line' in template
    assert out["general_mood"].startswith("Your general mood: ") and 'The "Your general mood" line' in template
    assert 'begin with "You"' in template or 'begins with "You"' in template
