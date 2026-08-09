# Machine-readable output (`-format json`)

`trazo -format json` (or `-json`) prints a single self-describing envelope meant
for CI and downstream tools. Its shape is a contract with its own version,
independent of the product and trace-schema versions.

## Envelope

```json
{
  "outputVersion": "1.1",
  "trazoVersion": "0.2.0",
  "traceSchemaVersion": "0.1.0",
  "generatedAt": "2026-08-07T12:00:00Z",
  "summary": {
    "files": 4,
    "runs": 2,
    "evaluations": 3,
    "findings": 3,
    "good": 0,
    "neutral": 2,
    "bad": 1,
    "fileErrors": 1
  },
  "results": [
    {
      "evaluatorName": "tool_calls",
      "runId": "run-1",
      "findings": [
        { "stepIndex": 3, "judgment": "bad", "comment": "..." }
      ]
    }
  ],
  "errors": [
    { "file": "broken.json", "kind": "invalid_json", "error": "unexpected end of JSON input" }
  ]
}
```

## Fields

| Field | Type | Notes |
|-------|------|-------|
| `outputVersion` | string | Version of this envelope contract. See [Stability](#stability). |
| `trazoVersion` | string | Product version that produced the output. |
| `traceSchemaVersion` | string | Trace schema version this build supports. |
| `generatedAt` | RFC3339 string | UTC generation time. |
| `summary` | object | Aggregate counts, see below. |
| `results` | array | One entry per (evaluator, run); `[]` when there is nothing to report. |
| `errors` | array | One entry per file that failed to read, parse, or validate; `[]` when none. |

`summary`:

| Field | Meaning |
|-------|---------|
| `files` | Trace files considered. |
| `runs` | Distinct runs evaluated. |
| `evaluations` | Entries in `results` (evaluator x run). |
| `findings` | Total findings across all results. |
| `good` / `neutral` / `bad` | Findings by severity. |
| `fileErrors` | Entries in `errors`. |

Each `results[i]` object: `evaluatorName` (string), `runId` (string), `findings`
(array). Each finding: `stepIndex` (int; `-1` is run-level), `judgment` (`good` |
`neutral` | `bad`), `comment` (string), and `score` (number, omitted when zero).

Each `errors[i]` object: `file` (path as given to trazo), `kind` (category, see
below), and `error` (message). `kind` lets a consumer react by category instead
of matching message strings:

| `kind` | Meaning |
|--------|---------|
| `read_file` | The file could not be read. |
| `invalid_json` | The bytes are not valid JSON. |
| `invalid_trace` | JSON parsed but failed structural validation (`Run.Validate`). |
| `evaluator` | An evaluator returned an error. |
| `canceled` | The context was canceled (Ctrl+C, timeout) before processing. |

## Stability

`outputVersion` follows semver-like rules for the envelope:

- Additive, backward-compatible changes (a new field) bump the **minor**.
- Renaming or removing a field, or changing a field's meaning, bumps the
  **major**.

Consumers should ignore unknown fields so a minor bump does not break them.
Empty collections are always `[]`, never `null`, so indexing is safe without a
nil check. See [versioning.md](versioning.md) for how this relates to the product
and trace-schema versions.

## Exit codes

The envelope is independent of the process exit code, which CI can gate on
directly: `0` clean, `1` at least one `bad` finding, `2` at least one file error.
