package cli

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
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

// cursorKey is this machine's Cursor API key: aayu's secret, else
// CURSOR_API_KEY, the two that agentenv.Detect takes as Cursor's cloud agent.
func (e *Env) cursorKey(cfg config.Delegate) string {
	key, _, _ := e.SecretStore().Get(orDefault(cfg.CursorSecret, "cursor-api-key"))
	return cmp.Or(key, e.Getenv("CURSOR_API_KEY"))
}

// cursorLocal reports whether a Cursor Kanha runs locally here: this
// machine has no Cursor API key, and the Cursor CLI is on its PATH.
func (e *Env) cursorLocal(cfg config.Delegate) bool {
	if e.cursorKey(cfg) != "" {
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
	bin, _ := delegate.LookLocal(tool, e.lookPath()) // Start's kanhaCanStart checked it is on the PATH
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

// localCheck is this machine's record of f's Kanha checks: the last check,
// and the events it gave the Kanha within kanhaSettle of its last check.
type localCheck struct {
	At   time.Time `json:"at"`
	Sent []string  `json:"sent,omitempty"`
}

// kanhaSettle is how far before the last check a check reads events again:
// an event is stamped before its ledger write lands, so one stamped earlier
// can arrive after the check. Sent keeps a check from giving one twice.
const kanhaSettle = 10 * time.Minute

func (e *Env) readLocalCheck(f int) localCheck {
	var c localCheck
	if b, err := os.ReadFile(e.kanhaCheckPath(f)); err == nil {
		_ = json.Unmarshal(b, &c)
	}
	return c
}

// writeLocalCheck writes f's check record, under the lock, which made its directory.
func (e *Env) writeLocalCheck(f int, c localCheck) error {
	b, _ := json.Marshal(c)
	return os.WriteFile(e.kanhaCheckPath(f), b, 0o600)
}

// lockLocalCheck holds f's check on this machine for one resume, so a wake
// or the inbox never resumes the Kanha's session while another check runs
// in it. held is when the running check began.
func (e *Env) lockLocalCheck(f int) (unlock func(), held time.Time, err error) {
	p := e.kanhaCheckPath(f) + ".lock"
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return nil, time.Time{}, err
	}
	fh, err := os.OpenFile(p, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, time.Time{}, err
	}
	if ok, err := tryLock(fh); !ok {
		_ = fh.Close()
		return nil, e.readLocalCheck(f).At, err // the running check wrote its start
	}
	return func() { _ = fh.Close() }, time.Time{}, nil
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
func (e *Env) checkLocalKanha(ctx context.Context, c *github.Client, w *ledger.Writer, repo string, f int, now bool) (string, error) {
	at := e.Now().UTC()
	chk := e.readLocalCheck(f)
	if last := chk.At; !now && !last.IsZero() && at.Before(last.Add(e.checkMinutes())) {
		return fmt.Sprintf("the Kanha of #%d: not due; its next check is at %s", f, last.Add(e.checkMinutes()).Format(time.RFC3339)), nil
	}
	unlock, held, err := e.lockLocalCheck(f)
	if err != nil {
		return "", Wrap(ExitExternal, err)
	}
	if unlock == nil {
		return fmt.Sprintf("the Kanha of #%d: a check here runs since %s; this one resumes nothing", f, held.UTC().Format(time.RFC3339)), nil
	}
	defer unlock()
	// Read under the lock: a check that held it may have moved last_check and Sent.
	plan, appr, _, err := readMission(ctx, w, f)
	if err != nil {
		return "", err
	}
	if plan == nil || appr == nil || appr.Parent == nil || !localHere(appr.Parent) {
		return fmt.Sprintf("the Kanha of #%d no longer runs on this machine", f), nil
	}
	p := appr.Parent
	chk = e.readLocalCheck(f)
	chk.At = at
	if err := e.writeLocalCheck(f, chk); err != nil {
		return "", Wrap(ExitExternal, err)
	}
	q := *p
	if last, perr := time.Parse(time.RFC3339, p.LastCheck); perr == nil {
		q.LastCheck = last.Add(-kanhaSettle).UTC().Format(time.RFC3339)
	}
	all, err := kanhaEvents(ctx, w, f, plan, &q)
	if err != nil {
		return "", err
	}
	evs := slices.DeleteFunc(slices.Clone(all), func(ev delegate.Event) bool { return slices.Contains(chk.Sent, sentKey(ev)) })
	if len(evs) == 0 {
		return fmt.Sprintf("the Kanha of #%d: no event since its last check", f), nil
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
	// The parent first: if its write fails, Sent stays, and the next check gives these events again and
	// writes the parent then. A repeated event is safe; a last_check left behind holds the Feature.
	q.LastCheck = max(p.LastCheck, evs[len(evs)-1].At)
	if err := e.writeParent(ctx, w, f, appr.Version, &q); err != nil {
		return "", Errorf(ExitExternal, "the Kanha of #%d took %d event(s), and this check is not recorded, so the next check gives them again: %s", f, len(evs), oneLineErr(err))
	}
	chk.Sent = nil // every event read is now given, and kept as sent until a later check no longer reads it
	for _, ev := range all {
		chk.Sent = append(chk.Sent, sentKey(ev))
	}
	_ = e.writeLocalCheck(f, chk) // lost, it gives these events once more at the next check
	return fmt.Sprintf("the Kanha of #%d: resumed %s session %s with %d event(s)", f, delegate.Names[p.Host], p.Session, len(evs)), nil
}

// sentKey names one Kanha event for this machine's record of what it gave the Kanha.
func sentKey(ev delegate.Event) string {
	sum := sha256.Sum256([]byte(ev.At + "\x00" + ev.Text))
	return hex.EncodeToString(sum[:8])
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
		note, err := e.checkLocalKanha(ctx, c, w, repo, f, true)
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
