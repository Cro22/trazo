# Changelog

All notable changes to trazo are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project aims
to follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html) for the
product version. The trace **schema** version is tracked separately; see
[docs/versioning.md](docs/versioning.md).

## [Unreleased]

### Added

- `trazo version` subcommand and `-version` flag, reporting the product version,
  the supported trace schema version, the git commit, and the build time. Commit
  and build metadata come from the Go toolchain's build info (no ldflags needed).
- [docs/versioning.md](docs/versioning.md): the product and schema versioning
  policy, the schema compatibility contract, and the release process.
- Versioned, self-describing JSON output: `-format json` now emits an envelope
  with `outputVersion`, `trazoVersion`, `traceSchemaVersion`, `generatedAt`, an
  aggregate `summary`, `results`, and structured `errors`. Documented in
  [docs/output.md](docs/output.md).
- Evaluator policy file: `-config <path>` loads a versioned, stdlib-only JSON
  policy (the new `config` package) that pins which evaluators run and their
  thresholds, for reproducibility across dev, CI, and teams. Precedence is
  defaults < config < explicitly-set flags. Documented in
  [docs/config.md](docs/config.md), with an example config.
- Black-box CLI tests (`cmd/trazo/cli_test.go`) that build the binary and assert
  exit codes (0/1/2), each output format, invalid flags, missing paths, empty
  directories, and config/flag precedence.
- A dedicated `schema conformance` CI job that runs the Go schema-sync tests and
  validates fixtures and emitter output against the published JSON Schema, plus a
  test that the shipped example config always loads.
- `-verbose` flag: prints operational metrics to stderr after a run
  (`loaded=N valid=V invalid=I evaluated=E duration=Xms`), so batch runs are
  observable without parsing the results.
- Typed file errors: `runner.FileError` now carries an `ErrorKind`
  (`read_file`, `invalid_json`, `invalid_trace`, `evaluator`, `canceled`),
  surfaced as `errors[].kind` in the JSON output (bumped to `outputVersion` 1.1,
  additive) and tagged in the text output.
- This changelog.

### Changed

- BREAKING (JSON output): the top-level keys `evaluations` and `fileErrors` are
  now `results` and `errors`, nested under the new envelope. Consumers should
  read `outputVersion` and ignore unknown fields.

## [0.1.0]

The hardening round: formalize the trace contract and round the core out from a
working MVP toward production-grade.

### Added

- Formal JSON Schema (`trajectory/trace.schema.json`, draft 2020-12) as the
  strict external contract, kept in sync with the Go types by a dependency-free
  test and validated against fixtures and emitter output from Python.
- Trace schema versioning: `version` is required and semver-gated by major (see
  docs/versioning.md).
- Explicit `toolCallId` for authoritative tool call/result pairing, with
  name/FIFO as a documented fallback.
- CLI: single-file input, `-recursive`, `-validate` (structure-only), up-front
  `-format` validation, and a proper `-help`.
- Golden tests for the text, JSON, and Markdown output formats.

### Changed

- `Evaluator` interface takes a `context.Context`, so Ctrl+C or a CI timeout
  aborts in-flight work (notably the LLM judge).
- `Run.Validate` deepened: non-negative quantities, monotonic step timestamps,
  steps within the run interval, per-type required fields.
- Text output regrouped by run with a summary footer.
- Output formatting centralized in the `report` package; `Evaluator.go` renamed
  to `evaluator.go`.

[Unreleased]: https://github.com/Cro22/trazo/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/Cro22/trazo/releases/tag/v0.1.0
