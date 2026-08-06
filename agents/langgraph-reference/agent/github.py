"""Minimal GitHub REST client using only the standard library.

Kept dependency-free and injectable so the graph can run against a fake in tests
without touching the network.
"""

from __future__ import annotations

import json
import urllib.error
import urllib.request
from dataclasses import dataclass
from typing import Any, Optional, Protocol


@dataclass
class Issue:
    number: int
    title: str
    body: str
    labels: list[str]

    def to_dict(self) -> dict[str, Any]:
        return {
            "number": self.number,
            "title": self.title,
            "labels": self.labels,
        }


class IssueSource(Protocol):
    """The surface the tools depend on. Real client and fakes both satisfy it."""

    def fetch_issues(self, repo: str, state: str = "open", limit: int = 5) -> list[Issue]: ...


class GitHubError(RuntimeError):
    """Raised when the GitHub API call fails. Surfaced as a tool error."""


class GitHubClient:
    """Fetches issues from the public GitHub REST API.

    A token is optional; without one the anonymous rate limit applies, which is
    plenty for a small triage demo.
    """

    def __init__(self, token: Optional[str] = None, *, timeout: float = 10.0) -> None:
        self._token = token
        self._timeout = timeout

    def fetch_issues(self, repo: str, state: str = "open", limit: int = 5) -> list[Issue]:
        if "/" not in repo:
            raise GitHubError(f"repo must be owner/name, got {repo!r}")
        url = f"https://api.github.com/repos/{repo}/issues?state={state}&per_page={limit}"
        req = urllib.request.Request(url, headers=self._headers())
        try:
            with urllib.request.urlopen(req, timeout=self._timeout) as resp:
                payload = json.loads(resp.read().decode("utf-8"))
        except urllib.error.HTTPError as exc:
            raise GitHubError(f"GitHub API {exc.code}: {exc.reason}") from exc
        except urllib.error.URLError as exc:
            raise GitHubError(f"GitHub API unreachable: {exc.reason}") from exc

        issues: list[Issue] = []
        for item in payload:
            # The issues endpoint also returns pull requests; skip them.
            if "pull_request" in item:
                continue
            issues.append(
                Issue(
                    number=item["number"],
                    title=item.get("title", ""),
                    body=item.get("body") or "",
                    labels=[lbl["name"] for lbl in item.get("labels", [])],
                )
            )
        return issues

    def _headers(self) -> dict[str, str]:
        headers = {
            "Accept": "application/vnd.github+json",
            "User-Agent": "trazo-langgraph-reference",
        }
        if self._token:
            headers["Authorization"] = f"Bearer {self._token}"
        return headers
