package evaluator

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Cro22/trazo/trajectory"
)

type fakeJudge struct {
	reply string
	err   error
}

func (f fakeJudge) Complete(ctx context.Context, prompt string) (string, error) {
	return f.reply, f.err
}

func llmOut(output string) trajectory.Step {
	return trajectory.Step{
		Type:   trajectory.StepTypeCallLLM,
		LLM:    "m",
		Output: json.RawMessage(output),
	}
}

func judgeRun(steps ...trajectory.Step) *trajectory.Run {
	return &trajectory.Run{ID: "run-judge", Agent: "github-triage", Steps: steps}
}

func TestLLMJudge_GoodVerdict(t *testing.T) {
	run := judgeRun(llmOut(`"1 bug, 1 doc. Clear report."`))
	e := &LLMJudgeEvaluator{Client: fakeJudge{reply: `{"judgment":"good","score":0.9,"comment":"clear"}`}}
	eval, err := e.EvaluateRun(run)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(eval.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(eval.Findings))
	}
	f := eval.Findings[0]
	if f.Judgment != JudgmentGood || f.Score != 0.9 {
		t.Errorf("unexpected finding: %+v", f)
	}
}

func TestLLMJudge_BadVerdictWithProse(t *testing.T) {
	// The judge wraps the JSON in prose and a code fence; we still parse it.
	reply := "Here is my grade:\n```json\n{\"judgment\": \"bad\", \"score\": 0.1, \"comment\": \"empty\"}\n```"
	e := &LLMJudgeEvaluator{Client: fakeJudge{reply: reply}}
	eval, _ := e.EvaluateRun(judgeRun(llmOut(`""`)))
	if len(eval.Findings) != 1 || eval.Findings[0].Judgment != JudgmentBad {
		t.Fatalf("expected 1 bad finding, got %+v", eval.Findings)
	}
}

func TestLLMJudge_UnparseableIsNeutral(t *testing.T) {
	e := &LLMJudgeEvaluator{Client: fakeJudge{reply: "I cannot comply."}}
	eval, _ := e.EvaluateRun(judgeRun(llmOut(`"x"`)))
	if len(eval.Findings) != 1 || eval.Findings[0].Judgment != JudgmentNeutral {
		t.Fatalf("expected 1 neutral finding, got %+v", eval.Findings)
	}
}

func TestLLMJudge_NoLLMCallNoFindings(t *testing.T) {
	run := judgeRun(trajectory.Step{Type: trajectory.StepTypeToolCall, Tool: "t"})
	eval, err := (&LLMJudgeEvaluator{Client: fakeJudge{reply: "unused"}}).EvaluateRun(run)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(eval.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(eval.Findings))
	}
}

func TestLLMJudge_ClientErrorPropagates(t *testing.T) {
	e := &LLMJudgeEvaluator{Client: fakeJudge{err: errors.New("network down")}}
	_, err := e.EvaluateRun(judgeRun(llmOut(`"x"`)))
	if err == nil {
		t.Fatal("expected an error when the client fails")
	}
}

func TestLLMJudge_NilClientErrors(t *testing.T) {
	_, err := (&LLMJudgeEvaluator{}).EvaluateRun(judgeRun(llmOut(`"x"`)))
	if err == nil {
		t.Fatal("expected an error when no client is configured")
	}
}

func TestGeminiClient_CompleteAgainstFakeServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"{\"judgment\":\"good\",\"score\":1,\"comment\":\"ok\"}"}]}}]}`))
	}))
	defer server.Close()

	client := &GeminiClient{APIKey: "test", Model: "gemini-2.5-flash", Endpoint: server.URL, HTTPClient: server.Client()}
	eval, err := (&LLMJudgeEvaluator{Client: client}).EvaluateRun(judgeRun(llmOut(`"a report"`)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(eval.Findings) != 1 || eval.Findings[0].Judgment != JudgmentGood {
		t.Fatalf("expected 1 good finding via the HTTP client, got %+v", eval.Findings)
	}
}

func TestGeminiClient_HTTPErrorSurfaces(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad key", http.StatusForbidden)
	}))
	defer server.Close()

	client := &GeminiClient{APIKey: "x", Model: "m", Endpoint: server.URL, HTTPClient: server.Client()}
	_, err := client.Complete(context.Background(), "hi")
	if err == nil {
		t.Fatal("expected an error on a 403 response")
	}
}
