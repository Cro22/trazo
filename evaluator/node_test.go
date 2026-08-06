package evaluator

import (
	"testing"

	"github.com/Cro22/trazo/trajectory"
)

func TestNodeTransition_EndsAtTerminal(t *testing.T) {
	run := runWith(nodeStep("start"), nodeStep("router"), nodeStep("end"))
	eval, err := (&NodeTransitionEvaluator{}).EvaluateRun(run)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(eval.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d: %+v", len(eval.Findings), eval.Findings)
	}
}

func TestNodeTransition_EndsAtNonTerminalFlagged(t *testing.T) {
	run := runWith(nodeStep("start"), nodeStep("fallback_handler"))
	eval, _ := (&NodeTransitionEvaluator{}).EvaluateRun(run)
	if len(eval.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(eval.Findings))
	}
	if eval.Findings[0].StepIndex != 1 || eval.Findings[0].Judgment != JudgmentNeutral {
		t.Errorf("unexpected finding: %+v", eval.Findings[0])
	}
}

func TestNodeTransition_CaseInsensitiveTerminal(t *testing.T) {
	run := runWith(nodeStep("START"), nodeStep("END"))
	eval, _ := (&NodeTransitionEvaluator{}).EvaluateRun(run)
	if len(eval.Findings) != 0 {
		t.Errorf("expected 0 findings for END, got %d", len(eval.Findings))
	}
}

func TestNodeTransition_NoNodeTransitions(t *testing.T) {
	run := runWith(
		trajectory.Step{Type: trajectory.StepTypeToolCall, Tool: "t"},
	)
	eval, _ := (&NodeTransitionEvaluator{}).EvaluateRun(run)
	if len(eval.Findings) != 0 {
		t.Errorf("expected 0 findings when there are no node transitions, got %d", len(eval.Findings))
	}
}

func TestNodeTransition_IgnoresTrailingNonNodeSteps(t *testing.T) {
	// A terminal node followed by non-node steps still counts as ending terminal.
	run := runWith(
		nodeStep("start"),
		nodeStep("end"),
		trajectory.Step{Type: trajectory.StepTypeToolCall, Tool: "t"},
	)
	eval, _ := (&NodeTransitionEvaluator{}).EvaluateRun(run)
	if len(eval.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d: %+v", len(eval.Findings), eval.Findings)
	}
}

func TestNodeTransition_CustomTerminalSet(t *testing.T) {
	run := runWith(nodeStep("start"), nodeStep("complete"))
	eval, _ := (&NodeTransitionEvaluator{TerminalNodes: []string{"complete"}}).EvaluateRun(run)
	if len(eval.Findings) != 0 {
		t.Errorf("expected 0 findings with custom terminal set, got %d", len(eval.Findings))
	}
}
