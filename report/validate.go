package report

import (
	"fmt"
	"strings"

	"github.com/Cro22/trazo/runner"
)

// ValidateSummary renders the outcome of a validate-only run: how many trace
// files were checked, how many passed structural validation, and the details of
// each that failed. total is the number of files considered, which the runner
// Response alone does not carry (valid files produce no evaluations in this
// mode).
func ValidateSummary(resp *runner.Response, total int) string {
	invalid := len(resp.FileErrors)
	valid := total - invalid
	if valid < 0 {
		valid = 0
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Validated %d file(s): %d valid, %d invalid.\n", total, valid, invalid)
	for _, fe := range resp.FileErrors {
		b.WriteString(fileErrorLine(fe))
	}
	return b.String()
}
