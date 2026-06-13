package evaluator

import (
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
	eval, err := evaluator.EvaluateRun(run)
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
	eval, err := evaluator.EvaluateRun(run)
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
	eval, err := evaluator.EvaluateRun(run)
	if err != nil {
		t.Fatalf("unexpected error in EvaluateRun: %v", err)
	}

	if len(eval.Findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(eval.Findings))
	} else if eval.Findings[0].StepIndex != 1 {
		t.Errorf("expected StepIndex 1, got %d", eval.Findings[0].StepIndex)
	}
}
