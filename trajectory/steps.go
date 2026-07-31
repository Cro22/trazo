package trajectory

import (
	"encoding/json"
	"time"
)

type StepType string

const (
	StepTypeCallLLM        StepType = "llm_call"
	StepTypeToolCall       StepType = "tool_call"
	StepTypeToolResult     StepType = "tool_result"
	StepTypeNodeTransition StepType = "node_transition"
)

type Run struct {
	Steps     []Step    `json:"steps"`
	ID        string    `json:"id"`
	Agent     string    `json:"agent"`
	Version   string    `json:"version"`
	StartTime time.Time `json:"startTime"`
	EndTime   time.Time `json:"endTime"`
}

type Step struct {
	LLM          string          `json:"llm,omitempty"`
	Tool         string          `json:"tool,omitempty"`
	Node         string          `json:"node,omitempty"`
	Type         StepType        `json:"type"`
	Timestamp    time.Time       `json:"timestamp"`
	Input        json.RawMessage `json:"input,omitempty"`
	Output       json.RawMessage `json:"output,omitempty"`
	Cost         float64         `json:"cost,omitempty"`
	InputTokens  int             `json:"inputTokens,omitempty"`
	OutputTokens int             `json:"outputTokens,omitempty"`
	DurationMs   int64           `json:"durationMs"`
	Error        string          `json:"error,omitempty"`
}

func LoadRun(data []byte) (*Run, error) {
	var run Run
	err := json.Unmarshal(data, &run)
	if err != nil {
		return nil, err
	}
	return &run, nil
}
