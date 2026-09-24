"""Provider selection in config.py, which does all its work at import time.

Each case sets a clean environment, reloads the module with importlib.reload and inspects
what it built. Constructing the chat and embedding clients opens no connection, so nothing
here touches the network; Qdrant, the one import-time connection, is replaced by a fake.
`load_dotenv` is disabled so a developer's local .env cannot change the outcome. When the file
is done, the module is reloaded with LLM_PROVIDER=ollama, the state conftest.py sets up.
"""

import importlib

import dotenv
import pytest
import qdrant_client
from langchain_google_genai import ChatGoogleGenerativeAI
from langchain_ollama import OllamaEmbeddings
from langchain_ollama.chat_models import ChatOllama

import config
from agy_chat import ChatAgy

CONFIG_VARS = (
    "LLM_PROVIDER", "GEMINI_API_KEY", "GEMINI_MODEL", "GEMINI_MAX_RETRIES", "OLLAMA_HOST",
    "OLLAMA_MODEL_HEAVY", "OLLAMA_MODEL_LIGHT", "EMBEDDING_MODEL", "AGY_BIN", "AGY_TIMEOUT_S",
    "AGY_MODEL", "VECTOR_STORE", "QDRANT_URL", "RETRIEVER_K", "GAMEDATA_DIR",
)


@pytest.fixture(scope="module", autouse=True)
def restore_config_afterwards():
    """Once this module is done, reload config the way conftest.py left it (ollama). Doing
    this once rather than after every case keeps the file fast: each reload builds HTTP
    clients, which costs a noticeable fraction of a second on some machines."""
    yield
    with pytest.MonkeyPatch.context() as mp:
        mp.setattr(dotenv, "load_dotenv", lambda *args, **kwargs: False)
        for name in CONFIG_VARS:
            mp.delenv(name, raising=False)
        mp.setenv("LLM_PROVIDER", "ollama")
        importlib.reload(config)


@pytest.fixture
def load(monkeypatch):
    """Returns a function that reloads config under exactly the given environment. Every
    case calls it first, so no case depends on what the previous one left behind."""
    monkeypatch.setattr(dotenv, "load_dotenv", lambda *args, **kwargs: False)

    def _load(**env):
        for name in CONFIG_VARS:
            monkeypatch.delenv(name, raising=False)
        for name, value in env.items():
            monkeypatch.setenv(name, value)
        return importlib.reload(config)

    return _load


def test_gemini_builds_one_shared_model_with_the_pinned_name(load):
    cfg = load(LLM_PROVIDER="gemini", GEMINI_API_KEY="test-key")
    assert isinstance(cfg.llm_light, ChatGoogleGenerativeAI)
    assert cfg.llm_heavy is cfg.llm_light
    assert cfg.llm_light.model == "gemini-3.5-flash"
    assert cfg.llm_light.temperature == 0.7
    assert cfg.llm_light.max_retries == 1
    assert (cfg.CHAT_MODEL_LIGHT, cfg.CHAT_MODEL_HEAVY) == ("gemini-3.5-flash", "gemini-3.5-flash")


def test_gemini_model_and_retries_can_be_overridden(load):
    cfg = load(LLM_PROVIDER="gemini", GEMINI_API_KEY="test-key", GEMINI_MODEL="gemini-4-pro", GEMINI_MAX_RETRIES="3")
    assert cfg.llm_light.model == "gemini-4-pro"
    assert cfg.llm_light.max_retries == 3
    assert (cfg.CHAT_MODEL_LIGHT, cfg.CHAT_MODEL_HEAVY) == ("gemini-4-pro", "gemini-4-pro")


@pytest.mark.parametrize(
    "env",
    [
        pytest.param({"LLM_PROVIDER": "gemini"}, id="gemini without a key"),
        pytest.param({"LLM_PROVIDER": "gemini", "GEMINI_API_KEY": ""}, id="gemini with an empty key"),
        pytest.param({}, id="no provider at all defaults to gemini and still needs a key"),
    ],
)
def test_gemini_without_a_key_fails_at_import(load, env):
    with pytest.raises(ValueError, match=r"^GEMINI_API_KEY is required when LLM_PROVIDER=gemini$"):
        load(**env)


def test_ollama_builds_two_models_that_default_to_the_same_name(load):
    cfg = load(LLM_PROVIDER="ollama")
    assert isinstance(cfg.llm_light, ChatOllama) and isinstance(cfg.llm_heavy, ChatOllama)
    assert cfg.llm_light is not cfg.llm_heavy
    assert (cfg.llm_light.model, cfg.llm_heavy.model) == ("llama3.1:8b", "llama3.1:8b")
    assert cfg.llm_light.base_url == cfg.llm_heavy.base_url == "http://localhost:11434"
    assert (cfg.CHAT_MODEL_LIGHT, cfg.CHAT_MODEL_HEAVY) == ("llama3.1:8b", "llama3.1:8b")


def test_ollama_light_and_heavy_models_and_host_can_differ(load):
    cfg = load(
        LLM_PROVIDER="ollama",
        OLLAMA_MODEL_LIGHT="llama3.2:3b",
        OLLAMA_MODEL_HEAVY="qwen3:14b",
        OLLAMA_HOST="http://ollama:11434",
    )
    assert (cfg.llm_light.model, cfg.llm_heavy.model) == ("llama3.2:3b", "qwen3:14b")
    assert (cfg.CHAT_MODEL_LIGHT, cfg.CHAT_MODEL_HEAVY) == ("llama3.2:3b", "qwen3:14b")
    assert cfg.llm_heavy.base_url == "http://ollama:11434"


