"""Experimental LangChain ChatModel provider shelling out to the local Antigravity CLI (`agy`).

This provider is intended for local experiments and evals only. It requires the `agy` CLI
binary and active credentials on the host machine and is not suitable for containerized deployment.
"""

from __future__ import annotations

import json
import subprocess
from typing import Any

from langchain_core.callbacks.manager import CallbackManagerForLLMRun
from langchain_core.language_models.chat_models import BaseChatModel
from langchain_core.messages import AIMessage, BaseMessage, HumanMessage, SystemMessage
from langchain_core.outputs import ChatGeneration, ChatResult


class ChatAgy(BaseChatModel):
    """LangChain chat model that shells out to the local Antigravity (`agy`) CLI."""

    binary: str = "agy"
    timeout_s: int = 120
    model: str | None = None

    @property
    def _llm_type(self) -> str:
        return "agy"

    def _format_message(self, message: BaseMessage) -> str:
        if isinstance(message, SystemMessage):
            role = "System"
        elif isinstance(message, HumanMessage):
            role = "Human"
        elif isinstance(message, AIMessage):
            role = "Assistant"
        else:
            role = message.type.capitalize()
        content = message.content if isinstance(message.content, str) else str(message.content)
        return f"{role}: {content}"

    def _generate(
        self,
        messages: list[BaseMessage],
        stop: list[str] | None = None,
        run_manager: CallbackManagerForLLMRun | None = None,
        **kwargs: Any,
    ) -> ChatResult:
        prompt = "\n\n".join(self._format_message(msg) for msg in messages)
        cmd = [
            self.binary,
            "-p",
            prompt,
            "--output-format",
            "json",
            "--print-timeout",
            f"{self.timeout_s}s",
        ]
        if self.model:
            cmd.extend(["--model", self.model])

        try:
            proc = subprocess.run(
                cmd,
                capture_output=True,
                text=True,
                timeout=self.timeout_s + 30,
            )
        except subprocess.TimeoutExpired as exc:
            stderr_preview = (exc.stderr[:500] if exc.stderr else "").strip()
            raise RuntimeError(
                f"agy process timed out after {self.timeout_s + 30}s (stderr: {stderr_preview!r})"
            ) from exc
        except Exception as exc:
            raise RuntimeError(f"Failed to execute agy binary {self.binary!r}: {exc}") from exc

        stderr_preview = (proc.stderr[:500] if proc.stderr else "").strip()
        if proc.returncode != 0:
            raise RuntimeError(
                f"agy process failed with exit code {proc.returncode}: {stderr_preview}"
            )

        try:
            data = json.loads(proc.stdout)
            response_text = data["response"]
        except Exception as exc:
            raise RuntimeError(
                f"Failed to parse agy JSON output (exit code {proc.returncode}, stderr: {stderr_preview!r}): {exc}"
            ) from exc

        if not isinstance(response_text, str) or not response_text.strip():
            raise RuntimeError(
                f"agy returned no text (response={response_text!r}, stderr: {stderr_preview!r})"
            )

        generation = ChatGeneration(message=AIMessage(content=response_text))
        return ChatResult(generations=[generation])

    def bind_tools(self, *args: Any, **kwargs: Any) -> Any:
        raise NotImplementedError(
            "ChatAgy does not support tool calling because the agy CLI returns plain text. "
            "Use another LLM provider (e.g. Gemini or Ollama) for tool-calling paths."
        )
