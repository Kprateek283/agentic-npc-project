# --- Context Formatter Utility ---


def _format_emotion_map(emotions: dict) -> str:
    """Format an emotion map as sorted 'k=v.2f' pairs, omitting values rounding to 0.00."""
    parts = []
    for k in sorted(emotions.keys()):
        v = emotions[k]
        formatted = f"{v:.2f}"
        if formatted in ("0.00", "-0.00"):
            continue
        parts.append(f"{k}={formatted}")
    if not parts:
        return "nothing in particular"
    return ", ".join(parts)


def format_dynamic_context(dynamic_context: dict) -> dict:
    """Formats the transport-agnostic dynamic context (see router.py) for prompts."""
    speaker_emotions = dynamic_context.get("speaker_emotions", {})
    general_mood = dynamic_context.get("general_mood", {})
    memory_lines = dynamic_context.get("memory_lines", [])
    speaker = dynamic_context.get("speaker", "")

    formatted_speaker = f"Toward you: {_format_emotion_map(speaker_emotions)}"
    formatted_mood = f"Your general mood: {_format_emotion_map(general_mood)}"
    formatted_memories = "\n".join(f"- {line}" for line in memory_lines)

    return {
        "emotions": formatted_speaker,
        "general_mood": formatted_mood,
        "npc_memories": formatted_memories,
        "current_quest_step": dynamic_context.get("quest_step", 0),
        "completion_rate": dynamic_context.get("completion_rate", 0.0),
        "speaker": speaker or "a stranger you don't know",
    }
