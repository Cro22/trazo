package evaluator

import (
	"context"
	"os"
	"testing"

	"github.com/Cro22/trazo/trajectory"
)

func TestToolCallEvaluator_EvaluateRun(t *testing.T) {
	data, err := os.ReadFile("../testdata/sample_run.json")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	run, err := trajectory.LoadRun(data)
	if err != nil {
		t.Fatalf("failed to load run: %v", err)
	}

	evaluator := &ToolCallEvaluator{}
	eval, err := evaluator.EvaluateRun(context.Background(), run)
	if err != nil {
		t.Fatalf("unexpected error in EvaluateRun: %v", err)
	}

	if len(eval.Findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(eval.Findings))
	} else if eval.Findings[0].StepIndex != 3 {
		t.Errorf("expected StepIndex 3, got %d", eval.Findings[0].StepIndex)
	}
}

func TestToolCallEvaluator_Success(t *testing.T) {
	data, err := os.ReadFile("../testdata/sample_run_ok.json")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	run, err := trajectory.LoadRun(data)
	if err != nil {
		t.Fatalf("failed to load run: %v", err)
	}

	evaluator := &ToolCallEvaluator{}
	eval, err := evaluator.EvaluateRun(context.Background(), run)
	if err != nil {
		t.Fatalf("unexpected error in EvaluateRun: %v", err)
	}

	if len(eval.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(eval.Findings))
	}
}

func TestToolCallEvaluator_MissingResult(t *testing.T) {
	data, err := os.ReadFile("../testdata/sample_run_missing_result.json")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	run, err := trajectory.LoadRun(data)
	if err != nil {
		t.Fatalf("failed to load run: %v", err)
	}

	evaluator := &ToolCallEvaluator{}
	eval, err := evaluator.EvaluateRun(context.Background(), run)
	if err != nil {
		t.Fatalf("unexpected error in EvaluateRun: %v", err)
	}

	if len(eval.Findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(eval.Findings))
	} else if eval.Findings[0].StepIndex != 1 {
		t.Errorf("expected StepIndex 1, got %d", eval.Findings[0].StepIndex)
	}
}

func idToolCall(tool, id string) trajectory.Step {
	return trajectory.Step{Type: trajectory.StepTypeToolCall, Tool: tool, ToolCallID: id}
}

func idToolResult(tool, id string) trajectory.Step {
	return trajectory.Step{Type: trajectory.StepTypeToolResult, Tool: tool, ToolCallID: id}
}

func TestToolCallEvaluator_PairsByIDOutOfOrder(t *testing.T) {
	// Two calls to the same tool, results returned in the opposite order. By id
	// each result still finds its own call, so there are no orphans.
	run := &trajectory.Run{ID: "r", Steps: []trajectory.Step{
		idToolCall("search", "c1"),
		idToolCall("search", "c2"),
		idToolResult("search", "c2"),
		idToolResult("search", "c1"),
	}}
	eval, _ := (&ToolCallEvaluator{}).EvaluateRun(context.Background(), run)
	if len(eval.Findings) != 0 {
		t.Fatalf("expected 0 findings with id pairing, got %d: %+v", len(eval.Findings), eval.Findings)
	}
}

func TestToolCallEvaluator_ResultWithUnknownIDIsOrphan(t *testing.T) {
	// The result's id matches no pending call: it is an orphan, and the call is
	// left unanswered. The id is authoritative, so there is no name fallback.
	run := &trajectory.Run{ID: "r", Steps: []trajectory.Step{
		idToolCall("t", "c1"),
		idToolResult("t", "c2"),
	}}
	eval, _ := (&ToolCallEvaluator{}).EvaluateRun(context.Background(), run)
	if len(eval.Findings) != 2 {
		t.Fatalf("expected 2 neutral findings (orphan result + unanswered call), got %d: %+v",
			len(eval.Findings), eval.Findings)
	}
	for _, f := range eval.Findings {
		if f.Judgment != JudgmentNeutral {
			t.Errorf("expected neutral, got %s", f.Judgment)
		}
	}
}

func TestToolCallEvaluator_IDMatchWinsOverName(t *testing.T) {
	// Even with different tool names, a matching id pairs them.
	run := &trajectory.Run{ID: "r", Steps: []trajectory.Step{
		idToolCall("a", "x"),
		idToolResult("b", "x"),
	}}
	eval, _ := (&ToolCallEvaluator{}).EvaluateRun(context.Background(), run)
	if len(eval.Findings) != 0 {
		t.Fatalf("expected id match to pair across names, got %d: %+v", len(eval.Findings), eval.Findings)
	}
}

func TestToolCallEvaluator_OrphanResult(t *testing.T) {
	data, err := os.ReadFile("../testdata/sample_run_orphan_result.json")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	run, err := trajectory.LoadRun(data)
	if err != nil {
		t.Fatalf("failed to load run: %v", err)
	}

	evaluator := &ToolCallEvaluator{}
	eval, err := evaluator.EvaluateRun(context.Background(), run)
	if err != nil {
		t.Fatalf("unexpected error in EvaluateRun: %v", err)
	}

	if len(eval.Findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(eval.Findings))
	} else if eval.Findings[0].StepIndex != 2 {
		t.Errorf("expected StepIndex 2, got %d", eval.Findings[0].StepIndex)
	}
}
