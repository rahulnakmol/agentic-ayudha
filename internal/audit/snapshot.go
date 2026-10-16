package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/autonomy"
)

// SnapshotVersion is the schema version of a GitHub inputs snapshot.
const SnapshotVersion = 1

// Snapshot is everything a report read from GitHub, frozen at the cursor and
// written to the ledger beside the report. GitHub's history is mutable (a
// comment can be edited) and partly short-lived (rule suites are listed for a
// month), so `--replay` regenerates a report from its snapshot and the ledger
// commit alone, byte for byte; a live re-run with the same inputs reads the
// same snapshot when GitHub has not changed.
type Snapshot struct {
	Version     int                    `json:"version"`
	Inputs      Inputs                 `json:"inputs"` // without the snapshot reference itself
	GateSince   time.Time              `json:"merge_gate_since,omitzero"`
	RuleSuites  []RuleSuite            `json:"rule_suites"`
	PRs         []PR                   `json:"prs"`
	Items       []Item                 `json:"items"`
	Runs        []Run                  `json:"runs"`
	Subjects    map[string]string      `json:"subjects"`
	OnMain      map[string]bool        `json:"on_main"`
	Licence     Licence                `json:"licence"`
	Corrections []autonomy.Correction  `json:"corrections"`
	NeverMerges []NeverMerge           `json:"never_merges"`
	AgentPRs    []AgentPR              `json:"agent_prs"`
	Streaks     []autonomy.LayerStreak `json:"streaks"`
	GateRuns    []Run                  `json:"gate_runs,omitempty"` // retro only
	Bugs        []Bug                  `json:"bugs,omitempty"`      // retro only
}

// MaxBody bounds the comment text Fetch keeps, and so the snapshot: the audit
// reads only how a comment starts ("Solo-mode bypass:").
const MaxBody = 160

// Freeze returns the snapshot of what Fetch read from GitHub, as JSON, and
// records its path and SHA-256 in in.
func Freeze(in *Inputs, d *Data, path string) []byte {
	stored := *in
	stored.Snapshot, stored.SnapshotSHA256 = "", ""
	s := Snapshot{Version: SnapshotVersion, Inputs: stored, GateSince: d.GateSince, RuleSuites: d.RuleSuites, PRs: d.PRs, Runs: d.Runs,
		Subjects: d.Subjects, OnMain: d.OnMain, Licence: d.Licence, Corrections: d.Corrections, NeverMerges: d.NeverMerges, AgentPRs: d.AgentPRs, Streaks: d.Streaks,
		GateRuns: d.GateRuns, Bugs: d.Bugs}
	s.Items = d.Items
	b, _ := json.MarshalIndent(s, "", " ")
	b = append(b, '\n')
	sum := sha256.Sum256(b)
	in.Snapshot, in.SnapshotSHA256 = path, hex.EncodeToString(sum[:])
	return b
}

// Thaw reads a snapshot into the inputs and the GitHub part of d; the ledger
// part comes from reading the ledger at the recorded commit.
func Thaw(b []byte, path string, d *Data) (Inputs, error) {
	var s Snapshot
	if err := json.Unmarshal(b, &s); err != nil {
		return Inputs{}, fmt.Errorf("%s: %w", path, err)
	}
	if s.Version != SnapshotVersion {
		return Inputs{}, fmt.Errorf("%s: snapshot version %d, this binary reads %d", path, s.Version, SnapshotVersion)
	}
	d.GateSince, d.RuleSuites, d.PRs, d.Items, d.Runs = s.GateSince, s.RuleSuites, s.PRs, s.Items, s.Runs
	d.Subjects, d.OnMain, d.Licence = s.Subjects, s.OnMain, s.Licence
	d.Corrections, d.NeverMerges, d.AgentPRs, d.Streaks = s.Corrections, s.NeverMerges, s.AgentPRs, s.Streaks
	d.GateRuns, d.Bugs = s.GateRuns, s.Bugs
	in := s.Inputs
	sum := sha256.Sum256(b)
	in.Snapshot, in.SnapshotSHA256 = path, hex.EncodeToString(sum[:])
	return in, nil
}
