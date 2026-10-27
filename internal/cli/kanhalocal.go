package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/delegate"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/triage"
)

// A local Kanha (Story #1012, ST13, Y9): Codex, Copilot, and OpenCode, and
// Cursor on a machine with no Cursor API key, run the Kanha as a session of
// the tool's CLI on the starter's machine. aayu starts it with the brief and
// resumes it with each event; it never runs in between. Events for it are
// recorded (tellKanha), and a check on its machine gives them to it: every
// [mission] check_minutes (`aayu mission wake <f>`), or at once when the
// person opens `aayu inbox`. While the machine is off, nothing gives the
// Kanha its events, so no story of its Feature starts.

// thisMachine is the host part of a local Kanha's login@machine here.
func thisMachine() string {
	h, _ := os.Hostname()
	return h
}

// localHere reports whether p is a local Kanha on this machine.
func localHere(p *ready.Parent) bool {
	_, host, ok := strings.Cut(p.Machine, "@")
	return p.Mode == "local" && ok && host == thisMachine()
}

// cursorLocal reports whether a Cursor Kanha runs locally here: this
// machine has no Cursor API key, and the Cursor CLI is on its PATH.
func (e *Env) cursorLocal(cfg config.Delegate) bool {
	if key, _, _ := e.SecretStore().Get(orDefault(cfg.CursorSecret, "cursor-api-key")); key != "" {
		return false
	}
	_, ok := delegate.LookLocal(delegate.Cursor, e.lookPath())
	return ok
}

// startLocalKanha starts tool's CLI headless in dir on the brief, as the
// person's own session, and returns its session ID. The session runs aayu,
// so it keeps the person's GitHub sign-in, and AAYU_AGENT=1 refuses it every
// human-only step.
func (e *Env) startLocalKanha(ctx context.Context, tool, dir, brief string) (string, error) {
	if e.LocalKanha != nil {
		return e.LocalKanha(ctx, tool, dir, brief)
	}
	bin, ok := delegate.LookLocal(tool, e.lookPath())
	if !ok {
		return "", fmt.Errorf("%s's CLI is not on this machine's PATH", delegate.Names[tool])
	}
	id := ""
	if tool == delegate.Copilot { // Copilot takes the session ID aayu picks
		id = delegate.NewSessionID()
	}
	ctx, cancel := context.WithTimeout(ctx, e.localLimit())
	defer cancel()
	out, err := e.delegateExec()(ctx, dir, e.kanhaEnv(), "", bin, delegate.KanhaStart(tool, id, brief)...)
	if err != nil {
		return "", err
	}
	if id == "" {
		id = delegate.KanhaSession(tool, out)
	}
	if id == "" {
		return "", fmt.Errorf("%s: no session ID in the CLI's output: %s", delegate.Names[tool], clipText(out, 200))
	}
	if tool == delegate.Cursor { // a new empty chat: the brief is its first run
		_, err = e.delegateExec()(ctx, dir, e.kanhaEnv(), "", bin, delegate.KanhaResume(tool, id, brief)...)
	}
	return id, err
}

// kanhaEnv is a local Kanha's environment: the person's own, marked as an
// agent lane.
func (e *Env) kanhaEnv() []string {
	env := slices.DeleteFunc(slices.Clone(e.Environ), func(kv string) bool { return strings.HasPrefix(kv, "AAYU_AGENT=") })
	return append(env, "AAYU_AGENT=1")
}

// kanhaEvents are the events recorded for Feature f's Kanha after p's last
// check, oldest first: each story's Kanha lines (tellKanha).
func kanhaEvents(ctx context.Context, w *ledger.Writer, f int, plan *MissionPlan, p *ready.Parent) ([]delegate.Event, error) {
	var out []delegate.Event
	for _, it := range plan.Graph {
		var st delegate.State
		if _, err := readLedgerJSON(ctx, w, delegate.LedgerPath(it.Number), &st); err != nil {
			return nil, Wrap(ExitExternal, err)
		}
		if st.Feature != 0 && st.Feature != f {
			continue
		}
		for _, ev := range st.Events {
			if ev.Status == "kanha" && ev.Text != "" && ev.At > p.LastCheck {
				out = append(out, ev)
			}
		}
	}
	slices.SortStableFunc(out, func(a, b delegate.Event) int { return strings.Compare(a.At, b.At) })
	return out, nil
}

