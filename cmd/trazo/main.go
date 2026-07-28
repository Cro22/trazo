package main

import (
	"fmt"
	"log"

	"github.com/Cro22/trazo/evaluator"
	"github.com/Cro22/trazo/runner"
)

func main() {
	evaluators := []evaluator.Evaluator{&evaluator.ToolCallEvaluator{}}
	run, err := runner.NewRunner(evaluators).Run("./testdata/runs")
	if err != nil {
		log.Fatalf("Error running: %v", err)
	}

	for _, findings := range run.Evaluations {
		fmt.Printf("RunID %s. Findings: %d Evaluator: %s\n", findings.RunID, len(findings.Findings), findings.EvaluatorName)
		for _, s := range findings.Findings {
			fmt.Printf("Step %d: Comment: %s, Score: %f, Judgment: %s\n", s.StepIndex, s.Comment, s.Score, s.Judgment)
		}
	}

	for _, fe := range run.FileErrors {
		fmt.Printf("File Error: %s → %v\n", fe.File, fe.Err)
	}
}
