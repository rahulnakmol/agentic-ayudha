package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/notify"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// HaltVariable is the repository Actions variable `aayu halt` sets (WP5.7).
// Every workflow checks vars.AAYUDHA_HALT in its first step and every verb
// checks it before and while it runs; `aayu resume` deletes it.
const HaltVariable = "AAYUDHA_HALT"

// DefaultHaltPoll is how often a running verb re-reads the halt variable.
const DefaultHaltPoll = 30 * time.Second

func init() {
	register("halt", runHalt)
	register("resume", runResume)
}

func (e *Env) haltPoll() time.Duration {
	if e.HaltPoll > 0 {
		return e.HaltPoll
	}
	return DefaultHaltPoll
}

func halted(reason string) error {
	return Errorf(ExitBlocked, "the repository is halted (%s): %s; a human clears it with `aayu resume`", HaltVariable, reason)
}

// lookupEnv reads a process variable, telling unset from empty: a workflow
// passes AAYUDHA_HALT from vars, empty when the repository is not halted.
func (e *Env) lookupEnv(k string) (string, bool) {
	for _, kv := range e.Environ {
		if v, ok := strings.CutPrefix(kv, k+"="); ok {
			return v, true
		}
	}
	v := e.Getenv(k)
	return v, v != ""
}

// haltAPI reads the halt variable through the API. ok is false when there is
// no repository or token to ask with; the verb then fails on its own need for them.
func (e *Env) haltAPI(ctx context.Context) (reason string, ok bool, err error) {
	repo := e.Repo
	if repo == "" {
		repo = e.originRepo(ctx)
	}
	if repo == "" {
		return "", false, nil
	}
	tok, err := e.Resolver().Human(ctx)
	if err != nil {
		return "", false, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.githubAPI()+"/repos/"+repo+"/actions/variables/"+HaltVariable, nil)
	if err != nil {
		return "", true, err
	}
	req.Header.Set("Authorization", "Bearer "+tok.Value)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := e.HTTP.Do(req)
	if err != nil {
		return "", true, fmt.Errorf("reading %s: %w", HaltVariable, err)
	}
	defer func() { _ = resp.Body.Close() }()
	switch resp.StatusCode {
	case http.StatusNotFound:
		return "", true, nil
	case http.StatusOK:
		var v struct {
			Value string `json:"value"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
			return "", true, fmt.Errorf("reading %s: %w", HaltVariable, err)
		}
		if v.Value == "" {
			v.Value = "halted"
		}
		return v.Value, true, nil
	}
	return "", true, fmt.Errorf("reading %s: GitHub answered %s", HaltVariable, resp.Status)
}

// checkHalt is the halt gate every verb passes before it runs. Agent lanes and
// CI fail closed when GitHub cannot say; a human gets a warning.
func (e *Env) checkHalt(ctx context.Context) error {
	if v, set := e.lookupEnv(HaltVariable); set {
		if v != "" {
			return halted(v)
		}
		return nil
	}
	reason, ok, err := e.haltAPI(ctx)
	e.haltWatch = ok && err == nil
	switch {
	case !ok:
		return nil
	case err != nil && (e.Agent() || e.CI):
		return Errorf(ExitBlocked, "cannot confirm the repository is not halted: %v", err)
	case err != nil:
		fmt.Fprintf(e.Err, "aayu: warning: %v; continuing\n", err)
	case reason != "":
		return halted(reason)
	}
	return nil
}

type haltCause struct{ reason string }

func (h haltCause) Error() string { return "halted: " + h.reason }

// watchHalt re-reads the halt variable every poll while a verb runs and
// cancels the verb's context when it is set. stop ends the watch and returns
// the halt error when the verb was stopped by a halt. It watches only when
// checkHalt could read the variable through the API (in a workflow the job's
// first step and the environment already decided).
func (e *Env) watchHalt(ctx context.Context) (context.Context, func() error) {
	if !e.haltWatch {
		return ctx, func() error { return nil }
	}
	ctx, cancel := context.WithCancelCause(ctx)
	done := make(chan struct{})
	go func() {
		t := time.NewTicker(e.haltPoll())
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-t.C:
				if reason, ok, err := e.haltAPI(ctx); ok && err == nil && reason != "" {
					cancel(haltCause{reason})
					return
				}
			}
		}
	}()
	return ctx, func() error {
		close(done)
		var h haltCause
		if errors.As(context.Cause(ctx), &h) {
			cancel(nil)
			return halted(h.reason)
		}
		cancel(nil)
		return nil
	}
}

// haltCached is the halt reason for the pre-tool hook, which runs on every
// agent tool call: it asks GitHub at most once per poll and caches the answer.
func (e *Env) haltCached(ctx context.Context) string {
	path := filepath.Join(e.stateDir(), "state", "halt.json")
	var c struct {
		Checked time.Time `json:"checked"`
		Reason  string    `json:"reason"`
	}
	if b, err := os.ReadFile(path); err == nil && json.Unmarshal(b, &c) == nil && e.Now().Sub(c.Checked) < e.haltPoll() {
		return c.Reason
	}
	if v, set := e.lookupEnv(HaltVariable); set {
		return v
	}
	reason, ok, err := e.haltAPI(ctx)
	if !ok || err != nil {
		return ""
	}
	c.Checked, c.Reason = e.Now(), reason
	if b, err := json.Marshal(c); err == nil && os.MkdirAll(filepath.Dir(path), 0o755) == nil {
		_ = os.WriteFile(path, b, 0o600)
	}
	return reason
}

type haltReport struct {
	Repo    string `json:"repo"`
	Halted  bool   `json:"halted"`
	Reason  string `json:"reason,omitempty"`
	Changed bool   `json:"changed"`
	Paged   string `json:"paged,omitempty"`
	Pause   string `json:"budget_pause_cleared,omitempty"`
	DryRun  bool   `json:"dry_run,omitempty"`
}

// variable sets (value != "") or deletes the halt variable through actions.
func (e *Env) variable(ctx context.Context, c *github.Client, repo, value string) (bool, error) {
	owner, name, _ := github.SplitRepo(repo)
	cur, _, err := c.REST.Actions.GetRepoVariable(ctx, owner, name, HaltVariable)
	exists := err == nil
	if err != nil && !github.NotFound(err) {
		return false, fmt.Errorf("reading %s: %w", HaltVariable, err)
	}
	path := "/repos/" + repo + "/actions/variables"
	a := actions.Action{Kind: "github.variable.set", Target: repo, Seat: string(verbs.Founder),
		Payload: map[string]any{"name": HaltVariable, "value": value}}
	var perform func(ctx context.Context) (any, error)
	switch {
	case value == "" && !exists:
		return false, nil
	case value == "":
		a.Kind, a.Payload = "github.variable.delete", map[string]any{"name": HaltVariable}
		a.Request = &actions.Request{Method: http.MethodDelete, Path: path + "/" + HaltVariable}
		perform = func(ctx context.Context) (any, error) {
			_, err := c.REST.Actions.DeleteRepoVariable(ctx, owner, name, HaltVariable)
			return nil, err
		}
	case exists && cur.Value == value:
		return false, nil
	case exists:
		a.Request = &actions.Request{Method: http.MethodPatch, Path: path + "/" + HaltVariable}
		perform = func(ctx context.Context) (any, error) {
			_, err := c.REST.Actions.UpdateRepoVariable(ctx, owner, name, HaltVariable, gh.ActionsUpdateVariableRequest{Value: &value})
			return nil, err
		}
	default:
		a.Request = &actions.Request{Method: http.MethodPost, Path: path}
		perform = func(ctx context.Context) (any, error) {
			_, err := c.REST.Actions.CreateRepoVariable(ctx, owner, name, gh.ActionsCreateVariableRequest{Name: HaltVariable, Value: value})
			return nil, err
		}
	}
	if _, err := c.Write(ctx, a, perform); err != nil {
		if github.External(err) && strings.Contains(err.Error(), "403") {
			return false, Wrap(ExitBlocked, fmt.Errorf("%w: setting repository variables needs a token with Variables: write (fine-grained) or repo scope", err))
		}
		return false, err
	}
	return true, nil
}

func (e *Env) page(ctx context.Context, p notify.Page) string {
	n, err := e.Notifier(ctx)
	if err != nil {
		return "not sent: " + err.Error()
	}
	st, err := n.Page(ctx, p)
	if err != nil {
		return "failed: " + err.Error()
	}
	return st
}

// runHalt sets the halt variable: every workflow's first step and every verb
// refuse to run until `aayu resume`, and running verbs stop within one poll.
func runHalt(ctx context.Context, e *Env, in Values) error {
	c, tok, err := e.HumanGitHub(ctx)
	if err != nil {
		return err
	}
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
	who := "unknown"
	if u, err := identity.WhoAmI(ctx, e.HTTP, e.githubAPI(), tok); err == nil {
		who = "@" + u.Login
	}
	reason := strings.TrimSpace(in.String("reason"))
	if reason == "" {
		reason = "no reason given"
	}
	value := fmt.Sprintf("%s (by %s at %s)", reason, who, e.Now().UTC().Format(time.RFC3339))
	if cur, _, err := e.haltAPI(ctx); err == nil && cur != "" {
		value = cur // already halted: keep the first reason
	}
	changed, err := e.variable(ctx, c, repo, value)
	if err != nil {
		return err
	}
	r := haltReport{Repo: repo, Halted: true, Reason: value, Changed: changed, DryRun: e.DryRun}
	if changed && !e.DryRun {
		r.Paged = e.page(ctx, notify.Page{Key: "halt:" + repo, Urgent: true, Subject: "aayudha halted " + repo, Body: value + "\n\nEvery agent lane, verb, and aayudha workflow stops until a human runs `aayu resume`."})
	}
	return e.Emit(r, func(w io.Writer) error {
		_, err := fmt.Fprintf(w, "%s halted: %s\nEvery workflow and verb refuses to run, and running verbs stop within %s. `aayu resume` clears it.\n", repo, value, e.haltPoll())
		return err
	})
}

// runResume clears the halt variable and the [budget] breaker's pause file.
func runResume(ctx context.Context, e *Env, _ Values) error {
	if err := e.requireHuman(ctx, "resume", AccessWrite); err != nil {
		return err
	}
	c, _, err := e.HumanGitHub(ctx)
	if err != nil {
		return err
	}
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
	changed, err := e.variable(ctx, c, repo, "")
	if err != nil {
		return err
	}
	r := haltReport{Repo: repo, Changed: changed, DryRun: e.DryRun}
	if _, err := os.Stat(e.PauseFile()); err == nil {
		r.Pause = e.PauseFile()
		if !e.DryRun {
			if err := os.Remove(e.PauseFile()); err != nil {
				return err
			}
		}
	}
	_ = os.Remove(filepath.Join(e.stateDir(), "state", "halt.json"))
	if changed && !e.DryRun {
		r.Paged = e.page(ctx, notify.Page{Key: "resume:" + repo, Urgent: true, Subject: "aayudha resumed " + repo, Body: "The halt is cleared; agent lanes, verbs, and workflows run again."})
	}
	return e.Emit(r, func(w io.Writer) error {
		state := "was not halted"
		if changed {
			state = "resumed"
		}
		fmt.Fprintf(w, "%s %s\n", repo, state)
		if r.Pause != "" {
			fmt.Fprintf(w, "cleared the [budget] breaker pause (%s)\n", r.Pause)
		}
		return nil
	})
}
