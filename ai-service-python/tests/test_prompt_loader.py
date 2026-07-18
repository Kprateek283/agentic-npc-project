import json

import pytest
from agents.prompt_loader import load_static_prompt

GAMEDATA = "gamedata/npcs"


def test_loads_string_occupation():
    prompt, name, occupation, summary = load_static_prompt(
        f"{GAMEDATA}/elara/personality.json", f"{GAMEDATA}/elara/backstory.json")
    assert name == "Elara"
    assert occupation == "Herbalist"
    assert "Elara" in prompt and "Herbalist" in prompt


def test_list_occupation_is_joined():
    # kaelen authors occupation as ["Magistrate", "Village Leader"] — must not render as a list.
    _, name, occupation, _ = load_static_prompt(
        f"{GAMEDATA}/kaelen/personality.json", f"{GAMEDATA}/kaelen/backstory.json")
    assert name == "Kaelen"
    assert occupation == "Magistrate, Village Leader"
    assert "[" not in occupation


def test_malformed_json_raises(tmp_path):
    bad = tmp_path / "personality.json"
    bad.write_text("{ this is not valid json ")
    backstory = tmp_path / "backstory.json"
    backstory.write_text(json.dumps({"core_facts": []}))
    with pytest.raises(Exception):
        load_static_prompt(str(bad), str(backstory))