def test_provider_name_is_case_insensitive(load):
    cfg = load(LLM_PROVIDER="OLLAMA")
    assert cfg.LLM_PROVIDER == "ollama"
    assert isinstance(cfg.llm_light, ChatOllama)


def test_agy_uses_the_cli_for_lore_and_ollama_for_tools(load):
    cfg = load(LLM_PROVIDER="agy")
    assert isinstance(cfg.llm_light, ChatAgy)
    assert (cfg.llm_light.binary, cfg.llm_light.timeout_s, cfg.llm_light.model) == ("agy", 120, None)
    assert isinstance(cfg.llm_heavy, ChatOllama)
    assert cfg.llm_heavy.model == "llama3.1:8b"
    assert (cfg.CHAT_MODEL_LIGHT, cfg.CHAT_MODEL_HEAVY) == ("agy", "llama3.1:8b")


def test_agy_reports_its_model_when_one_is_set(load):
    cfg = load(LLM_PROVIDER="agy", AGY_MODEL="gemini-3.5-flash", AGY_BIN="/opt/agy", AGY_TIMEOUT_S="45",
               OLLAMA_MODEL_HEAVY="qwen3:14b")
    assert (cfg.llm_light.binary, cfg.llm_light.timeout_s, cfg.llm_light.model) == ("/opt/agy", 45, "gemini-3.5-flash")
    assert (cfg.CHAT_MODEL_LIGHT, cfg.CHAT_MODEL_HEAVY) == ("agy (gemini-3.5-flash)", "qwen3:14b")


@pytest.mark.parametrize("provider", ["openai", "anthropic", "gemini2"])
def test_an_unknown_provider_fails_at_import(load, provider):
    with pytest.raises(ValueError, match=r"Unsupported LLM_PROVIDER=.*expected 'gemini', 'ollama', or 'agy'"):
        load(LLM_PROVIDER=provider)


def test_surrounding_whitespace_in_a_value_is_ignored(load):
    cfg = load(LLM_PROVIDER=" Ollama\n", OLLAMA_MODEL_HEAVY=" llama3.1:8b ")
    assert cfg.LLM_PROVIDER == "ollama"
    assert cfg.OLLAMA_MODEL_HEAVY == "llama3.1:8b"


@pytest.mark.parametrize(
    "env",
    [
        pytest.param({"LLM_PROVIDER": "gemini", "GEMINI_API_KEY": "test-key"}, id="gemini"),
        pytest.param({"LLM_PROVIDER": "ollama"}, id="ollama"),
        pytest.param({"LLM_PROVIDER": "agy"}, id="agy"),
    ],
)
def test_embeddings_stay_on_ollama_in_every_mode(load, env):
    cfg = load(**env, OLLAMA_HOST="http://ollama:11434")
    assert isinstance(cfg.embeddings, OllamaEmbeddings)
    assert cfg.embeddings.model == "nomic-embed-text"
    assert cfg.embeddings.base_url == "http://ollama:11434"


def test_embedding_model_can_be_overridden(load):
    assert load(LLM_PROVIDER="ollama", EMBEDDING_MODEL="mxbai-embed-large").embeddings.model == "mxbai-embed-large"


def test_a_non_numeric_agy_timeout_fails_at_import(load):
    with pytest.raises(ValueError, match=r"^AGY_TIMEOUT_S must be an integer, got '2m'$"):
        load(LLM_PROVIDER="agy", AGY_TIMEOUT_S="2m")


def test_vector_store_defaults(load):
    cfg = load(LLM_PROVIDER="ollama")
    assert (cfg.VECTOR_STORE, cfg.RETRIEVER_K, cfg.GAMEDATA_DIR) == ("faiss", 3, "../gamedata")


def test_an_unknown_vector_store_fails_at_import(load):
    with pytest.raises(ValueError, match=r"Unsupported VECTOR_STORE='chroma'"):
        load(LLM_PROVIDER="ollama", VECTOR_STORE="chroma")


class FakeQdrant:
    reachable = True
    urls: list = []

    def __init__(self, url, timeout):
        FakeQdrant.urls.append((url, timeout))

    def get_collections(self):
        if not FakeQdrant.reachable:
            raise ConnectionError("connection refused")
        return []


@pytest.mark.parametrize("reachable", [True, False], ids=["reachable qdrant loads", "unreachable qdrant fails loudly"])
def test_qdrant_is_checked_at_import(load, monkeypatch, reachable):
    monkeypatch.setattr(qdrant_client, "QdrantClient", FakeQdrant)
    monkeypatch.setattr(FakeQdrant, "reachable", reachable)
    monkeypatch.setattr(FakeQdrant, "urls", [])
    env = {"LLM_PROVIDER": "ollama", "VECTOR_STORE": "qdrant", "QDRANT_URL": "http://qdrant:6333"}
    if reachable:
        assert load(**env).VECTOR_STORE == "qdrant"
    else:
        with pytest.raises(RuntimeError, match=r"VECTOR_STORE=qdrant but Qdrant is unreachable at http://qdrant:6333"):
            load(**env)
    assert FakeQdrant.urls == [("http://qdrant:6333", 5)]
