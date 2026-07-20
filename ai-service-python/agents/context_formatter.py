# --- Context Formatter Utility ---


def _attribute(description: str, speaker: str) -> str:
    """Rewrite a memory so the NPC can tell who did what relative to the person it is
    speaking with now. Memory descriptions start with the actor's id (see game_handler's
    memoryDesc); when that actor IS the current speaker, replace it with "You" so the NPC
    reacts to them personally. Memories caused by anyone else keep that other id, so the
    NPC treats them as a third party and does not blame the current speaker for them."""
    if speaker and description.startswith(speaker + " "):
        return "You" + description[len(speaker):]
    return description


def format_dynamic_context(dynamic_context: dict) -> dict:
    """Formats the transport-agnostic dynamic context (see router.py) for prompts."""
    e = dynamic_context["emotions"]
    formatted_emotions = (
        f"Joy={e['joy']:.2f}, Sadness={e['sadness']:.2f}, Anger={e['anger']:.2f}, "
        f"Fear={e['fear']:.2f}, Trust={e['trust']:.2f}"
    )

    speaker = dynamic_context.get("speaker", "")
    return {
        "emotions": formatted_emotions,
        "npc_memories": "\n".join(
            f"- {_attribute(description, speaker)}" for description in dynamic_context["memories"]
        ),
        "current_quest_step": dynamic_context["quest_step"],
        "completion_rate": dynamic_context["completion_rate"],
        "speaker": speaker or "a stranger you don't know",
    }
