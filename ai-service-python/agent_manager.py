import glob
import logging
import os

import config
from agents.npc_agent import NpcAgent

logger = logging.getLogger(__name__)

live_agents = {}


def load_agents_on_startup():
    """
    Finds all NPC config files, creates a NpcAgent instance for each,
    and stores them in the live_agents dictionary.
    """
    logger.info("--- Loading all NPC agents on startup... ---")

    # We must scan for the personality file, as it's our key
    npc_config_files = glob.glob(os.path.join(config.GAMEDATA_DIR, "npcs", "*", "personality.json"))

    if not npc_config_files:
        logger.warning(
            "No NPC personality files found in %s. No agents will be loaded.",
            os.path.join(config.GAMEDATA_DIR, "npcs"),
        )
        return

    for personality_path in npc_config_files:
        try:
            base_path = os.path.dirname(personality_path)
            backstory_path = os.path.join(base_path, "backstory.json")
            lore_path = os.path.join(base_path, "lore.json")

            if not all(os.path.exists(p) for p in [backstory_path, lore_path]):
                logger.warning("Skipping agent at %s. Missing backstory.json or lore.json.", base_path)
                continue

            # The NPC directory name (e.g., "elara")
            # is the unique key we use to find the agent.
            npc_name = os.path.basename(base_path)
            agent = NpcAgent(personality_path, backstory_path, lore_path)
            live_agents[npc_name] = agent

        except Exception as e:
            logger.exception("CRITICAL ERROR: Failed to load agent from %s: %s", personality_path, e)

    logger.info("--- Successfully loaded %d NPC agents. ---", len(live_agents))


def get_agent(agent_key: str):
    """
    Safely retrieves an agent from the registry.

    The canonical key is the NPC directory name (e.g. "elara"). For backward
    compatibility, legacy path strings ending with "personality.json" are
    reduced to their directory name before lookup.
    """
    if agent_key.endswith("personality.json"):
        agent_key = os.path.basename(os.path.dirname(agent_key))
    return live_agents.get(agent_key)