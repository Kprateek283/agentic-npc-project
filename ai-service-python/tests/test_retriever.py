"""Retriever construction, with a deterministic fake embedding — no Ollama, no network.

The fake is a normalised bag-of-words hash: texts sharing distinctive words land close in
vector space, so retrieval is meaningful without a real embedding model.
"""

import json
import math
import re

import pytest
import tools.lore_retriever_tool as lore_mod
from langchain_core.embeddings import Embeddings

DIM = 256


class FakeEmbeddings(Embeddings):
    def _vec(self, text):
        v = [0.0] * DIM
        for word in re.findall(r"[a-z0-9]+", text.lower()):
            v[hash(word) % DIM] += 1.0
        norm = math.sqrt(sum(x * x for x in v)) or 1.0
        return [x / norm for x in v]

    def embed_documents(self, texts):
        return [self._vec(t) for t in texts]

    def embed_query(self, text):
        return self._vec(text)


@pytest.fixture(autouse=True)
def fake_embeddings(monkeypatch):
    monkeypatch.setattr(lore_mod, "embeddings", FakeEmbeddings())
    monkeypatch.setattr(lore_mod, "VECTOR_STORE", "faiss")


def write_lore(tmp_path, facts):
    d = tmp_path / "npcs" / "tester"
    d.mkdir(parents=True)
    path = d / "lore.json"
    path.write_text(json.dumps({"known_facts": facts}))
    return str(path)


def test_retrieves_the_relevant_fact(tmp_path):
    facts = [
        "The mayor of the town is Marcus the magistrate.",
        "Silverleaf grows on the northern riverbank.",
        "A dragon breathes fire upon the distant mountain.",
    ]
    lore_path = write_lore(tmp_path, facts)
    retriever, tool = lore_mod.create_lore_tool_from_file(lore_path)

    docs = retriever.invoke("who is the mayor")
    assert docs, "expected at least one retrieved document"
    assert "mayor" in docs[0].page_content.lower()

    # The @tool wrapper returns joined page content and is callable.
    assert "mayor" in tool.invoke("who is the mayor").lower()


def test_empty_known_facts_returns_none(tmp_path):
    retriever, tool = lore_mod.create_lore_tool_from_file(write_lore(tmp_path, []))
    assert retriever is None
    assert tool is None


def test_missing_file_returns_none():
    retriever, tool = lore_mod.create_lore_tool_from_file("does/not/exist/lore.json")
    assert retriever is None
    assert tool is None
