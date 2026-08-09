# Evaluator policy file (`-config`)

Passing `-config <path>` loads a JSON policy that pins which evaluators run and
with what thresholds. Commit it to a repo and every developer, CI job, and
teammate evaluates traces the same way, instead of remembering a string of flags.

The format is stdlib-only JSON (no new dependency) and is versioned in its own
right. A full example is [trazo.config.example.json](trazo.config.example.json).

```json
{
  "version": 1,
  "evaluators": {
    "tool_calls": { "enabled": true },
    "loops": { "enabled": true, "maxRepeats": 3 },
    "cost_latency": {
      "enabled": true,
      "maxStepCost": 0.05,
      "maxStepLatencyMs": 30000,
      "maxRunCost": 0.2,
      "maxRunLatencyMs": 120000
    },
    "node_transitions": {
      "enabled": true,
      "terminalNodes": ["end", "__end__", "finish", "done"]
    },
    "llm_judge": { "enabled": false, "model": "gemini-2.5-flash" }
  }
}
```

## Rules

- `version` is required and must be `1` (the format version this build accepts).
- Every field is optional beyond `version`: an omitted field keeps its built-in
  default, so a partial config is valid. `{"version": 1, "evaluators": {}}` is the
  default policy.
- Unknown fields are rejected. A typo like `maxRepeat` is an error, not a silent
  no-op.
- Thresholds must be non-negative; `terminalNodes` entries must be non-empty.
- Omitting `node_transitions.terminalNodes` falls back to the evaluator's default
  terminal set.

## Precedence

From lowest to highest:

1. Built-in defaults.
2. The `-config` file.
3. Explicitly-set command-line flags.

A flag only overrides the config when you actually pass it, so the config pins a
reproducible baseline while a one-off flag (`-max-step-cost 0.01`) still wins for
a single run. Example:

```bash
# Reproducible team policy, committed to the repo:
trazo -config trazo.config.json ./traces

# Same policy, but tighten one threshold for this run only:
trazo -config trazo.config.json -max-step-cost 0.01 ./traces
```

## Using it as a library

The policy is a normal Go package, independent of the CLI:

```go
cfg, err := config.Load("trazo.config.json")
evals, err := cfg.Build(func(model string) (evaluator.Evaluator, error) {
    // construct your judge, or return an error to disable it
})
resp := runner.NewRunner(evals).RunFiles(ctx, files)
```
