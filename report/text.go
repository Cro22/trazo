package report

import (
	"fmt"
	"strings"

	"github.com/Cro22/trazo/runner"
)

// Text renders the evaluation results as the plain-text CLI format: one header
// line per evaluation, one line per finding, then any file errors. It is the
// default CLI output and is kept deliberately terse and greppable.
func Text(resp *runner.Response) string {
	var b strings.Builder
	for _, e := range resp.Evaluations {
		fmt.Fprintf(&b, "RunID %s. Findings: %d Evaluator: %s\n", e.RunID, len(e.Findings), e.EvaluatorName)
		for _, f := range e.Findings {
			fmt.Fprintf(&b, "Step %d: Comment: %s, Score: %f, Judgment: %s\n", f.StepIndex, f.Comment, f.Score, f.Judgment)
		}
	}
	for _, fe := range resp.FileErrors {
		fmt.Fprintf(&b, "File Error: %s → %v\n", fe.File, fe.Err)
	}
	return b.String()
}
