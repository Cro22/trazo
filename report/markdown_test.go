package report

import (
	"errors"
	"strings"
	"testing"

	"github.com/Cro22/trazo/evaluator"
	"github.com/Cro22/trazo/runner"
)

func TestMarkdown_WithFindingsFails(t *testing.T) {
	resp := &runner.Response{
		Evaluations: []*evaluator.Evaluation{
			{
				EvaluatorName: "tool_calls",
				RunID:         "run-1",
				Findings: []evaluator.Finding{
					{StepIndex: 3, Judgment: evaluator.JudgmentBad, Comment: "tool x fails: boom"},
				},
			},
			{
				EvaluatorName: "cost_latency",
				RunID:         "run-1",
				Findings: []evaluator.Finding{
					{StepIndex: -1, Judgment: evaluator.JudgmentNeutral, Comment: "run: cost too high"},
				},
			},
		},
	}
	md := Markdown(resp)

	if !strings.Contains(md, "**Verdict: FAIL**") {
		t.Errorf("expected FAIL verdict, got:\n%s", md)
	}
	if !strings.Contains(md, "bad 1, neutral 1, good 0") {
		t.Errorf("expected counts line, got:\n%s", md)
	}
	if !strings.Contains(md, "| run-1 | tool_calls | 3 | bad | tool x fails: boom |") {
		t.Errorf("expected findings row, got:\n%s", md)
	}
	// Run-level finding renders the step as "run".
	if !strings.Contains(md, "| run-1 | cost_latency | run | neutral |") {
		t.Errorf("expected run-level step label, got:\n%s", md)
	}
}

func TestMarkdown_CleanPasses(t *testing.T) {
	resp := &runner.Response{
		Evaluations: []*evaluator.Evaluation{
			{EvaluatorName: "tool_calls", RunID: "run-ok", Findings: []evaluator.Finding{}},
		},
	}
	md := Markdown(resp)
	if !strings.Contains(md, "**Verdict: PASS**") {
		t.Errorf("expected PASS, got:\n%s", md)
	}
	if !strings.Contains(md, "No findings. All clean.") {
		t.Errorf("expected clean message, got:\n%s", md)
	}
}

func TestMarkdown_FileErrorsFail(t *testing.T) {
	resp := &runner.Response{
		FileErrors: []runner.FileError{
			{File: "broken.json", Err: errors.New("unexpected end of JSON input")},
		},
	}
	md := Markdown(resp)
	if !strings.Contains(md, "**Verdict: FAIL**") {
		t.Errorf("expected FAIL on file errors, got:\n%s", md)
	}
	if !strings.Contains(md, "`broken.json`: unexpected end of JSON input") {
		t.Errorf("expected file error line, got:\n%s", md)
	}
}

func TestMarkdown_EscapesPipesAndNewlines(t *testing.T) {
	resp := &runner.Response{
		Evaluations: []*evaluator.Evaluation{
			{
				EvaluatorName: "e",
				RunID:         "r",
				Findings: []evaluator.Finding{
					{StepIndex: 0, Judgment: evaluator.JudgmentBad, Comment: "a|b\nc"},
				},
			},
		},
	}
	md := Markdown(resp)
	if !strings.Contains(md, `a\|b; c`) {
		t.Errorf("expected escaped comment, got:\n%s", md)
	}
}
