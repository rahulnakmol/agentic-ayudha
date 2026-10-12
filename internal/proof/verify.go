package proof

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
)

// Expect is what a pack must agree with: the PR's current head commit and the
// story's Ready snapshot. Empty fields are not checked.
type Expect struct {
	HeadSHA            string
	CompletionGoalHash string
	Criteria           []int // criterion numbers in the Ready snapshot's Completion Goal
}

// Opener returns an artifact's bytes. It is not called for artifacts whose
// bytes were never pushed (StoreNone) or were purged (tombstoned).
type Opener func(a Artifact) ([]byte, error)

// Report is the result of `proof verify`.
type Report struct {
	Story      int      `json:"story"`
	HeadSHA    string   `json:"pr_head_sha"`
	Authority  string   `json:"authority"`
	Problems   []string `json:"problems"`
	Hashed     int      `json:"artifacts_hashed"`
	NotStored  int      `json:"artifacts_not_stored"`
	FloorsHold bool     `json:"floors_hold"`
	OK         bool     `json:"ok"`
}

// Verify checks a pack's integrity: every stored artifact's bytes match its
// SHA-256 and size, every Ready criterion maps to at least one artifact in the
// pack, every evidence reference names an artifact, the pack proves the PR's
// current head commit, and it was built against the Ready Completion Goal.
// Floor results are reported (FloorsHold) but are not integrity problems.
func Verify(p *Pack, exp Expect, open Opener) Report {
	r := Report{Story: p.Story, HeadSHA: p.PRHeadSHA, Authority: p.Authority.Kind, Problems: []string{}}
	bad := func(format string, args ...any) { r.Problems = append(r.Problems, fmt.Sprintf(format, args...)) }

	if exp.HeadSHA != "" && p.PRHeadSHA != exp.HeadSHA {
		bad("pack proves %s, but the PR head is %s (sha ≠ head)", short(p.PRHeadSHA), short(exp.HeadSHA))
	}
	if exp.CompletionGoalHash != "" && p.CompletionGoalHash != exp.CompletionGoalHash {
		bad("pack was built against Completion Goal %s, but the Ready snapshot has %s", short(p.CompletionGoalHash), short(exp.CompletionGoalHash))
	}
	seen := map[string]bool{}
	for _, a := range p.Artifacts {
		if err := CleanPath(a.Path); err != nil {
			bad("%v", err)
			continue
		}
		if seen[a.Path] {
			bad("artifact %s is listed twice", a.Path)
		}
		seen[a.Path] = true
		if a.Store == StoreNone || a.Tombstone != nil {
			r.NotStored++
			continue
		}
		b, err := open(a)
		if err != nil {
			bad("artifact %s (%s): %v", a.Path, a.Store, err)
			continue
		}
		sum := sha256.Sum256(b)
		if got := hex.EncodeToString(sum[:]); got != a.SHA256 || int64(len(b)) != a.Size {
			bad("artifact %s: hash mismatch (pack says %s, %d bytes; stored bytes are %s, %d bytes)", a.Path, short(a.SHA256), a.Size, short(got), len(b))
			continue
		}
		r.Hashed++
	}
	mapped := map[int]bool{}
	for _, c := range p.Criteria {
		for _, ref := range c.Evidence {
			if !seen[ref] {
				bad("criterion %d: evidence %s is not an artifact of this pack", c.ID, ref)
				continue
			}
			mapped[c.ID] = true
		}
		if exp.Criteria != nil && !slices.Contains(exp.Criteria, c.ID) {
			bad("criterion %d is not in the Ready snapshot's Completion Goal", c.ID)
		}
	}
	for _, n := range exp.Criteria {
		if !mapped[n] {
			bad("criterion %d has no evidence in the pack (missing criterion mapping)", n)
		}
	}
	r.FloorsHold = p.FloorsHold()
	r.OK = len(r.Problems) == 0
	return r
}

func short(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	if s == "" {
		return "(none)"
	}
	return s
}
