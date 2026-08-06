// Package report renders a runner Response as human-facing formats. Markdown is
// meant for CI job summaries and PR comments; the JSON form lives in the CLI.
package report

import (
	"fmt"
	"strings"

	"github.com/Cro22/trazo/evaluator"
	"github.com/Cro22/trazo/runner"
)

// Markdown renders the evaluation results as a Markdown document: a verdict, a
// summary, a findings table, and any file errors.
func Markdown(resp *runner.Response) string {
	bad, neutral, good := counts(resp)
	total := bad + neutral + good

	var b strings.Builder
	b.WriteString("# trazo evaluation report\n\n")

	verdict := "PASS"
	if bad > 0 || len(resp.FileErrors) > 0 {
		verdict = "FAIL"
	}
	fmt.Fprintf(&b, "**Verdict: %s**\n\n", verdict)
	fmt.Fprintf(&b, "- Evaluations: %d\n", len(resp.Evaluations))
	fmt.Fprintf(&b, "- Findings: %d (bad %d, neutral %d, good %d)\n", total, bad, neutral, good)
	fmt.Fprintf(&b, "- File errors: %d\n\n", len(resp.FileErrors))

	if total > 0 {
		b.WriteString("## Findings\n\n")
		b.WriteString("| Run | Evaluator | Step | Severity | Comment |\n")
		b.WriteString("|-----|-----------|------|----------|---------|\n")
		for _, e := range resp.Evaluations {
			for _, f := range e.Findings {
				fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n",
					e.RunID, e.EvaluatorName, stepLabel(f.StepIndex), f.Judgment, cell(f.Comment))
			}
		}
		b.WriteString("\n")
	}

	if len(resp.FileErrors) > 0 {
		b.WriteString("## File errors\n\n")
		for _, fe := range resp.FileErrors {
			fmt.Fprintf(&b, "- `%s`: %s\n", fe.File, cell(fe.Err.Error()))
		}
		b.WriteString("\n")
	}

	if total == 0 && len(resp.FileErrors) == 0 {
		b.WriteString("No findings. All clean.\n")
	}
	return b.String()
}

func counts(resp *runner.Response) (bad, neutral, good int) {
	for _, e := range resp.Evaluations {
		for _, f := range e.Findings {
			switch f.Judgment {
			case evaluator.JudgmentBad:
				bad++
			case evaluator.JudgmentNeutral:
				neutral++
			case evaluator.JudgmentGood:
				good++
			}
		}
	}
	return bad, neutral, good
}

// stepLabel renders the step index, using "run" for the run-level sentinel (-1).
func stepLabel(idx int) string {
	if idx < 0 {
		return "run"
	}
	return fmt.Sprintf("%d", idx)
}

// cell makes a string safe for a Markdown table cell: single line, escaped pipes.
func cell(s string) string {
	s = strings.ReplaceAll(s, "\n", "; ")
	s = strings.ReplaceAll(s, "|", "\\|")
	return s
}
