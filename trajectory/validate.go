package trajectory

import (
	"errors"
	"fmt"
	"time"
)

// Validate checks structural invariants of a Run and returns every violation
// joined into a single error, so callers see the full list at once. It does
// not judge agent behavior (e.g. tool call/result pairing); that is the job
// of evaluators.
//
// Strictness policy: hard invariants (non-negative quantities, monotonic step
// timestamps, steps within the run interval) reject the trace. Optional fields
// are validated only when present: an absent version or an empty step list is
// accepted, since older emitters produced such traces and they carry no
// ambiguity. Schema-version compatibility is a separate concern (see the
// schema doc), not enforced here.
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

	// prevTS tracks the last step with a usable (non-zero) timestamp, so the
	// monotonicity check skips over steps whose timestamp is already flagged
	// as missing instead of reporting a spurious ordering error against a zero.
	var prevTS time.Time
	var prevIdx int
	havePrev := false

	for i, step := range r.Steps {
		if step.Timestamp.IsZero() {
			errs = append(errs, fmt.Errorf("step %d: timestamp is missing", i))
		} else {
			if havePrev && step.Timestamp.Before(prevTS) {
				errs = append(errs, fmt.Errorf("step %d: timestamp is before step %d", i, prevIdx))
			}
			if !r.StartTime.IsZero() && step.Timestamp.Before(r.StartTime) {
				errs = append(errs, fmt.Errorf("step %d: timestamp is before run startTime", i))
			}
			if !r.EndTime.IsZero() && step.Timestamp.After(r.EndTime) {
				errs = append(errs, fmt.Errorf("step %d: timestamp is after run endTime", i))
			}
			prevTS = step.Timestamp
			prevIdx = i
			havePrev = true
		}

		if step.Cost < 0 {
			errs = append(errs, fmt.Errorf("step %d: cost is negative (%g)", i, step.Cost))
		}
		if step.InputTokens < 0 {
			errs = append(errs, fmt.Errorf("step %d: inputTokens is negative (%d)", i, step.InputTokens))
		}
		if step.OutputTokens < 0 {
			errs = append(errs, fmt.Errorf("step %d: outputTokens is negative (%d)", i, step.OutputTokens))
		}
		if step.DurationMs < 0 {
			errs = append(errs, fmt.Errorf("step %d: durationMs is negative (%d)", i, step.DurationMs))
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
