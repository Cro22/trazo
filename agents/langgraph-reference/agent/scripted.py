"""A scripted chat model for deterministic, offline runs.

Used by the failure scenarios (M4) and reused by the tests. It returns a preset
sequence of AIMessages regardless of input, so the graph runs with no network
and no API key. bind_tools is a no-op: the script already encodes the tool calls.
"""

from __future__ import annotations

from typing import Any, Sequence

from langchain_core.language_models import BaseChatModel
from langchain_core.messages import AIMessage, BaseMessage
from langchain_core.outputs import ChatGeneration, ChatResult


class ScriptedChatModel(BaseChatModel):
    """Yields the next scripted AIMessage per call, in order. Once the script is
    exhausted it returns a plain final answer, which ends the graph."""

    responses: list[AIMessage]

    def bind_tools(self, tools: Sequence[Any], **kwargs: Any) -> "ScriptedChatModel":
        return self

    def _generate(
        self,
        messages: list[BaseMessage],
        stop: list[str] | None = None,
        run_manager: Any | None = None,
        **kwargs: Any,
    ) -> ChatResult:
        if self.responses:
            message = self.responses.pop(0)
        else:
            message = AIMessage(content="(no more scripted responses)")
        return ChatResult(generations=[ChatGeneration(message=message)])

    @property
    def _llm_type(self) -> str:
        return "scripted"


def tool_call_message(name: str, args: dict, call_id: str) -> AIMessage:
    """Build an AIMessage that requests a single tool call."""
    return AIMessage(
        content="",
        tool_calls=[{"name": name, "args": args, "id": call_id, "type": "tool_call"}],
    )
