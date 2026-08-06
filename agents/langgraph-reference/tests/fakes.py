"""Test doubles. The scripted chat model is shipped in agent.scripted and reused
here so tests and the M4 scenarios share one implementation."""

from __future__ import annotations

from agent.github import Issue
from agent.scripted import ScriptedChatModel, tool_call_message

__all__ = ["ScriptedChatModel", "tool_call_message", "FakeIssueSource"]


class FakeIssueSource:
    def __init__(self, issues: list[Issue]) -> None:
        self._issues = issues

    def fetch_issues(self, repo: str, state: str = "open", limit: int = 5) -> list[Issue]:
        return list(self._issues[:limit])
