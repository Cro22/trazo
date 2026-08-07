package report

import (
	"encoding/json"

	"github.com/Cro22/trazo/evaluator"
	"github.com/Cro22/trazo/runner"
)

type fileErrorJSON struct {
	File  string `json:"file"`
	Error string `json:"error"`
}

type responseJSON struct {
	Evaluations []*evaluator.Evaluation `json:"evaluations"`
	FileErrors  []fileErrorJSON         `json:"fileErrors"`
}

// JSON renders the evaluation results as indented JSON. Empty slices serialize as
// [] rather than null so consumers can index without a nil check, and errors are
// flattened to strings.
func JSON(resp *runner.Response) (string, error) {
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
		return "", err
	}
	return string(data), nil
}
