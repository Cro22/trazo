package evaluator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Cro22/trazo/trajectory"
)

// TestEvaluators_HonorCancelledContext pins the interface contract added in the
// context.Context milestone: every evaluator must return ctx.Err() promptly when
// handed an already-cancelled context, rather than doing the work anyway. The
// LLM judge uses a fake client so no network is touched.
func TestEvaluators_HonorCancelledContext(t *testing.T) {
	start := time.Date(2026, 8, 7, 9, 0, 0, 0, time.UTC)
	run := &trajectory.Run{
		ID:        "run-ctx",
		Agent:     "ctx_probe",
		Version:   "1.0.0",
		StartTime: start,
		EndTime:   start.Add(2 * time.Second),
		Steps: []trajectory.Step{
			{Node: "start", Type: trajectory.StepTypeNodeTransition, Timestamp: start},
			{LLM: "gpt-4o", Type: trajectory.StepTypeCallLLM, Timestamp: start.Add(time.Second)},
		},
	}

	evaluators := map[string]Evaluator{
		"tool_calls":       &ToolCallEvaluator{},
		"loops":            &LoopEvaluator{},
		"cost_latency":     &CostLatencyEvaluator{},
		"node_transitions": &NodeTransitionEvaluator{},
		"llm_judge":        &LLMJudgeEvaluator{Client: fakeJudge{reply: "unused"}},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	for name, e := range evaluators {
		t.Run(name, func(t *testing.T) {
			_, err := e.EvaluateRun(ctx, run)
			if !errors.Is(err, context.Canceled) {
				t.Errorf("expected context.Canceled, got %v", err)
			}
		})
	}
}
