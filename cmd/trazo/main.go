package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

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
	flag.Parse()

	evaluators := []evaluator.Evaluator{&evaluator.ToolCallEvaluator{}}
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
