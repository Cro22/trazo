package evaluator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Cro22/trazo/trajectory"
)

// DefaultJudgeModel is the model used by the built-in Gemini judge client.
const DefaultJudgeModel = "gemini-2.5-flash"

const defaultJudgeTimeout = 30 * time.Second

// Completion is the minimal LLM surface the judge needs. It is an interface so
// the evaluator can be tested with a fake and run in production against Gemini.
type Completion interface {
	Complete(ctx context.Context, prompt string) (string, error)
}

// LLMJudgeEvaluator asks a model to grade the agent's final output. It is opt-in
// (it needs a Client and, in production, a network call), so it is not part of
// the default evaluator set. It judges the run's last llm_call output once.
type LLMJudgeEvaluator struct {
	Client  Completion
	Timeout time.Duration
}

func (e *LLMJudgeEvaluator) timeout() time.Duration {
	if e.Timeout <= 0 {
		return defaultJudgeTimeout
	}
	return e.Timeout
}

func (e *LLMJudgeEvaluator) EvaluateRun(run *trajectory.Run) (*Evaluation, error) {
	eva := &Evaluation{
		EvaluatorName: "llm_judge",
		RunID:         run.ID,
		Findings:      []Finding{},
	}
	if e.Client == nil {
		return nil, errors.New("llm_judge: no client configured")
	}

	idx, output := lastLLMOutput(run)
	if idx == -1 {
		return eva, nil // nothing to judge
	}

	ctx, cancel := context.WithTimeout(context.Background(), e.timeout())
	defer cancel()

	raw, err := e.Client.Complete(ctx, judgePrompt(run, output))
	if err != nil {
		return nil, fmt.Errorf("llm_judge: %w", err)
	}

	verdict, err := parseVerdict(raw)
	if err != nil {
		// An unparseable verdict is surfaced for review, not a hard failure.
		eva.Findings = append(eva.Findings, Finding{
			StepIndex: idx,
			Judgment:  JudgmentNeutral,
			Comment:   "llm_judge: could not parse verdict: " + truncate(raw, 120),
		})
		return eva, nil
	}

	eva.Findings = append(eva.Findings, Finding{
		StepIndex: idx,
		Judgment:  verdict.judgment,
		Score:     verdict.Score,
		Comment:   "llm_judge: " + verdict.Comment,
	})
	return eva, nil
}

// lastLLMOutput returns the index and decoded text of the run's last llm_call,
// or (-1, "") if there is none.
func lastLLMOutput(run *trajectory.Run) (int, string) {
	for i := len(run.Steps) - 1; i >= 0; i-- {
		if run.Steps[i].Type == trajectory.StepTypeCallLLM {
			return i, decodeText(run.Steps[i].Output)
		}
	}
	return -1, ""
}

// decodeText renders a raw JSON payload as readable text: a JSON string is
// unquoted, anything else is returned verbatim.
func decodeText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return string(raw)
}

func judgePrompt(run *trajectory.Run, output string) string {
	return fmt.Sprintf(
		"You are grading an AI agent run. Agent: %s.\n"+
			"The agent's final output was:\n---\n%s\n---\n"+
			"Grade it. Respond with ONLY a JSON object: "+
			`{"judgment": "good|neutral|bad", "score": 0.0-1.0, "comment": "short reason"}. `+
			"good = correct and useful; bad = wrong or failed; neutral = ambiguous.",
		run.Agent, output,
	)
}

type verdict struct {
	Judgment string  `json:"judgment"`
	Score    float64 `json:"score"`
	Comment  string  `json:"comment"`
	judgment Judgment
}

// parseVerdict extracts the JSON verdict object from the model reply, tolerating
// surrounding prose or code fences.
func parseVerdict(raw string) (verdict, error) {
	start := strings.IndexByte(raw, '{')
	end := strings.LastIndexByte(raw, '}')
	if start == -1 || end == -1 || end < start {
		return verdict{}, errors.New("no JSON object found")
	}
	var v verdict
	if err := json.Unmarshal([]byte(raw[start:end+1]), &v); err != nil {
		return verdict{}, err
	}
	switch strings.ToLower(strings.TrimSpace(v.Judgment)) {
	case "good":
		v.judgment = JudgmentGood
	case "bad":
		v.judgment = JudgmentBad
	case "neutral":
		v.judgment = JudgmentNeutral
	default:
		return verdict{}, fmt.Errorf("invalid judgment %q", v.Judgment)
	}
	return v, nil
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// GeminiClient is the production Completion backed by the Gemini REST API. It
// uses only the standard library so the Go core stays dependency-free.
type GeminiClient struct {
	APIKey     string
	Model      string
	HTTPClient *http.Client
	// Endpoint is the API base URL; empty means the public Gemini endpoint.
	Endpoint string
}

// NewGeminiClient builds a client, reading the key from GEMINI_API_KEY.
func NewGeminiClient(model string) (*GeminiClient, error) {
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		return nil, errors.New("GEMINI_API_KEY not set")
	}
	if model == "" {
		model = DefaultJudgeModel
	}
	return &GeminiClient{APIKey: key, Model: model, HTTPClient: http.DefaultClient}, nil
}

func (c *GeminiClient) base() string {
	if c.Endpoint != "" {
		return c.Endpoint
	}
	return "https://generativelanguage.googleapis.com"
}

func (c *GeminiClient) Complete(ctx context.Context, prompt string) (string, error) {
	reqBody := map[string]any{
		"contents": []map[string]any{
			{"parts": []map[string]any{{"text": prompt}}},
		},
		"generationConfig": map[string]any{
			"temperature":     0,
			"maxOutputTokens": 256,
		},
	}
	buf, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", c.base(), c.Model, c.APIKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gemini API %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var parsed struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Candidates) == 0 || len(parsed.Candidates[0].Content.Parts) == 0 {
		return "", errors.New("gemini API returned no content")
	}
	return parsed.Candidates[0].Content.Parts[0].Text, nil
}
