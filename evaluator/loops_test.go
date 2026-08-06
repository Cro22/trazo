package evaluator

import (
	"encoding/json"
	"testing"

	"github.com/Cro22/trazo/trajectory"
)

func toolCall(tool, input string) trajectory.Step {
	return trajectory.Step{
		Type:  trajectory.StepTypeToolCall,
		Tool:  tool,
		Input: json.RawMessage(input),
	}
}

func nodeStep(node string) trajectory.Step {
	return trajectory.Step{Type: trajectory.StepTypeNodeTransition, Node: node}
}

func runWith(steps ...trajectory.Step) *trajectory.Run {
	return &trajectory.Run{ID: "run-loop", Steps: steps}
}

func TestLoopEvaluator_RepeatedToolCallFlagged(t *testing.T) {
	run := runWith(
		toolCall("fetch_issues", `{"repo":"x/y"}`),
		toolCall("fetch_issues", `{"repo":"x/y"}`),
		toolCall("fetch_issues", `{"repo":"x/y"}`),
	)
	eval, err := (&LoopEvaluator{}).EvaluateRun(run)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(eval.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(eval.Findings))
	}
	if eval.Findings[0].StepIndex != 2 {
		t.Errorf("expected finding at step 2 (third repeat), got %d", eval.Findings[0].StepIndex)
	}
	if eval.Findings[0].Judgment != JudgmentBad {
		t.Errorf("expected bad, got %s", eval.Findings[0].Judgment)
	}
}

func TestLoopEvaluator_DistinctInputsNotFlagged(t *testing.T) {
	// classify_issue called three times with different titles is legitimate
	// fan-out, not a loop.
	run := runWith(
		toolCall("classify_issue", `{"title":"a"}`),
		toolCall("classify_issue", `{"title":"b"}`),
		toolCall("classify_issue", `{"title":"c"}`),
	)
	eval, _ := (&LoopEvaluator{}).EvaluateRun(run)
	if len(eval.Findings) != 0 {
		t.Errorf("expected 0 findings for distinct inputs, got %d: %+v", len(eval.Findings), eval.Findings)
	}
}

func TestLoopEvaluator_BelowThresholdNotFlagged(t *testing.T) {
	run := runWith(
		toolCall("fetch_issues", `{"repo":"x/y"}`),
		toolCall("fetch_issues", `{"repo":"x/y"}`),
	)
	eval, _ := (&LoopEvaluator{}).EvaluateRun(run)
	if len(eval.Findings) != 0 {
		t.Errorf("expected 0 findings below threshold, got %d", len(eval.Findings))
	}
}

func TestLoopEvaluator_CustomThreshold(t *testing.T) {
	run := runWith(
		toolCall("fetch_issues", `{"repo":"x/y"}`),
		toolCall("fetch_issues", `{"repo":"x/y"}`),
	)
	eval, _ := (&LoopEvaluator{MaxRepeats: 2}).EvaluateRun(run)
	if len(eval.Findings) != 1 {
		t.Fatalf("expected 1 finding with MaxRepeats=2, got %d", len(eval.Findings))
	}
	if eval.Findings[0].StepIndex != 1 {
		t.Errorf("expected finding at step 1, got %d", eval.Findings[0].StepIndex)
	}
}

func TestLoopEvaluator_CanonicalInputMatches(t *testing.T) {
	// Same object, different key order and whitespace: still one loop key.
	run := runWith(
		toolCall("t", `{"a":1,"b":2}`),
		toolCall("t", `{ "b":2, "a":1 }`),
		toolCall("t", `{"a":1,  "b":2}`),
	)
	eval, _ := (&LoopEvaluator{}).EvaluateRun(run)
	if len(eval.Findings) != 1 {
		t.Fatalf("expected 1 finding for canonically equal inputs, got %d", len(eval.Findings))
	}
}

func TestLoopEvaluator_NodeCycleFlagged(t *testing.T) {
	run := runWith(
		nodeStep("router"),
		nodeStep("router"),
		nodeStep("router"),
	)
	eval, _ := (&LoopEvaluator{}).EvaluateRun(run)
	if len(eval.Findings) != 1 {
		t.Fatalf("expected 1 finding for node cycle, got %d", len(eval.Findings))
	}
	if eval.Findings[0].StepIndex != 2 {
		t.Errorf("expected finding at step 2, got %d", eval.Findings[0].StepIndex)
	}
}

func TestLoopEvaluator_OnlyFlagsOncePerKey(t *testing.T) {
	// Four identical calls should still produce a single finding.
	run := runWith(
		toolCall("t", `{"x":1}`),
		toolCall("t", `{"x":1}`),
		toolCall("t", `{"x":1}`),
		toolCall("t", `{"x":1}`),
	)
	eval, _ := (&LoopEvaluator{}).EvaluateRun(run)
	if len(eval.Findings) != 1 {
		t.Fatalf("expected exactly 1 finding, got %d", len(eval.Findings))
	}
}
