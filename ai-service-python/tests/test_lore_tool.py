"""The lore tool (tools/lore_retriever_tool.py) beyond tests/test_retriever.py: malformed lore,
the per-NPC collection name, k, chunking, the Qdrant branch and the real gamedata.

Embeddings are a deterministic fake (one dimension per distinct word, normalised), the vector
store is in-process FAISS, and the Qdrant branch is served by a fake that records its call.
"""

import glob
import json
import math
import os
import re

import langchain_qdrant
import pytest
from langchain_core.embeddings import Embeddings
from pydantic import ValidationError

import tools.lore_retriever_tool as lore_mod

DIM = 512
GAMEDATA = os.path.join(os.path.dirname(__file__), "..", "..", "gamedata")


class WordEmbeddings(Embeddings):
    """Deterministic: each word maps to a fixed slot by a stable checksum (not hash())."""

    def _vec(self, text):
        v = [0.0] * DIM
        for word in re.findall(r"[a-z0-9]+", text.lower()):
            v[sum(word.encode()) * 31 % DIM] += 1.0
        norm = math.sqrt(sum(x * x for x in v)) or 1.0
        return [x / norm for x in v]

    def embed_documents(self, texts):
        return [self._vec(t) for t in texts]

    def embed_query(self, text):
        return self._vec(text)


@pytest.fixture(autouse=True)
def offline(monkeypatch):
    monkeypatch.setattr(lore_mod, "embeddings", WordEmbeddings())
    monkeypatch.setattr(lore_mod, "VECTOR_STORE", "faiss")
    monkeypatch.setattr(lore_mod, "RETRIEVER_K", 3)


def lore_file(tmp_path, content, npc="tester"):
    d = tmp_path / "npcs" / npc
    d.mkdir(parents=True)
    path = d / "lore.json"
    path.write_text(content if isinstance(content, str) else json.dumps(content))
    return str(path)


@pytest.mark.parametrize(
    "content",
    [
        pytest.param('{"known_facts": ["unterminated', id="truncated JSON"),
        pytest.param("", id="an empty file"),
        pytest.param({"facts": ["Marcus guards the gate."]}, id="no known_facts key"),
        pytest.param({"known_facts": None}, id="known_facts is null"),
        pytest.param({"known_facts": []}, id="known_facts is empty"),
        pytest.param({"known_facts": ["", ""]}, id="only empty facts"),
    ],
)
def test_missing_or_malformed_lore_gives_no_tool(tmp_path, content):
    assert lore_mod.create_lore_tool_from_file(lore_file(tmp_path, content)) == (None, None)


@pytest.mark.parametrize(
    "content, error",
    [
        pytest.param(["Marcus guards the gate."], AttributeError, id="a JSON list instead of an object"),
        pytest.param({"known_facts": [42]}, ValidationError, id="a fact that is not a string"),
    ],
)
def test_lore_of_the_wrong_shape_raises(tmp_path, content, error):
    # Pinned, not endorsed (see the results file): only unreadable JSON is caught; valid JSON of
    # the wrong shape raises out of create_lore_tool_from_file and stops the agent from loading.
    with pytest.raises(error):
        lore_mod.create_lore_tool_from_file(lore_file(tmp_path, content))


def test_a_string_instead_of_a_list_is_split_into_single_characters(tmp_path):
    # Pinned, not endorsed: iterating a string gives one "fact" per character.
    retriever, _ = lore_mod.create_lore_tool_from_file(lore_file(tmp_path, {"known_facts": "gate"}))
    assert sorted(d.page_content for d in retriever.vectorstore.docstore._dict.values()) == ["a", "e", "g", "t"]


@pytest.mark.parametrize(
    "path, want",
    [
        pytest.param("gamedata/npcs/elara/lore.json", "lore_elara", id="relative path"),
        pytest.param("/srv/app/gamedata/npcs/kaelen/lore.json", "lore_kaelen", id="absolute path"),
        pytest.param("../gamedata/npcs/lian/lore.json", "lore_lian", id="parent-relative path, as GAMEDATA_DIR defaults"),
        pytest.param("lore.json", "lore_", id="a bare file name gives an empty NPC part"),
    ],
)
def test_collection_name_is_lore_underscore_directory(path, want):
    assert lore_mod._collection_name(path) == want


