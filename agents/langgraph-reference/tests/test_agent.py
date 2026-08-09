"""End-to-end tests for the triage agent using a scripted LLM (no network)."""

from __future__ import annotations

import itertools
import json
import shutil
import subprocess
from datetime import datetime, timedelta, timezone
from pathlib import Path

import pytest
from langchain_core.messages import AIMessage

from agent.app import run_triage
from agent.github import Issue
from fakes import FakeIssueSource, ScriptedChatModel, tool_call_message

REPO_ROOT = Path(__file__).resolve().parents[3]

ISSUES = [
    Issue(number=41, title="Typo in README", body="", labels=[]),
    Issue(number=42, title="Build fails on Windows", body="panic on startup", labels=["bug"]),
]


def _clock():
    base = datetime(2026, 8, 6, 10, 0, 0, tzinfo=timezone.utc)
    counter = itertools.count()
    return lambda: base + timedelta(seconds=next(counter))


def _script() -> list[AIMessage]:
    # fetch -> classify -> final report. Exercises both tools then a plain answer.
    return [
        tool_call_message("fetch_issues", {"repo": "golang/example"}, "c1"),
        tool_call_message("classify_issue", {"title": "Build fails on Windows"}, "c2"),
        AIMessage(content="Triage: #42 bug (build failure), #41 documentation. 1 bug, 1 doc."),
    ]


def _run(tmp_path: Path):
    return run_triage(
        "golang/example",
        tmp_path,
        llm=ScriptedChatModel(responses=_script()),
        source=FakeIssueSource(ISSUES),
        model="gemini-2.0-flash",
        run_id="triage-test",
        iteration_cap=5,
        clock=_clock(),
    )


def test_agent_emits_expected_step_sequence(tmp_path) -> None:
    result = _run(tmp_path)
    assert result.report.startswith("Triage:")
    assert result.trace_path.name == "triage-test.json"

    data = json.loads(result.trace_path.read_text(encoding="utf-8"))
    types = [s["type"] for s in data["steps"]]
    assert types == [
        "node_transition",  # start
        "llm_call",         # decide -> fetch_issues
        "tool_call",        # fetch_issues
        "tool_result",
        "llm_call",         # decide -> classify_issue
        "tool_call",        # classify_issue
        "tool_result",
        "llm_call",         # final report
        "node_transition",  # end
    ]
    tools_called = [s["tool"] for s in data["steps"] if s["type"] == "tool_call"]
    assert tools_called == ["fetch_issues", "classify_issue"]


@pytest.mark.skipif(shutil.which("go") is None, reason="Go toolchain not available")
def test_agent_trace_evaluates_clean_in_go(tmp_path) -> None:
    result = _run(tmp_path)
    proc = subprocess.run(
        ["go", "run", "./cmd/trazo", "-dir", str(tmp_path), "-json"],
        cwd=REPO_ROOT,
        capture_output=True,
        text=True,
    )
    assert proc.stdout, proc.stderr
    out = json.loads(proc.stdout)
    assert out["errors"] == []
    # Clean means every evaluator (tool_calls, loops, cost_latency, node) is clean.
    findings = [f for e in out["results"] if e["runId"] == result.run_id for f in e["findings"]]
    assert findings == [], findings
