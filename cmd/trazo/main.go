package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"

	"github.com/Cro22/trazo/evaluator"
	"github.com/Cro22/trazo/report"
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

func main() {
	dir := flag.String("dir", "./testdata/runs", "directory containing run JSON files")
	asJSON := flag.Bool("json", false, "print results as JSON (alias for -format json)")
	format := flag.String("format", "text", "output format: text, json, or md")

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

	// Cancel in-flight evaluation on Ctrl+C (SIGINT) or SIGTERM so a long run,
	// notably one using the network-bound LLM judge, stops promptly.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	resp, err := runner.NewRunner(evaluators).Run(ctx, *dir)
	if err != nil {
		log.Fatalf("Error running: %v", err)
	}

	out := *format
	if *asJSON {
		out = "json"
	}
	switch out {
	case "json":
		s, err := report.JSON(resp)
		if err != nil {
			log.Fatalf("Error encoding JSON: %v", err)
		}
		fmt.Println(s)
	case "md", "markdown":
		fmt.Print(report.Markdown(resp))
	case "text":
		fmt.Print(report.Text(resp))
	default:
		log.Fatalf("unknown -format %q (want text, json, or md)", out)
	}

	os.Exit(exitCode(resp))
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
