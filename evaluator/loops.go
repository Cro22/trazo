package evaluator

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Cro22/trazo/trajectory"
)

// DefaultMaxRepeats is the repetition count at which LoopEvaluator flags a loop
// when MaxRepeats is left unset.
const DefaultMaxRepeats = 3

// LoopEvaluator flags repetition that suggests the agent is stuck: the same tool
// invoked with identical input, or the same node visited, at least MaxRepeats
// times. This is the loop the ToolCallEvaluator cannot see (every call there is
// still paired), so a runaway that never errors surfaces here instead.
//
// Identical input is compared on canonical JSON, so key ordering does not matter
// and a tool called with different arguments each time (legitimate fan-out) is
// not mistaken for a loop.
type LoopEvaluator struct {
	// MaxRepeats is the threshold; values <= 0 fall back to DefaultMaxRepeats.
	MaxRepeats int
}

func (e *LoopEvaluator) maxRepeats() int {
	if e.MaxRepeats <= 0 {
		return DefaultMaxRepeats
	}
	return e.MaxRepeats
}

func (e *LoopEvaluator) EvaluateRun(ctx context.Context, run *trajectory.Run) (*Evaluation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	eva := &Evaluation{
		EvaluatorName: "loops",
		RunID:         run.ID,
		Findings:      []Finding{},
	}
	limit := e.maxRepeats()
	counts := map[string]int{}
	flagged := map[string]bool{}

	for i, step := range run.Steps {
		key := loopKey(step)
		if key == "" {
			continue
		}
		counts[key]++
		if counts[key] >= limit && !flagged[key] {
			flagged[key] = true
			eva.Findings = append(eva.Findings, Finding{
				StepIndex: i,
				Judgment:  JudgmentBad,
				Comment:   loopComment(step, limit),
			})
		}
	}
	return eva, nil
}

// loopKey returns the repetition key for a step, or "" for step types that do
// not participate in loop detection.
func loopKey(step trajectory.Step) string {
	switch step.Type {
	case trajectory.StepTypeToolCall:
		return "tool:" + step.Tool + ":" + canonicalJSON(step.Input)
	case trajectory.StepTypeNodeTransition:
		return "node:" + step.Node
	default:
		return ""
	}
}

func loopComment(step trajectory.Step, limit int) string {
	if step.Type == trajectory.StepTypeNodeTransition {
		return fmt.Sprintf("node %s visited at least %d times (possible loop)", step.Node, limit)
	}
	return fmt.Sprintf("tool %s called with identical input at least %d times (possible loop)", step.Tool, limit)
}

// canonicalJSON normalizes a raw JSON payload so semantically equal inputs share
// a key regardless of whitespace or object key order. Non-JSON payloads fall
// back to their raw bytes.
func canonicalJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	out, err := json.Marshal(v)
	if err != nil {
		return string(raw)
	}
	return string(out)
}