// kanhaCheckPath is where this checkout keeps the time of the last check of
// Feature f's local Kanha: a timer, so it is not on the ledger.
func (e *Env) kanhaCheckPath(f int) string {
	return filepath.Join(e.stateDir(), "state", "kanha", strconv.Itoa(f)+".json")
}

// lastLocalCheck is the time of this machine's last check of f's Kanha.
func (e *Env) lastLocalCheck(f int) time.Time {
	var c struct {
		At time.Time `json:"at"`
	}
	if b, err := os.ReadFile(e.kanhaCheckPath(f)); err == nil {
		_ = json.Unmarshal(b, &c)
	}
	return c.At
}

func (e *Env) markLocalCheck(f int, at time.Time) error {
	p := e.kanhaCheckPath(f)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	b, _ := json.Marshal(map[string]time.Time{"at": at})
	return os.WriteFile(p, b, 0o600)
}

// checkMinutes is [mission] check_minutes.
func (e *Env) checkMinutes() time.Duration {
	l, err := e.Config()
	if err != nil || l.Config.Mission.CheckMinutes < 1 {
		return 5 * time.Minute
	}
	return time.Duration(l.Config.Mission.CheckMinutes) * time.Minute
}

// checkLocalKanha is one check of Feature f's local Kanha on this machine:
// once check_minutes passed since the last one, or at once (now), it resumes
// the Kanha's session with the events since its last check, one per line,
// and records that check in the parent. It says what it did.
func (e *Env) checkLocalKanha(ctx context.Context, c *github.Client, w *ledger.Writer, repo string, f int, plan *MissionPlan, appr *ready.Approval, now bool) (string, error) {
	p := appr.Parent
	at := e.Now().UTC()
	if last := e.lastLocalCheck(f); !now && !last.IsZero() && at.Before(last.Add(e.checkMinutes())) {
		return fmt.Sprintf("the Kanha of #%d: not due; its next check is at %s", f, last.Add(e.checkMinutes()).Format(time.RFC3339)), nil
	}
	if err := e.markLocalCheck(f, at); err != nil {
		return "", Wrap(ExitExternal, err)
	}
	evs, err := kanhaEvents(ctx, w, f, plan, p)
	if err != nil || len(evs) == 0 {
		return fmt.Sprintf("the Kanha of #%d: no event since its last check", f), err
	}
	lines := make([]string, len(evs))
	for i, ev := range evs {
		lines[i] = ev.Text
	}
	text := strings.Join(lines, "\n")
	log, err := e.Actions()
	if err != nil {
		return "", err
	}
	_, err = log.Do(ctx, actions.Action{Kind: "mission.kanha.tell", Target: fmt.Sprintf("%s#%d@%s", repo, f, p.Host), Story: fmt.Sprintf("#%d", f),
		Seat: "engineering-manager", Host: e.Host, Identity: c.Identity(), Payload: map[string]any{"session": p.Session, "event": text, "mode": "local"}},
		func(ctx context.Context) (actions.Outcome, error) {
			return actions.Outcome{Attempts: 1}, e.resumeLocalKanha(ctx, p, kanhaPrompt(text))
		})
	if err != nil {
		return "", Wrap(ExitExternal, fmt.Errorf("the Kanha of #%d on %s: %w", f, p.Machine, err))
	}
	q := *p
	q.LastCheck = evs[len(evs)-1].At
	if err := e.writeParent(ctx, w, f, appr.Version, &q); err != nil {
		return "", err
	}
	return fmt.Sprintf("the Kanha of #%d: resumed %s session %s with %d event(s)", f, delegate.Names[p.Host], p.Session, len(evs)), nil
}

