package report

import (
	"fmt"
	"strings"

	"github.com/Cro22/trazo/evaluator"
	"github.com/Cro22/trazo/runner"
)

// Text renders the evaluation results as a human-facing report: findings grouped
// by run with the severity up front and aligned columns, runs with no findings
// marked "clean", a one-line summary, and any file errors last. For
// machine-readable output use JSON instead.
func Text(resp *runner.Response) string {
	groups, order := groupByRun(resp)
	rows := allRows(groups, order)
	sevW, stepW, evalW := columnWidths(rows)

	var b strings.Builder
	for _, runID := range order {
		g := groups[runID]
		b.WriteString(runHeader(runID, g.agent))
		if len(g.rows) == 0 {
			b.WriteString("  clean\n")
			b.WriteString("\n")
			continue
		}
		b.WriteString("\n")
		for _, r := range g.rows {
			fmt.Fprintf(&b, "  %-*s  %-*s  %-*s  %s\n",
				sevW, r.severity, stepW, r.step, evalW, r.evaluator, r.comment)
		}
		b.WriteString("\n")
	}

	b.WriteString(summaryLine(resp))
	if len(resp.FileErrors) > 0 {
		b.WriteString("\nFile errors:\n")
		for _, fe := range resp.FileErrors {
			b.WriteString(fileErrorLine(fe))
		}
	}
	return b.String()
}

type textRow struct {
	severity  string
	step      string
	evaluator string
	comment   string
}

type runGroup struct {
	agent string
	rows  []textRow
}

// groupByRun collects findings across every evaluator into one group per run,
// preserving the first-seen run order so output is deterministic.
func groupByRun(resp *runner.Response) (map[string]*runGroup, []string) {
	groups := map[string]*runGroup{}
	var order []string
	for _, e := range resp.Evaluations {
		g, ok := groups[e.RunID]
		if !ok {
			g = &runGroup{agent: e.Agent}
			groups[e.RunID] = g
			order = append(order, e.RunID)
		}
		if g.agent == "" {
			g.agent = e.Agent
		}
		for _, f := range e.Findings {
			g.rows = append(g.rows, textRow{
				severity:  severityTag(f.Judgment),
				step:      stepText(f.StepIndex),
				evaluator: e.EvaluatorName,
				comment:   oneline(f.Comment),
			})
		}
	}
	return groups, order
}

func allRows(groups map[string]*runGroup, order []string) []textRow {
	var rows []textRow
	for _, runID := range order {
		rows = append(rows, groups[runID].rows...)
	}
	return rows
}

func columnWidths(rows []textRow) (sev, step, eval int) {
	for _, r := range rows {
		sev = maxInt(sev, len(r.severity))
		step = maxInt(step, len(r.step))
		eval = maxInt(eval, len(r.evaluator))
	}
	return sev, step, eval
}

func runHeader(runID, agent string) string {
	if agent == "" {
		return runID
	}
	return fmt.Sprintf("%s (%s)", runID, agent)
}

func severityTag(j evaluator.Judgment) string {
	switch j {
	case evaluator.JudgmentBad:
		return "[BAD]"
	case evaluator.JudgmentNeutral:
		return "[NEUTRAL]"
	case evaluator.JudgmentGood:
		return "[GOOD]"
	default:
		return "[" + strings.ToUpper(string(j)) + "]"
	}
}

// stepText renders the step index, using "run" for the run-level sentinel (-1).
func stepText(idx int) string {
	if idx < 0 {
		return "run"
	}
	return fmt.Sprintf("step %d", idx)
}

func summaryLine(resp *runner.Response) string {
	s := Summarize(resp, 0)
	return fmt.Sprintf("Summary: %s, %d bad, %d neutral, %d good, %s\n",
		plural(s.Runs, "run"), s.Bad, s.Neutral, s.Good, plural(s.FileErrors, "file error"))
}

// oneline flattens a possibly multi-line message onto a single line so table
// rows stay aligned.
func oneline(s string) string {
	return strings.ReplaceAll(s, "\n", "; ")
}

// fileErrorLine renders one file error, tagging it with its kind when known so
// the reader can tell a parse error from an invalid trace at a glance.
func fileErrorLine(fe runner.FileError) string {
	msg := oneline(fe.Err.Error())
	if fe.Kind != "" {
		return fmt.Sprintf("  %s [%s]: %s\n", fe.File, fe.Kind, msg)
	}
	return fmt.Sprintf("  %s: %s\n", fe.File, msg)
}

func plural(n int, unit string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, unit)
	}
	return fmt.Sprintf("%d %ss", n, unit)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
