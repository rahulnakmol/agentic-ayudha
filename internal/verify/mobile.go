package verify

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// Mobile is the 0.1 part of the mobile floor (WP4.9): XCUITest and Espresso
// are deferred to 0.2, so a story on a mobile platform records
// "skipped: mobile", which blocks ship unless a floor-waiver Decision exists.
type Mobile struct{}

// Name implements Floor.
func (Mobile) Name() string { return "mobile" }

// MobilePlatforms are the platforms the mobile floor covers.
var MobilePlatforms = []string{"ios", "android"}

// Applies implements Floor.
func (Mobile) Applies(r *Run) (bool, string) {
	if !slices.ContainsFunc(r.Verify.Platforms, func(p string) bool { return slices.Contains(MobilePlatforms, strings.ToLower(p)) }) {
		return false, "no ios or android in [verify] platforms"
	}
	return true, ""
}

// Run implements Floor: there is nothing to run in 0.1.
func (Mobile) Run(_ context.Context, r *Run) (Result, error) {
	c := proof.Check{Result: proof.Skipped, Tool: "aayudha", Version: "0.1", Detail: proof.SkippedMobile}
	if !r.Floors.Mobile.SkippedBlocksShip {
		c.Detail = "skipped: mobile floor deferred to 0.2 (not blocking: [verify.floors.mobile] skipped_blocks_ship = false)"
	}
	return Result{Check: c}, nil
}

// Waiver is a floor-waiver Decision: a Decision under the story whose Links
// say "waives #<story>" and whose title or body names the floor.
type Waiver struct {
	Decision int
	Title    string
	Body     string
	Links    string
}

var word = regexp.MustCompile(`[a-z0-9-]+`)

// ApplyWaivers marks each check that does not hold as waived when a waiver
// names its floor, recording the Decision in the check (WP4.9, WP5.6).
func ApplyWaivers(checks []proof.Check, story int, ws []Waiver) []string {
	var applied []string
	for i := range checks {
		c := &checks[i]
		if c.Result != proof.Fail && c.Result != proof.Blocked && !proof.BlocksShip(*c) {
			continue
		}
		for _, w := range ws {
			links, _ := workitems.ParseLinks(w.Links)
			waives := slices.ContainsFunc(links, func(l workitems.Link) bool { return l.Kind == "waives" && l.Number == story })
			names := slices.Contains(word.FindAllString(strings.ToLower(w.Title+" "+w.Body), -1), c.Floor)
			if waives && names {
				c.Detail = fmt.Sprintf("%s (waived by #%d: %s)", c.Detail, w.Decision, w.Title)
				c.Result, c.Waiver = proof.Waived, fmt.Sprintf("#%d", w.Decision)
				applied = append(applied, c.Floor+" by #"+fmt.Sprint(w.Decision))
				break
			}
		}
	}
	return applied
}
