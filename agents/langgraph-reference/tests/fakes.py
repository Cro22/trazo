"""Test doubles: a scripted chat model and a fake issue source.

The scripted model returns a preset sequence of AIMessages regardless of input,
so the graph is driven deterministically with no network or API key.
"""

from __future__ import annotations

from typing import Any, Sequence

from langchain_core.language_models import BaseChatModel
from langchain_core.messages import AIMessage, BaseMessage
from langchain_core.outputs import ChatGeneration, ChatResult

from agent.github import Issue


class ScriptedChatModel(BaseChatModel):
    """Yields the next scripted AIMessage on each call. bind_tools is a no-op."""

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
        if not self.responses:
            message: AIMessage = AIMessage(content="(no more scripted responses)")
        else:
            message = self.responses.pop(0)
        return ChatResult(generations=[ChatGeneration(message=message)])

    @property
    def _llm_type(self) -> str:
        return "scripted"


class FakeIssueSource:
    def __init__(self, issues: list[Issue]) -> None:
        self._issues = issues

    def fetch_issues(self, repo: str, state: str = "open", limit: int = 5) -> list[Issue]:
        return list(self._issues[:limit])


def tool_call_message(name: str, args: dict, call_id: str) -> AIMessage:
    return AIMessage(
        content="",
        tool_calls=[{"name": name, "args": args, "id": call_id, "type": "tool_call"}],
    )
