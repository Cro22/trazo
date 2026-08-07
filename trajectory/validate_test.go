package trajectory

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func validRun() *Run {
	start := time.Date(2026, 6, 12, 14, 0, 0, 0, time.UTC)
	return &Run{
		ID:        "run-valid",
		Agent:     "data_extractor",
		Version:   "0.1.0",
		StartTime: start,
		EndTime:   start.Add(5 * time.Second),
		Steps: []Step{
			{Node: "start", Type: StepTypeNodeTransition, Timestamp: start},
			{LLM: "gpt-4o", Type: StepTypeCallLLM, Timestamp: start.Add(time.Second), Input: json.RawMessage(`{"user":"hi"}`)},
			{Tool: "postgres_query", Type: StepTypeToolCall, Timestamp: start.Add(2 * time.Second)},
			{Tool: "postgres_query", Type: StepTypeToolResult, Timestamp: start.Add(3 * time.Second)},
		},
	}
}

func TestValidate_ValidRun(t *testing.T) {
	if err := validRun().Validate(); err != nil {
		t.Errorf("expected valid run, got: %v", err)
	}
}

func TestValidate_InvalidRuns(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(r *Run)
		wantMsg string
	}{
		{"empty id", func(r *Run) { r.ID = "" }, "id is empty"},
		{"empty agent", func(r *Run) { r.Agent = "" }, "agent is empty"},
		{"missing startTime", func(r *Run) { r.StartTime = time.Time{} }, "startTime is missing"},
		{"missing endTime", func(r *Run) { r.EndTime = time.Time{} }, "endTime is missing"},
		{"endTime before startTime", func(r *Run) { r.EndTime = r.StartTime.Add(-time.Second) }, "endTime is before startTime"},
		{"missing step timestamp", func(r *Run) { r.Steps[2].Timestamp = time.Time{} }, "step 2: timestamp is missing"},
		{"llm_call without llm", func(r *Run) { r.Steps[1].LLM = "" }, "step 1: llm_call without llm"},
		{"tool_call without tool", func(r *Run) { r.Steps[2].Tool = "" }, "step 2: tool_call without tool"},
		{"tool_result without tool", func(r *Run) { r.Steps[3].Tool = "" }, "step 3: tool_result without tool"},
		{"node_transition without node", func(r *Run) { r.Steps[0].Node = "" }, "step 0: node_transition without node"},
		{"unknown step type", func(r *Run) { r.Steps[1].Type = "banana" }, `step 1: unknown step type "banana"`},
		{"negative cost", func(r *Run) { r.Steps[1].Cost = -0.01 }, "step 1: cost is negative"},
		{"negative inputTokens", func(r *Run) { r.Steps[1].InputTokens = -5 }, "step 1: inputTokens is negative"},
		{"negative outputTokens", func(r *Run) { r.Steps[1].OutputTokens = -5 }, "step 1: outputTokens is negative"},
		{"negative durationMs", func(r *Run) { r.Steps[1].DurationMs = -1 }, "step 1: durationMs is negative"},
		{"timestamp before previous", func(r *Run) { r.Steps[2].Timestamp = r.Steps[1].Timestamp.Add(-time.Second) }, "step 2: timestamp is before step 1"},
		{"step before run startTime", func(r *Run) { r.Steps[0].Timestamp = r.StartTime.Add(-time.Second) }, "step 0: timestamp is before run startTime"},
		{"step after run endTime", func(r *Run) { r.Steps[3].Timestamp = r.EndTime.Add(time.Second) }, "step 3: timestamp is after run endTime"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			run := validRun()
			tc.mutate(run)
			err := run.Validate()
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantMsg)
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Errorf("expected error containing %q, got: %v", tc.wantMsg, err)
			}
		})
	}
}

// TestValidate_Version pins the schema-compatibility gate: version is required
// and must be semver sharing the build's supported major. Same-major minor/patch
// differences are accepted (additive, backward compatible); a missing version,
// non-semver, or cross-major version is rejected.
func TestValidate_Version(t *testing.T) {
	accepted := []string{"0.1.0", "0.0.1", "0.9.9", "0.1.5"}
	for _, v := range accepted {
		t.Run("accept "+v, func(t *testing.T) {
			run := validRun()
			run.Version = v
			if err := run.Validate(); err != nil {
				t.Errorf("version %q should be accepted, got: %v", v, err)
			}
		})
	}

	rejected := []struct {
		v       string
		wantMsg string
	}{
		{"", "version is empty"},
		{"1.0.0", "unsupported"},
		{"2.3.4", "unsupported"},
		{"0.1", "not semver"},
		{"v0.1.0", "not semver"},
		{"0.1.x", "not semver"},
	}
	for _, tc := range rejected {
		t.Run("reject "+tc.v, func(t *testing.T) {
			run := validRun()
			run.Version = tc.v
			err := run.Validate()
			if err == nil {
				t.Fatalf("version %q should be rejected", tc.v)
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Errorf("version %q: expected error containing %q, got: %v", tc.v, tc.wantMsg, err)
			}
		})
	}
}

// TestValidate_LenientOptionalFields pins the remaining pragmatic leniency: an
// empty step list and zero quantities are accepted, since they carry no
// structural ambiguity (a run with no steps simply has nothing to evaluate).
func TestValidate_LenientOptionalFields(t *testing.T) {
	t.Run("empty steps", func(t *testing.T) {
		run := validRun()
		run.Steps = nil
		if err := run.Validate(); err != nil {
			t.Errorf("empty step list should be accepted, got: %v", err)
		}
	})
	t.Run("zero cost and tokens", func(t *testing.T) {
		run := validRun()
		for i := range run.Steps {
			run.Steps[i].Cost = 0
			run.Steps[i].InputTokens = 0
			run.Steps[i].OutputTokens = 0
			run.Steps[i].DurationMs = 0
		}
		if err := run.Validate(); err != nil {
			t.Errorf("zero quantities should be accepted, got: %v", err)
		}
	})
}

// TestValidate_MissingTimestampSkipsOrdering guards the monotonicity check:
// a step with a missing timestamp is flagged as missing but must not produce
// a spurious ordering error against the following step.
func TestValidate_MissingTimestampSkipsOrdering(t *testing.T) {
	run := validRun()
	run.Steps[1].Timestamp = time.Time{}
	err := run.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "step 1: timestamp is missing") {
		t.Errorf("expected missing-timestamp error, got: %v", err)
	}
	if strings.Contains(err.Error(), "is before step") {
		t.Errorf("missing timestamp must not trigger an ordering error, got: %v", err)
	}
}

func TestValidate_CollectsAllErrors(t *testing.T) {
	run := validRun()
	run.ID = ""
	run.Agent = ""
	run.Steps[2].Tool = ""

	err := run.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	for _, want := range []string{"id is empty", "agent is empty", "step 2: tool_call without tool"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected joined error to contain %q, got: %v", want, err)
		}
	}
}
