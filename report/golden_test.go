package report

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Cro22/trazo/evaluator"
	"github.com/Cro22/trazo/runner"
)

// update regenerates the golden files instead of comparing against them:
//
//	go test ./report -run TestGolden -update
//
// Review the diff before committing; a golden change is an intentional change to
// user-facing output.
var update = flag.Bool("update", false, "update golden files")

// goldenFixture is a single Response that exercises the interesting cases shared
// by all three renderers: multiple evaluators over one run, a scored finding, a
// run-level finding (step -1), a comment with a pipe and a newline (Markdown
// escaping), and a file error.
func goldenFixture() *runner.Response {
	return &runner.Response{
		Evaluations: []*evaluator.Evaluation{
			{
				EvaluatorName: "tool_calls",
				RunID:         "run-1",
				Agent:         "data_extractor",
				Findings: []evaluator.Finding{
					{StepIndex: 3, Judgment: evaluator.JudgmentBad, Comment: "tool postgres_query fails: connection refused"},
					{StepIndex: 2, Judgment: evaluator.JudgmentNeutral, Comment: "tool_call postgres_query has no matching result"},
				},
			},
			{
				EvaluatorName: "cost_latency",
				RunID:         "run-1",
				Agent:         "data_extractor",
				Findings: []evaluator.Finding{
					{StepIndex: -1, Judgment: evaluator.JudgmentNeutral, Score: 1.5, Comment: "run cost 1.50 over budget|limit\nsecond line"},
				},
			},
			{
				EvaluatorName: "tool_calls",
				RunID:         "run-ok",
				Agent:         "data_extractor",
				Findings:      []evaluator.Finding{},
			},
		},
		FileErrors: []runner.FileError{
			{File: "broken.json", Kind: runner.ErrorKindInvalidJSON, Err: errors.New("unexpected end of JSON input")},
		},
	}
}

func TestGolden(t *testing.T) {
	resp := goldenFixture()

	meta := Meta{
		TrazoVersion:       "0.2.0",
		TraceSchemaVersion: "0.1.0",
		Files:              4,
		GeneratedAt:        time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC),
	}
	jsonOut, err := JSON(resp, meta)
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}

	cases := map[string]string{
		"text.golden":     Text(resp),
		"json.golden":     jsonOut,
		"markdown.golden": Markdown(resp),
	}

	for name, got := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("testdata", name)
			if *update {
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatalf("write golden %s: %v", path, err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden %s: %v (run with -update to create it)", path, err)
			}
			if got != string(want) {
				t.Errorf("%s mismatch (run with -update to accept):\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
			}
		})
	}
}
