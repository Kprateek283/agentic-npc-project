"""Unit tests for the experimental ChatAgy LangChain chat model."""

import json
import subprocess

import pytest
from langchain_core.messages import AIMessage, HumanMessage, SystemMessage

from agy_chat import ChatAgy


def test_successful_call_without_model(monkeypatch: pytest.MonkeyPatch) -> None:
    captured_commands: list[list[str]] = []
    captured_kwargs: list[dict] = []

    def mock_run(cmd: list[str], **kwargs: object) -> subprocess.CompletedProcess[str]:
        captured_commands.append(cmd)
        captured_kwargs.append(kwargs)
        return subprocess.CompletedProcess(
            args=cmd,
            returncode=0,
            stdout=json.dumps({"response": "I am an NPC."}),
            stderr="",
        )

    monkeypatch.setattr(subprocess, "run", mock_run)

    chat = ChatAgy(binary="agy", timeout_s=60)
    messages = [
        SystemMessage(content="You are a blacksmith in Riverwood."),
        HumanMessage(content="What weapons do you sell?"),
    ]
    response = chat.invoke(messages)

    assert isinstance(response, AIMessage)
    assert response.content == "I am an NPC."

    assert len(captured_commands) == 1
    cmd = captured_commands[0]
    assert cmd[0] == "agy"
    assert "-p" in cmd
    prompt_index = cmd.index("-p") + 1
    flattened_prompt = cmd[prompt_index]
    assert "System: You are a blacksmith in Riverwood." in flattened_prompt
    assert "Human: What weapons do you sell?" in flattened_prompt
    assert "--output-format" in cmd
    assert cmd[cmd.index("--output-format") + 1] == "json"
    assert "--print-timeout" in cmd
    assert cmd[cmd.index("--print-timeout") + 1] == "60s"
    assert "--model" not in cmd

    assert captured_kwargs[0].get("capture_output") is True
    assert captured_kwargs[0].get("text") is True
    assert captured_kwargs[0].get("timeout") == 90


def test_successful_call_with_model(monkeypatch: pytest.MonkeyPatch) -> None:
    captured_commands: list[list[str]] = []

    def mock_run(cmd: list[str], **kwargs: object) -> subprocess.CompletedProcess[str]:
        captured_commands.append(cmd)
        return subprocess.CompletedProcess(
            args=cmd,
            returncode=0,
            stdout=json.dumps({"response": "Custom model reply"}),
            stderr="",
        )

    monkeypatch.setattr(subprocess, "run", mock_run)

    chat = ChatAgy(binary="custom-agy", timeout_s=45, model="gemini-3.5-pro")
    response = chat.invoke([HumanMessage(content="Hello!")])

    assert response.content == "Custom model reply"
    assert len(captured_commands) == 1
    cmd = captured_commands[0]
    assert cmd[0] == "custom-agy"
    assert "--model" in cmd
    assert cmd[cmd.index("--model") + 1] == "gemini-3.5-pro"
    assert cmd[cmd.index("--print-timeout") + 1] == "45s"


def test_flattened_prompt_system_before_human(monkeypatch: pytest.MonkeyPatch) -> None:
    captured_prompt: str = ""

    def mock_run(cmd: list[str], **kwargs: object) -> subprocess.CompletedProcess[str]:
        nonlocal captured_prompt
        captured_prompt = cmd[cmd.index("-p") + 1]
        return subprocess.CompletedProcess(
            args=cmd,
            returncode=0,
            stdout=json.dumps({"response": "Lore reply"}),
            stderr="",
        )

    monkeypatch.setattr(subprocess, "run", mock_run)

    chat = ChatAgy()
    messages = [
        SystemMessage(content="Ancient lore of Skyrim."),
        HumanMessage(content="Tell me about the dragons."),
        AIMessage(content="Dragons were ancient creatures."),
        HumanMessage(content="Where did they come from?"),
    ]
    chat.invoke(messages)

    expected_prompt = (
        "System: Ancient lore of Skyrim.\n\n"
        "Human: Tell me about the dragons.\n\n"
        "Assistant: Dragons were ancient creatures.\n\n"
        "Human: Where did they come from?"
    )
    assert captured_prompt == expected_prompt
    assert captured_prompt.index("System: Ancient lore") < captured_prompt.index("Human: Tell me")


