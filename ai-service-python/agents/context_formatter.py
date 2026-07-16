# --- Context Formatter Utility ---


def format_dynamic_context(dynamic_context: dict) -> dict:
    """Formats the transport-agnostic dynamic context (see router.py) for prompts."""
    e = dynamic_context["emotions"]
    formatted_emotions = (
        f"Joy={e['joy']:.2f}, Sadness={e['sadness']:.2f}, Anger={e['anger']:.2f}, "
        f"Fear={e['fear']:.2f}, Trust={e['trust']:.2f}"
    )

    return {
        "emotions": formatted_emotions,
        "npc_memories": "\n".join(f"- {description}" for description in dynamic_context["memories"]),
        "current_quest_step": dynamic_context["quest_step"],
        "completion_rate": dynamic_context["completion_rate"],
    }
