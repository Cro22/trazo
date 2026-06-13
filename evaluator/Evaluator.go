package evaluator

import (
	"github.com/Cro22/trazo/trajectory"
)

type Evaluator interface {
	EvaluateRun(run *trajectory.Run) (*Evaluation, error)
}

type Judgment string

const (
	JudgmentGood Judgment = "good"
	// JudgmentNeutral marks an anomaly that needs human review rather than
	// immediate action: the cause may be external to the agent (a timeout, an
	// interrupted run, broken instrumentation), so it is surfaced but not blamed
	// on the trajectory itself.
	JudgmentNeutral Judgment = "JudgmentNeutral"
	// JudgmentBad marks a failure attributable to the agent and actionable right
	// away: the fault is real and present in the trajectory (for example, a tool
	// that returned an error).
	JudgmentBad Judgment = "bad"
)

type Evaluation struct {
	EvaluatorName string    `json:"evaluatorName"`
	RunID         string    `json:"runId"`
	Findings      []Finding `json:"findings"`
}

type Finding struct {
	StepIndex int      `json:"stepIndex"`
	Judgment  Judgment `json:"judgment"`
	Score     float64  `json:"score,omitempty"`
	Comment   string   `json:"comment"`
}
