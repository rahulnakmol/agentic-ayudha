package evals

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/models"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// A step is saved as soon as it returns, failed or not; a rerun on the same
// directory reuses only a step that finished under the same key and sample,
// and gives do the last saved attempt otherwise, so a caller can raise the
// cap of a step that stopped at it.
func TestStepsSaveAndResume(t *testing.T) {
	dir := t.TempDir()
	c := Case{ID: "survey/tinyjs-rust"}
	fake := &models.Fake{Replies: []models.Response{{Text: "engineering v1", CostUSD: 0.25}, {Text: "engineering v2"}, {Text: "sample 2"}}}
	h := harness(t, fake, "gpt-6.1-sol")
	h.Steps, h.model = dir, "claude-opus-5-5"
	req := models.Request{System: "survey", Messages: []models.Message{{Role: "user", Content: "the repo"}}, MaxTokens: 8000}
	sample := func(i int) context.Context { return context.WithValue(context.Background(), sampleKey{}, i) }
	var calls int
	call := func(status string, fail error) func(*Step, *[]CallLog) (Step, error) {
		return func(_ *Step, log *[]CallLog) (Step, error) {
			calls++
			if fail != nil {
				return Step{Findings: []string{"half a section"}}, fail
			}
			resp, err := h.complete(verbs.LaneSpec, "skill", log)(context.Background(), req)
			return Step{Status: status, Output: resp.Text}, err
		}
	}

	// A call that errors is saved with its status and findings.
	if _, err := h.step(sample(0), c, "engineering", stepKey(req), call("", errors.New("gateway 502"))); err == nil {
		t.Fatal("the step's error was swallowed")
	}
	// The rerun does the work again, seeing the failed attempt.
	var seen *Step
	st, err := h.step(sample(0), c, "engineering", stepKey(req), func(prev *Step, log *[]CallLog) (Step, error) {
		seen = prev
		return call("", nil)(prev, log)
	})
	if err != nil || seen == nil || seen.Status != StepError || seen.Error != "gateway 502" || len(seen.Findings) != 1 {
		t.Fatalf("the rerun did not get the failed attempt: %+v, %v", seen, err)
	}
	if st.Status != StepDone || st.Output != "engineering v1" || st.CostUSD != 0.25 || len(st.Calls) != 1 || st.Calls[0].Step != "engineering" {
		t.Fatalf("done step: %+v", st)
	}
	var saved Step
	if b, err := os.ReadFile(filepath.Join(dir, "survey", "tinyjs-rust", "0", "engineering.json")); err != nil || json.Unmarshal(b, &saved) != nil || saved.Output != "engineering v1" {
		t.Fatalf("not saved where the workflow uploads it: %v %+v", err, saved)
	}

	// A new run (another harness) on the same directory reuses the done step without a call.
	h2 := harness(t, fake, "gpt-6.1-sol")
	h2.Steps, h2.model = dir, "claude-opus-5-5"
	before := calls
	st, err = h2.step(sample(0), c, "engineering", stepKey(req), call("", nil))
	if err != nil || calls != before || !st.Resumed || st.Output != "engineering v1" {
		t.Fatalf("a done step was not resumed: %+v %v (%d calls)", st, err, calls-before)
	}
	// Another sample never shares it.
	if st, _ = h2.step(sample(1), c, "engineering", stepKey(req), call("", nil)); st.Resumed || calls != before+1 {
		t.Fatalf("sample 2 reused sample 1's step: %+v", st)
	}
	// A higher cap is another key: the step reruns, given the earlier attempt.
	more := req
	more.MaxTokens = 16000
	seen = nil
	st, _ = h2.step(sample(0), c, "engineering", stepKey(more), func(prev *Step, log *[]CallLog) (Step, error) {
		seen = prev
		return Step{Status: StepTruncated, Output: "cut"}, nil
	})
	if st.Resumed || seen == nil || seen.Output != "engineering v1" {
		t.Fatalf("a raised cap reused the old step: %+v, prev %+v", st, seen)
	}
	// A truncated step is saved but not reused.
	st, _ = h2.step(sample(0), c, "engineering", stepKey(more), func(prev *Step, _ *[]CallLog) (Step, error) {
		seen = prev
		return Step{Output: "whole"}, nil
	})
	if st.Resumed || seen.Status != StepTruncated || st.Status != StepDone {
		t.Fatalf("a truncated step was reused: %+v, prev %+v", st, seen)
	}
	// So is a step of another model.
	h2.model = "claude-sonnet-5-5"
	if st, _ = h2.step(sample(0), c, "engineering", stepKey(more), func(*Step, *[]CallLog) (Step, error) { return Step{}, nil }); st.Resumed {
		t.Fatal("a step of another model was reused")
	}
}
