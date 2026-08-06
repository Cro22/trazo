"""LangChain tools for the triage agent.

Two tools, both cheap and deterministic on our side:
- fetch_issues: pulls open issues from a repo via the injected IssueSource.
- classify_issue: keyword-based label, no LLM call, so classification cost is zero
  and the trace stays reproducible.
"""

from __future__ import annotations

from typing import Any

from langchain_core.tools import StructuredTool

from .github import IssueSource

# Ordered: first matching category wins.
_KEYWORDS: list[tuple[str, tuple[str, ...]]] = [
    ("bug", ("bug", "crash", "error", "fail", "broken", "regression", "panic")),
    ("documentation", ("doc", "docs", "readme", "typo", "comment", "example")),
    ("feature", ("feature", "add ", "support", "request", "enhancement", "proposal")),
    ("question", ("question", "how do", "how to", "why", "help")),
]


def classify_issue(title: str, body: str = "") -> str:
    """Classify an issue into bug|documentation|feature|question|other by keyword."""
    text = f"{title}\n{body}".lower()
    for label, needles in _KEYWORDS:
        if any(n in text for n in needles):
            return label
    return "other"


def build_tools(source: IssueSource) -> list[StructuredTool]:
    """Build the tool set bound to a concrete issue source."""

    def fetch_issues(repo: str, state: str = "open", limit: int = 5) -> dict[str, Any]:
        """Fetch open issues for a GitHub repo (owner/name). Returns count and issues."""
        issues = source.fetch_issues(repo, state=state, limit=limit)
        return {"count": len(issues), "issues": [i.to_dict() for i in issues]}

    def classify(title: str, body: str = "") -> str:
        """Classify a single issue by its title (and optional body)."""
        return classify_issue(title, body)

    return [
        StructuredTool.from_function(
            func=fetch_issues,
            name="fetch_issues",
            description="Fetch open issues for a GitHub repository given as owner/name.",
        ),
        StructuredTool.from_function(
            func=classify,
            name="classify_issue",
            description="Classify one issue into bug, documentation, feature, question or other.",
        ),
    ]
