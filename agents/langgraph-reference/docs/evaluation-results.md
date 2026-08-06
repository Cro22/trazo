# Evaluation results (Hito 4 M4, refreshed in Hito 5)

Reproducible failure matrix: each scenario is a deterministic, offline run
(scripted LLM, controlled issue source, no API key) whose trace is evaluated by
the unmodified Go core. Every finding and severity below is pasted from the real
Go runner output.

Since hito 5 the runner runs four evaluators per file: `tool_calls`, `loops`,
`cost_latency` and `node_transitions` (plus an opt-in `llm_judge`). The runaway
loop that `tool_calls` could not see is now caught by `loops`.

## How to reproduce

From `agents/langgraph-reference/` (venv active), emit the three scenario traces:

```bash
python -m agent run --repo golang/example --traces-dir ./_scenarios --scenario tool-error
python -m agent run --repo golang/example --traces-dir ./_scenarios --scenario orphan-tool-call
python -m agent run --repo golang/example --traces-dir ./_scenarios --scenario runaway-loop
```

Then evaluate them with the Go core (from the repo root):

```bash
go run ./cmd/trazo -dir ./agents/langgraph-reference/_scenarios
# For the true CI exit code, build the binary (go run remaps a non-zero exit to 1):
go build -o trazo ./cmd/trazo && ./trazo -dir ./agents/langgraph-reference/_scenarios ; echo $?
```

## Summary matrix

| Scenario | Evaluator | Finding | Severity |
|----------|-----------|---------|----------|
| tool-error | tool_calls | `tool fetch_issues fails: rate limited: 403 Forbidden` | **bad** |
| orphan-tool-call | tool_calls | `tool_call fetch_issues has no matching result` | **neutral** |
| runaway-loop | loops | `tool fetch_issues called with identical input at least 3 times (possible loop)` | **bad** |

The process exit code for the batch is `1`, because at least one finding is
`bad` (file-error precedence would make it `2`, but there are no file errors).

## Real Go runner output (text)

```
RunID scenario-orphan-tool-call. Findings: 1 Evaluator: tool_calls
Step 2: Comment: tool_call fetch_issues has no matching result, Score: 0.000000, Judgment: neutral
RunID scenario-orphan-tool-call. Findings: 0 Evaluator: loops
RunID scenario-orphan-tool-call. Findings: 0 Evaluator: cost_latency
RunID scenario-orphan-tool-call. Findings: 0 Evaluator: node_transitions
RunID scenario-runaway-loop. Findings: 0 Evaluator: tool_calls
RunID scenario-runaway-loop. Findings: 1 Evaluator: loops
Step 8: Comment: tool fetch_issues called with identical input at least 3 times (possible loop), Score: 0.000000, Judgment: bad
RunID scenario-runaway-loop. Findings: 0 Evaluator: cost_latency
RunID scenario-runaway-loop. Findings: 0 Evaluator: node_transitions
RunID scenario-tool-error. Findings: 1 Evaluator: tool_calls
Step 3: Comment: tool fetch_issues fails: rate limited: 403 Forbidden, Score: 0.000000, Judgment: bad
RunID scenario-tool-error. Findings: 0 Evaluator: loops
RunID scenario-tool-error. Findings: 0 Evaluator: cost_latency
RunID scenario-tool-error. Findings: 0 Evaluator: node_transitions
```

## Scenario detail

### 1. tool-error -> JudgmentBad (tool_calls)

The `fetch_issues` tool raises a `403 Forbidden`. LangGraph's ToolNode
(`handle_tool_errors=True`) turns it into a tool message so the agent can react,
and the tracing callback records a `tool_result` carrying that `error`. The
scripted agent then handles it with a final "aborting triage" message.

`ToolCallEvaluator` maps any `tool_result` with a non-empty `error` to **bad**:
the failure is real and present in the trajectory, so it is actionable now.

Why bad and not neutral: severity here is a property of the trace (a tool
errored), not of whether the agent recovered. Recovery-aware grading would need a
richer evaluator.

### 2. orphan-tool-call -> JudgmentNeutral (tool_calls)

The agent issues a `tool_call` but its result is dropped from the trace (the
scenario simulates lost or interrupted instrumentation via a handler that skips
the first result). The trace therefore has a `tool_call` with no matching
`tool_result`.

`ToolCallEvaluator` flags the unmatched call as **neutral**: the cause may be
external to the agent (a crash, a dropped event, a killed run), so it is
surfaced for human review rather than blamed on the trajectory.

### 3. runaway-loop -> JudgmentBad (loops)

The agent asks for the same tool on every turn; the iteration cap (5) truncates
the loop after four identical `fetch_issues` calls. Every executed call is still
paired, so `ToolCallEvaluator` sees nothing wrong (`Findings: 0`).

This was the documented gap. The hito 5 `LoopEvaluator` closes it: it keys tool
calls by name plus canonical input and flags **bad** once the same key repeats at
least `MaxRepeats` times (default 3). Distinct inputs are treated as legitimate
fan-out, so a normal run that classifies several different issues is not flagged;
only genuine repetition is. Here the four identical `fetch_issues` calls trip the
threshold at step 8.

## Definition-of-done check (Hito 4)

- At least one scenario yields `JudgmentBad`: tool-error and runaway-loop.
- At least one scenario yields `JudgmentNeutral`: orphan-tool-call.
- The Go core is unmodified in behavior for these traces; these are its real
  outputs across all evaluators.
