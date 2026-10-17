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

// BuilderOf decides the builder's family for a PR with commits. claims are
// every model the PR declares: the Aayudha-Model trailers of all its commits,
// its body's Aayudha-Model lines, and the pack's model, all self-reported. A
// session that changed model declares each one, so every claim is a
// candidate. host is the declared host (an Aayudha-Host trailer or body line,
// or the pack's host); lane is the build lane's family on the pack's host.
//
// The family is known only when a record covers the PR (a delegation, or a
// checkpoint naming a model family for every commit), the records name one
// family, and no claim contradicts it. Otherwise every family the records,
// the claims, or (with neither) the lane name is a candidate, and a reviewer
// is of a different family only when it is none of them.
//
// A claim aayu cannot place in a family (an unknown model prefix) makes the
// builder unknown. So does a declared host that picks the model itself with
// no model named, unless aayu knows the families that host's modes may use
// (managedFamilies). An unknown builder is never blocked: its review is a
// labelled same-family review (rung C), because no reviewer can be shown to
// differ from it (Decision #469).
func BuilderOf(rec Records, commits, claims []string, host string, lane verbs.Family) Builder {
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
	recorded := slices.Clone(fams)
	var claimed []verbs.Family
	var unplaced []string
	managed := false
	for _, c := range claims {
		switch f := verbs.FamilyOf(c); {
		case verbs.Family(c) == verbs.FamilyHostManaged:
			managed = true
		case f == "":
			unplaced = append(unplaced, c)
		case !slices.Contains(claimed, f):
			claimed = append(claimed, f)
		}
	}
	covered := rec.Receiver != "" && rec.Delegated != "" || len(commits) > 0 && named == len(commits)
	agrees := !managed && len(unplaced) == 0 && len(recorded) == 1 && !slices.ContainsFunc(claimed, func(f verbs.Family) bool { return f != recorded[0] })
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
	if len(unplaced) > 0 {
		why = append(why, fmt.Sprintf("the PR declares %s, which aayu cannot place in a model family", strings.Join(unplaced, ", ")))
		return Builder{Source: "unknown: " + strings.Join(why, "; ")}
	}
	for _, f := range claimed {
		add(f)
	}
	if len(claimed) > 0 {
		why = append(why, fmt.Sprintf("the Aayudha-Model declarations claim %v (self-reported, unverified)", claimed))
	}
	if len(recorded) == 0 && lane != "" && lane != verbs.FamilyHostManaged {
		add(lane)
		why = append(why, fmt.Sprintf("the build lane's family on the pack's host is %s", lane))
	}
	if len(fams) == 0 && (managed || host != "" || lane == verbs.FamilyHostManaged) {
		on := "the build host"
		if host != "" {
			on = host
		}
		if mf, ok := managedFamilies[host]; ok {
			why = append(why, fmt.Sprintf("%s picks the model for its modes, which may be any of %v", on, mf))
			return Builder{Family: verbs.FamilyHostManaged, Candidates: slices.Clone(mf), Source: "host-managed: " + strings.Join(why, "; ")}
		}
		why = append(why, fmt.Sprintf("%s declares no model", on))
	}
	return Builder{Candidates: fams, Source: "unknown: " + strings.Join(why, "; ")}
}

// managedFamilies are the families a host's own modes may run when a PR
// declares the host but no model. Amp routes its modes to Anthropic or
// OpenAI models, so a reviewer of a different family is of neither
// (Decision #469). A host missing here makes such a builder unknown.
var managedFamilies = map[string][]verbs.Family{
	string(verbs.HostAmp): {verbs.FamilyAnthropic, verbs.FamilyOpenAI},
}

// Unknown reports that no evidence places the builder in any family: no
// record, no placeable model claim, no lane, no host whose modes aayu knows.
// No reviewer can be shown to differ, so every review of it is rung C.
func (b Builder) Unknown() bool { return b.Family == "" && len(b.Candidates) == 0 }

// The review ladder's rungs (Decision #469). A counts: a reviewer of a family
// outside every family the builder may be. B is A through a pay-as-you-go
// key the user added. C is the default: the builder's own family, or any
// review of an unknown builder; it is labelled everywhere and never counts
// toward autonomy. Human is a person's review.
const (
	RungA     = "A"
	RungB     = "B"
	RungC     = "C"
	RungHuman = "human"
)

// Rung is the rung of a review by a model of family f: A when the builder's
// family is known well enough and f is none of its candidates, else C (f
// unknown or host-managed included).
func (b Builder) Rung(f verbs.Family) string {
	if f == "" || f == verbs.FamilyHostManaged || b.Unknown() || slices.Contains(b.Candidates, f) {
		return RungC
	}
	return RungA
}

// RungLabel is the line the PR check, the inspect comment, and the state
// carry for a review at rung by reviewer, of family f.
func (b Builder) RungLabel(rung, reviewer string, f verbs.Family) string {
	of := string(f)
	if of == "" || f == verbs.FamilyHostManaged {
		of = "family unknown"
	}
	builder := "builder family unknown"
	switch {
	case b.Family != "" && b.Family != verbs.FamilyHostManaged:
		builder = "builder " + string(b.Family)
	case len(b.Candidates) == 1:
		builder = fmt.Sprintf("builder %s (declared, not recorded)", b.Candidates[0])
	case len(b.Candidates) > 1:
		builder = fmt.Sprintf("builder any of %v", b.Candidates)
	}
	switch rung {
	case RungA, RungB:
		paid := ""
		if rung == RungB {
			paid = ", paid"
		}
		return fmt.Sprintf("different-family review (%s%s): %s (%s), %s", rung, paid, reviewer, of, builder)
	case RungC:
		return fmt.Sprintf("same-family review (C): %s (%s), %s; it never counts toward autonomy, and never-layers and protected paths still need a person", reviewer, of, builder)
	}
	return "human review: " + reviewer
}

// Note is the verdict's line about the builder.
func (b Builder) Note() string {
	if b.Family == verbs.FamilyHostManaged {
		return fmt.Sprintf("builder family host-managed (%s); a different-family reviewer is of none of %v", strings.TrimPrefix(b.Source, "host-managed: "), b.Candidates)
	}
	if b.Family != "" {
		return fmt.Sprintf("builder family %s (%s)", b.Family, b.Source)
	}
	if b.Unknown() {
		return fmt.Sprintf("builder family unknown (%s); every review is a labelled same-family review", strings.TrimPrefix(b.Source, "unknown: "))
	}
	return fmt.Sprintf("builder family unknown (%s); the reviewers are of none of %v", strings.TrimPrefix(b.Source, "unknown: "), b.Candidates)
}
