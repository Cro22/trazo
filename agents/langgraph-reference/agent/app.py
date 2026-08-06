"""Orchestration: wire the emitter, tools and graph, run one triage, flush trace.

This is the injectable core. Tests call run_triage with a fake model and a fake
issue source; the CLI (__main__) supplies a real Gemini model and GitHubClient.
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime
from pathlib import Path
from typing import Callable, Optional, Union

from langchain_core.language_models import BaseChatModel
from langchain_core.messages import AIMessage, HumanMessage, SystemMessage

from .github import IssueSource
from .graph import DEFAULT_ITERATION_CAP, SYSTEM_PROMPT, build_agent
from .tools import build_tools
from .tracing import TracingCallbackHandler
from trazo_emitter import TraceRecorder

AGENT_NAME = "github-triage"
AGENT_VERSION = "0.0.1"


@dataclass
class TriageResult:
    run_id: str
    report: str
    trace_path: Path
    steps: int


def run_triage(
    repo: str,
    traces_dir: Union[str, Path],
    *,
    llm: BaseChatModel,
    source: IssueSource,
    model: str,
    run_id: str,
    iteration_cap: int = DEFAULT_ITERATION_CAP,
    clock: Optional[Callable[[], datetime]] = None,
) -> TriageResult:
    recorder = TraceRecorder(run_id, AGENT_NAME, AGENT_VERSION, clock=clock)
    recorder.record_node_transition("start")

    tools = build_tools(source)
    agent = build_agent(llm, tools, iteration_cap=iteration_cap)
    handler = TracingCallbackHandler(recorder, model=model)

    config = {
        "configurable": {"thread_id": run_id},
        "callbacks": [handler],
        # Stop the runtime well after our own iteration cap would have ended it.
        "recursion_limit": iteration_cap * 2 + 4,
    }
    initial = {
        "messages": [
            SystemMessage(content=SYSTEM_PROMPT),
            HumanMessage(content=f"Triage the repository {repo}"),
        ],
        "iterations": 0,
    }
    state = agent.invoke(initial, config=config)

    recorder.record_node_transition("end")
    trace_path = recorder.flush(traces_dir)

    return TriageResult(
        run_id=run_id,
        report=_final_report(state),
        trace_path=trace_path,
        steps=len(recorder.steps),
    )


def _final_report(state: dict) -> str:
    for message in reversed(state.get("messages", [])):
        if isinstance(message, AIMessage) and not message.tool_calls and message.content:
            return message.content if isinstance(message.content, str) else str(message.content)
    return ""
