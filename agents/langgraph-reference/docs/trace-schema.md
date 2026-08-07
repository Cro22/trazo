# Trazo trace schema (Hito 4, M1)

This document describes the JSON trace format that the Go core consumes. It is a
description of the Go code, not a new contract. The source of truth is:

- `trajectory/steps.go` — `Run`, `Step`, `StepType` and `LoadRun`.
- `trajectory/validate.go` — `Run.Validate`, the structural acceptance gate.
- `evaluator/toolcalls.go` — `ToolCallEvaluator`, which defines how tool calls
  and results are paired.

If any statement here disagrees with the Go code, the Go code wins. Verified
against the repo at commit level of branch `feature/0.0.1`.

## Machine-readable schema

A JSON Schema (draft 2020-12) mirrors this document at
[`trajectory/trace.schema.json`](../../../trajectory/trace.schema.json). It is
derived from the Go types, not a competing source of truth: a Go test
(`trajectory/schema_test.go`) fails the build if the schema's step-type `enum` or
per-type required fields drift from the `trajectory` constants, and a Python test
(`tests/test_schema.py`) validates every committed fixture and the emitter output
against it.

Two intentional gaps between the schema and `Run.Validate`:

- The schema is **stricter** on unknown fields: it sets `additionalProperties:
  false`, while the Go loader ignores unknown fields. The schema defines the
  intended contract; the loader is lenient.
- The schema is **weaker** on cross-field temporal invariants: `endTime` not
  before `startTime`, monotonic step timestamps, and steps within
  `[startTime, endTime]` cannot be expressed in JSON Schema and are enforced only
  by `Run.Validate` in Go. Passing the schema does not exempt a trace from
  `Run.Validate`.

Validate a file against it with any draft 2020-12 validator, e.g. from the
Python side:

```bash
python -c "import json,jsonschema; s=json.load(open('trajectory/trace.schema.json')); jsonschema.validate(json.load(open('agents/langgraph-reference/docs/sample-trace.json')), s)"
```

## File layout

- One run per file. One JSON object at the top level.
- The runner (`runner/runner.go`) reads a directory and processes every file
  whose extension is `.json`. Non-`.json` files and subdirectories are skipped,
  so this doc (`.md`) can live next to `sample-trace.json` without interfering.
- Encoding: UTF-8, no BOM.

## Run envelope

The top-level object deserializes into `trajectory.Run`.

| JSON field  | Go type     | Required | Notes |
|-------------|-------------|----------|-------|
| `id`        | string      | yes      | Non-empty. Used as `runId` in evaluator output. |
| `agent`     | string      | yes      | Non-empty. Logical agent name. |
| `version`   | string      | no*      | Not checked by `Validate`, but present in every fixture. Treat as required by convention. |
| `startTime` | RFC3339 time| yes      | Must be non-zero. |
| `endTime`   | RFC3339 time| yes      | Must be non-zero and not before `startTime`. |
| `steps`     | array<Step> | yes**    | May be empty and still pass `Validate`, but a run with no steps has nothing to evaluate. |

\* Not enforced by `Validate`; include it anyway.
\** An absent `steps` deserializes to an empty slice; it passes validation but is
degenerate.

Timestamps are Go `time.Time`, so any RFC3339 string Go's JSON decoder accepts is
valid. Use UTC with a `Z` suffix, as the fixtures do. A "zero" time (missing or
`0001-01-01T00:00:00Z`) fails validation.

## Step object

Each element of `steps` deserializes into `trajectory.Step`.

| JSON field     | Go type           | Emitted when            | Notes |
|----------------|-------------------|-------------------------|-------|
| `type`         | string (StepType) | always                  | One of the four types below. Unknown values fail validation. |
| `timestamp`    | RFC3339 time      | always                  | Must be non-zero for every step. |
| `llm`          | string            | `type == llm_call`      | Required for `llm_call`; omit otherwise. |
| `tool`         | string            | `tool_call`/`tool_result` | Required for both; the fallback pairing key (see below). |
| `toolCallId`   | string            | optional                | Correlates a `tool_result` with its `tool_call`. Preferred over the tool name when present. |
| `node`         | string            | `type == node_transition` | Required for `node_transition`. |
| `input`        | raw JSON          | optional                | Any JSON value (object, array, string, number). Payload is opaque to the core. |
| `output`       | raw JSON          | optional                | Any JSON value. In fixtures it appears as an object, an array, and a bare string. |
| `cost`         | number (float64)  | optional                | Omitted when zero. |
| `inputTokens`  | integer           | optional                | Omitted when zero. |
| `outputTokens` | integer           | optional                | Omitted when zero. |
| `durationMs`   | integer (int64)   | always serialized       | Field has no `omitempty`, so `0` is written explicitly. Emit it on every step. |
| `error`        | string            | optional                | Present means the step failed. On a `tool_result` it drives a `bad` finding. |

