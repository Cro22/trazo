package runner

import (
	"fmt"
	"os"
	"path/filepath"
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

// TestRunner_Run_DeterministicOrder writes many valid runs whose ids follow the
// sorted filename order, then runs several times. Files are processed
// concurrently, so this asserts the assembled output still matches directory
// order every time (no dependence on goroutine scheduling).
func TestRunner_Run_DeterministicOrder(t *testing.T) {
	dir := t.TempDir()
	const n = 50
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("run-%03d", i)
		content := fmt.Sprintf(`{
  "id": %q,
  "agent": "stress",
  "version": "0.0.1",
  "startTime": "2026-06-12T14:00:00Z",
  "endTime": "2026-06-12T14:00:05Z",
  "steps": [
    {"tool": "t", "type": "tool_call", "timestamp": "2026-06-12T14:00:01Z", "durationMs": 1},
    {"tool": "t", "type": "tool_result", "timestamp": "2026-06-12T14:00:02Z", "durationMs": 1}
  ]
}`, id)
		name := filepath.Join(dir, fmt.Sprintf("run-%03d.json", i))
		if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
			t.Fatalf("writing fixture: %v", err)
		}
	}

	runner := NewRunner([]evaluator.Evaluator{&evaluator.ToolCallEvaluator{}})

	for attempt := 0; attempt < 5; attempt++ {
		resp, err := runner.Run(dir)
		if err != nil {
			t.Fatalf("unexpected error in Run: %v", err)
		}
		if len(resp.Evaluations) != n {
			t.Fatalf("attempt %d: expected %d evaluations, got %d", attempt, n, len(resp.Evaluations))
		}
		for i, eval := range resp.Evaluations {
			want := fmt.Sprintf("run-%03d", i)
			if eval.RunID != want {
				t.Fatalf("attempt %d: evaluation %d out of order: got %s, want %s", attempt, i, eval.RunID, want)
			}
		}
	}
}
