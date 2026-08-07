package evaluator

import (
	"context"
	"testing"
	"time"

	"github.com/Cro22/trazo/trajectory"
)

var costBase = time.Date(2026, 8, 6, 10, 0, 0, 0, time.UTC)

func llmStep(cost float64, durationMs int64) trajectory.Step {
	return trajectory.Step{
		Type:       trajectory.StepTypeCallLLM,
		LLM:        "m",
		Cost:       cost,
		DurationMs: durationMs,
	}
}

func costRun(durationS int, steps ...trajectory.Step) *trajectory.Run {
	return &trajectory.Run{
		ID:        "run-cost",
		StartTime: costBase,
		EndTime:   costBase.Add(time.Duration(durationS) * time.Second),
		Steps:     steps,
	}
}

func countJudgment(eval *Evaluation, j Judgment) int {
	n := 0
	for _, f := range eval.Findings {
		if f.Judgment == j {
			n++
		}
	}
	return n
}

func TestCostLatency_StepCostOverBudget(t *testing.T) {
	run := costRun(1, llmStep(0.10, 100))
	eval, err := (&CostLatencyEvaluator{}).EvaluateRun(context.Background(), run)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(eval.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(eval.Findings), eval.Findings)
	}
	if eval.Findings[0].StepIndex != 0 || eval.Findings[0].Judgment != JudgmentNeutral {
		t.Errorf("unexpected finding: %+v", eval.Findings[0])
	}
}

func TestCostLatency_StepLatencyOverBudget(t *testing.T) {
	run := costRun(1, llmStep(0.001, 40000))
	eval, _ := (&CostLatencyEvaluator{}).EvaluateRun(context.Background(), run)
	if len(eval.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(eval.Findings))
	}
	if eval.Findings[0].StepIndex != 0 {
		t.Errorf("expected step 0, got %d", eval.Findings[0].StepIndex)
	}
}

func TestCostLatency_AggregateRunCost(t *testing.T) {
	// Five steps at 0.05 each: none over the per-step budget (strict >), but the
	// run total 0.25 exceeds the run budget 0.20.
	run := costRun(1,
		llmStep(0.05, 10), llmStep(0.05, 10), llmStep(0.05, 10),
		llmStep(0.05, 10), llmStep(0.05, 10),
	)
	eval, _ := (&CostLatencyEvaluator{}).EvaluateRun(context.Background(), run)
	if len(eval.Findings) != 1 {
		t.Fatalf("expected 1 run-level finding, got %d: %+v", len(eval.Findings), eval.Findings)
	}
	if eval.Findings[0].StepIndex != runLevelStep {
		t.Errorf("expected run-level (-1), got %d", eval.Findings[0].StepIndex)
	}
}

func TestCostLatency_RunLatencyOverBudget(t *testing.T) {
	run := costRun(200, llmStep(0.001, 10)) // 200s wall-clock > 120s default
	eval, _ := (&CostLatencyEvaluator{}).EvaluateRun(context.Background(), run)
	if len(eval.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(eval.Findings))
	}
	if eval.Findings[0].StepIndex != runLevelStep {
		t.Errorf("expected run-level (-1), got %d", eval.Findings[0].StepIndex)
	}
}

func TestCostLatency_WithinBudget(t *testing.T) {
	run := costRun(5, llmStep(0.001, 500), llmStep(0.002, 800))
	eval, _ := (&CostLatencyEvaluator{}).EvaluateRun(context.Background(), run)
	if len(eval.Findings) != 0 {
		t.Errorf("expected 0 findings within budget, got %d: %+v", len(eval.Findings), eval.Findings)
	}
}

func TestCostLatency_CustomThresholds(t *testing.T) {
	run := costRun(5, llmStep(0.01, 2000))
	e := &CostLatencyEvaluator{MaxStepCost: 0.005, MaxStepLatencyMs: 1000}
	eval, _ := e.EvaluateRun(context.Background(), run)
	// Both step cost and step latency breach the tighter budgets.
	if countJudgment(eval, JudgmentNeutral) != 2 {
		t.Fatalf("expected 2 neutral findings, got %d: %+v", len(eval.Findings), eval.Findings)
	}
}
