package runner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Cro22/trazo/evaluator"
)

func TestRunner_Run(t *testing.T) {
	evals := []evaluator.Evaluator{&evaluator.ToolCallEvaluator{}}
	runner := NewRunner(evals)

	resp, err := runner.Run(context.Background(), "../testdata/runs")
	if err != nil {
		t.Fatalf("unexpected error in Run: %v", err)
	}

	// Two valid fixtures (sample_run.json, sample_run_ok.json) with one
	// evaluator each => 2 evaluations.
	if len(resp.Evaluations) != 2 {
		t.Errorf("expected 2 evaluations, got %d", len(resp.Evaluations))
	}

	// broken.json is malformed and invalid_run.json fails Validate => both land
	// in FileErrors, not aborting the batch. ReadDir returns names sorted, and
	// FileError.File carries the path as read (joined with the input dir).
	if len(resp.FileErrors) != 2 {
		t.Fatalf("expected 2 file errors, got %d", len(resp.FileErrors))
	}
	wantBroken := filepath.Join("../testdata/runs", "broken.json")
	wantInvalid := filepath.Join("../testdata/runs", "invalid_run.json")
	if resp.FileErrors[0].File != wantBroken {
		t.Errorf("expected first file error on %s, got %s", wantBroken, resp.FileErrors[0].File)
	}
	if resp.FileErrors[1].File != wantInvalid {
		t.Errorf("expected second file error on %s, got %s", wantInvalid, resp.FileErrors[1].File)
	}
}

func TestCollectFiles_NonRecursiveSkipsSubdirs(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.json"), "{}")
	writeFile(t, filepath.Join(dir, "notes.txt"), "ignore me")
	sub := filepath.Join(dir, "nested")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, filepath.Join(sub, "b.json"), "{}")

	files, err := CollectFiles(dir, false)
	if err != nil {
		t.Fatalf("CollectFiles: %v", err)
	}
	if len(files) != 1 || files[0] != filepath.Join(dir, "a.json") {
		t.Fatalf("non-recursive should return only a.json, got %v", files)
	}
}

func TestCollectFiles_RecursiveDescends(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.json"), "{}")
	sub := filepath.Join(dir, "nested")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, filepath.Join(sub, "b.json"), "{}")

	files, err := CollectFiles(dir, true)
	if err != nil {
		t.Fatalf("CollectFiles: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("recursive should find both files, got %v", files)
	}
	// WalkDir yields lexical order: the top-level a.json before nested/b.json.
	if files[0] != filepath.Join(dir, "a.json") || files[1] != filepath.Join(sub, "b.json") {
		t.Fatalf("recursive order unexpected: %v", files)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// TestRunner_Run_ContextCancelled asserts that an already-cancelled context
// short-circuits every file: no evaluations are produced and each eligible file
// surfaces a context.Canceled error instead of being evaluated.
func TestRunner_Run_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	runner := NewRunner([]evaluator.Evaluator{&evaluator.ToolCallEvaluator{}})
	resp, err := runner.Run(ctx, "../testdata/runs")
	if err != nil {
		t.Fatalf("Run itself should not error on cancellation, got: %v", err)
	}
	if len(resp.Evaluations) != 0 {
		t.Errorf("expected no evaluations under a cancelled context, got %d", len(resp.Evaluations))
	}
	if len(resp.FileErrors) == 0 {
		t.Fatal("expected file errors under a cancelled context, got none")
	}
	for _, fe := range resp.FileErrors {
		if !errors.Is(fe.Err, context.Canceled) {
			t.Errorf("file %s: expected context.Canceled, got %v", fe.File, fe.Err)
		}
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
		resp, err := runner.Run(context.Background(), dir)
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
