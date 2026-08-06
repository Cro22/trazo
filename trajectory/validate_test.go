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
		Version:   "1.2.0",
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
