package evaluator

import (
	"context"

	"github.com/Cro22/trazo/trajectory"
)

// Evaluator judges a single run and reports findings. Implementations must
// honor ctx: return ctx.Err() promptly if it is cancelled (the LLM judge, for
// instance, ties its network call to ctx so a cancelled run does not hang).
type Evaluator interface {
	EvaluateRun(ctx context.Context, run *trajectory.Run) (*Evaluation, error)
}

type Judgment string

const (
	// JudgmentGood marks behavior that is correct and expected: the agent did
	// what it should at this step, so the finding is positive and needs no action.
	JudgmentGood Judgment = "good"
	// JudgmentNeutral marks an anomaly that needs human review rather than
	// immediate action: the cause may be external to the agent (a timeout, an
	// interrupted run, broken instrumentation), so it is surfaced but not blamed
	// on the trajectory itself.
	JudgmentNeutral Judgment = "neutral"
	// JudgmentBad marks a failure attributable to the agent and actionable right
	// away: the fault is real and present in the trajectory (for example, a tool
	// that returned an error).
	JudgmentBad Judgment = "bad"
)

type Evaluation struct {
	EvaluatorName string `json:"evaluatorName"`
	RunID         string `json:"runId"`
	// Agent is the logical agent name of the run, copied from the trace by the
	// runner for human-facing output. It is not serialized (json:"-") so the
	// machine-readable JSON output stays keyed on runId alone.
	Agent    string    `json:"-"`
	Findings []Finding `json:"findings"`
}

type Finding struct {
	StepIndex int      `json:"stepIndex"`
	Judgment  Judgment `json:"judgment"`
	Score     float64  `json:"score,omitempty"`
	Comment   string   `json:"comment"`
}
