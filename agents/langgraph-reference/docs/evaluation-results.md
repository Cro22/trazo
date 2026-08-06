# Evaluation results (Hito 4, M4)

Reproducible failure matrix: each scenario is a deterministic, offline run
(scripted LLM, controlled issue source, no API key) whose trace is evaluated by
the unmodified Go core. Every finding and severity below is pasted from the real
Go runner output.

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

| Scenario | Trace shape | Finding | Severity |
|----------|-------------|---------|----------|
| tool-error | llm -> tool_call -> tool_result(error) -> llm | `tool fetch_issues fails: rate limited: 403 Forbidden` | **bad** |
| orphan-tool-call | llm -> tool_call -> (result dropped) -> llm | `tool_call fetch_issues has no matching result` | **neutral** |
| runaway-loop | llm/tool_call/tool_result x2 then a capped llm turn | (none from ToolCallEvaluator) | n/a today |

The process exit code for the batch is `1`, because at least one finding is
`bad` (file-error precedence would make it `2`, but there are no file errors).

## Real Go runner output (all three at once)

Text:

```
RunID scenario-orphan-tool-call. Findings: 1 Evaluator: tool_calls
Step 2: Comment: tool_call fetch_issues has no matching result, Score: 0.000000, Judgment: neutral
RunID scenario-runaway-loop. Findings: 0 Evaluator: tool_calls
RunID scenario-tool-error. Findings: 1 Evaluator: tool_calls
Step 3: Comment: tool fetch_issues fails: rate limited: 403 Forbidden, Score: 0.000000, Judgment: bad
```

JSON (`-json`):

```json
{
  "evaluations": [
    {
      "evaluatorName": "tool_calls",
      "runId": "scenario-orphan-tool-call",
      "findings": [
        {
          "stepIndex": 2,
          "judgment": "neutral",
          "comment": "tool_call fetch_issues has no matching result"
        }
      ]
    },
    {
      "evaluatorName": "tool_calls",
      "runId": "scenario-runaway-loop",
      "findings": []
    },
    {
      "evaluatorName": "tool_calls",
      "runId": "scenario-tool-error",
      "findings": [
        {
          "stepIndex": 3,
          "judgment": "bad",
          "comment": "tool fetch_issues fails: rate limited: 403 Forbidden"
        }
      ]
    }
  ],
  "fileErrors": []
}
```

## Scenario detail

### 1. tool-error -> JudgmentBad

The `fetch_issues` tool raises a `403 Forbidden`. LangGraph's ToolNode
(`handle_tool_errors=True`) turns it into a tool message so the agent can react,
and the tracing callback records a `tool_result` carrying that `error`. The scripted
agent then handles it with a final "aborting triage" message.

`ToolCallEvaluator` maps any `tool_result` with a non-empty `error` to **bad**:
the failure is real and present in the trajectory, so it is actionable now.

Why bad and not neutral: in the current evaluator the severity is a property of
the trace (a tool errored), not of whether the agent recovered. Recovery-aware
grading (error handled cleanly => downgrade to neutral) would need a richer
evaluator; that is a hito 5 concern, not a change to this one.

### 2. orphan-tool-call -> JudgmentNeutral

The agent issues a `tool_call` but its result is dropped from the trace (the
scenario simulates lost or interrupted instrumentation via a handler that skips
the first result). The trace therefore has a `tool_call` with no matching
`tool_result`.

`ToolCallEvaluator` flags the unmatched call as **neutral**: the cause may be
external to the agent (a crash, a dropped event, a killed run), so it is
surfaced for human review rather than blamed on the trajectory.

### 3. runaway-loop -> no finding today (the hito 5 gap)

The agent asks for a tool on every turn; the iteration cap (3) truncates the
loop. The trace shows three `llm_call` turns with two completed tool
call/result pairs and a final capped turn whose tool never executes.

`ToolCallEvaluator` returns **zero findings**: every executed call is paired and
no tool errored, so this evaluator has nothing to say about repetition. This is
exactly the blind spot a dedicated loop-detection evaluator (planned for hito 5)
is meant to cover. It is documented here rather than hidden, and the trace is
retained as the input that evaluator will be tested against.

## Definition-of-done check (Hito 4)

- At least one scenario yields `JudgmentBad`: tool-error. Confirmed above.
- At least one scenario yields `JudgmentNeutral`: orphan-tool-call. Confirmed above.
- The Go core is unmodified; these are its real outputs.
