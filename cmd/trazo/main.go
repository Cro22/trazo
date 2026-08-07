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
	log.SetFlags(0)
	log.SetPrefix("trazo: ")

	// `trazo version` is a subcommand, handled before flag parsing so it works
	// without any other arguments.
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Print(versionReport(readBuildDetails()))
		return
	}

	showVersion := flag.Bool("version", false, "print version information and exit")
	dir := flag.String("dir", "./testdata/runs", "directory of traces to scan when no PATH is given")
	recursive := flag.Bool("recursive", false, "descend into subdirectories when PATH is a directory")
	validate := flag.Bool("validate", false, "only check that traces load and pass structural validation; skip evaluators")
	asJSON := flag.Bool("json", false, "print results as JSON (alias for -format json)")
	format := flag.String("format", "text", "output format: text, json, or md")

	maxRepeats := flag.Int("max-repeats", evaluator.DefaultMaxRepeats, "loops: identical tool/node repeats before flagging")
	maxStepCost := flag.Float64("max-step-cost", evaluator.DefaultMaxStepCost, "cost: max USD per step")
	maxStepLatencyMs := flag.Int64("max-step-latency-ms", evaluator.DefaultMaxStepLatencyMs, "latency: max ms per step")
	maxRunCost := flag.Float64("max-run-cost", evaluator.DefaultMaxRunCost, "cost: max USD per run")
	maxRunLatencyMs := flag.Int64("max-run-latency-ms", evaluator.DefaultMaxRunLatencyMs, "latency: max ms per run")
	terminalNodes := flag.String("terminal-nodes", "", "node: comma-separated terminal node names (default end,__end__,finish,done)")
	llmJudge := flag.Bool("llm-judge", false, "enable the LLM-as-judge evaluator (needs GEMINI_API_KEY)")
	judgeModel := flag.String("judge-model", evaluator.DefaultJudgeModel, "model for -llm-judge")

	flag.Usage = usage
	flag.Parse()

	if *showVersion {
		fmt.Print(versionReport(readBuildDetails()))
		return
	}

	if flag.NArg() > 1 {
		log.Printf("at most one PATH may be given, got %d", flag.NArg())
		flag.Usage()
		os.Exit(exitFileError)
	}

	out := *format
	if *asJSON {
		out = "json"
	}
	if out != "text" && out != "json" && out != "md" && out != "markdown" {
		log.Fatalf("unknown -format %q (want text, json, or md)", out)
	}

	// PATH (positional) takes precedence over -dir; -dir is the fallback default.
	path := *dir
	if flag.NArg() == 1 {
		path = flag.Arg(0)
	}

	files, err := resolveFiles(path, *recursive)
	if err != nil {
		log.Fatalf("%v", err)
	}
	if len(files) == 0 {
		log.Printf("no .json traces found under %q", path)
	}

	evaluators := buildEvaluators(*validate, evaluatorConfig{
		maxRepeats:       *maxRepeats,
		maxStepCost:      *maxStepCost,
		maxStepLatencyMs: *maxStepLatencyMs,
		maxRunCost:       *maxRunCost,
		maxRunLatencyMs:  *maxRunLatencyMs,
		terminalNodes:    splitCSV(*terminalNodes),
		llmJudge:         *llmJudge,
		judgeModel:       *judgeModel,
	})

	// Cancel in-flight evaluation on Ctrl+C (SIGINT) or SIGTERM so a long run,
	// notably one using the network-bound LLM judge, stops promptly.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	resp := runner.NewRunner(evaluators).RunFiles(ctx, files)

	render(out, *validate, resp, len(files))
	os.Exit(exitCode(resp))
}

func usage() {
	w := flag.CommandLine.Output()
	fmt.Fprintf(w, "trazo evaluates agent trace files (trazo JSON format) and reports findings.\n\n")
	fmt.Fprintf(w, "Usage:\n  trazo [flags] [PATH]\n  trazo version\n\n")
	fmt.Fprintf(w, "PATH is a single trace file or a directory of .json traces. If omitted, -dir is scanned.\n\n")
	fmt.Fprintf(w, "Flags:\n")
	flag.PrintDefaults()
	fmt.Fprintf(w, "\nExit codes: 0 clean, 1 a bad finding, 2 a file error or invalid trace.\n")
}

// resolveFiles turns a PATH into the concrete list of trace files to evaluate: a
// single file is used as-is, a directory is scanned (recursively when asked).
func resolveFiles(path string, recursive bool) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return runner.CollectFiles(path, recursive)
	}
	return []string{path}, nil
}

func render(out string, validate bool, resp *runner.Response, total int) {
	switch out {
	case "json":
		s, err := report.JSON(resp)
		if err != nil {
			log.Fatalf("encoding JSON: %v", err)
		}
		fmt.Println(s)
	case "md", "markdown":
		fmt.Print(report.Markdown(resp))
	default: // text
		if validate {
			fmt.Print(report.ValidateSummary(resp, total))
		} else {
			fmt.Print(report.Text(resp))
		}
	}
}

type evaluatorConfig struct {
	maxRepeats       int
	maxStepCost      float64
	maxStepLatencyMs int64
	maxRunCost       float64
	maxRunLatencyMs  int64
	terminalNodes    []string
	llmJudge         bool
	judgeModel       string
}

// buildEvaluators assembles the evaluator set. In validate-only mode it returns
// none, so the runner just loads and structurally validates each file. The LLM
// judge is opt-in and constructed last because it can fail (missing API key).
func buildEvaluators(validate bool, cfg evaluatorConfig) []evaluator.Evaluator {
	if validate {
		return nil
	}
	evaluators := []evaluator.Evaluator{
		&evaluator.ToolCallEvaluator{},
		&evaluator.LoopEvaluator{MaxRepeats: cfg.maxRepeats},
		&evaluator.CostLatencyEvaluator{
			MaxStepCost:      cfg.maxStepCost,
			MaxStepLatencyMs: cfg.maxStepLatencyMs,
			MaxRunCost:       cfg.maxRunCost,
			MaxRunLatencyMs:  cfg.maxRunLatencyMs,
		},
		&evaluator.NodeTransitionEvaluator{TerminalNodes: cfg.terminalNodes},
	}
	if cfg.llmJudge {
		client, err := evaluator.NewGeminiClient(cfg.judgeModel)
		if err != nil {
			log.Fatalf("llm-judge: %v", err)
		}
		evaluators = append(evaluators, &evaluator.LLMJudgeEvaluator{Client: client})
	}
	return evaluators
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
