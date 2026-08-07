# Versioning and compatibility

Trazo carries two independent version numbers. Keeping them separate is
deliberate: the tool and the data format evolve at different rates.

| Version | What it tracks | Where it lives |
|---------|----------------|----------------|
| Product version | The CLI and the Go library (behavior, flags, output shape) | `main.Version`, reported by `trazo version` |
| Trace schema version | The on-disk trace JSON format | `trajectory.SchemaVersion`, stamped in each trace's `version` field |

Both follow [Semantic Versioning](https://semver.org/): `MAJOR.MINOR.PATCH`.

## Trace schema compatibility

Trazo accepts a trace whose **major** schema version matches the binary's
supported major. Within that major, minor and patch differences are compatible.

- **Patch** (`0.1.0` -> `0.1.1`): clarifications or fixes with no field changes.
- **Minor** (`0.1.0` -> `0.2.0`): backward-compatible additions, typically a new
  optional field. Older traces still validate; newer traces still load on an
  older binary of the same major, because unknown fields are ignored by the Go
  loader. (Adding the optional `toolCallId` field was such a minor bump.)
- **Major** (`0.x` -> `1.0.0`): a breaking change: a removed or renamed field, or
  a changed meaning. Traces across a major boundary are **rejected**.

Current supported schema major: **0**.
Current schema version: **0.1.0**.

### What happens to an incompatible trace

The gate is `trajectory.checkVersion`, run as part of `Run.Validate`:

- Missing or non-semver `version` -> rejected as an invalid trace.
- A `2.0.0` trace on a major-0 binary -> rejected with a message naming the
  supported version, for example:

  ```
  run: schema version "2.0.0" (major 2) is unsupported; this build accepts major 0 (0.1.0)
  ```

Rejected traces surface as file errors (CLI exit code 2); they do not abort the
rest of a batch.

### Migrating older traces

Within a major there is nothing to migrate: an older minor validates as-is. A
future major bump will ship with either a documented migration for the changed
fields or an explicit decision to drop support for the old major, recorded in the
[changelog](../CHANGELOG.md). We do not silently coerce across a major.

## Product versioning

The product version bumps on user-visible changes to the CLI or library:

- **Patch**: bug fixes, no interface change.
- **Minor**: backward-compatible features (a new flag, a new evaluator, an
  additive field in the JSON output).
- **Major**: breaking changes to flags, exit codes, the library API, or the
  machine-readable output contract.

The machine-readable JSON output is a contract in its own right and carries its
own `schemaVersion` field; see the output section of the README once published.

## Release process

1. Move the `[Unreleased]` entries in [CHANGELOG.md](../CHANGELOG.md) under a new
   `[X.Y.Z]` heading with the date.
2. Bump `main.Version` (and `trajectory.SchemaVersion` plus `supportedMajor` if
   the trace format changed).
3. Tag the commit `vX.Y.Z`. The build embeds the commit automatically, so
   `trazo version` reports the tagged revision.
4. `go build ./... && go test ./...` must be green, and the Python suite too.

`trazo version` prints all of this at a glance:

```
trazo 0.2.0
trace schema 0.1.0
commit 891c879de1ad
built 2026-08-07T21:01:56Z
go1.25.0
```
