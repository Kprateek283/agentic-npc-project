from agents.context_formatter import format_dynamic_context


def base_ctx():
    return {
        "speaker_emotions": {"trust": -1.0, "anger": 1.0},
        "general_mood": {"anger": 0.25},
        "memory_lines": ["You threw a stone at Elara 5 times.", "Someone gave an apple."],
        "quest_step": 2,
        "completion_rate": 0.66,
        "speaker": "alice",
    }


def test_formats_two_labelled_lines():
    out = format_dynamic_context(base_ctx())
    assert out["emotions"] == "Toward you: anger=1.00, trust=-1.00"
    assert out["general_mood"] == "Your general mood: anger=0.25"
    assert out["npc_memories"] == "- You threw a stone at Elara 5 times.\n- Someone gave an apple."
    assert out["current_quest_step"] == 2
    assert out["completion_rate"] == 0.66
    assert out["speaker"] == "alice"


def test_empty_maps_say_nothing_in_particular():
    ctx = {
        "speaker_emotions": {},
        "general_mood": {},
        "memory_lines": [],
        "quest_step": 0,
        "completion_rate": 0.0,
    }
    out = format_dynamic_context(ctx)
    assert out["emotions"] == "Toward you: nothing in particular"
    assert out["general_mood"] == "Your general mood: nothing in particular"
    assert out["npc_memories"] == ""
    assert out["speaker"] == "a stranger you don't know"


def test_omits_emotions_rounding_to_zero():
    ctx = {
        "speaker_emotions": {"joy": 0.001, "anger": 0.75, "fear": -0.004},
        "general_mood": {"sadness": 0.000},
        "memory_lines": [],
    }
    out = format_dynamic_context(ctx)
    assert out["emotions"] == "Toward you: anger=0.75"
    assert out["general_mood"] == "Your general mood: nothing in particular"


def test_sorting_by_emotion_name():
    ctx = {
        "speaker_emotions": {"trust": 0.5, "fear": 0.1, "anger": -0.2, "joy": 0.8},
        "general_mood": {"joy": 0.1, "anger": 0.4},
        "memory_lines": [],
    }
    out = format_dynamic_context(ctx)
    assert out["emotions"] == "Toward you: anger=-0.20, fear=0.10, joy=0.80, trust=0.50"
    assert out["general_mood"] == "Your general mood: anger=0.40, joy=0.10"


def test_zero_completion_passes_through():
    ctx = base_ctx()
    ctx["quest_step"] = 0
    ctx["completion_rate"] = 0.0
    out = format_dynamic_context(ctx)
    assert out["current_quest_step"] == 0
    assert out["completion_rate"] == 0.0
