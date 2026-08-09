package main

import (
	"strings"
	"testing"

	"github.com/Cro22/trazo/trajectory"
)

func TestVersionReport_Fields(t *testing.T) {
	got := versionReport(buildDetails{
		commit:    "abcdef1234567890",
		buildTime: "2026-08-07T12:00:00Z",
		goVersion: "go1.25.0",
	})

	wantLines := []string{
		"trazo " + Version,
		"trace schema " + trajectory.SchemaVersion,
		"commit abcdef123456", // truncated to 12 chars
		"built 2026-08-07T12:00:00Z",
		"go1.25.0",
	}
	for _, line := range wantLines {
		if !strings.Contains(got, line) {
			t.Errorf("version report missing %q, got:\n%s", line, got)
		}
	}
}

func TestVersionReport_DirtyAndUnknown(t *testing.T) {
	dirty := versionReport(buildDetails{commit: "deadbeefcafebabe", modified: true, goVersion: "go1.25.0"})
	if !strings.Contains(dirty, "commit deadbeefcafe-dirty") {
		t.Errorf("expected truncated dirty commit, got:\n%s", dirty)
	}

	unknown := versionReport(buildDetails{commit: "unknown", goVersion: "go1.25.0"})
	if !strings.Contains(unknown, "commit unknown\n") {
		t.Errorf("expected literal unknown commit, got:\n%s", unknown)
	}
	// With no build time, the "built" line is omitted entirely.
	if strings.Contains(unknown, "built ") {
		t.Errorf("did not expect a built line without build time, got:\n%s", unknown)
	}
}
