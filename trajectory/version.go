package trajectory

import (
	"fmt"
	"strconv"
	"strings"
)

// SchemaVersion is the trace schema version this build emits and treats as
// canonical. It is semantic (MAJOR.MINOR.PATCH). Bump the MINOR for additive,
// backward-compatible changes (a new optional field, like toolCallId) and the
// MAJOR for a breaking change (a removed/renamed field, a changed meaning).
const SchemaVersion = "0.1.0"

// supportedMajor is the schema MAJOR version this build can evaluate. Traces are
// accepted across MINOR/PATCH differences within this major, since those are
// additive and backward compatible, and rejected across a MAJOR boundary, where
// the shape may have changed incompatibly. Keep it equal to SchemaVersion's
// major.
const supportedMajor = 0

// checkVersion validates a run's version field: it must be present, be valid
// semver, and share this build's supported MAJOR. A cross-major trace is
// reported with the supported version so the message is actionable.
func checkVersion(v string) error {
	if v == "" {
		return fmt.Errorf("run: version is empty (expected semver compatible with %s)", SchemaVersion)
	}
	major, _, _, err := parseSemver(v)
	if err != nil {
		return fmt.Errorf("run: version %q is not semver MAJOR.MINOR.PATCH: %w", v, err)
	}
	if major != supportedMajor {
		return fmt.Errorf("run: schema version %q (major %d) is unsupported; this build accepts major %d (%s)",
			v, major, supportedMajor, SchemaVersion)
	}
	return nil
}

// parseSemver splits a MAJOR.MINOR.PATCH string. It is intentionally strict:
// exactly three dot-separated non-negative integers, no pre-release or build
// metadata, matching what the emitter produces and what the JSON Schema pattern
// enforces.
func parseSemver(v string) (major, minor, patch int, err error) {
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return 0, 0, 0, fmt.Errorf("want 3 dot-separated parts, got %d", len(parts))
	}
	nums := [3]int{}
	for i, p := range parts {
		if p == "" {
			return 0, 0, 0, fmt.Errorf("part %d is empty", i+1)
		}
		n, convErr := strconv.Atoi(p)
		if convErr != nil {
			return 0, 0, 0, fmt.Errorf("part %d %q is not an integer", i+1, p)
		}
		if n < 0 {
			return 0, 0, 0, fmt.Errorf("part %d %q is negative", i+1, p)
		}
		nums[i] = n
	}
	return nums[0], nums[1], nums[2], nil
}