`input` and `output` are `json.RawMessage`: the core stores them verbatim and
never parses them. Evaluators only look at typed fields (`type`, `tool`, `error`,
timestamps, tokens, cost). So payloads can be any shape without breaking loading
or evaluation.

### StepType values

`type` must be exactly one of (`trajectory/steps.go`):

- `llm_call` — a model invocation. Requires `llm`. Carries `input`, `output`,
  `cost`, `inputTokens`, `outputTokens`.
- `tool_call` — the agent invokes a tool. Requires `tool`. Carries `input`.
- `tool_result` — the result of a tool invocation. Requires `tool`. Carries
  `output` or `error`.
- `node_transition` — the graph moves to a node. Requires `node`. Used to mark
  control flow (`start`, a router node, `end`).

Any other value (e.g. `teleport` in `testdata/runs/invalid_run.json`) is a
validation error: `step N: unknown step type "..."`.

## Validation rules (the acceptance gate)

`Run.Validate` collects every violation and joins them (via `errors.Join`) so a
bad file reports all problems at once. It checks structure only; it does not
judge agent behavior. Rules:

1. `id` non-empty, `agent` non-empty.
2. `startTime` and `endTime` non-zero; `endTime` not before `startTime`.
3. Every step `timestamp` non-zero.
4. Per-type required field present: `llm_call`->`llm`, `tool_call`/`tool_result`
   ->`tool`, `node_transition`->`node`.
5. `type` is one of the four known values.

The runner (`runner/runner.go`) treats a file as a `fileError` if it cannot be
read, cannot be unmarshaled, or fails `Validate`. Such files are skipped for
evaluation and reported separately.

## Tool call / result pairing (what the emitter must respect)

`ToolCallEvaluator` (`evaluator/toolcalls.go`) walks the steps in order and pairs
tool calls with results. Key facts the Python emitter must honor:

- Preferred key is `toolCallId`. When a `tool_result` carries a `toolCallId`, it
  matches the pending `tool_call` with the same id, regardless of order or name.
  The id is authoritative: a `toolCallId` that matches no pending call is an
  orphan, with no name fallback. The emitter (`TraceRecorder`) generates a
  `toolCallId` per call by default and copies it onto the result via the
  `ToolCall` handle, so emitted traces always pair precisely.
- Fallback (no `toolCallId` on the result): the `tool` **name**, order-sensitive
  and FIFO. A `tool_result` matches the earliest still-pending `tool_call` with
  the same `tool`. So emit a call before its result, and do not interleave two
  pending calls of the same tool name if you need them paired deterministically.
- A `tool_result` that matches no pending call -> `neutral` finding
  ("without matching tool_call").
- A `tool_call` with no later matching result -> `neutral` finding
  ("has no matching result").
- A `tool_result` whose `error` is non-empty -> `bad` finding, independently of
  pairing.

Severity mapping (`evaluator/Evaluator.go`): `bad` = failure attributable to the
agent/trajectory (a tool that errored); `neutral` = anomaly with a possibly
external cause (dropped/orphan step) that is surfaced but not blamed; `good` =
correct behavior.

## Sample trace and verification command

`sample-trace.json` (this directory) is a hand-written clean run for a GitHub
triage agent: `start` -> `llm_call` -> `tool_call fetch_issues` ->
`tool_result` -> `llm_call` -> `end`. It loads, validates, and evaluates with
zero findings.

Verify from the repo root:

```bash
go run ./cmd/trazo -dir ./agents/langgraph-reference/docs
```

Expected output:

```
run-triage-demo-01 (github-triage)  clean

Summary: 1 run, 0 bad, 0 neutral, 0 good, 0 file errors
```

Add `-json` for machine-readable output, or `-validate` for a structure-only
pass that skips the evaluators.

### Exit codes

`cmd/trazo/main.go` exits with:

- `0` — clean (no file errors, no `bad` findings).
- `1` — at least one `bad` finding.
- `2` — at least one file error (unreadable, malformed, or failed `Validate`).
  File errors take precedence over `bad`.

Note: `go run` reports a non-zero program exit as its own `exit status N` and
itself returns `1`. To observe the true exit code (needed for CI), build and run
the binary:

```bash
go build -o trazo ./cmd/trazo
./trazo -dir <dir> -json
echo $?   # 0, 1, or 2
```
