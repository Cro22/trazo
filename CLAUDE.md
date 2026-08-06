# CLAUDE.md — Trazo Hito 4: LangGraph Reference Agent

## What this project is

Trazo is a trajectory evaluation framework for LLM agents, written in Go by the repo owner (Jesús). It ingests agent run traces as JSON files and runs evaluators over them, producing findings with a severity taxonomy:

- `JudgmentBad` — failures attributable to the agent itself
- `JudgmentNeutral` — ambiguous failures with external causes

Hito 4 (this work) adds a **Python reference agent** built with LangGraph that emits traces in trazo's JSON format, proving the framework end-to-end: real agent → real traces → real evaluations.

## Working boundary — read this first

**Claude implements both the Go core and the Python side.** On 2026-07-31 the
owner delegated the Go work (hitos 3.5, 5, 6) to Claude; on 2026-08-06 he
reaffirmed it. The former "Go core is READ-ONLY" rule is **lifted**. You may
modify Go files under `trajectory`, `evaluator`, `runner`, `cmd`, etc.

What stays in force:

- **Milestone-by-milestone with checkpoints.** Do not chain milestones. At each
  checkpoint, stop and wait for the owner's review (see Workflow rule below).
- **Schema changes are a big deal.** The trace JSON schema (`trajectory` types)
  is consumed by the Python emitter and documented in
  `agents/langgraph-reference/docs/trace-schema.md`. Before changing it, describe
  the change and its blast radius, and get owner sign-off. Then update the schema
  doc in the same milestone.
- **Keep the core coherent.** Match existing style and test conventions; every Go
  change ships with tests and `go build ./... && go test ./...` green.

Hito 4 (Python reference agent) work lives in `agents/langgraph-reference/`.

## Source of truth for the trace format

The trace JSON schema is defined by the Go code, not by this document and not by your assumptions. Before writing any Python:

1. Read the `trajectory` package: `Run` and `Step` types, the `StepType` constants, and how payloads use `json.RawMessage`.
2. Read the loader and its tests — test fixtures are the canonical examples of valid trace files.
3. Read `ToolCallEvaluator` to understand tool call/result pairing and orphan detection — the Python emitter must produce traces that pair correctly.

If the Go loader has a `Validate()` method, use it as the acceptance gate. If it does not exist yet (it was a pending item), validation = the Go runner loads the file without error and evaluators produce sane output.

## Environment

- Windows host, PowerShell. Go side opens in GoLand.
- Python: 3.11+. Use a venv inside `agents/langgraph-reference/`. Manage deps with `requirements.txt` (keep it minimal and pinned).
- LLM provider: Gemini via `GEMINI_API_KEY` env var (same convention as the owner's other projects). Never hardcode keys. Never print keys.
- Keep costs low: cheap model tier, low max tokens, cap iterations. This project is also about cost discipline.

## Conventions

- All code, comments, commit messages, and docs in English.
- No em-dashes in any written output (README, docs, comments).
- Type hints everywhere. `pytest` for tests. Small modules, no framework soup.
- Do not add LangSmith, Langfuse, or any external observability. Trazo IS the observability layer here — that is the whole point.
- Commit at the end of each milestone with a message like `hito4-m1: trace schema extracted and documented`.

## Workflow rule

Work milestone by milestone. At the end of each milestone: stop, summarize what was done, list files touched, show how to verify it, and WAIT for the owner to review before starting the next milestone. Do not chain milestones in one run.

---

## Milestones

### M1 — Extract and document the trace schema

- Read the Go `trajectory` package and loader test fixtures.
- Write `agents/langgraph-reference/docs/trace-schema.md`: field-by-field description of a valid trace file (Run envelope, Step array, every StepType, payload shape per type, timestamps, IDs).
- Hand-write one sample trace JSON (`docs/sample-trace.json`) and verify the Go runner loads and evaluates it: `go run ./cmd/... <dir>` or whatever the repo's entrypoint is (discover it, document the exact command).
- Deliverable: schema doc + validated sample + the exact verification command in the doc.

**Checkpoint: owner reviews the schema doc against his own mental model before any Python exists.**

### M2 — Python trace emitter

- `agents/langgraph-reference/trazo_emitter/`: a small library (no LangGraph dependency yet) that builds Run/Step objects and serializes them to trazo-format JSON files.
- API sketch: `TraceRecorder` with methods per StepType (e.g., `record_llm_call`, `record_tool_call`, `record_tool_result`), correlation between tool calls and results, `flush(dir)` writing one file per run.
- Unit tests: serialization matches the schema doc; a flushed file passes the Go-side verification from M1 (write a small script or documented manual step that runs the Go runner against pytest's output dir).
- Deliverable: emitter package + passing tests + cross-language verification demonstrated.

### M3 — LangGraph agent

- Build a tool-calling agent in LangGraph with a deliberately simple, verifiable task: **GitHub repository triage**. Given a repo name, the agent uses 2-3 tools (e.g., `fetch_issues` via GitHub REST, `classify_issue`, `summarize`) and produces a short triage report.
- Graph shape: at minimum an LLM node, a tool node, and a conditional edge with an iteration cap. Use LangGraph checkpointing (in-memory is fine).
- Wire the emitter: every LLM call, tool call, and tool result becomes a Step. Prefer instrumenting via LangGraph/LangChain callbacks or node wrappers so agent logic stays clean of tracing code.
- CLI entrypoint: `python -m agent run --repo <owner/name> --traces-dir <dir>`.
- Deliverable: agent runs end-to-end against a real public repo, trace file lands in the dir, Go runner evaluates it clean.

### M4 — Failure scenarios and evaluation demo

- Add a `--scenario` flag with scripted failure modes:
  - `orphan-tool-call`: agent emits a tool call whose result is dropped (should surface via ToolCallEvaluator orphan detection).
  - `tool-error`: a tool returns an error the agent must handle; trace shows it. Depending on handling, this maps to JudgmentBad or JudgmentNeutral — document which and why.
  - `runaway-loop`: iteration cap triggers; trace shows the truncated loop.
- Run all scenarios, collect the Go evaluator output for each, and write `docs/evaluation-results.md`: scenario → trace → findings → severity, with the actual output pasted in.
- Deliverable: reproducible failure matrix. This document is the CV artifact.

### M5 — README and polish

- Update the repo root README (this is the ONE exception to the Go-side read-only rule, and only the README): add a "Reference agent" section with an architecture diagram (ASCII or mermaid), the quickstart (3 commands: install, run agent, run evaluation), and a link to `evaluation-results.md`.
- Sanity pass: pinned deps, no dead code, no TODOs left in shipped files, cost note (approx tokens/$ per agent run).
- Deliverable: a stranger can clone, run the agent, and see trazo findings in under 10 minutes.

### M6 (OPTIONAL, gated — do not start without explicit owner go-ahead)

- Swap Gemini for a local inference backend (vLLM or llama.cpp on the owner's RTX 3090) behind the same agent code, as a dual-target experiment. Out of scope until M1-M5 are merged and reviewed.

---

## Definition of done (Hito 4)

1. `python -m agent run` produces trace files the unmodified Go core loads and evaluates.
2. At least one scenario produces a `JudgmentBad` finding and at least one produces `JudgmentNeutral`, documented with real output.
3. Go core untouched (README excepted).
4. Owner has reviewed every milestone checkpoint.
