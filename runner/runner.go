package runner

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"

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

// fileResult holds the outcome of processing a single file. A file yields
// either one FileError (unreadable, malformed, or failed Validate) or a set of
// evaluations, plus any per-evaluator errors, mirroring the sequential path.
type fileResult struct {
	evals []*evaluator.Evaluation
	errs  []FileError
}

func NewRunner(evals []evaluator.Evaluator) *Runner {
	return &Runner{evals: evals}
}

// Run reads every .json file in dir and evaluates it. Files are processed
// concurrently (bounded by the CPU count) but results are assembled in the
// original directory order, so output is deterministic regardless of scheduling.
// ctx is propagated to every evaluator, so cancelling it (Ctrl+C, a CI timeout)
// aborts in-flight work rather than letting it run to completion.
func (r *Runner) Run(ctx context.Context, dir string) (*Response, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	// Collect eligible files first so their index fixes the output order.
	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		files = append(files, entry.Name())
	}

	results := make([]fileResult, len(files))

	workers := runtime.NumCPU()
	if workers > len(files) {
		workers = len(files)
	}
	sem := make(chan struct{}, max(workers, 1))
	var wg sync.WaitGroup

	for i, name := range files {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, name string) {
			defer wg.Done()
			defer func() { <-sem }()
			results[i] = r.processFile(ctx, dir, name)
		}(i, name)
	}
	wg.Wait()

	// Assemble in order to keep output deterministic.
	response := Response{}
	for _, res := range results {
		response.Evaluations = append(response.Evaluations, res.evals...)
		response.FileErrors = append(response.FileErrors, res.errs...)
	}
	return &response, nil
}

func (r *Runner) processFile(ctx context.Context, dir, name string) fileResult {
	var res fileResult

	if err := ctx.Err(); err != nil {
		res.errs = append(res.errs, FileError{File: name, Err: err})
		return res
	}

	fileBytes, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		res.errs = append(res.errs, FileError{File: name, Err: err})
		return res
	}
	run, err := trajectory.LoadRun(fileBytes)
	if err != nil {
		res.errs = append(res.errs, FileError{File: name, Err: err})
		return res
	}
	if err := run.Validate(); err != nil {
		res.errs = append(res.errs, FileError{File: name, Err: err})
		return res
	}
	for _, judge := range r.evals {
		eval, err := judge.EvaluateRun(ctx, run)
		if err != nil {
			res.errs = append(res.errs, FileError{File: name, Err: err})
			continue
		}
		res.evals = append(res.evals, eval)
	}
	return res
}