def test_the_qdrant_branch_uses_the_per_npc_collection_and_recreates_it(tmp_path, monkeypatch):
    calls = []

    class FakeStore:
        def as_retriever(self, search_kwargs):
            calls.append(("as_retriever", search_kwargs))
            return "qdrant-retriever"

    def from_documents(texts, embeddings, **kwargs):
        calls.append(("from_documents", [t.page_content for t in texts], kwargs))
        return FakeStore()

    monkeypatch.setattr(langchain_qdrant.QdrantVectorStore, "from_documents", staticmethod(from_documents))
    monkeypatch.setattr(lore_mod, "VECTOR_STORE", "qdrant")
    monkeypatch.setattr(lore_mod, "QDRANT_URL", "http://qdrant:6333")
    path = lore_file(tmp_path, {"known_facts": ["Marcus guards the gate."]}, npc="marcus")

    retriever, tool = lore_mod.create_lore_tool_from_file(path)

    assert retriever == "qdrant-retriever" and tool is not None
    assert calls == [
        ("from_documents", ["Marcus guards the gate."],
         {"url": "http://qdrant:6333", "collection_name": "lore_marcus", "force_recreate": True}),
        ("as_retriever", {"k": 3}),
    ]


FACTS = [
    "Marcus the guard watches the northern gate at night.",
    "Silverleaf grows on the riverbank near the mill.",
    "The dragon sleeps beneath the eastern mountain.",
    "Baelor forges swords from shadow iron ore.",
    "The Whispering Caves are full of deceptive echoes.",
]


@pytest.mark.parametrize("k", [1, 2, 4])
def test_the_retriever_returns_k_facts(tmp_path, monkeypatch, k):
    monkeypatch.setattr(lore_mod, "RETRIEVER_K", k)
    retriever, _ = lore_mod.create_lore_tool_from_file(lore_file(tmp_path, {"known_facts": FACTS}))
    assert len(retriever.invoke("gate guard")) == k


def test_the_tool_answers_with_the_best_facts_joined_by_newlines(tmp_path, monkeypatch):
    monkeypatch.setattr(lore_mod, "RETRIEVER_K", 2)
    _, tool = lore_mod.create_lore_tool_from_file(lore_file(tmp_path, {"known_facts": FACTS}))
    assert tool.name == "lore_book_search"
    assert tool.description == "Search for information about game lore, items, people, and places."
    answer = tool.invoke("who watches the northern gate")
    lines = answer.split("\n")
    assert len(lines) == 2
    assert lines[0] == "Marcus the guard watches the northern gate at night."
    assert all(line in FACTS for line in lines)


def test_a_fact_longer_than_500_characters_is_split_into_chunks(tmp_path):
    long_fact = " ".join(f"word{i:03d}" for i in range(150))  # 150 x 7 chars + spaces = 1199 characters
    retriever, _ = lore_mod.create_lore_tool_from_file(lore_file(tmp_path, {"known_facts": [long_fact]}))
    chunks = [d.page_content for d in retriever.vectorstore.docstore._dict.values()]
    assert len(chunks) == 3
    assert all(len(c) <= 500 for c in chunks)
    assert chunks[0].startswith("word000") and long_fact.endswith(chunks[-1][-7:])


NPC_LORE = sorted(glob.glob(os.path.join(GAMEDATA, "npcs", "*", "lore.json")))


@pytest.mark.parametrize("path", NPC_LORE, ids=[os.path.basename(os.path.dirname(p)) for p in NPC_LORE])
def test_every_npcs_real_lore_builds_a_tool(path):
    retriever, tool = lore_mod.create_lore_tool_from_file(path)
    assert retriever is not None and tool is not None
    with open(path) as f:
        facts = json.load(f)["known_facts"]
    stored = sorted(d.page_content for d in retriever.vectorstore.docstore._dict.values())
    assert stored == sorted(facts), "every fact is indexed whole (none is over 500 characters)"
