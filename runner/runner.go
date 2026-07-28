package runner

import (
	"os"
	"path/filepath"

	"github.com/Cro22/trazo/evaluator"
	"github.com/Cro22/trazo/trajectory"
)

type Runner struct {
	evals []evaluator.Evaluator
}

type FileError struct {
	File string
	Err  error
}

type Response struct {
	Evaluations []*evaluator.Evaluation `json:"evaluations"`
	FileErrors  []FileError             `json:"file_errors"`
}

func NewRunner(evals []evaluator.Evaluator) *Runner {
	return &Runner{evals: evals}
}

func (r *Runner) Run(dir string) (*Response, error) {
	files, err := os.ReadDir(dir)
	evaluation := Response{}
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if filepath.Ext(file.Name()) != ".json" {
			continue
		}
		//TODO Add Goroutines
		fileDir := filepath.Join(dir, file.Name())
		fileBytes, err := os.ReadFile(fileDir)
		if err != nil {
			evaluation.FileErrors = append(evaluation.FileErrors, FileError{File: file.Name(), Err: err})
			continue
		}
		run, err := trajectory.LoadRun(fileBytes)
		if err != nil {
			evaluation.FileErrors = append(evaluation.FileErrors, FileError{File: file.Name(), Err: err})
			continue
		}
		for _, judge := range r.evals {
			eval, err := judge.EvaluateRun(run)
			if err != nil {
				evaluation.FileErrors = append(evaluation.FileErrors, FileError{File: file.Name(), Err: err})
				continue
			}
			evaluation.Evaluations = append(evaluation.Evaluations, eval)
		}
	}
	return &evaluation, nil
}
