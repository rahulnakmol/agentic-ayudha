package loops

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// RecordFile is where a repository records which agent tool runs each of its
// loops (`aayu install --loops`); doctor compares it with the loops, and the
// Actions fallback leaves a recorded loop to its tool (Appendix B S-7.1d,
// S-7.1e).
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
	NoHost  = "n/a"
)

// State is one loop and the tool that owns it.
type State struct {
	Loop   string `json:"loop"`
	Owner  string `json:"owner,omitempty"` // the tools it is recorded in, comma-separated
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// States reports each loop with its owner tool: active (set up as it is
// now), stale (set up, but the loop changed since), missing (no tool; the
// Actions fallback runs it when it can), double (recorded in two tools, so it
// runs twice), or n/a (the loop runs in no agent tool).
func States(ls []Loop, rec Record, repo, branch, tz string) []State {
	var out []State
	for _, l := range ls {
		s := State{Loop: l.Name}
		owners := rec.Owners(l.Name)
		s.Owner = strings.Join(owners, ",")
		agent := slices.ContainsFunc(AgentHosts, l.Runs)
		switch {
		case len(owners) == 0 && !agent:
			s.Status, s.Detail = NoHost, "its hosts name no agent tool"
		case len(owners) == 0:
			s.Status = Missing
			if l.Runs(HostActions) {
				s.Detail = "only the Actions fallback runs it"
			}
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
