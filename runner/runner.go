package runner

import (
	"context"
	"io/fs"
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

// CollectFiles returns the .json files under root as paths joined with root,
// in a deterministic order. When recursive is true it descends into
// subdirectories (lexical order, courtesy of filepath.WalkDir); otherwise it
// reads only the top level (sorted, courtesy of os.ReadDir). Non-.json files are
// skipped, so docs and other artifacts can sit alongside traces.
func CollectFiles(root string, recursive bool) ([]string, error) {
	if recursive {
		var files []string
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if filepath.Ext(d.Name()) == ".json" {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		return files, nil
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		files = append(files, filepath.Join(root, entry.Name()))
	}
	return files, nil
}

// Run reads every .json file in dir (non-recursively) and evaluates it. It is a
// convenience wrapper over CollectFiles + RunFiles; callers needing a single
// file, recursion, or a precomputed list should use those directly.
func (r *Runner) Run(ctx context.Context, dir string) (*Response, error) {
	files, err := CollectFiles(dir, false)
	if err != nil {
		return nil, err
	}
	return r.RunFiles(ctx, files), nil
}

// RunFiles evaluates an explicit list of trace file paths. Files are processed
// concurrently (bounded by the CPU count) but results are assembled in the
// input order, so output is deterministic regardless of scheduling. ctx is
// propagated to every evaluator, so cancelling it (Ctrl+C, a CI timeout) aborts
// in-flight work rather than letting it run to completion. Each FileError
// carries the path as given, so errors are unambiguous across subdirectories.
func (r *Runner) RunFiles(ctx context.Context, files []string) *Response {
	results := make([]fileResult, len(files))

	workers := runtime.NumCPU()
	if workers > len(files) {
		workers = len(files)
	}
	sem := make(chan struct{}, max(workers, 1))
	var wg sync.WaitGroup

	for i, path := range files {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, path string) {
			defer wg.Done()
			defer func() { <-sem }()
			results[i] = r.processFile(ctx, path)
		}(i, path)
	}
	wg.Wait()

	// Assemble in order to keep output deterministic.
	response := Response{}
	for _, res := range results {
		response.Evaluations = append(response.Evaluations, res.evals...)
		response.FileErrors = append(response.FileErrors, res.errs...)
	}
	return &response
}

func (r *Runner) processFile(ctx context.Context, path string) fileResult {
	var res fileResult

	if err := ctx.Err(); err != nil {
		res.errs = append(res.errs, FileError{File: path, Err: err})
		return res
	}

	fileBytes, err := os.ReadFile(path)
	if err != nil {
		res.errs = append(res.errs, FileError{File: path, Err: err})
		return res
	}
	run, err := trajectory.LoadRun(fileBytes)
	if err != nil {
		res.errs = append(res.errs, FileError{File: path, Err: err})
		return res
	}
	if err := run.Validate(); err != nil {
		res.errs = append(res.errs, FileError{File: path, Err: err})
		return res
	}
	for _, judge := range r.evals {
		eval, err := judge.EvaluateRun(ctx, run)
		if err != nil {
			res.errs = append(res.errs, FileError{File: path, Err: err})
			continue
		}
		res.evals = append(res.evals, eval)
	}
	return res
}
