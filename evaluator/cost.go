package evaluator

import (
	"fmt"

	"github.com/Cro22/trazo/trajectory"
)

// Default thresholds for CostLatencyEvaluator. All are overridable per field.
const (
	DefaultMaxStepCost      = 0.05   // USD, a single step
	DefaultMaxStepLatencyMs = 30000  // 30s, a single step
	DefaultMaxRunCost       = 0.20   // USD, whole run
	DefaultMaxRunLatencyMs  = 120000 // 120s, whole run wall-clock
)

// runLevelStep marks a finding that is about the run as a whole rather than a
// single step.
const runLevelStep = -1

// CostLatencyEvaluator flags steps and runs that exceed cost or latency budgets.
// Findings are neutral: a breach is a warning for review, and both cost (large
// inputs, upstream pricing) and latency (a slow API) can have causes outside the
// agent's control. Step latency uses the step's durationMs; run latency uses the
// run wall-clock (endTime - startTime); run cost is the sum of step costs.
type CostLatencyEvaluator struct {
	MaxStepCost      float64
	MaxStepLatencyMs int64
	MaxRunCost       float64
	MaxRunLatencyMs  int64
}

func (e *CostLatencyEvaluator) maxStepCost() float64 {
	if e.MaxStepCost <= 0 {
		return DefaultMaxStepCost
	}
	return e.MaxStepCost
}

func (e *CostLatencyEvaluator) maxStepLatencyMs() int64 {
	if e.MaxStepLatencyMs <= 0 {
		return DefaultMaxStepLatencyMs
	}
	return e.MaxStepLatencyMs
}

func (e *CostLatencyEvaluator) maxRunCost() float64 {
	if e.MaxRunCost <= 0 {
		return DefaultMaxRunCost
	}
	return e.MaxRunCost
}

func (e *CostLatencyEvaluator) maxRunLatencyMs() int64 {
	if e.MaxRunLatencyMs <= 0 {
		return DefaultMaxRunLatencyMs
	}
	return e.MaxRunLatencyMs
}

func (e *CostLatencyEvaluator) EvaluateRun(run *trajectory.Run) (*Evaluation, error) {
	eva := &Evaluation{
		EvaluatorName: "cost_latency",
		RunID:         run.ID,
		Findings:      []Finding{},
	}

	stepCostLimit := e.maxStepCost()
	stepLatencyLimit := e.maxStepLatencyMs()

	var totalCost float64
	for i, step := range run.Steps {
		totalCost += step.Cost
		if step.Cost > stepCostLimit {
			eva.Findings = append(eva.Findings, Finding{
				StepIndex: i,
				Judgment:  JudgmentNeutral,
				Comment:   fmt.Sprintf("step cost $%.4f exceeds budget $%.4f", step.Cost, stepCostLimit),
			})
		}
		if step.DurationMs > stepLatencyLimit {
			eva.Findings = append(eva.Findings, Finding{
				StepIndex: i,
				Judgment:  JudgmentNeutral,
				Comment:   fmt.Sprintf("step latency %dms exceeds budget %dms", step.DurationMs, stepLatencyLimit),
			})
		}
	}

	if totalCost > e.maxRunCost() {
		eva.Findings = append(eva.Findings, Finding{
			StepIndex: runLevelStep,
			Judgment:  JudgmentNeutral,
			Comment:   fmt.Sprintf("run: total cost $%.4f exceeds budget $%.4f", totalCost, e.maxRunCost()),
		})
	}

	runMs := run.EndTime.Sub(run.StartTime).Milliseconds()
	if runMs > e.maxRunLatencyMs() {
		eva.Findings = append(eva.Findings, Finding{
			StepIndex: runLevelStep,
			Judgment:  JudgmentNeutral,
			Comment:   fmt.Sprintf("run: duration %dms exceeds budget %dms", runMs, e.maxRunLatencyMs()),
		})
	}

	return eva, nil
}
