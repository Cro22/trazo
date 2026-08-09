# Privacy and data handling

Trazo evaluates agent traces. Those traces are a faithful recording of what an
agent did, which means they can contain whatever the agent read, wrote, or was
told. This document describes exactly what a trace holds, where that data can
leave your machine, and how to keep private data private.

## What a trace file contains

A trace is a JSON `Run` with a list of `Step`s. The fields that can carry
sensitive content are:

| Field | Present on | Can contain |
| --- | --- | --- |
| `input` | any step | the LLM prompt, tool arguments, user-supplied text |
| `output` | any step | the LLM completion, tool results, fetched documents |
| `error` | any step | error strings, which sometimes echo inputs or paths |
| `agent`, `llm`, `tool`, `node` | identifiers | model names, internal tool and node names |

`input` and `output` are opaque payloads (`json.RawMessage`): trazo does not
inspect or constrain their contents, so anything the agent handled can end up
there verbatim. Prompts, retrieved passages, API responses, file contents, PII
in a user request: if the agent saw it, the trace can hold it.

The remaining fields (`cost`, `inputTokens`, `outputTokens`, `durationMs`,
timestamps, IDs, step `type`) are operational metadata and are not sensitive on
their own.

## Where data goes

By default, trazo is **fully local**. The core loads trace files from disk, runs
the structural evaluators (tool calls, loops, cost, latency, nodes) entirely
in-process, and writes findings to stdout. No trace data leaves the machine. The
core is standard-library only and opens no network connections in this mode.

There is exactly **one** component that sends trace data off the machine: the
[LLM judge](llm-judge.md). When you pass `-llm-judge`, the agent's final
`llm_call` **output** is placed in a prompt and sent to the Gemini API for
grading. That output text leaves your machine and is subject to the provider's
data-handling terms.

Nothing else is transmitted: not tool inputs, not intermediate steps, not the
whole trace. Only the final answer, and only when the judge is explicitly
enabled.

## How to disable network egress

- **Do not pass `-llm-judge`** (and leave `evaluators.llm_judge.enabled` false or
  absent in any config file). This is the default. With the judge off, trazo
  makes no outbound calls at all.
- If you want a hard guarantee, run trazo on a host with no network access, or
  omit `GEMINI_API_KEY` from the environment. Without the key the judge cannot be
  constructed and the run fails fast rather than sending anything.

## Redaction

Trazo does not redact for you; it evaluates whatever it is given. Redaction
belongs at trace-creation time, in whatever emits the trace (for the reference
agent, that is the Python `trazo_emitter`). Strip or mask secrets, credentials,
and PII from `input`, `output`, and `error` payloads before writing the file.
Because payloads are opaque to the core, a redacted trace evaluates exactly like
an unredacted one; the structural evaluators care about shape, not content.

If you enable the judge, remember that redaction of the final output directly
changes what is sent to Gemini. A masked answer is a masked prompt.

## Do not commit private traces

Real agent runs make excellent test fixtures, which is precisely why they leak.
Treat trace files from real workloads as you would logs or a database dump:

- Keep them out of the repository. Add your traces directory to `.gitignore`.
- The trace fixtures that **are** committed under `testdata/` and
  `agents/langgraph-reference/docs/` are hand-authored or produced against public
  repositories with synthetic content. Keep it that way: do not replace them with
  captures from private runs.
- Before sharing a trace for a bug report, open it and check `input` / `output` /
  `error` on every step. A trace is human-readable JSON; there is no excuse for
  not looking.

## API keys

`GEMINI_API_KEY` is read from the environment and is the only secret trazo
consumes. It is never written to a trace, never logged, and never printed. Never
hardcode it, never commit it, and never paste it into a config file (the config
schema has no field for it, by design).
