"""Unit tests for the trace emitter: serialization must match the schema doc."""

from __future__ import annotations

import json
from datetime import datetime, timezone

from trazo_emitter import ToolCall, TraceRecorder, to_rfc3339


def _dt(second: int) -> datetime:
    return datetime(2026, 8, 6, 10, 0, second, tzinfo=timezone.utc)


def test_rfc3339_uses_z_suffix() -> None:
    assert to_rfc3339(_dt(0)) == "2026-08-06T10:00:00Z"
    # Naive datetime is assumed UTC.
    assert to_rfc3339(datetime(2026, 8, 6, 10, 0, 0)) == "2026-08-06T10:00:00Z"


def test_duration_ms_always_present_even_when_zero() -> None:
    rec = TraceRecorder("run-1", "agent", "0.0.1", start_time=_dt(0))
    rec.record_node_transition("start", timestamp=_dt(0))
    step = rec.to_run(end_time=_dt(1)).steps[0].to_dict()
    assert step["durationMs"] == 0
    assert "durationMs" in step


def test_optional_fields_omitted_when_empty() -> None:
    rec = TraceRecorder("run-1", "agent", "0.0.1", start_time=_dt(0))
    rec.record_tool_call("fetch_issues", timestamp=_dt(1))
    step = rec.steps[0].to_dict()
    # No cost/tokens/output/error/node/llm on a bare tool_call.
    for absent in ("cost", "inputTokens", "outputTokens", "output", "error", "node", "llm"):
        assert absent not in step
    assert step["tool"] == "fetch_issues"
    assert step["type"] == "tool_call"


def test_llm_call_serialization() -> None:
    rec = TraceRecorder("run-1", "agent", "0.0.1", start_time=_dt(0))
    rec.record_llm_call(
        "gemini-2.0-flash",
        input={"system": "s", "user": "u"},
        output="answer",
        cost=0.00004,
        input_tokens=320,
        output_tokens=24,
        duration_ms=900,
        timestamp=_dt(1),
    )
    step = rec.steps[0].to_dict()
    assert step == {
        "type": "llm_call",
        "timestamp": "2026-08-06T10:00:01Z",
        "llm": "gemini-2.0-flash",
        "input": {"system": "s", "user": "u"},
        "output": "answer",
        "cost": 0.00004,
        "inputTokens": 320,
        "outputTokens": 24,
        "durationMs": 900,
    }


def test_tool_call_returns_handle_for_correlation() -> None:
    rec = TraceRecorder("run-1", "agent", "0.0.1", start_time=_dt(0))
    handle = rec.record_tool_call("fetch_issues", timestamp=_dt(1))
    assert isinstance(handle, ToolCall)
    assert handle.tool == "fetch_issues"
    assert handle.step_index == 0
    assert handle.id == "call-1"  # auto-generated
    # Passing the handle to the result reuses the tool name and the id.
    rec.record_tool_result(handle, output={"count": 2}, timestamp=_dt(2))
    assert rec.steps[1].tool == "fetch_issues"
    assert rec.steps[1].type.value == "tool_result"
    assert rec.steps[0].to_dict()["toolCallId"] == "call-1"
    assert rec.steps[1].to_dict()["toolCallId"] == "call-1"


def test_tool_call_ids_are_unique_and_paired() -> None:
    rec = TraceRecorder("run-1", "agent", "0.0.1", start_time=_dt(0))
    a = rec.record_tool_call("search", timestamp=_dt(1))
    b = rec.record_tool_call("search", timestamp=_dt(2))
    assert (a.id, b.id) == ("call-1", "call-2")
    rec.record_tool_result(b, output="rb", timestamp=_dt(3))
    rec.record_tool_result(a, output="ra", timestamp=_dt(4))
    ids = [s.to_dict().get("toolCallId") for s in rec.steps]
    assert ids == ["call-1", "call-2", "call-2", "call-1"]


def test_tool_result_by_bare_name_has_no_id() -> None:
    rec = TraceRecorder("run-1", "agent", "0.0.1", start_time=_dt(0))
    rec.record_tool_result("db", error="boom2", timestamp=_dt(1))
    assert "toolCallId" not in rec.steps[0].to_dict()


def test_tool_result_accepts_bare_name() -> None:
    rec = TraceRecorder("run-1", "agent", "0.0.1", start_time=_dt(0))
    rec.record_tool_result("db", error="boom", timestamp=_dt(1))
    step = rec.steps[0].to_dict()
    assert step["tool"] == "db"
    assert step["error"] == "boom"


def test_end_time_clamped_to_last_step() -> None:
    rec = TraceRecorder("run-1", "agent", "0.0.1", start_time=_dt(0))
    rec.record_node_transition("start", timestamp=_dt(5))
    # Ask for an end_time before the last step; it should be clamped up.
    run = rec.to_run(end_time=_dt(1))
    assert run.end_time == _dt(5)


def test_full_run_shape_and_flush(tmp_path) -> None:
    rec = TraceRecorder("run-triage", "github-triage", "0.0.1", start_time=_dt(0))
    rec.record_node_transition("start", duration_ms=5, timestamp=_dt(0))
    call = rec.record_tool_call("fetch_issues", input={"repo": "x/y"}, timestamp=_dt(1))
    rec.record_tool_result(call, output={"count": 0}, timestamp=_dt(2))
    rec.record_node_transition("end", timestamp=_dt(3))

    path = rec.flush(tmp_path, end_time=_dt(4))
    assert path.name == "run-triage.json"

    data = json.loads(path.read_text(encoding="utf-8"))
    assert data["id"] == "run-triage"
    assert data["agent"] == "github-triage"
    assert data["startTime"] == "2026-08-06T10:00:00Z"
    assert data["endTime"] == "2026-08-06T10:00:04Z"
    assert [s["type"] for s in data["steps"]] == [
        "node_transition",
        "tool_call",
        "tool_result",
        "node_transition",
    ]
