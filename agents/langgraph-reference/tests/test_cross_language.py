"""Cross-language verification: a flushed trace must load and evaluate in Go.

Skipped automatically if the Go toolchain is not on PATH. When present, this
runs the real Go runner against files produced by the emitter and asserts the
findings, proving the Python output is schema-correct end to end.
"""

from __future__ import annotations

import json
import shutil
import subprocess
from datetime import datetime, timezone
from pathlib import Path

import pytest

from trazo_emitter import TraceRecorder

REPO_ROOT = Path(__file__).resolve().parents[3]

pytestmark = pytest.mark.skipif(
    shutil.which("go") is None, reason="Go toolchain not available on PATH"
)


def _dt(second: int) -> datetime:
    return datetime(2026, 8, 6, 10, 0, second, tzinfo=timezone.utc)


def _run_trazo(traces_dir: Path) -> dict:
    proc = subprocess.run(
        ["go", "run", "./cmd/trazo", "-dir", str(traces_dir), "-json"],
        cwd=REPO_ROOT,
        capture_output=True,
        text=True,
    )
    assert proc.stdout, f"no stdout from trazo; stderr={proc.stderr}"
    return json.loads(proc.stdout)


def _findings_for(result: dict, run_id: str) -> list[dict]:
    if not any(ev["runId"] == run_id for ev in result["evaluations"]):
        raise AssertionError(f"run {run_id} not found in {result}")
    # Aggregate findings across every evaluator for this run.
    return [f for ev in result["evaluations"] if ev["runId"] == run_id for f in ev["findings"]]


def test_clean_run_evaluates_with_no_findings(tmp_path) -> None:
    rec = TraceRecorder("run-clean", "github-triage", "0.0.1", start_time=_dt(0))
    rec.record_node_transition("start", timestamp=_dt(0))
    call = rec.record_tool_call("fetch_issues", input={"repo": "golang/example"}, timestamp=_dt(1))
    rec.record_tool_result(call, output={"count": 0}, timestamp=_dt(2))
    rec.record_node_transition("end", timestamp=_dt(3))
    rec.flush(tmp_path, end_time=_dt(4))

    result = _run_trazo(tmp_path)
    assert result["fileErrors"] == []
    assert _findings_for(result, "run-clean") == []


def test_tool_error_produces_bad_finding(tmp_path) -> None:
    rec = TraceRecorder("run-toolerr", "github-triage", "0.0.1", start_time=_dt(0))
    call = rec.record_tool_call("fetch_issues", timestamp=_dt(0))
    rec.record_tool_result(call, error="rate limited: 403", timestamp=_dt(1))
    rec.flush(tmp_path, end_time=_dt(2))

    findings = _findings_for(_run_trazo(tmp_path), "run-toolerr")
    assert any(f["judgment"] == "bad" for f in findings), findings


def test_orphan_tool_call_produces_neutral_finding(tmp_path) -> None:
    rec = TraceRecorder("run-orphan", "github-triage", "0.0.1", start_time=_dt(0))
    rec.record_tool_call("fetch_issues", timestamp=_dt(0))  # no result emitted
    rec.flush(tmp_path, end_time=_dt(1))

    findings = _findings_for(_run_trazo(tmp_path), "run-orphan")
    assert any(f["judgment"] == "neutral" for f in findings), findings
