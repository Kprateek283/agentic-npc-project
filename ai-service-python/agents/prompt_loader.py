import json


def load_static_prompt(personality_path: str, backstory_path: str) -> (str, str, str, str):
    """Loads static files and builds the core system prompt."""
    try:
        with open(personality_path, 'r') as f:
            personality = json.load(f)
        with open(backstory_path, 'r') as f:
            # Use .get("core_facts", []) to safely handle missing keys
            backstory_text = "\n".join(fact["fact"] for fact in json.load(f).get("core_facts", []))

        npc_name = personality.get("name", "Unknown NPC")
        # gamedata authors occupation as either a string or a list of roles
        # (e.g. ["Villager", "Dumb Brother"]); the Go seeder normalises it too.
        occupation = personality.get("occupation", "Villager")
        npc_occupation = ", ".join(occupation) if isinstance(occupation, list) else occupation
        personality_summary = personality.get("summary", "A standard villager.")

        static_system_prompt = f"""
You are a video game NPC. You MUST stay in character at all times.
Your name is: {npc_name}
Your occupation is: {npc_occupation}
Your core personality is: {personality_summary}
Your detailed backstory is:
{backstory_text}

Never invent biographical details about yourself — family members, titles, lineage, or
personal history — that are not stated in your occupation or backstory above. If you are
"the Herbalist", you are the Herbalist, not "the herbalist's daughter". When unsure about a
detail of your own past, stay vague rather than making one up.
"""
        return static_system_prompt, npc_name, npc_occupation, personality_summary

    except Exception as e:
        print(f"CRITICAL ERROR: Failed to load static files from {personality_path}: {e}")
        raise