package evaluator

import (
	"github.com/Cro22/trazo/trajectory"
)

type Evaluator interface {
	EvaluateRun(run *trajectory.Run) (*Evaluation, error)
}

type Judgment string

const (
	JudgmentGood    Judgment = "good"
	JudgmentNeutral Judgment = "JudgmentNeutral"
	JudgmentBad     Judgment = "bad"
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