// kanhaPrompt is the events for a local Kanha's resume, marked as data: an
// event can carry a builder's stop words or a receiver CLI's error output.
func kanhaPrompt(events string) string {
	return triage.Untrusted("aayu: the events of your mission since your last check", events)
}

// resumeLocalKanha resumes the local session p with the prompt. A turn ends
// at [delegate] local_timeout_minutes, so a hung CLI never holds the inbox
// or the wake; its events stay unread for the next check.
func (e *Env) resumeLocalKanha(ctx context.Context, p *ready.Parent, prompt string) error {
	bin, ok := delegate.LookLocal(p.Host, e.lookPath())
	if !ok {
		return fmt.Errorf("%s's CLI is not on this machine's PATH", delegate.Names[p.Host])
	}
	ctx, cancel := context.WithTimeout(ctx, e.localLimit())
	defer cancel()
	_, err := e.delegateExec()(ctx, config.RepoRoot(e.Dir), e.kanhaEnv(), "", bin, delegate.KanhaResume(p.Host, p.Session, prompt)...)
	return err
}

// localLimit is the longest run of a local CLI: [delegate] local_timeout_minutes, else 60 minutes.
func (e *Env) localLimit() time.Duration {
	if l, err := e.Config(); err == nil && l.Config.Delegate.LocalTimeoutMinutes > 0 {
		return time.Duration(l.Config.Delegate.LocalTimeoutMinutes) * time.Minute
	}
	return 60 * time.Minute
}

// checkHere checks every started mission whose Kanha is local on this
// machine, at once: `aayu inbox` runs it as it opens. A mission it cannot
// check is named, never fatal to the inbox.
func (e *Env) checkHere(ctx context.Context) []string {
	repo, c, w, err := e.missionLedger(ctx)
	if err != nil {
		return nil
	}
	t, err := w.Read(ctx)
	if err != nil {
		return nil
	}
	paths, _ := t.List(ctx, "missions/")
	t.Close()
	var notes []string
	for _, path := range paths {
		var f int
		if _, err := fmt.Sscanf(path, "missions/%d/plan.json", &f); err != nil || path != MissionPlanPath(f) {
			continue
		}
		plan, appr, _, err := readMission(ctx, w, f)
		if err != nil || plan == nil || plan.StartedBy == "" || plan.Pause != nil || appr == nil || appr.Parent == nil || !localHere(appr.Parent) {
			continue
		}
		note, err := e.checkLocalKanha(ctx, c, w, repo, f, plan, appr, true)
		if err != nil {
			note = fmt.Sprintf("the Kanha of #%d: not checked: %s", f, oneLineErr(err))
		}
		notes = append(notes, note)
	}
	return notes
}

// kanhaLine is where f's Kanha runs, for the plan view; for a local Kanha
// with an event older than one check, it says the mission is paused until
// that machine's next check.
func (e *Env) kanhaLine(ctx context.Context, w *ledger.Writer, f int, plan *MissionPlan, p *ready.Parent) (line string, paused string, err error) {
	if p == nil {
		return "", "", nil
	}
	if p.Mode != "local" {
		return fmt.Sprintf("#%d's Kanha runs on %s: %s", f, delegate.Names[p.Host], orDefault(p.URL, p.Session)), "", nil
	}
	line = fmt.Sprintf("#%d's Kanha is local: it pauses while %s is off", f, p.Machine)
	evs, err := kanhaEvents(ctx, w, f, plan, p)
	if err != nil || len(evs) == 0 {
		return line, "", err
	}
	at, perr := time.Parse(time.RFC3339, evs[0].At)
	switch {
	case perr != nil || e.Now().Sub(at) <= e.checkMinutes():
	case localHere(p): // this machine is on: no check has run here since the event
		paused = fmt.Sprintf("#%d waits: its Kanha runs on this machine, and an event waits since %s; run aayu mission wake %d, or open aayu inbox", f, evs[0].At, f)
	default:
		paused = fmt.Sprintf("#%d paused: its Kanha runs on %s, which is off; it resumes at the next check", f, p.Machine)
	}
	return line, paused, nil
}
