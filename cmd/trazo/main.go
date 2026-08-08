package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/Cro22/trazo/config"
	"github.com/Cro22/trazo/evaluator"
	"github.com/Cro22/trazo/report"
	"github.com/Cro22/trazo/runner"
	"github.com/Cro22/trazo/trajectory"
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
	configPath := flag.String("config", "", "path to a JSON evaluator policy file (see docs/config.md)")
	dir := flag.String("dir", "./testdata/runs", "directory of traces to scan when no PATH is given")
	recursive := flag.Bool("recursive", false, "descend into subdirectories when PATH is a directory")
	verbose := flag.Bool("verbose", false, "print operational metrics (loaded/valid/invalid/evaluated/duration) to stderr")
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

	// Policy precedence: built-in defaults < config file < explicitly-set flags.
	// The config file pins a reproducible policy; a flag the user actually passed
	// still wins over it (flag.Visit reports only the flags that were set).
	cfg := config.Default()
	if *configPath != "" {
		loaded, err := config.Load(*configPath)
		if err != nil {
			log.Fatalf("%v", err)
		}
		cfg = *loaded
	}
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "max-repeats":
			cfg.Evaluators.Loops.MaxRepeats = *maxRepeats
		case "max-step-cost":
			cfg.Evaluators.CostLatency.MaxStepCost = *maxStepCost
		case "max-step-latency-ms":
			cfg.Evaluators.CostLatency.MaxStepLatencyMs = *maxStepLatencyMs
		case "max-run-cost":
			cfg.Evaluators.CostLatency.MaxRunCost = *maxRunCost
		case "max-run-latency-ms":
			cfg.Evaluators.CostLatency.MaxRunLatencyMs = *maxRunLatencyMs
		case "terminal-nodes":
			cfg.Evaluators.NodeTransitions.TerminalNodes = splitCSV(*terminalNodes)
		case "llm-judge":
			cfg.Evaluators.LLMJudge.Enabled = *llmJudge
		case "judge-model":
			cfg.Evaluators.LLMJudge.Model = *judgeModel
		}
	})

	// In validate-only mode the runner just loads and structurally validates each
	// file, so no evaluators are built regardless of the policy.
	var evaluators []evaluator.Evaluator
	if !*validate {
		evaluators, err = cfg.Build(func(model string) (evaluator.Evaluator, error) {
			client, cerr := evaluator.NewGeminiClient(model)
			if cerr != nil {
				return nil, cerr
			}
			return &evaluator.LLMJudgeEvaluator{Client: client}, nil
		})
		if err != nil {
			log.Fatalf("llm-judge: %v", err)
		}
	}

	// Cancel in-flight evaluation on Ctrl+C (SIGINT) or SIGTERM so a long run,
	// notably one using the network-bound LLM judge, stops promptly.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	start := time.Now()
	resp := runner.NewRunner(evaluators).RunFiles(ctx, files)
	elapsed := time.Since(start)

	if *verbose {
		invalid := distinctInvalidFiles(resp)
		evaluated := report.Summarize(resp, len(files)).Runs
		log.Printf("loaded=%d valid=%d invalid=%d evaluated=%d duration=%s",
			len(files), len(files)-invalid, invalid, evaluated, elapsed.Round(time.Millisecond))
	}

	meta := report.Meta{
		TrazoVersion:       Version,
		TraceSchemaVersion: trajectory.SchemaVersion,
		Files:              len(files),
		GeneratedAt:        time.Now(),
	}
	render(out, *validate, resp, meta)
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

func render(out string, validate bool, resp *runner.Response, meta report.Meta) {
	switch out {
	case "json":
		s, err := report.JSON(resp, meta)
		if err != nil {
			log.Fatalf("encoding JSON: %v", err)
		}
		fmt.Println(s)
	case "md", "markdown":
		fmt.Print(report.Markdown(resp))
	default: // text
		if validate {
			fmt.Print(report.ValidateSummary(resp, meta.Files))
		} else {
			fmt.Print(report.Text(resp))
		}
	}
}

// distinctInvalidFiles counts the unique files that produced at least one error,
// so a file with several evaluator errors is still counted once.
func distinctInvalidFiles(resp *runner.Response) int {
	seen := map[string]bool{}
	for _, fe := range resp.FileErrors {
		seen[fe.File] = true
	}
	return len(seen)
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
