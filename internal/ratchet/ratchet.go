// Package ratchet keeps this repository's coverage floor from loosening
// (private research note video-learnings-2026-10.md P7; Appendix B S-4.3r and D-4.3r).
// The floor lives in ratchets.toml at the repository root. CI measures the
// statement coverage of `go test ./...` on every PR and push: coverage below
// the floor (less its noise margin) fails; coverage that holds well above it
// fails until the floor is tightened with -update, which never loosens it;
// and a floor lower than the base's fails unless the file names a new floor
// waiver that a human accepted (`aayu decide --waiver coverage`, WP5.6). It is
// for this repository only in 0.2: no product repository runs it, and it uses
// no model.
package ratchet

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"

	gh "github.com/google/go-github/v90/github"
	"github.com/pelletier/go-toml/v2"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/decide"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// File is the ratchet file at the repository root.
const File = "ratchets.toml"

// Ratchet is one ratcheted floor.
type Ratchet struct {
	Floor  float64 `toml:"floor"`            // the floor, in percent
	Margin float64 `toml:"margin"`           // the measurement noise the floor tolerates, in points
	Waiver int     `toml:"waiver,omitempty"` // the accepted floor-waiver Decision that allowed the last loosening
}

// Ratchets is ratchets.toml.
type Ratchets struct {
	Coverage Ratchet `toml:"coverage"`
}

// Parse reads ratchets.toml.
func Parse(b []byte) (Ratchets, error) {
	var r Ratchets
	if err := toml.Unmarshal(b, &r); err != nil {
		return r, fmt.Errorf("%s: %w", File, err)
	}
	if r.Coverage.Floor <= 0 || r.Coverage.Floor > 100 || r.Coverage.Margin <= 0 {
		return r, fmt.Errorf("%s: [coverage] needs a floor in (0, 100] and a positive margin", File)
	}
	return r, nil
}

// Check compares the measured coverage with the floor and, on a PR (base is
// not nil), the floor with the base's and whether the floor is stale. A push
// to dev only checks the floor: two PRs that each raised coverage a little
// leave the tightening to the next PR rather than failing dev. It returns the problems and the waiver
// Decision that must be verified as accepted (0 when none is needed).
func Check(cur Ratchet, base *Ratchet, measured float64) (problems []string, waiver int) {
	if measured < cur.Floor-cur.Margin {
		problems = append(problems, fmt.Sprintf("coverage %.1f %% is below its ratchet floor %.1f %% (margin %.1f): add the tests this change lost", measured, cur.Floor, cur.Margin))
	}
	if base != nil && cur.Floor < base.Floor {
		if cur.Waiver == 0 || cur.Waiver == base.Waiver {
			problems = append(problems, fmt.Sprintf("the coverage floor loosens from %.1f %% to %.1f %% without a new waiver: a floor only tightens; "+
				"loosening needs `aayu decide --waiver coverage <story-or-bug>`, accepted by a human, named as `waiver = <decision>` in %s", base.Floor, cur.Floor, File))
		} else {
			waiver = cur.Waiver
		}
	}
	if base != nil && measured >= cur.Floor+2*cur.Margin { // a PR tightens; a push to dev only guards
		problems = append(problems, fmt.Sprintf("coverage %.1f %% holds above its floor %.1f %%: tighten it to %.1f %% with "+
			"`go test -coverprofile=coverage.out ./... && go run ./internal/ratchet/cmd/ratchet -update -coverprofile coverage.out` and commit %s",
			measured, cur.Floor, Tighten(cur, measured), File))
	}
	return problems, waiver
}

// Tighten is the floor that measured supports: measured less the margin,
// rounded down to a tenth, and never below the current floor.
func Tighten(cur Ratchet, measured float64) float64 {
	return math.Max(cur.Floor, math.Floor((measured-cur.Margin)*10)/10)
}

var floorLine = regexp.MustCompile(`(?m)^floor = [0-9.]+`)

// SetFloor rewrites the coverage floor in the file's text, keeping its comments.
func SetFloor(b []byte, floor float64) []byte {
	return floorLine.ReplaceAll(b, []byte("floor = "+strconv.FormatFloat(floor, 'f', 1, 64)))
}

// Total is the statement coverage of a Go coverprofile, in percent, as
// `go tool cover -func` reports it: each block counted once, covered when any
// profile line for it has a count.
func Total(r io.Reader) (float64, error) {
	type block struct {
		stmts   int
		covered bool
	}
	blocks := map[string]*block{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "mode:") || line == "" {
			continue
		}
		f := strings.Fields(line)
		if len(f) != 3 {
			return 0, fmt.Errorf("coverprofile: malformed line %q", line)
		}
		stmts, err1 := strconv.Atoi(f[1])
		count, err2 := strconv.Atoi(f[2])
		if err1 != nil || err2 != nil {
			return 0, fmt.Errorf("coverprofile: malformed line %q", line)
		}
		b := blocks[f[0]]
		if b == nil {
			b = &block{stmts: stmts}
			blocks[f[0]] = b
		}
		b.covered = b.covered || count > 0
	}
	if err := sc.Err(); err != nil {
		return 0, err
	}
	var all, hit int
	for _, b := range blocks {
		all += b.stmts
		if b.covered {
			hit += b.stmts
		}
	}
	if all == 0 {
		return 0, fmt.Errorf("coverprofile: no statements")
	}
	return 100 * float64(hit) / float64(all), nil
}

// AcceptedWaiver checks that is (read with Issues.Get, so ClosedBy is set) is
// a floor-waiver Decision for floor that a human accepted: labelled
// decision:waiver, titled "Waive the <floor> floor on #…", closed as
// completed by someone who is not a bot, with "waive, accepted by @" in its
// Decision section, and accepted by the floor-waiver authority c resolves for
// a waiver of a parentType item (decide.WaiverAcceptedBy): the rule `verify`
// applies (WP5.6, Bug #599).
func AcceptedWaiver(is *gh.Issue, floor string, c *config.Config, parentType string) error {
	n := is.GetNumber()
	_, v := workitems.Sections(is.GetBody())
	labels := map[string]bool{}
	for _, l := range is.Labels {
		labels[l.GetName()] = true
	}
	by := is.GetClosedBy()
	switch {
	case !labels[workitems.LabelWaiver]:
		return fmt.Errorf("#%d is not a floor-waiver Decision (no %s label)", n, workitems.LabelWaiver)
	case !strings.HasPrefix(is.GetTitle(), "Waive the "+floor+" floor on #"):
		return fmt.Errorf("#%d waives another floor: %q", n, is.GetTitle())
	case is.GetState() != "closed" || is.GetStateReason() != "completed" || !strings.HasPrefix(strings.ToLower(strings.TrimSpace(v["Decision"])), "waive, accepted by @"):
		return fmt.Errorf("#%d is not accepted: a human accepts it with `aayu decide %d --accept waive`", n, n)
	case by == nil || identity.IsBot(identity.Account{Login: by.GetLogin(), Type: by.GetType()}, c.Identity.BotLogins):
		return fmt.Errorf("#%d was not closed by a person", n)
	}
	if err := decide.WaiverAcceptedBy(c, parentType, by.GetLogin(), v["Decision"]); err != nil {
		return fmt.Errorf("#%d is not accepted: %w", n, err)
	}
	return nil
}
