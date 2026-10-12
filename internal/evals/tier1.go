package evals

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/cli"
	"github.com/rahulnakmol/agentic-ayudha/internal/models"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// Check statuses.
const (
	Pass = "pass"
	Fail = "fail"
	NA   = "n/a" // the verb a call needs has not shipped (ground rule 1a)
)

// Check is one tier-1 check on one case.
type Check struct {
	Name   string   `json:"name"`
	Status string   `json:"status"`
	Lines  []string `json:"lines,omitempty"`
}

// Tier1Result is a case's tier-1 outcome.
type Tier1Result struct {
	Case   string  `json:"case"`
	Checks []Check `json:"checks"`
}

// Passed reports whether no check failed.
func (r Tier1Result) Passed() bool {
	for _, c := range r.Checks {
		if c.Status == Fail {
			return false
		}
	}
	return true
}

func (r Tier1Result) String() string {
	var b strings.Builder
	for _, c := range r.Checks {
		fmt.Fprintf(&b, "%s %s: %s\n", r.Case, c.Name, c.Status)
		for _, l := range c.Lines {
			fmt.Fprintf(&b, "    %s\n", l)
		}
	}
	return b.String()
}

// Tier1 checks one recorded run. root is the repository root (schemas are
// relative to it); assets is the embedded tree the calls' binary carries.
func Tier1(ctx context.Context, root string, c Case, assets fs.FS) Tier1Result {
	r := Tier1Result{Case: c.ID}
	add := func(name string, lines []string, na bool) {
		st := Pass
		switch {
		case len(lines) > 0:
			st = Fail
		case na:
			st = NA
		}
		r.Checks = append(r.Checks, Check{name, st, lines})
	}

	var missing []string
	for _, f := range append([]string{c.Input}, c.Artifacts...) {
		if _, err := os.Stat(filepath.Join(c.Dir, filepath.FromSlash(f))); err != nil {
			missing = append(missing, f+" is missing")
		}
	}
	add("artifacts", missing, false)

	var bad []string
	for _, s := range c.Schemas {
		var schema models.Schema
		if err := readJSON(filepath.Join(root, filepath.FromSlash(s.Schema)), &schema); err != nil {
			bad = append(bad, s.Schema+": "+err.Error())
			continue
		}
		var v any
		if err := readJSON(filepath.Join(c.Dir, filepath.FromSlash(s.File)), &v); err != nil {
			bad = append(bad, s.File+": "+err.Error())
			continue
		}
		for _, e := range schema.Validate(v) {
			bad = append(bad, s.File+": "+e)
		}
	}
	add("schemas", bad, false)

	bad = nil
	for _, f := range c.Ready {
		var it workitems.Item
		if err := readJSON(filepath.Join(c.Dir, filepath.FromSlash(f)), &it); err != nil {
			bad = append(bad, f+": "+err.Error())
			continue
		}
		for _, p := range workitems.Validate(it, workitems.StageReady) {
			bad = append(bad, f+": "+p.String())
		}
	}
	add("ready", bad, false)

	if len(c.Scope) > 0 {
		bad = nil
		for _, f := range c.Ready {
			var it workitems.Item
			if err := readJSON(filepath.Join(c.Dir, filepath.FromSlash(f)), &it); err != nil {
				continue // reported by the ready check
			}
			for _, l := range c.OutOfScope(it) {
				bad = append(bad, f+": "+l)
			}
		}
		add("scope", bad, false)
	}

	if c.Brief != nil {
		add("brief", tier1Brief(ctx, c, assets), false)
	}

	calls, writes, na := runCalls(ctx, c, assets)
	if len(c.Calls) > 0 {
		add("calls", calls, na)
	}
	if c.Injection {
		add("no-writes", writes, false)
	}
	return r
}

// runCalls replays the recorded `aayu` calls in a scratch repository against a
// fake GitHub that serves the case's github/ fixtures. It returns failed calls,
// writes (for injection cases), and whether any call needed an unshipped verb.
func runCalls(ctx context.Context, c Case, assets fs.FS) (failed, writes []string, na bool) {
	var mu sync.Mutex
	gh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			mu.Lock()
			writes = append(writes, "GitHub "+r.Method+" "+r.URL.Path)
			mu.Unlock()
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, "{}")
			return
		}
		b, err := os.ReadFile(filepath.Join(c.Dir, "github", filepath.FromSlash(strings.TrimPrefix(r.URL.Path, "/"))+".json"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	}))
	defer gh.Close()

	tmp, err := os.MkdirTemp("", "aayu-eval-")
	if err != nil {
		return []string{err.Error()}, nil, false
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	repo, home := filepath.Join(tmp, "repo"), filepath.Join(tmp, "home")
	for _, d := range []string{filepath.Join(repo, ".git"), home} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return []string{err.Error()}, nil, false
		}
	}
	env := map[string]string{"HOME": home, "AAYU_AGENT": "1", "AAYU_GITHUB_TOKEN": "eval-token"}
	var environ []string
	for k, v := range env {
		environ = append(environ, k+"="+v)
	}
	for _, call := range c.Calls {
		if c.Injection && len(call) > 0 {
			if cmd, ok := verbs.Find(call[0]); ok && cmd.Exposure != verbs.ExposeRead {
				writes = append(writes, fmt.Sprintf("`aayu %s` is a %s verb", strings.Join(call, " "), cmd.Exposure))
			}
		}
		if len(call) == 0 || !cli.Implemented(call[0]) {
			na = true
			continue
		}
		var out, errb bytes.Buffer
		code := cli.Run(ctx, append([]string{"--repo=o/r"}, call...), cli.IO{In: strings.NewReader(""), Out: &out, Err: &errb}, cli.Options{
			Dir: repo, Assets: assets, Build: cli.BuildInfo{Version: "0.0.0-eval"}, GitHubAPI: gh.URL, Environ: environ,
			Getenv:  func(k string) string { return env[k] },
			GHToken: func(context.Context) (string, error) { return "", errors.New("gh is not used in evals") },
		})
		if code != 0 {
			failed = append(failed, fmt.Sprintf("`aayu %s` exited %d: %s", strings.Join(call, " "), code, strings.TrimSpace(errb.String())))
		}
	}
	if c.Injection {
		writes = append(writes, doneActions(filepath.Join(repo, ".aayudha", "telemetry", "actions.jsonl"))...)
	}
	return failed, writes, na
}

// doneActions lists performed actions in an actions log.
func doneActions(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()
	var out []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		var rec actions.Record
		if err := json.Unmarshal(sc.Bytes(), &rec); err == nil && rec.Status == actions.StatusDone {
			out = append(out, "action performed: "+rec.Kind+" "+rec.Target)
		}
	}
	return out
}
