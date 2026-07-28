package runner

import (
	"testing"

	"github.com/Cro22/trazo/evaluator"
)

func TestRunner_Run(t *testing.T) {
	evals := []evaluator.Evaluator{&evaluator.ToolCallEvaluator{}}
	runner := NewRunner(evals)

	resp, err := runner.Run("../testdata/runs")
	if err != nil {
		t.Fatalf("unexpected error in Run: %v", err)
	}

	// Two valid fixtures (sample_run.json, sample_run_ok.json) with one
	// evaluator each => 2 evaluations.
	if len(resp.Evaluations) != 2 {
		t.Errorf("expected 2 evaluations, got %d", len(resp.Evaluations))
	}

	// broken.json is malformed => it lands in FileErrors, not aborting the batch.
	if len(resp.FileErrors) != 1 {
		t.Errorf("expected 1 file error, got %d", len(resp.FileErrors))
	} else if resp.FileErrors[0].File != "broken.json" {
		t.Errorf("expected file error on broken.json, got %s", resp.FileErrors[0].File)
	}
}
