"""Scripted failure scenarios for the evaluation demo (M4).

Each scenario is fully deterministic and offline (scripted LLM, controlled issue
source), so it needs no API key and produces the same trace every run. They map
to trazo findings as follows:

- tool-error:      a tool returns an error the agent then handles -> JudgmentBad.
- orphan-tool-call: a tool result is dropped (lost instrumentation) -> JudgmentNeutral.
- runaway-loop:    the iteration cap truncates a loop -> no ToolCallEvaluator
                   finding today (this is the gap hito 5's loop detector fills).
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Optional
from uuid import UUID

from .app import HandlerFactory
from .github import GitHubError, Issue, IssueSource
from .scripted import ScriptedChatModel, tool_call_message
from .tracing import TracingCallbackHandler
from trazo_emitter import TraceRecorder

SCENARIO_NAMES = ("tool-error", "orphan-tool-call", "runaway-loop")

_CANNED_ISSUES = [
    Issue(number=41, title="Typo in README", body="", labels=[]),
    Issue(number=42, title="Build fails on Windows", body="panic on startup", labels=["bug"]),
]


class _CannedSource:
    """Returns fixed issues, no network."""

    def fetch_issues(self, repo: str, state: str = "open", limit: int = 5) -> list[Issue]:
        return list(_CANNED_ISSUES[:limit])


class _FaultySource:
    """Every fetch fails, to drive the tool-error scenario."""

    def fetch_issues(self, repo: str, state: str = "open", limit: int = 5) -> list[Issue]:
        raise GitHubError("rate limited: 403 Forbidden")


class _DropFirstResultHandler(TracingCallbackHandler):
    """Records tool calls but drops the first tool result, simulating a lost or
    interrupted result so the trace carries an orphan tool_call."""

    def __init__(self, recorder: TraceRecorder, *, model: str) -> None:
        super().__init__(recorder, model=model)
        self._results_seen = 0

    def _drop_first(self, run_id: UUID) -> bool:
        self._results_seen += 1
        if self._results_seen == 1:
            self._pending.pop(run_id, None)
            self._tool_start.pop(run_id, None)
            return True
        return False

    def on_tool_end(self, output: object, *, run_id: UUID, **kwargs: object) -> None:
        if self._drop_first(run_id):
            return
        super().on_tool_end(output, run_id=run_id, **kwargs)

    def on_tool_error(self, error: BaseException, *, run_id: UUID, **kwargs: object) -> None:
        if self._drop_first(run_id):
            return
        super().on_tool_error(error, run_id=run_id, **kwargs)


@dataclass
class ScenarioSetup:
    llm: ScriptedChatModel
    source: IssueSource
    iteration_cap: int
    handler_factory: Optional[HandlerFactory]


def build_scenario(name: str, repo: str) -> ScenarioSetup:
    if name == "tool-error":
        return ScenarioSetup(
            llm=ScriptedChatModel(
                responses=[
                    tool_call_message("fetch_issues", {"repo": repo}, "c1"),
                    _final("Could not fetch issues (tool error); aborting triage."),
                ]
            ),
            source=_FaultySource(),
            iteration_cap=5,
            handler_factory=None,
        )
    if name == "orphan-tool-call":
        return ScenarioSetup(
            llm=ScriptedChatModel(
                responses=[
                    tool_call_message("fetch_issues", {"repo": repo}, "c1"),
                    _final("Fetched issues; producing triage report."),
                ]
            ),
            source=_CannedSource(),
            iteration_cap=5,
            handler_factory=lambda rec, model: _DropFirstResultHandler(rec, model=model),
        )
    if name == "runaway-loop":
        # cap 5 yields 4 identical fetch_issues calls, enough for LoopEvaluator
        # (default threshold 3) to flag the loop the ToolCallEvaluator misses.
        cap = 5
        return ScenarioSetup(
            # Every turn asks for a tool (distinct ids); the cap truncates the loop.
            llm=ScriptedChatModel(
                responses=[
                    tool_call_message("fetch_issues", {"repo": repo}, f"loop{i}")
                    for i in range(cap + 2)
                ],
            ),
            source=_CannedSource(),
            iteration_cap=cap,
            handler_factory=None,
        )
    raise ValueError(f"unknown scenario {name!r}; choose from {SCENARIO_NAMES}")


def _final(text: str):
    from langchain_core.messages import AIMessage

    return AIMessage(content=text)
