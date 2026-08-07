package report

import (
	"errors"
	"strings"
	"testing"

	"github.com/Cro22/trazo/runner"
)

func TestValidateSummary_MixedValidInvalid(t *testing.T) {
	resp := &runner.Response{
		FileErrors: []runner.FileError{
			{File: "testdata/broken.json", Err: errors.New("unexpected end of JSON input")},
			{File: "testdata/invalid.json", Err: errors.New("run: agent is empty\nstep 0: tool_call without tool")},
		},
	}
	got := ValidateSummary(resp, 5)

	if !strings.HasPrefix(got, "Validated 5 file(s): 3 valid, 2 invalid.\n") {
		t.Errorf("unexpected summary line:\n%s", got)
	}
	if !strings.Contains(got, "testdata/broken.json: unexpected end of JSON input") {
		t.Errorf("missing broken.json line:\n%s", got)
	}
	// Multi-line validation errors are flattened onto one line for readability.
	if !strings.Contains(got, "run: agent is empty; step 0: tool_call without tool") {
		t.Errorf("expected flattened error line:\n%s", got)
	}
}

func TestValidateSummary_AllValid(t *testing.T) {
	got := ValidateSummary(&runner.Response{}, 3)
	if got != "Validated 3 file(s): 3 valid, 0 invalid.\n" {
		t.Errorf("unexpected summary: %q", got)
	}
}
