package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Cro22/trazo/evaluator"
	"github.com/Cro22/trazo/trajectory"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "trazo.config.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

// stubEvaluator stands in for a real evaluator so Build can be tested without a
// network-backed judge.
type stubEvaluator struct{}

func (stubEvaluator) EvaluateRun(context.Context, *trajectory.Run) (*evaluator.Evaluation, error) {
	return &evaluator.Evaluation{}, nil
}

func noJudge(string) (evaluator.Evaluator, error) {
	return nil, errors.New("newJudge should not be called when the judge is disabled")
}

func TestDefault_BuildsFourEvaluators(t *testing.T) {
	cfg := Default()
	evals, err := cfg.Build(noJudge)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(evals) != 4 {
		t.Fatalf("default policy should build 4 evaluators (judge off), got %d", len(evals))
	}
}

func TestLoad_PartialLayersOverDefaults(t *testing.T) {
	// Only override one nested field; everything else must keep its default.
	path := writeConfig(t, `{"version": 1, "evaluators": {"loops": {"maxRepeats": 9}}}`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Evaluators.Loops.MaxRepeats != 9 {
		t.Errorf("maxRepeats override lost: got %d", cfg.Evaluators.Loops.MaxRepeats)
	}
	if !cfg.Evaluators.Loops.Enabled {
		t.Error("omitted loops.enabled should keep the default (true)")
	}
	if cfg.Evaluators.CostLatency.MaxStepCost != evaluator.DefaultMaxStepCost {
		t.Errorf("omitted cost_latency should keep defaults, got %v", cfg.Evaluators.CostLatency.MaxStepCost)
	}
}

func TestLoad_DisableEvaluator(t *testing.T) {
	path := writeConfig(t, `{"version": 1, "evaluators": {"cost_latency": {"enabled": false}, "loops": {"enabled": false}}}`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	evals, err := cfg.Build(noJudge)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	// tool_calls and node_transitions remain enabled by default => 2.
	if len(evals) != 2 {
		t.Fatalf("expected 2 evaluators after disabling two, got %d", len(evals))
	}
}

func TestLoad_RejectsUnknownField(t *testing.T) {
	path := writeConfig(t, `{"version": 1, "evaluators": {"loops": {"maxRepeat": 3}}}`)
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown-field error, got: %v", err)
	}
}

func TestLoad_RejectsBadVersion(t *testing.T) {
	for _, body := range []string{
		`{"evaluators": {}}`,           // missing version
		`{"version": 2, "evaluators": {}}`, // unsupported version
	} {
		path := writeConfig(t, body)
		if _, err := Load(path); err == nil {
			t.Errorf("expected version error for %s", body)
		}
	}
}

func TestLoad_RejectsNegativeThresholds(t *testing.T) {
	path := writeConfig(t, `{"version": 1, "evaluators": {"loops": {"maxRepeats": -1}}}`)
	if _, err := Load(path); err == nil {
		t.Fatal("expected negative-threshold error")
	}
}

// TestExampleConfigLoads keeps the shipped example policy honest: it must always
// load and build under the current loader, so the docs never drift from the code.
func TestExampleConfigLoads(t *testing.T) {
	cfg, err := Load(filepath.Join("..", "docs", "trazo.config.example.json"))
	if err != nil {
		t.Fatalf("example config failed to load: %v", err)
	}
	if _, err := cfg.Build(noJudge); err != nil {
		t.Fatalf("example config failed to build: %v", err)
	}
}

func TestBuild_JudgeEnabledCallsFactory(t *testing.T) {
	cfg := Default()
	cfg.Evaluators.LLMJudge.Enabled = true

	called := false
	evals, err := cfg.Build(func(model string) (evaluator.Evaluator, error) {
		called = true
		if model != evaluator.DefaultJudgeModel {
			t.Errorf("expected default judge model, got %q", model)
		}
		return stubEvaluator{}, nil
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if !called {
		t.Error("judge factory was not called though the judge is enabled")
	}
	if len(evals) != 5 {
		t.Fatalf("expected 5 evaluators with judge on, got %d", len(evals))
	}
}
