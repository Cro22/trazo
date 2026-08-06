package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/Cro22/trazo/evaluator"
	"github.com/Cro22/trazo/runner"
)

// Exit codes: 0 clean, 1 at least one JudgmentBad finding, 2 at least one
// file error (unreadable, malformed, or invalid trace). File errors take
// precedence so CI never mistakes a half-evaluated batch for a clean one.
const (
	exitClean     = 0
	exitBad       = 1
	exitFileError = 2
)

type fileErrorJSON struct {
	File  string `json:"file"`
	Error string `json:"error"`
}

type responseJSON struct {
	Evaluations []*evaluator.Evaluation `json:"evaluations"`
	FileErrors  []fileErrorJSON         `json:"fileErrors"`
}

func main() {
	dir := flag.String("dir", "./testdata/runs", "directory containing run JSON files")
	asJSON := flag.Bool("json", false, "print results as JSON")

	maxRepeats := flag.Int("max-repeats", evaluator.DefaultMaxRepeats, "loops: identical tool/node repeats before flagging")
	maxStepCost := flag.Float64("max-step-cost", evaluator.DefaultMaxStepCost, "cost: max USD per step")
	maxStepLatencyMs := flag.Int64("max-step-latency-ms", evaluator.DefaultMaxStepLatencyMs, "latency: max ms per step")
	maxRunCost := flag.Float64("max-run-cost", evaluator.DefaultMaxRunCost, "cost: max USD per run")
	maxRunLatencyMs := flag.Int64("max-run-latency-ms", evaluator.DefaultMaxRunLatencyMs, "latency: max ms per run")
	terminalNodes := flag.String("terminal-nodes", "", "node: comma-separated terminal node names (default end,__end__,finish,done)")
	llmJudge := flag.Bool("llm-judge", false, "enable the LLM-as-judge evaluator (needs GEMINI_API_KEY)")
	judgeModel := flag.String("judge-model", evaluator.DefaultJudgeModel, "model for --llm-judge")
	flag.Parse()

	evaluators := []evaluator.Evaluator{
		&evaluator.ToolCallEvaluator{},
		&evaluator.LoopEvaluator{MaxRepeats: *maxRepeats},
		&evaluator.CostLatencyEvaluator{
			MaxStepCost:      *maxStepCost,
			MaxStepLatencyMs: *maxStepLatencyMs,
			MaxRunCost:       *maxRunCost,
			MaxRunLatencyMs:  *maxRunLatencyMs,
		},
		&evaluator.NodeTransitionEvaluator{TerminalNodes: splitCSV(*terminalNodes)},
	}
	if *llmJudge {
		client, err := evaluator.NewGeminiClient(*judgeModel)
		if err != nil {
			log.Fatalf("llm-judge: %v", err)
		}
		evaluators = append(evaluators, &evaluator.LLMJudgeEvaluator{Client: client})
	}

	resp, err := runner.NewRunner(evaluators).Run(*dir)
	if err != nil {
		log.Fatalf("Error running: %v", err)
	}

	if *asJSON {
		printJSON(resp)
	} else {
		printText(resp)
	}

	os.Exit(exitCode(resp))
}

func printText(resp *runner.Response) {
	for _, findings := range resp.Evaluations {
		fmt.Printf("RunID %s. Findings: %d Evaluator: %s\n", findings.RunID, len(findings.Findings), findings.EvaluatorName)
		for _, s := range findings.Findings {
			fmt.Printf("Step %d: Comment: %s, Score: %f, Judgment: %s\n", s.StepIndex, s.Comment, s.Score, s.Judgment)
		}
	}

	for _, fe := range resp.FileErrors {
		fmt.Printf("File Error: %s → %v\n", fe.File, fe.Err)
	}
}

func printJSON(resp *runner.Response) {
	out := responseJSON{
		Evaluations: resp.Evaluations,
		FileErrors:  []fileErrorJSON{},
	}
	if out.Evaluations == nil {
		out.Evaluations = []*evaluator.Evaluation{}
	}
	for _, fe := range resp.FileErrors {
		out.FileErrors = append(out.FileErrors, fileErrorJSON{File: fe.File, Error: fe.Err.Error()})
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		log.Fatalf("Error encoding JSON: %v", err)
	}
	fmt.Println(string(data))
}

// splitCSV parses a comma-separated flag value into a trimmed, non-empty slice,
// returning nil for an empty value so the evaluator falls back to its default.
func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func exitCode(resp *runner.Response) int {
	if len(resp.FileErrors) > 0 {
		return exitFileError
	}
	for _, eval := range resp.Evaluations {
		for _, f := range eval.Findings {
			if f.Judgment == evaluator.JudgmentBad {
				return exitBad
			}
		}
	}
	return exitClean
}
