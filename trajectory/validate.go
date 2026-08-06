package trajectory

import (
	"errors"
	"fmt"
)

// Validate checks structural invariants of a Run and returns every violation
// joined into a single error, so callers see the full list at once. It does
// not judge agent behavior (e.g. tool call/result pairing); that is the job
// of evaluators.
func (r *Run) Validate() error {
	var errs []error

	if r.ID == "" {
		errs = append(errs, errors.New("run: id is empty"))
	}
	if r.Agent == "" {
		errs = append(errs, errors.New("run: agent is empty"))
	}
	if r.StartTime.IsZero() {
		errs = append(errs, errors.New("run: startTime is missing"))
	}
	if r.EndTime.IsZero() {
		errs = append(errs, errors.New("run: endTime is missing"))
	}
	if !r.StartTime.IsZero() && !r.EndTime.IsZero() && r.EndTime.Before(r.StartTime) {
		errs = append(errs, errors.New("run: endTime is before startTime"))
	}

	for i, step := range r.Steps {
		if step.Timestamp.IsZero() {
			errs = append(errs, fmt.Errorf("step %d: timestamp is missing", i))
		}
		switch step.Type {
		case StepTypeCallLLM:
			if step.LLM == "" {
				errs = append(errs, fmt.Errorf("step %d: llm_call without llm", i))
			}
		case StepTypeToolCall, StepTypeToolResult:
			if step.Tool == "" {
				errs = append(errs, fmt.Errorf("step %d: %s without tool", i, step.Type))
			}
		case StepTypeNodeTransition:
			if step.Node == "" {
				errs = append(errs, fmt.Errorf("step %d: node_transition without node", i))
			}
		default:
			errs = append(errs, fmt.Errorf("step %d: unknown step type %q", i, step.Type))
		}
	}

	return errors.Join(errs...)
}