def test_nonzero_exit_raises_runtime_error(monkeypatch: pytest.MonkeyPatch) -> None:
    def mock_run(cmd: list[str], **kwargs: object) -> subprocess.CompletedProcess[str]:
        return subprocess.CompletedProcess(
            args=cmd,
            returncode=127,
            stdout="",
            stderr="command not found: agy",
        )

    monkeypatch.setattr(subprocess, "run", mock_run)

    chat = ChatAgy()
    with pytest.raises(RuntimeError, match=r"exit code 127") as exc_info:
        chat.invoke([HumanMessage(content="Hello")])

    assert "command not found: agy" in str(exc_info.value)


def test_unparseable_stdout_raises_runtime_error(monkeypatch: pytest.MonkeyPatch) -> None:
    def mock_run(cmd: list[str], **kwargs: object) -> subprocess.CompletedProcess[str]:
        return subprocess.CompletedProcess(
            args=cmd,
            returncode=0,
            stdout="Error: internal crash or invalid json string",
            stderr="warning message",
        )

    monkeypatch.setattr(subprocess, "run", mock_run)

    chat = ChatAgy()
    with pytest.raises(RuntimeError, match=r"Failed to parse agy JSON output"):
        chat.invoke([HumanMessage(content="Hello")])


def test_timeout_raises_runtime_error(monkeypatch: pytest.MonkeyPatch) -> None:
    def mock_run(cmd: list[str], **kwargs: object) -> subprocess.CompletedProcess[str]:
        raise subprocess.TimeoutExpired(cmd=cmd, timeout=150, stderr="timed out waiting for response")

    monkeypatch.setattr(subprocess, "run", mock_run)

    chat = ChatAgy(timeout_s=120)
    with pytest.raises(RuntimeError, match=r"timed out after 150s"):
        chat.invoke([HumanMessage(content="Hello")])


def test_bind_tools_raises_not_implemented_error() -> None:
    chat = ChatAgy()
    with pytest.raises(NotImplementedError, match=r"does not support tool calling"):
        chat.bind_tools([])


def test_config_with_agy_provider(monkeypatch: pytest.MonkeyPatch) -> None:
    import importlib
    import config

    monkeypatch.setenv("LLM_PROVIDER", "agy")
    monkeypatch.setenv("AGY_BIN", "custom-agy")
    monkeypatch.setenv("AGY_TIMEOUT_S", "45")
    monkeypatch.setenv("AGY_MODEL", "gemini-3.5-flash")

    reloaded_config = importlib.reload(config)

    assert reloaded_config.LLM_PROVIDER == "agy"
    assert isinstance(reloaded_config.llm_light, ChatAgy)
    assert reloaded_config.llm_light.binary == "custom-agy"
    assert reloaded_config.llm_light.timeout_s == 45
    assert reloaded_config.llm_light.model == "gemini-3.5-flash"
    assert reloaded_config.CHAT_MODEL_LIGHT == "agy (gemini-3.5-flash)"
    assert reloaded_config.CHAT_MODEL_HEAVY == reloaded_config.OLLAMA_MODEL_HEAVY

    # Reload back to the provider conftest.py sets; deleting it would fall back to gemini,
    # which needs GEMINI_API_KEY and fails in CI.
    monkeypatch.setenv("LLM_PROVIDER", "ollama")
    monkeypatch.delenv("AGY_BIN", raising=False)
    monkeypatch.delenv("AGY_TIMEOUT_S", raising=False)
    monkeypatch.delenv("AGY_MODEL", raising=False)
    importlib.reload(config)
