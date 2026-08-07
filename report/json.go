package report

import (
	"encoding/json"
	"time"

	"github.com/Cro22/trazo/evaluator"
	"github.com/Cro22/trazo/runner"
)

// OutputVersion is the version of the machine-readable JSON output contract (the
// envelope shape below), independent of the product and trace-schema versions.
// Bump the minor for additive fields, the major for a breaking change. See
// docs/output.md.
const OutputVersion = "1.0"

// Meta is the context the caller supplies for the JSON envelope: the versions to
// stamp, the number of files considered, and the generation time. GeneratedAt is
// injected (not read from the clock here) so output is deterministic in tests.
type Meta struct {
	TrazoVersion       string
	TraceSchemaVersion string
	Files              int
	GeneratedAt        time.Time
}

type fileErrorJSON struct {
	File  string `json:"file"`
	Error string `json:"error"`
}

type jsonEnvelope struct {
	OutputVersion      string                  `json:"outputVersion"`
	TrazoVersion       string                  `json:"trazoVersion"`
	TraceSchemaVersion string                  `json:"traceSchemaVersion"`
	GeneratedAt        string                  `json:"generatedAt"`
	Summary            Summary                 `json:"summary"`
	Results            []*evaluator.Evaluation `json:"results"`
	Errors             []fileErrorJSON         `json:"errors"`
}

// JSON renders the results as a versioned, self-describing envelope: metadata, an
// aggregate summary, the per-run evaluations, and structured file errors. Empty
// slices serialize as [] (never null) so consumers can index without a nil
// check. The shape is a stable contract; see docs/output.md.
func JSON(resp *runner.Response, meta Meta) (string, error) {
	env := jsonEnvelope{
		OutputVersion:      OutputVersion,
		TrazoVersion:       meta.TrazoVersion,
		TraceSchemaVersion: meta.TraceSchemaVersion,
		GeneratedAt:        meta.GeneratedAt.UTC().Format(time.RFC3339),
		Summary:            Summarize(resp, meta.Files),
		Results:            resp.Evaluations,
		Errors:             []fileErrorJSON{},
	}
	if env.Results == nil {
		env.Results = []*evaluator.Evaluation{}
	}
	for _, fe := range resp.FileErrors {
		env.Errors = append(env.Errors, fileErrorJSON{File: fe.File, Error: fe.Err.Error()})
	}

	data, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
