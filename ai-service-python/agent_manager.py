import os
import glob
from agents.npc_agent import NpcAgent

# --- This is our "Singleton" Registry ---
live_agents = {}
# ----------------------------------------

def load_agents_on_startup():
    """
    Finds all NPC config files, creates a NpcAgent instance for each,
    and stores them in the live_agents dictionary.
    """
    print("--- Loading all NPC agents on startup... ---")

    # We must scan for the personality file, as it's our key
    npc_config_files = glob.glob("gamedata/npcs/*/personality.json")

    if not npc_config_files:
        print("WARNING: No NPC personality files found in gamedata/npcs/. No agents will be loaded.")
        return

    for personality_path in npc_config_files:
        try:
            base_path = os.path.dirname(personality_path)
            backstory_path = os.path.join(base_path, "backstory.json")
            lore_path = os.path.join(base_path, "lore.json")

            if not all(os.path.exists(p) for p in [backstory_path, lore_path]):
                print(f"WARNING: Skipping agent at {base_path}. Missing backstory.json or lore.json.")
                continue

            # The personality_path (e.g., "gamedata/npcs/elara/personality.json")
            # is the unique key we use to find the agent.
            agent = NpcAgent(personality_path, backstory_path, lore_path)
            live_agents[personality_path] = agent

        except Exception as e:
            print(f"CRITICAL ERROR: Failed to load agent from {personality_path}: {e}")

    print(f"--- Successfully loaded {len(live_agents)} NPC agents. ---")


def get_agent(agent_key: str):
    """
    Safely retrieves an agent from the registry.

    The canonical key is the personality path the Go orchestrator sends
    ("gamedata/npcs/elara/personality.json"); exact matches win. As a convenience for
    REST/eval callers, a bare NPC directory name ("elara") also resolves.
    """
    agent = live_agents.get(agent_key)
    if agent is not None:
        return agent
    return live_agents.get(os.path.join("gamedata", "npcs", agent_key, "personality.json"))