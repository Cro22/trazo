package report

import (
	"github.com/Cro22/trazo/evaluator"
	"github.com/Cro22/trazo/runner"
)

// Summary is the aggregate outcome of a run batch, shared by the text footer and
// the JSON envelope so the two never disagree.
type Summary struct {
	Files       int `json:"files"`
	Runs        int `json:"runs"`
	Evaluations int `json:"evaluations"`
	Findings    int `json:"findings"`
	Good        int `json:"good"`
	Neutral     int `json:"neutral"`
	Bad         int `json:"bad"`
	FileErrors  int `json:"fileErrors"`
}

// Summarize aggregates a Response. files is the number of trace files considered
// (which the Response alone does not carry, since valid files with no findings
// still count); pass 0 when it is not known.
func Summarize(resp *runner.Response, files int) Summary {
	s := Summary{
		Files:       files,
		Evaluations: len(resp.Evaluations),
		FileErrors:  len(resp.FileErrors),
	}
	seen := map[string]bool{}
	for _, e := range resp.Evaluations {
		if !seen[e.RunID] {
			seen[e.RunID] = true
			s.Runs++
		}
		for _, f := range e.Findings {
			s.Findings++
			switch f.Judgment {
			case evaluator.JudgmentBad:
				s.Bad++
			case evaluator.JudgmentNeutral:
				s.Neutral++
			case evaluator.JudgmentGood:
				s.Good++
			}
		}
	}
	return s
}
