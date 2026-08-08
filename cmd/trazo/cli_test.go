package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// trazoBin is the freshly built binary under test, shared across the black-box
// tests below. Building once in TestMain keeps the suite fast.
var trazoBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "trazo-cli-test")
	if err != nil {
		fmt.Fprintln(os.Stderr, "mktemp:", err)
		os.Exit(1)
	}
	bin := filepath.Join(dir, "trazo")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "building trazo:", err)
		os.RemoveAll(dir)
		os.Exit(1)
	}
	trazoBin = bin

	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// Fixture paths, relative to this package directory (cmd/trazo).
const (
	cleanDir    = "../../testdata/ci/clean"
	cleanFile   = "../../testdata/ci/clean/triage_clean.json"
	failingFile = "../../testdata/ci/failing/tool_error.json"
	brokenFile  = "../../testdata/runs/broken.json"
)

type cliResult struct {
	stdout string
	stderr string
	code   int
}

func runCLI(t *testing.T, args ...string) cliResult {
	t.Helper()
	cmd := exec.Command(trazoBin, args...)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()

	code := 0
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			code = ee.ExitCode()
		} else {
			t.Fatalf("running %v: %v", args, err)
		}
	}
	return cliResult{stdout: out.String(), stderr: errb.String(), code: code}
}

func TestCLI_TextCleanExit0(t *testing.T) {
	r := runCLI(t, "-dir", cleanDir)
	if r.code != 0 {
		t.Fatalf("expected exit 0, got %d (stderr: %s)", r.code, r.stderr)
	}
	if !strings.Contains(r.stdout, "clean") || !strings.Contains(r.stdout, "Summary:") {
		t.Errorf("expected a clean summary, got:\n%s", r.stdout)
	}
}

func TestCLI_BadExit1(t *testing.T) {
	r := runCLI(t, failingFile)
	if r.code != 1 {
		t.Fatalf("expected exit 1 on a bad finding, got %d (stdout: %s)", r.code, r.stdout)
	}
	if !strings.Contains(r.stdout, "[BAD]") {
		t.Errorf("expected a BAD finding in output, got:\n%s", r.stdout)
	}
}

func TestCLI_FileErrorExit2(t *testing.T) {
	r := runCLI(t, brokenFile)
	if r.code != 2 {
		t.Fatalf("expected exit 2 on a file error, got %d", r.code)
	}
	if !strings.Contains(r.stdout, "File errors:") {
		t.Errorf("expected a file errors section, got:\n%s", r.stdout)
	}
}

func TestCLI_JSONBadExit1(t *testing.T) {
	r := runCLI(t, "-json", failingFile)
	if r.code != 1 {
		t.Fatalf("expected exit 1, got %d", r.code)
	}
	var env struct {
		OutputVersion string `json:"outputVersion"`
		Summary       struct {
			Bad int `json:"bad"`
		} `json:"summary"`
		Results []json.RawMessage `json:"results"`
	}
	if err := json.Unmarshal([]byte(r.stdout), &env); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, r.stdout)
	}
	if env.OutputVersion == "" {
		t.Error("JSON envelope missing outputVersion")
	}
	if env.Summary.Bad < 1 {
		t.Errorf("expected at least one bad finding in summary, got %d", env.Summary.Bad)
	}
	if len(env.Results) == 0 {
		t.Error("expected non-empty results")
	}
}

func TestCLI_MarkdownFileErrorExit2(t *testing.T) {
	r := runCLI(t, "-format", "md", brokenFile)
	if r.code != 2 {
		t.Fatalf("expected exit 2, got %d", r.code)
	}
	if !strings.Contains(r.stdout, "**Verdict: FAIL**") || !strings.Contains(r.stdout, "File errors") {
		t.Errorf("expected a FAIL markdown report with file errors, got:\n%s", r.stdout)
	}
}

func TestCLI_DirectoryDoesNotExist(t *testing.T) {
	r := runCLI(t, "./does-not-exist-xyz")
	if r.code == 0 {
		t.Fatalf("expected a non-zero exit for a missing path, got 0")
	}
	if !strings.Contains(r.stderr, "trazo:") {
		t.Errorf("expected a prefixed error on stderr, got:\n%s", r.stderr)
	}
}

func TestCLI_InvalidFormat(t *testing.T) {
	r := runCLI(t, "-format", "xml", cleanFile)
	if r.code == 0 {
		t.Fatalf("expected a non-zero exit for an invalid format")
	}
	if !strings.Contains(r.stderr, "unknown -format") {
		t.Errorf("expected an unknown-format error, got:\n%s", r.stderr)
	}
}

func TestCLI_TwoPathsError(t *testing.T) {
	r := runCLI(t, cleanFile, failingFile)
	if r.code != 2 {
		t.Fatalf("expected exit 2 for two PATH args, got %d", r.code)
	}
	if !strings.Contains(r.stderr, "at most one PATH") {
		t.Errorf("expected a one-PATH error, got:\n%s", r.stderr)
	}
}

func TestCLI_Version(t *testing.T) {
	r := runCLI(t, "version")
	if r.code != 0 {
		t.Fatalf("expected exit 0, got %d", r.code)
	}
	if !strings.Contains(r.stdout, "trazo "+Version) || !strings.Contains(r.stdout, "trace schema ") {
		t.Errorf("unexpected version output:\n%s", r.stdout)
	}
}

func TestCLI_EmptyDirectory(t *testing.T) {
	empty := t.TempDir()
	r := runCLI(t, "-dir", empty)
	if r.code != 0 {
		t.Fatalf("expected exit 0 for an empty directory, got %d", r.code)
	}
	if !strings.Contains(r.stderr, "no .json traces") {
		t.Errorf("expected a no-traces notice on stderr, got:\n%s", r.stderr)
	}
}

// TestCLI_FlagOverridesConfig pins the precedence rule: a config sets a lax cost
// threshold (no finding), and an explicit -max-step-cost flag overrides it (a
// finding appears). Both runs stay exit 0 because a cost finding is neutral.
func TestCLI_FlagOverridesConfig(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "policy.json")
	cfg := `{"version":1,"evaluators":{"cost_latency":{"enabled":true,"maxStepCost":1.0,"maxRunCost":1.0,"maxStepLatencyMs":600000,"maxRunLatencyMs":600000}}}`
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	withConfig := runCLI(t, "-config", cfgPath, cleanFile)
	if withConfig.code != 0 {
		t.Fatalf("config-only run should be clean (exit 0), got %d:\n%s", withConfig.code, withConfig.stdout)
	}
	if strings.Contains(withConfig.stdout, "cost_latency") {
		t.Errorf("lax config should produce no cost finding, got:\n%s", withConfig.stdout)
	}

	overridden := runCLI(t, "-config", cfgPath, "-max-step-cost", "0.00001", cleanFile)
	if !strings.Contains(overridden.stdout, "cost_latency") {
		t.Errorf("flag should override config and produce a cost finding, got:\n%s", overridden.stdout)
	}
}
