package loops

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// RecordFile is where a repository records which of its loops are set up on
// which agent host (`aayu install --loops --record`); doctor compares it with
// the loops (Appendix B S-7.1d).
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

// Digest identifies what a host holds for the loop, its prompt and its
// trigger, so a changed loop shows as stale until it is set up again.
func (l Loop) Digest(repo, branch, tz string) string {
	sum := sha256.Sum256([]byte(l.AgentPrompt(repo, branch) + "\n" + l.Cron + "\n" + l.Label(tz)))
	return hex.EncodeToString(sum[:6])
}

// Loop states on a host.
const (
	Active  = "active"
	Stale   = "stale"
	Missing = "missing"
	// Session: the host has no trigger for the loop's event, so the user's
	// next session there picks it up: aayu's SessionStart hook lists the
	// events since the last session (Appendix B D-7.1e).
	Session = "session"
	NoHost  = "n/a"
)

// sessionNote says how the user's next session on host picks up an event.
var sessionNote = map[string]string{
	HostAmp:    "the aayudha plugin shows each new event to you when your next Amp session starts (a notice, not agent context); ask the agent to run it",
	HostCursor: "the sessionStart hook gives each new event to the agent when your next Cursor session starts",
	HostClaude: "the SessionStart hook gives each new event to the agent when your next Claude Code session starts",
	HostCodex:  "the SessionStart hook gives each new event to the agent when your next Codex session starts",
}

// State is one loop on one agent host.
type State struct {
	Host   string `json:"host"`
	Loop   string `json:"loop"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// States reports each loop on each of hosts: active (set up as it is now),
// stale (set up, but the loop changed since), missing, session (the host has
// no trigger for its event, so the next session picks it up), or n/a (the
// loop does not run there).
func States(ls []Loop, rec Record, hosts []string, repo, branch, tz string) []State {
	var out []State
	for _, h := range hosts {
		for _, l := range ls {
			s := State{Host: h, Loop: l.Name}
			ok, why := l.Native(h)
			e, set := rec[h][l.Name]
			switch {
			case !l.Runs(h):
				s.Status, s.Detail = NoHost, "the loop's hosts leave it out"
			case !ok && sessionNote[h] != "":
				s.Status, s.Detail = Session, why+"; "+sessionNote[h]
			case !ok:
				s.Status, s.Detail = NoHost, why
			case !set:
				s.Status = Missing
			case e.Digest != l.Digest(repo, branch, tz):
				s.Status, s.Detail = Stale, fmt.Sprintf("set up %s (%s); the loop changed since", e.At, e.ID)
			default:
				s.Status, s.Detail = Active, fmt.Sprintf("set up %s (%s)", e.At, e.ID)
			}
			out = append(out, s)
		}
	}
	return out
}
