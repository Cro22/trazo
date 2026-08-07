// Package config defines trazo's evaluator policy file: a small, versioned JSON
// document that pins which evaluators run and with what thresholds, so a policy
// is reproducible across a developer's machine, CI, and a team. It is
// stdlib-only, like the rest of the core, and independent of the CLI: another Go
// program can Load a policy and build the same evaluator set.
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/Cro22/trazo/evaluator"
)

// SupportedVersion is the config file format version this build understands. The
// file must declare it explicitly; a different value is rejected rather than
// guessed at.
const SupportedVersion = 1

// Config is the whole policy file. Version is the config format version (not the
// product or trace-schema version).
type Config struct {
	Version    int        `json:"version"`
	Evaluators Evaluators `json:"evaluators"`
}

type Evaluators struct {
	ToolCalls       ToolCalls       `json:"tool_calls"`
	Loops           Loops           `json:"loops"`
	CostLatency     CostLatency     `json:"cost_latency"`
	NodeTransitions NodeTransitions `json:"node_transitions"`
	LLMJudge        LLMJudge        `json:"llm_judge"`
}

type ToolCalls struct {
	Enabled bool `json:"enabled"`
}

type Loops struct {
	Enabled    bool `json:"enabled"`
	MaxRepeats int  `json:"maxRepeats"`
}

type CostLatency struct {
	Enabled          bool    `json:"enabled"`
	MaxStepCost      float64 `json:"maxStepCost"`
	MaxStepLatencyMs int64   `json:"maxStepLatencyMs"`
	MaxRunCost       float64 `json:"maxRunCost"`
	MaxRunLatencyMs  int64   `json:"maxRunLatencyMs"`
}

type NodeTransitions struct {
	Enabled       bool     `json:"enabled"`
	TerminalNodes []string `json:"terminalNodes"`
}

type LLMJudge struct {
	Enabled bool   `json:"enabled"`
	Model   string `json:"model"`
}

// Default is the built-in policy, matching the CLI's flag defaults: the four
// structural evaluators enabled, the LLM judge off. TerminalNodes is left nil so
// the node evaluator falls back to its own default set.
func Default() Config {
	return Config{
		Version: SupportedVersion,
		Evaluators: Evaluators{
			ToolCalls: ToolCalls{Enabled: true},
			Loops:     Loops{Enabled: true, MaxRepeats: evaluator.DefaultMaxRepeats},
			CostLatency: CostLatency{
				Enabled:          true,
				MaxStepCost:      evaluator.DefaultMaxStepCost,
				MaxStepLatencyMs: evaluator.DefaultMaxStepLatencyMs,
				MaxRunCost:       evaluator.DefaultMaxRunCost,
				MaxRunLatencyMs:  evaluator.DefaultMaxRunLatencyMs,
			},
			NodeTransitions: NodeTransitions{Enabled: true},
			LLMJudge:        LLMJudge{Enabled: false, Model: evaluator.DefaultJudgeModel},
		},
	}
}

// Load reads a policy file, layering it over Default so an omitted field keeps
// its default (partial configs are valid). Unknown fields are rejected, so a
// typo like "maxRepeat" is an error instead of being silently ignored. The file
// must declare version == SupportedVersion.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := Default()
	// Clear Version so we can tell whether the file actually declared it: an
	// omitted version leaves it 0 and fails the check below.
	cfg.Version = 0

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("config %s: %w", path, err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config %s: %w", path, err)
	}
	return &cfg, nil
}

func (c *Config) validate() error {
	if c.Version != SupportedVersion {
		return fmt.Errorf("version %d is unsupported (this build accepts version %d)", c.Version, SupportedVersion)
	}
	if c.Evaluators.Loops.MaxRepeats < 0 {
		return fmt.Errorf("loops.maxRepeats is negative (%d)", c.Evaluators.Loops.MaxRepeats)
	}
	cl := c.Evaluators.CostLatency
	if cl.MaxStepCost < 0 || cl.MaxRunCost < 0 {
		return fmt.Errorf("cost_latency costs must be non-negative")
	}
	if cl.MaxStepLatencyMs < 0 || cl.MaxRunLatencyMs < 0 {
		return fmt.Errorf("cost_latency latencies must be non-negative")
	}
	for i, n := range c.Evaluators.NodeTransitions.TerminalNodes {
		if n == "" {
			return fmt.Errorf("node_transitions.terminalNodes[%d] is empty", i)
		}
	}
	return nil
}

// Build turns the policy into the evaluator set to run. The LLM judge is
// constructed via newJudge only when enabled; newJudge may fail (missing API
// key), so the caller supplies it and handles that error.
func (c *Config) Build(newJudge func(model string) (evaluator.Evaluator, error)) ([]evaluator.Evaluator, error) {
	var evals []evaluator.Evaluator
	e := c.Evaluators

	if e.ToolCalls.Enabled {
		evals = append(evals, &evaluator.ToolCallEvaluator{})
	}
	if e.Loops.Enabled {
		evals = append(evals, &evaluator.LoopEvaluator{MaxRepeats: e.Loops.MaxRepeats})
	}
	if e.CostLatency.Enabled {
		evals = append(evals, &evaluator.CostLatencyEvaluator{
			MaxStepCost:      e.CostLatency.MaxStepCost,
			MaxStepLatencyMs: e.CostLatency.MaxStepLatencyMs,
			MaxRunCost:       e.CostLatency.MaxRunCost,
			MaxRunLatencyMs:  e.CostLatency.MaxRunLatencyMs,
		})
	}
	if e.NodeTransitions.Enabled {
		evals = append(evals, &evaluator.NodeTransitionEvaluator{TerminalNodes: e.NodeTransitions.TerminalNodes})
	}
	if e.LLMJudge.Enabled {
		judge, err := newJudge(e.LLMJudge.Model)
		if err != nil {
			return nil, err
		}
		evals = append(evals, judge)
	}
	return evals, nil
}
