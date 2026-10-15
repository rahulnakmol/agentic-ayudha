package inspect

import (
	"fmt"
	"slices"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// Builder is what inspect knows of the builder's model family and where it
// came from (Bug #171). The reviewer is of none of Candidates.
type Builder struct {
	Family     verbs.Family   `json:"family,omitempty"`     // empty when unknown
	Candidates []verbs.Family `json:"candidates,omitempty"` // every family the evidence names
	Source     string         `json:"source"`
}

// Records are what aayu wrote on the ledger about who built a PR, outside the
// PR itself: the delegation record (internal/delegate, WP4.8b/WP5.5b), whose
// receiver built the whole PR, and the model build recorded for each
// checkpoint commit (build/<story>/state.json).
type Records struct {
	Receiver    string            // delegation receiver, "" without a delegation
	Delegated   verbs.Family      // the receiver's family
	Checkpoints map[string]string // checkpoint commit -> model build recorded
}

// BuilderOf decides the builder's family for a PR with commits. claim is the
// pack's model, read from the Aayudha-Model trailer the builder writes into
// its own commits, so self-reported; lane is the build lane's family on the
// pack's host. The family is known only when a record covers the PR (a
// delegation, or a checkpoint naming a model family for every commit), the
// records name one family, and the claim does not contradict it. Otherwise it
// is unknown, and every family the records, the claim, or (with no record) the
// lane name is a candidate.
func BuilderOf(rec Records, commits []string, claim string, lane verbs.Family) Builder {
	var fams []verbs.Family
	add := func(f verbs.Family) {
		if f != "" && f != verbs.FamilyHostManaged && !slices.Contains(fams, f) {
			fams = append(fams, f)
		}
	}
	named := 0 // commits whose checkpoint record names a model family
	for _, c := range commits {
		if f := verbs.FamilyOf(rec.Checkpoints[c]); f != "" {
			named++
			add(f)
		}
	}
	add(rec.Delegated)
	recorded, claimed := slices.Clone(fams), verbs.FamilyOf(claim)
	covered := rec.Receiver != "" && rec.Delegated != "" || len(commits) > 0 && named == len(commits)
	agrees := claim == "" || len(recorded) == 1 && claimed == recorded[0]
	if covered && len(recorded) == 1 && agrees {
		src := fmt.Sprintf("the build record on the ledger names it for all %d commit(s); aayu build took it from AAYU_MODEL or the host's build lane, which the builder's environment sets, so it is attributed, not verified", len(commits))
		if rec.Receiver != "" {
			src = fmt.Sprintf("the delegation record on the ledger: aayu delegate handed the story to %s", rec.Receiver)
		}
		return Builder{Family: recorded[0], Candidates: recorded, Source: src}
	}
	var why []string
	switch {
	case len(recorded) == 0:
		why = append(why, "no build or delegation record on the ledger names a model family for the PR's commits")
	case !covered:
		why = append(why, fmt.Sprintf("the build record names a model family for %d of %d commits", named, len(commits)))
	case len(recorded) > 1:
		why = append(why, fmt.Sprintf("the records name %d families", len(recorded)))
	default:
		why = append(why, fmt.Sprintf("the records say %s", recorded[0]))
	}
	add(claimed)
	if claim != "" {
		why = append(why, fmt.Sprintf("the Aayudha-Model trailer claims %s (self-reported, unverified)", claim))
	}
	if len(recorded) == 0 && lane != "" && lane != verbs.FamilyHostManaged {
		add(lane)
		why = append(why, fmt.Sprintf("the build lane's family on the pack's host is %s", lane))
	}
	return Builder{Candidates: fams, Source: "unknown: " + strings.Join(why, "; ")}
}

// Note is the verdict's line about the builder.
func (b Builder) Note() string {
	if b.Family != "" {
		return fmt.Sprintf("builder family %s (%s)", b.Family, b.Source)
	}
	return fmt.Sprintf("builder family unknown (%s); the reviewers are of none of %v", strings.TrimPrefix(b.Source, "unknown: "), b.Candidates)
}
