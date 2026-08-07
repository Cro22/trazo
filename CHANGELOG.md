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
- This changelog.

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
