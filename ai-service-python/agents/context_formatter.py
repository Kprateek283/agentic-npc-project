# --- Context Formatter Utility ---

def format_dynamic_context(dynamic_context: dict) -> dict:
    """Helper function to format dynamic context from gRPC."""
    emotions = dynamic_context["emotions"]
    formatted_emotions = f"Joy={emotions.joy:.2f}, Sadness={emotions.sadness:.2f}, Anger={emotions.anger:.2f}, Fear={emotions.fear:.2f}, Trust={emotions.trust:.2f}"

    return {
        "emotions": formatted_emotions,
        "npc_memories": "\n".join([f"- {mem.description}" for mem in dynamic_context['memories']]),
        "current_quest_step": dynamic_context["quest_step"],
        "completion_rate": dynamic_context["completion_rate"],
    }