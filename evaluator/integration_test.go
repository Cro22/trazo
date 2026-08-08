package evaluator

import (
	"context"
	"os"
	"testing"

	"github.com/Cro22/trazo/trajectory"
)

// TestComplexRun_AllEvaluators exercises a single realistic trace against the
// full structural evaluator set: interleaved tool calls paired by id (no
// orphans), a tool error, a high-cost step, and a node visited enough times to
// look like a loop. It documents how the evaluators divide the work on one trace.
func TestComplexRun_AllEvaluators(t *testing.T) {
	data, err := os.ReadFile("../testdata/complex_run.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	run, err := trajectory.LoadRun(data)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if err := run.Validate(); err != nil {
		t.Fatalf("fixture must be a valid trace: %v", err)
	}

	ctx := context.Background()

	// tool_calls: exactly one bad (the fetch 429), and no orphans, because the
	// interleaved search/fetch calls each pair with their result by id.
	tc, _ := (&ToolCallEvaluator{}).EvaluateRun(ctx, run)
	if got := countBy(tc, JudgmentBad); got != 1 {
		t.Errorf("tool_calls: expected 1 bad, got %d (%+v)", got, tc.Findings)
	}
	if got := countBy(tc, JudgmentNeutral); got != 0 {
		t.Errorf("tool_calls: expected 0 neutral (no orphans), got %d (%+v)", got, tc.Findings)
	}

	// loops: the node "retry" is visited three times, hitting the default limit.
	lp, _ := (&LoopEvaluator{}).EvaluateRun(ctx, run)
	if got := countBy(lp, JudgmentBad); got != 1 {
		t.Errorf("loops: expected 1 bad, got %d (%+v)", got, lp.Findings)
	}

	// cost_latency: the llm step costs 0.10, over the default per-step budget. A
	// cost overrun is neutral, not bad: it needs review but is not necessarily the
	// agent's fault.
	cl, _ := (&CostLatencyEvaluator{
		MaxStepCost:      DefaultMaxStepCost,
		MaxStepLatencyMs: DefaultMaxStepLatencyMs,
		MaxRunCost:       DefaultMaxRunCost,
		MaxRunLatencyMs:  DefaultMaxRunLatencyMs,
	}).EvaluateRun(ctx, run)
	if countBy(cl, JudgmentNeutral) < 1 {
		t.Errorf("cost_latency: expected at least 1 neutral, got %+v", cl.Findings)
	}

	// node_transitions: the run ends at the terminal node "end", so nothing here.
	nt, _ := (&NodeTransitionEvaluator{}).EvaluateRun(ctx, run)
	if len(nt.Findings) != 0 {
		t.Errorf("node_transitions: expected 0 findings (ends at terminal), got %+v", nt.Findings)
	}
}

func countBy(e *Evaluation, j Judgment) int {
	n := 0
	for _, f := range e.Findings {
		if f.Judgment == j {
			n++
		}
	}
	return n
}
