package main

import (
	"fmt"
	"log"
	"os"

	"github.com/Cro22/trazo/trajectory"
)

func main() {
	var testJson, errFile = os.ReadFile("./testdata/sample_run.json")
	if errFile != nil {
		log.Fatalf("Error reading file: %v", errFile)
	}
	run, err := trajectory.LoadRun(testJson)
	if err != nil {
		log.Fatalf("Error loading run: %v", err)
	}
	fmt.Printf("Agent version %s. Total Steps: %d\n", run.Version, len(run.Steps))
	for i, s := range run.Steps {
		if s.Error != "" {
			log.Printf("step %d error: %s\n", i, s.Error)
		}
		fmt.Printf("Step %d: Node: %s, Cost: %f, LLM: %s, Output: %s, Input: %s, Error: %s\n", i, s.Node, s.Cost, s.LLM, s.Output, s.Input, s.Error)
	}
}
