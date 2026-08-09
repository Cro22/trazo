# LLM-as-judge evaluator (`-llm-judge`)

Most trazo evaluators are structural: they reason about the shape of a trace
(orphan tool calls, loops, cost, latency) without ever reading the semantics of
what the agent produced. The LLM judge is the one exception. It asks a model to
grade the agent's final answer for correctness and usefulness, turning a
subjective "was this a good run?" into a `good` / `neutral` / `bad` judgment
alongside the structural findings.

Because it makes a network call and costs money, it is **opt-in** and is not part
of the default evaluator set.

## Enabling it

```sh
export GEMINI_API_KEY=...            # never commit this
go run ./cmd/trazo -llm-judge testdata/complex_run.json
```

Flags:

| Flag | Default | Meaning |
| --- | --- | --- |
| `-llm-judge` | off | enable the judge (requires `GEMINI_API_KEY`) |
| `-judge-model` | `gemini-2.5-flash` | model passed to the Gemini API |

The same two settings exist in the config file under `evaluators.llm_judge`
(`enabled`, `model`), so a policy file can pin the judge on for a whole team. As
everywhere else, an explicit `-llm-judge` flag overrides the config value.

## What it judges, and how

The judge looks at the run's **last `llm_call` output only**, once per run. If a
trace has no `llm_call` step there is nothing to grade and the evaluator returns
no findings. It does not re-grade intermediate reasoning or tool output; the
final answer is the thing a user sees, so it is the thing that gets graded.

The prompt asks the model to reply with a single JSON object:

```json
{"judgment": "good|neutral|bad", "score": 0.0-1.0, "comment": "short reason"}
```

- `good` maps to `JudgmentGood`, `bad` to `JudgmentBad`, `neutral` to
  `JudgmentNeutral` (trazo's own taxonomy).
- Surrounding prose or code fences are tolerated: the parser extracts the first
  `{ ... }` span from the reply.

## Determinism and cost discipline

- **Temperature 0.** The request pins `temperature: 0`, so for a given model and
  input the verdict is as stable as the provider allows. It is still an LLM, so
  treat verdicts as a strong signal, not a hard oracle.
- **Capped output.** `maxOutputTokens: 256`. The verdict is tiny; there is no
  reason to pay for more.
- **One call per run.** The judge never loops or retries internally. One trace is
  one completion. Cost scales linearly with the number of runs you judge, not
  with trace size.

## Timeouts, cancellation, and retries

- **Timeout.** Each judge call is bounded by a timeout (default **30s**,
  overridable on the evaluator). The bound is layered on top of the caller's
  context, so a run cancelled with Ctrl+C or by a CI timeout also aborts the
  in-flight network call promptly.
- **No retries.** A failed call is not retried. This is deliberate: retries hide
  provider instability and multiply cost. If you need retry semantics, wrap the
  provider at the HTTP layer.

## What happens when the judge fails

The two failure modes are treated differently on purpose.

1. **A malformed but returned verdict** (the model replied, but the JSON is
   missing, unparseable, or carries an unknown judgment word) becomes a single
   `JudgmentNeutral` finding whose comment quotes the offending reply. The run is
   still evaluated; you just get "the judge could not make up its mind" instead
   of a grade.

2. **A transport-level failure** (no API key, network error, non-200 status,
   timeout, empty candidate list) is surfaced as a run **error**, not a finding.
   In the CLI's JSON output it appears in `errors[]` with `kind: "evaluator"`;
   in text output it is reported as a failed evaluation. Crucially, the failure
   is isolated to the judge: the structural evaluators for that same file still
   run and still produce their findings. A judge outage degrades the report, it
   does not abort it.

So: an ambiguous answer is neutral; an unreachable provider is an error. Neither
one silently passes a bad run.

## Testing without a network

The judge depends only on a small `Completion` interface
(`Complete(ctx, prompt) (string, error)`), so tests inject a fake instead of
calling Gemini. See `evaluator/judge_test.go`: `fakeJudge` returns a canned reply
(or a canned error) and lets the tests exercise every branch above, including the
Gemini HTTP client against an `httptest` server. Running `go test ./evaluator/`
never touches the network and needs no API key.
