package evaluator

import (
	"context"
	"fmt"

	"github.com/Cro22/trazo/trajectory"
)

type ToolCallEvaluator struct {
}

func (e *ToolCallEvaluator) EvaluateRun(ctx context.Context, run *trajectory.Run) (*Evaluation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	eva := &Evaluation{
		EvaluatorName: "tool_calls",
		RunID:         run.ID,
		Findings:      []Finding{},
	}

	var pending []int

	for i, step := range run.Steps {
		switch step.Type {
		case trajectory.StepTypeToolCall:
			pending = append(pending, i)
		case trajectory.StepTypeToolResult:
			matched := matchPending(run, pending, step)
			if matched != -1 {
				pending = append(pending[:matched], pending[matched+1:]...)
			} else {
				eva.Findings = append(eva.Findings, Finding{
					StepIndex: i,
					Judgment:  JudgmentNeutral,
					Comment:   fmt.Sprintf("tool_result: %s without matching tool_call", step.Tool),
				})
			}
			if step.Error != "" {
				eva.Findings = append(eva.Findings, Finding{
					StepIndex: i,
					Judgment:  JudgmentBad,
					Comment:   fmt.Sprintf("tool %s fails: %s", step.Tool, step.Error),
				})
			}
		}
	}
	for _, pIdx := range pending {
		eva.Findings = append(eva.Findings, Finding{
			StepIndex: pIdx,
			Judgment:  JudgmentNeutral,
			Comment:   fmt.Sprintf("tool_call %s has no matching result", run.Steps[pIdx].Tool),
		})
	}
	return eva, nil
}

// matchPending finds the index within pending of the tool_call that a
// tool_result answers, or -1 if none. When the result carries a toolCallId, the
// match is by id and is authoritative: no id match means an orphan, with no
// fallback. Only when the result has no id do we fall back to the earliest
// pending call with the same tool name (FIFO), preserving legacy behavior.
func matchPending(run *trajectory.Run, pending []int, result trajectory.Step) int {
	if result.ToolCallID != "" {
		for j, pIdx := range pending {
			if run.Steps[pIdx].ToolCallID == result.ToolCallID {
				return j
			}
		}
		return -1
	}
	for j, pIdx := range pending {
		if run.Steps[pIdx].Tool == result.Tool {
			return j
		}
	}
	return -1
}
