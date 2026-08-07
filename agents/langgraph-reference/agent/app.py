"""Orchestration: wire the emitter, tools and graph, run one triage, flush trace.

This is the injectable core. Tests call run_triage with a fake model and a fake
issue source; the CLI (__main__) supplies a real Gemini model and GitHubClient.
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime
from pathlib import Path
from typing import Callable, Optional, Union

from langchain_core.callbacks import BaseCallbackHandler
from langchain_core.language_models import BaseChatModel
from langchain_core.messages import AIMessage, HumanMessage, SystemMessage

from .github import IssueSource
from .graph import DEFAULT_ITERATION_CAP, SYSTEM_PROMPT, build_agent
from .tools import build_tools
from .tracing import TracingCallbackHandler
from trazo_emitter import TraceRecorder

# Builds the callback handler that records steps. Injectable so scenarios (M4)
# can swap in a handler that drops a result to simulate lost instrumentation.
HandlerFactory = Callable[[TraceRecorder, str], BaseCallbackHandler]


def _default_handler(recorder: TraceRecorder, model: str) -> BaseCallbackHandler:
    return TracingCallbackHandler(recorder, model=model)

AGENT_NAME = "github-triage"


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
    handler_factory: Optional[HandlerFactory] = None,
) -> TriageResult:
    # The trace version is the schema version (SCHEMA_VERSION default), not the
    # agent's own version; the Go core gates compatibility on it.
    recorder = TraceRecorder(run_id, AGENT_NAME, clock=clock)
    recorder.record_node_transition("start")

    tools = build_tools(source)
    agent = build_agent(llm, tools, iteration_cap=iteration_cap)
    handler = (handler_factory or _default_handler)(recorder, model)

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
            return _text_of(message.content)
    return ""


def _text_of(content: object) -> str:
    """Flatten message content to plain text. Newer Gemini returns a list of
    content blocks (dicts with a 'text' field); older/other models return a str."""
    if isinstance(content, str):
        return content
    if isinstance(content, list):
        parts: list[str] = []
        for block in content:
            if isinstance(block, dict) and isinstance(block.get("text"), str):
                parts.append(block["text"])
            elif isinstance(block, str):
                parts.append(block)
        return "\n".join(parts)
    return str(content)
