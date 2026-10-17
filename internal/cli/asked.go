package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/doctor"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
)

// The asked gate (D-5.5c.20): an agent picks up only a story the person asked
// for. hosts.Routing tells every host's agent the same rule.

// askedTTL bounds "this session" when a host gives no session ID.
const askedTTL = 12 * time.Hour

// askedRecord is a line of .aayudha/state/asked.jsonl: a story that came out
// of the person's grill in an agent session.
type askedRecord struct {
	Repo    string `json:"repo"`
	Story   int    `json:"story"`
	Session string `json:"session,omitempty"`
	At      string `json:"at"`
}

func (e *Env) askedPath() string { return filepath.Join(e.stateDir(), "state", "asked.jsonl") }

// agentShell names the signal that an agent runs this process: AAYU_AGENT=1,
// or the marker a host sets in its agent's commands. Cursor's own terminal
// sets CURSOR_TRACE_ID for a person too, so it does not count.
func (e *Env) agentShell() string {
	if e.Agent() {
		return "AAYU_AGENT=1"
	}
	if host, how := doctor.DetectHost("", e.Getenv); host != "" && how != "CURSOR_TRACE_ID" {
		return how
	}
	return ""
}

// session is the host's ID for the agent session, when it exports one.
func (e *Env) session() string {
	for _, k := range []string{"AMP_THREAD_ID", "CODEX_THREAD_ID", "CLAUDE_CODE_SESSION_ID"} {
		if v := e.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

// recordGrilled notes that grill filed story n from the person's words in
// this session, so build may pick it up without asking again.
func (e *Env) recordGrilled(repo string, n int) error {
	if err := os.MkdirAll(filepath.Dir(e.askedPath()), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(e.askedPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	b, _ := json.Marshal(askedRecord{Repo: repo, Story: n, Session: e.session(), At: e.Now().UTC().Format(time.RFC3339)})
	_, err = f.Write(append(b, '\n'))
	return errors.Join(err, f.Close())
}

// grilledHere reports whether grill filed story n in this session.
func (e *Env) grilledHere(repo string, n int) bool {
	f, err := os.Open(e.askedPath())
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	s, now := bufio.NewScanner(f), e.Now()
	for s.Scan() {
		var r askedRecord
		if json.Unmarshal(s.Bytes(), &r) != nil || r.Repo != repo || r.Story != n || r.Session != e.session() {
			continue
		}
		if at, err := time.Parse(time.RFC3339, r.At); err == nil && now.Sub(at) < askedTTL {
			return true
		}
	}
	return false
}

// askedGate holds an agent's pickup of a Ready story the person did not ask
// for: one that did not come out of grill in this session and has no --asked
// words. It runs before the lease, so a held story stays untouched. A person
// in their own terminal, and a story already in progress (a later checkpoint,
// a delegation sync), pass. The confirmation or the hold is logged.
func (e *Env) askedGate(ctx context.Context, c *github.Client, repo string, n int, title string, labels []string, in Values) error {
	marker := e.agentShell()
	if marker == "" || contains(labels, labelInProgress) || e.grilledHere(repo, n) {
		return nil
	}
	words := in.String("asked")
	var held error
	if words == "" {
		cmd := "aayu build " + strconv.Itoa(n)
		if d := in.String("delegate"); d != "" {
			cmd += " --delegate " + d
		}
		if in.Bool("local") {
			cmd += " --local"
		}
		cmd += ` --asked "<their words>"`
		held = WithNext(Errorf(ExitBlocked, "build #%d %q: the person has not asked for this story in this session (%s), so build stopped before taking it. "+
			"Ask them first: \"Shall I build #%d, %s?\" Build it only if they say yes, and pass their words: %s",
			n, title, marker, n, title, cmd), "ask the person; on yes, "+cmd)
	}
	log, err := e.Actions()
	if err != nil {
		return err
	}
	_, err = log.Do(ctx, actions.Action{Kind: "build.asked", Target: fmt.Sprintf("%s#%d", repo, n), Story: fmt.Sprintf("#%d", n),
		Seat: "ai-native-engineer", Identity: c.Identity(), Payload: map[string]any{"words": words, "agent": marker, "session": e.session(), "held": held != nil}},
		func(context.Context) (actions.Outcome, error) { return actions.Outcome{}, held })
	if held != nil {
		return held
	}
	return err
}
