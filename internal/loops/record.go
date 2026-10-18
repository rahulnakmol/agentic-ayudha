package loops

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RecordFile is where a repository records which agent tool runs each of its
// agent loops (`aayu install --loops`); doctor compares it with the loops
// (Appendix B S-7.1d, S-7.1e). A loop that needs no agent runs in the Actions
// fallback and is never recorded (D-7.1f).
const RecordFile = Dir + "/active.json"

// AgentHosts are the hosts that run a loop in the user's own agent session.
var AgentHosts = []string{HostAmp, HostCursor, HostClaude, HostCodex}

// Entry is one loop set up on one host: what was set up (Digest), the host's
// own ID for it (a thread, subscription, routine, or task), and the date.
type Entry struct {
	Digest string `json:"digest"`
	ID     string `json:"id"`
	At     string `json:"at"`
}

// Record maps host → loop → Entry.
type Record map[string]map[string]Entry

// ReadRecord reads root's RecordFile; a repository without one has none.
func ReadRecord(root string) (Record, error) {
	r := Record{}
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(RecordFile)))
	if errors.Is(err, os.ErrNotExist) {
		return r, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("%s: %w", RecordFile, err)
	}
	return r, nil
}

// Write writes the record to root's RecordFile.
func (r Record) Write(root string) error {
	b, err := marshal(r)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, filepath.FromSlash(RecordFile)), b, 0o644)
}

// Set records loop as set up on host.
func (r Record) Set(host, loop string, e Entry) {
	if r[host] == nil {
		r[host] = map[string]Entry{}
	}
	r[host][loop] = e
}

// Delete forgets loop on host.
func (r Record) Delete(host, loop string) {
	delete(r[host], loop)
	if len(r[host]) == 0 {
		delete(r, host)
	}
}

// Owners are the agent tools loop is recorded as set up in. One tool runs
// the loops (Appendix B S-7.1e); two would run the same loop twice.
func (r Record) Owners(loop string) []string {
	var out []string
	for _, h := range AgentHosts {
		if _, ok := r[h][loop]; ok {
			out = append(out, h)
		}
	}
	return out
}

// Digest identifies what a tool holds for the loop, its prompt and its
// schedule, so a changed loop shows as stale until it is set up again.
func (l Loop) Digest(repo, branch, tz string) string {
	sum := sha256.Sum256([]byte(l.AgentPrompt(repo, branch) + "\n" + l.Cron + "\n" + l.Label(tz)))
	return hex.EncodeToString(sum[:6])
}

// Loop states.
const (
	Active  = "active"
	Stale   = "stale"
	Missing = "missing"
	Double  = "double"
	// Fallback is an event loop whose event the repository's tool has no
	// trigger for: a scheduled loop covers it there.
	Fallback = "fallback"
)

// noTrigger is the agent tool that runs this repository's loops when it has
// no trigger for event loop l's event, or "".
func (r Record) noTrigger(l Loop) string {
	out := ""
	for _, h := range AgentHosts {
		switch {
		case len(r[h]) == 0:
		case l.Runs(h):
			return "" // a recorded tool can run it: missing until it is set up there
		case out == "":
			out = h
		}
	}
	return out
}

// State is one loop and the tool that owns it.
type State struct {
	Loop   string `json:"loop"`
	Owner  string `json:"owner,omitempty"` // the tools it is recorded in, comma-separated
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// States reports each loop with what runs it: active (set up as it is now in
// its agent tool, or a loop with agent = false, which the Actions fallback
// runs), stale (set up, but the loop changed since), missing (an agent loop
// no tool runs), fallback (an event loop the recorded tool has no trigger
// for, which a scheduled loop covers), or double (it runs twice: recorded in two tools, or a loop
// the Actions fallback runs recorded in a tool as well).
func States(ls []Loop, rec Record, repo, branch, tz string) []State {
	var out []State
	for _, l := range ls {
		s := State{Loop: l.Name}
		owners := rec.Owners(l.Name)
		s.Owner = strings.Join(owners, ",")
		switch {
		case !l.NeedsAgent() && len(owners) == 0:
			s.Owner, s.Status, s.Detail = HostActions, Active, "needs no agent, so the GitHub Actions fallback runs it"
		case !l.NeedsAgent():
			s.Owner = HostActions + "," + s.Owner
			s.Status, s.Detail = Double, "needs no agent and runs in the Actions fallback, but is recorded in "+strings.Join(owners, " and ")+" as well; delete it there, then forget it with `aayu install --loops --host "+owners[0]+" --record "+l.Name+"=`"
		case len(owners) == 0 && l.IsEvent() && rec.noTrigger(l) != "":
			h := rec.noTrigger(l)
			if c, ok := Covers(ls, l, h); ok {
				s.Owner, s.Status, s.Detail = h, Fallback, fmt.Sprintf("%s has no %s trigger; the scheduled loop %s covers it", h, l.On, c.Name)
			} else {
				s.Status, s.Detail = Missing, fmt.Sprintf("%s has no %s trigger, and no scheduled loop with the same command runs there", h, l.On)
			}
		case len(owners) == 0:
			s.Status = Missing
		case len(owners) > 1:
			s.Status, s.Detail = Double, "recorded in "+strings.Join(owners, " and ")+", so it runs twice; remove it from all but one"
		default:
			e := rec[owners[0]][l.Name]
			if e.Digest != l.Digest(repo, branch, tz) {
				s.Status, s.Detail = Stale, fmt.Sprintf("set up %s (%s); the loop changed since", e.At, e.ID)
			} else {
				s.Status, s.Detail = Active, fmt.Sprintf("set up %s (%s)", e.At, e.ID)
			}
		}
		out = append(out, s)
	}
	return out
}
