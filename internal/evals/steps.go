package evals

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/models"
)

// A step is one saved unit of a run's work, such as a survey section: a model
// call or two, their output, and what the floors found in it. Each step is
// written to Harness.Steps as soon as it returns, failed or not, so a run lost
// to a later outage keeps it, and a rerun given the same directory reloads
// every step that finished and redoes the rest.
type Step struct {
	Name string `json:"name"`
	// Key identifies the work the step did (the case, the sample, the models,
	// and the caller's key, normally stepKey of its request). A finished step
	// is reused only under the same key, so raising a step's cap reruns it.
	Key string `json:"key"`
	// Status is StepDone, or StepTruncated, StepFailed, or StepError.
	Status   string    `json:"status"`
	Output   string    `json:"output,omitempty"`
	Findings []string  `json:"findings,omitempty"`
	Error    string    `json:"error,omitempty"`
	Calls    []CallLog `json:"calls,omitempty"`
	CostUSD  float64   `json:"cost_usd"`
	// Resumed marks a step loaded from an earlier run: its calls and cost were
	// charged to that run, not this one.
	Resumed bool `json:"resumed,omitempty"`
}

// Step statuses. Only a done step is reused on resume.
const (
	StepDone      = "done"
	StepTruncated = "truncated" // the reply stopped at its token cap
	StepFailed    = "failed"    // the floors failed
	StepError     = "error"     // a call errored
)

// sampleKey carries the index of the case's sample (0..k-1) a run is.
type sampleKey struct{}

// stepKey is the part of a step's key that its request decides: the prompt,
// the token cap, the reasoning effort, the structured-output schema, and
// anything else the caller names (a rubric, a section's settings).
func stepKey(req models.Request, extra ...string) string {
	b, _ := json.Marshal(struct {
		System    string
		Messages  []models.Message
		MaxTokens int
		Effort    string
		Format    *models.Format
		Extra     []string
	}{req.System, req.Messages, req.MaxTokens, req.Effort, req.Format, extra})
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// step runs do as the step name of case c's current sample, unless
// Harness.Steps holds that step done under the same key, which it returns
// (Resumed) without calling do. do gets the step's last saved attempt, if
// any, whatever its status or key (nil otherwise), so it can tell that the
// last try stopped at its cap, and logs its calls in calls. Whatever do
// returns, error included, is saved before step returns. A Step with no
// Status from a nil error is done.
func (h *Harness) step(ctx context.Context, c Case, name, key string, do func(prev *Step, calls *[]CallLog) (Step, error)) (Step, error) {
	sample, _ := ctx.Value(sampleKey{}).(int)
	b, _ := json.Marshal([]string{c.ID, strconv.Itoa(sample), name, h.model, h.fallback, key})
	sum := sha256.Sum256(b)
	key = hex.EncodeToString(sum[:])
	path := ""
	var prev *Step
	if h.Steps != "" {
		path = filepath.Join(h.Steps, filepath.FromSlash(c.ID), strconv.Itoa(sample), stepFile(name))
		var saved Step
		if raw, err := os.ReadFile(path); err == nil && json.Unmarshal(raw, &saved) == nil {
			if saved.Status == StepDone && saved.Key == key {
				saved.Resumed = true
				return saved, nil
			}
			prev = &saved
		}
	}
	var calls []CallLog
	st, err := do(prev, &calls)
	st.Name, st.Key, st.Resumed = name, key, false
	st.Calls, st.CostUSD = calls, 0
	for i := range st.Calls {
		st.Calls[i].Step = name
		st.CostUSD += st.Calls[i].CostUSD
		for _, r := range st.Calls[i].Retries {
			st.CostUSD += r.CostUSD
		}
	}
	switch {
	case err != nil:
		st.Status, st.Error = StepError, clip(actions.RedactString(err.Error()), 300)
	case st.Status == "":
		st.Status = StepDone
	}
	if path != "" {
		if werr := writeStep(path, st); werr != nil {
			return st, errors.Join(err, werr)
		}
	}
	return st, err
}

// stepFile is a step name as a file name in its sample's directory.
func stepFile(name string) string {
	return strings.NewReplacer("/", "_", "\\", "_").Replace(name) + ".json"
}

// writeStep saves st at path through a temporary file, so a run killed
// mid-write never leaves a torn step behind.
func writeStep(path string, st Step) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("save step: %w", err)
	}
	return nil
}
