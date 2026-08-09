"""Failure-scenario tests (M4): each scenario yields the expected trace shape and
the expected trazo severity when evaluated by the Go runner."""

from __future__ import annotations

import itertools
import json
import shutil
import subprocess
from datetime import datetime, timedelta, timezone
from pathlib import Path

import pytest

from agent.app import run_triage
from agent.scenarios import build_scenario

REPO_ROOT = Path(__file__).resolve().parents[3]


def _clock():
    base = datetime(2026, 8, 6, 10, 0, 0, tzinfo=timezone.utc)
    counter = itertools.count()
    return lambda: base + timedelta(seconds=next(counter))


def _run_scenario(name: str, tmp_path: Path):
    setup = build_scenario(name, "golang/example")
    return run_triage(
        "golang/example",
        tmp_path,
        llm=setup.llm,
        source=setup.source,
        model="scripted",
        run_id=f"scenario-{name}",
        iteration_cap=setup.iteration_cap,
        handler_factory=setup.handler_factory,
        clock=_clock(),
    )


def _steps(result) -> list[dict]:
    return json.loads(result.trace_path.read_text(encoding="utf-8"))["steps"]


def test_tool_error_records_an_errored_result(tmp_path) -> None:
    result = _run_scenario("tool-error", tmp_path)
    results = [s for s in _steps(result) if s["type"] == "tool_result"]
    assert len(results) == 1
    assert "403" in results[0]["error"]


def test_orphan_scenario_drops_the_result(tmp_path) -> None:
    steps = _steps(_run_scenario("orphan-tool-call", tmp_path))
    calls = [s for s in steps if s["type"] == "tool_call"]
    results = [s for s in steps if s["type"] == "tool_result"]
    assert len(calls) == 1 and len(results) == 0  # the result was dropped


def test_runaway_loop_truncates_at_cap(tmp_path) -> None:
    steps = _steps(_run_scenario("runaway-loop", tmp_path))
    llm_calls = [s for s in steps if s["type"] == "llm_call"]
    tool_calls = [s for s in steps if s["type"] == "tool_call"]
    # iteration_cap is 5: five agent turns, four of which execute the tool before
    # the forced stop, giving four identical calls for the loop detector.
    assert len(llm_calls) == 5
    assert len(tool_calls) == 4


@pytest.mark.skipif(shutil.which("go") is None, reason="Go toolchain not available")
@pytest.mark.parametrize(
    "name,judgment",
    [
        ("tool-error", "bad"),
        ("orphan-tool-call", "neutral"),
        ("runaway-loop", "bad"),  # now caught by the loop evaluator
    ],
)
def test_scenarios_produce_expected_severity_in_go(tmp_path, name, judgment) -> None:
    result = _run_scenario(name, tmp_path)
    proc = subprocess.run(
        ["go", "run", "./cmd/trazo", "-dir", str(tmp_path), "-json"],
        cwd=REPO_ROOT, capture_output=True, text=True,
    )
    assert proc.stdout, proc.stderr
    out = json.loads(proc.stdout)
    # Aggregate findings across every evaluator for this run.
    findings = [f for e in out["results"] if e["runId"] == result.run_id for f in e["findings"]]
    assert any(f["judgment"] == judgment for f in findings), findings
