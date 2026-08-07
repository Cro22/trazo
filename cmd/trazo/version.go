package main

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/Cro22/trazo/trajectory"
)

// Version is the trazo product (binary) version. It is distinct from the trace
// schema version (trajectory.SchemaVersion): the product version tracks the CLI
// and library, the schema version tracks the trace format. See docs/versioning.md
// for the policy on bumping each.
const Version = "0.2.0"

// buildDetails is the VCS/build metadata embedded by the Go toolchain. It is
// populated from runtime/debug build info, which `go build` fills in
// automatically from the enclosing git repo (no ldflags needed); under `go run`
// or `go test` it may be absent, in which case commit stays "unknown".
type buildDetails struct {
	commit    string
	modified  bool
	buildTime string
	goVersion string
}

func readBuildDetails() buildDetails {
	d := buildDetails{commit: "unknown", goVersion: runtime.Version()}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return d
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			d.commit = s.Value
		case "vcs.time":
			d.buildTime = s.Value
		case "vcs.modified":
			d.modified = s.Value == "true"
		}
	}
	return d
}

// versionReport renders the multi-line output of `trazo version`. It is kept pure
// (details passed in) so it can be tested without depending on build metadata.
func versionReport(d buildDetails) string {
	commit := d.commit
	if commit != "unknown" && len(commit) > 12 {
		commit = commit[:12]
	}
	if d.modified {
		commit += "-dirty"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "trazo %s\n", Version)
	fmt.Fprintf(&b, "trace schema %s\n", trajectory.SchemaVersion)
	fmt.Fprintf(&b, "commit %s\n", commit)
	if d.buildTime != "" {
		fmt.Fprintf(&b, "built %s\n", d.buildTime)
	}
	fmt.Fprintf(&b, "%s\n", d.goVersion)
	return b.String()
}
