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

	// broken.json is malformed and invalid_run.json fails Validate => both land
	// in FileErrors, not aborting the batch. ReadDir returns names sorted.
	if len(resp.FileErrors) != 2 {
		t.Fatalf("expected 2 file errors, got %d", len(resp.FileErrors))
	}
	if resp.FileErrors[0].File != "broken.json" {
		t.Errorf("expected first file error on broken.json, got %s", resp.FileErrors[0].File)
	}
	if resp.FileErrors[1].File != "invalid_run.json" {
		t.Errorf("expected second file error on invalid_run.json, got %s", resp.FileErrors[1].File)
	}
}
