package evaluator

import (
	"fmt"

	"github.com/Cro22/trazo/trajectory"
)

type ToolCallEvaluator struct {
}

func (e *ToolCallEvaluator) EvaluateRun(run *trajectory.Run) (*Evaluation, error) {
	eva := &Evaluation{
		EvaluatorName: "tool_calls",
		RunID:         run.ID,
		Findings:      []Finding{},
	}

	var pending []int

	for i, step := range run.Steps {
		if step.Type == trajectory.StepTypeToolCall {
			pending = append(pending, i)
		} else if step.Type == trajectory.StepTypeToolResult {
			matched := -1
			for j, pIdx := range pending {
				if run.Steps[pIdx].Tool == step.Tool {
					matched = j
					break
				}
			}
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
