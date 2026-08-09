"""Schema conformance: fixtures and emitter output validate against the published
JSON Schema (trajectory/trace.schema.json).

The Go types remain the source of truth; this proves the published schema agrees
with the real traces the core consumes and the ones the emitter produces. Skipped
automatically if jsonschema is not installed.
"""

from __future__ import annotations

import json
from datetime import datetime, timezone
from pathlib import Path

import pytest

jsonschema = pytest.importorskip("jsonschema")

from trazo_emitter import TraceRecorder

REPO_ROOT = Path(__file__).resolve().parents[3]
SCHEMA_PATH = REPO_ROOT / "trajectory" / "trace.schema.json"

# Structurally valid traces. Some of these carry behavior that evaluators flag
# (orphan results, tool errors); that is an evaluator concern, not a schema one,
# so they must still pass structural validation here.
VALID_FIXTURES = [
    "agents/langgraph-reference/docs/sample-trace.json",
    "testdata/runs/sample_run.json",
    "testdata/runs/sample_run_ok.json",
    "testdata/sample_run_orphan_result.json",
    "testdata/sample_run_missing_result.json",
    "testdata/ci/clean/triage_clean.json",
    "testdata/ci/failing/tool_error.json",
    "testdata/complex_run.json",
]


@pytest.fixture(scope="module")
def validator() -> "jsonschema.protocols.Validator":
    schema = json.loads(SCHEMA_PATH.read_text(encoding="utf-8"))
    cls = jsonschema.validators.validator_for(schema)
    cls.check_schema(schema)  # the schema itself must be a valid JSON Schema
    return cls(schema, format_checker=cls.FORMAT_CHECKER)


def _dt(second: int) -> datetime:
    return datetime(2026, 8, 6, 10, 0, second, tzinfo=timezone.utc)


@pytest.mark.parametrize("rel_path", VALID_FIXTURES)
def test_valid_fixtures_conform(validator, rel_path: str) -> None:
    trace = json.loads((REPO_ROOT / rel_path).read_text(encoding="utf-8"))
    validator.validate(trace)  # raises ValidationError on failure


def test_invalid_fixture_is_rejected(validator) -> None:
    # invalid_run.json has an empty agent, an unknown "teleport" step type, and a
    # tool_call missing its tool field. Each violates the schema.
    trace = json.loads((REPO_ROOT / "testdata/runs/invalid_run.json").read_text(encoding="utf-8"))
    errors = list(validator.iter_errors(trace))
    assert errors, "expected invalid_run.json to be rejected by the schema"


def test_emitter_output_conforms(validator, tmp_path) -> None:
    rec = TraceRecorder("run-schema-check", "github-triage", "0.0.1", start_time=_dt(0))
    rec.record_node_transition("start", timestamp=_dt(0))
    rec.record_llm_call("gemini-2.5-flash", output="calling a tool", timestamp=_dt(1))
    call = rec.record_tool_call("fetch_issues", input={"repo": "golang/example"}, timestamp=_dt(2))
    rec.record_tool_result(call, output={"count": 0}, timestamp=_dt(3))
    rec.record_node_transition("end", timestamp=_dt(4))
    rec.flush(tmp_path, end_time=_dt(5))

    written = list(tmp_path.glob("*.json"))
    assert written, "emitter wrote no trace file"
    for path in written:
        validator.validate(json.loads(path.read_text(encoding="utf-8")))


@pytest.mark.parametrize(
    "mutate",
    [
        pytest.param(lambda s: s.__setitem__("agent", ""), id="empty-agent"),
        pytest.param(lambda s: s["steps"][0].__setitem__("type", "teleport"), id="unknown-type"),
        pytest.param(lambda s: s["steps"][0].__setitem__("cost", -1), id="negative-cost"),
        pytest.param(lambda s: s["steps"][0].__setitem__("surprise", True), id="unknown-field"),
        pytest.param(lambda s: s["steps"][0].pop("node"), id="node-transition-without-node"),
    ],
)
def test_schema_rejects_mutations(validator, mutate) -> None:
    # Start from a known-good trace, break one invariant, expect rejection.
    trace = json.loads(
        (REPO_ROOT / "agents/langgraph-reference/docs/sample-trace.json").read_text(encoding="utf-8")
    )
    mutate(trace)
    errors = list(validator.iter_errors(trace))
    assert errors, "expected the mutated trace to be rejected"
