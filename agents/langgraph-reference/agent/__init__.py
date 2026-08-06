"""LangGraph GitHub-triage reference agent that emits trazo traces."""

import os as _os

# Trazo is the observability layer here (see CLAUDE.md): keep LangSmith tracing,
# which ships transitively with langchain, off. setdefault so an explicit opt-in
# still wins if someone really wants it.
_os.environ.setdefault("LANGSMITH_TRACING", "false")
_os.environ.setdefault("LANGCHAIN_TRACING_V2", "false")

from .app import TriageResult, run_triage
from .graph import DEFAULT_ITERATION_CAP, build_agent
from .tools import build_tools, classify_issue

__all__ = [
    "run_triage",
    "TriageResult",
    "build_agent",
    "build_tools",
    "classify_issue",
    "DEFAULT_ITERATION_CAP",
]
