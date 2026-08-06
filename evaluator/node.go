package evaluator

import (
	"fmt"
	"strings"

	"github.com/Cro22/trazo/trajectory"
)

// DefaultTerminalNodes are the node names treated as a clean end of a run when
// NodeTransitionEvaluator.TerminalNodes is unset. Matching is case-insensitive.
var DefaultTerminalNodes = []string{"end", "__end__", "finish", "done"}

// NodeTransitionEvaluator checks the control-flow bookkeeping a trace exposes:
// the sequence of visited nodes. The trace carries no edge information, so this
// evaluator does not validate individual transitions; it checks completeness,
// namely that a run with node transitions ends at a terminal node. A run that
// ends elsewhere (an error/fallback node, or a run cut short) is surfaced as
// neutral: the cause may be an interruption or timeout outside the agent.
type NodeTransitionEvaluator struct {
	// TerminalNodes overrides DefaultTerminalNodes when non-empty.
	TerminalNodes []string
}

func (e *NodeTransitionEvaluator) terminalSet() map[string]bool {
	names := e.TerminalNodes
	if len(names) == 0 {
		names = DefaultTerminalNodes
	}
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[strings.ToLower(n)] = true
	}
	return set
}

func (e *NodeTransitionEvaluator) EvaluateRun(run *trajectory.Run) (*Evaluation, error) {
	eva := &Evaluation{
		EvaluatorName: "node_transitions",
		RunID:         run.ID,
		Findings:      []Finding{},
	}

	lastIdx := -1
	for i, step := range run.Steps {
		if step.Type == trajectory.StepTypeNodeTransition {
			lastIdx = i
		}
	}
	if lastIdx == -1 {
		// No node transitions to reason about.
		return eva, nil
	}

	terminal := e.terminalSet()
	lastNode := run.Steps[lastIdx].Node
	if !terminal[strings.ToLower(lastNode)] {
		eva.Findings = append(eva.Findings, Finding{
			StepIndex: lastIdx,
			Judgment:  JudgmentNeutral,
			Comment:   fmt.Sprintf("run ended at node %q, not a terminal node", lastNode),
		})
	}

	return eva, nil
}
