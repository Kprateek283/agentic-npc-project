from agents.context_formatter import format_dynamic_context


def base_ctx():
    return {
        "emotions": {"joy": 0.7, "sadness": 0.1, "anger": 0.2, "fear": 0.0, "trust": 0.6},
        "memories": ["Player greeted Elara.", "Player gave an apple."],
        "quest_step": 2,
        "completion_rate": 0.66,
    }


def test_formats_emotions_and_memories():
    out = format_dynamic_context(base_ctx())
    assert out["emotions"] == "Joy=0.70, Sadness=0.10, Anger=0.20, Fear=0.00, Trust=0.60"
    assert out["npc_memories"] == "- Player greeted Elara.\n- Player gave an apple."
    assert out["current_quest_step"] == 2
    assert out["completion_rate"] == 0.66


def test_empty_memories_yield_empty_block():
    ctx = base_ctx()
    ctx["memories"] = []
    out = format_dynamic_context(ctx)
    assert out["npc_memories"] == ""


def test_zero_completion_passes_through():
    ctx = base_ctx()
    ctx["quest_step"] = 0
    ctx["completion_rate"] = 0.0
    out = format_dynamic_context(ctx)
    assert out["current_quest_step"] == 0
    assert out["completion_rate"] == 0.0


def test_speaker_attribution_marks_own_memories_as_you():
    # The current speaker's own actions become "You ..."; another player's stay third-party,
    # so the NPC blames the attacker, not an innocent current speaker.
    ctx = base_ctx()
    ctx["speaker"] = "alice"
    ctx["memories"] = ["alice triggered PLAYER_ATTACKED on Elara",
                       "bob gave apple to Elara"]
    out = format_dynamic_context(ctx)
    assert out["speaker"] == "alice"
    assert "- You triggered PLAYER_ATTACKED on Elara" in out["npc_memories"]
    assert "- bob gave apple to Elara" in out["npc_memories"]  # someone else -> unchanged


def test_missing_speaker_defaults_and_leaves_memories_intact():
    out = format_dynamic_context(base_ctx())  # no "speaker" key
    assert out["speaker"] == "a stranger you don't know"
    assert out["npc_memories"] == "- Player greeted Elara.\n- Player gave an apple."
