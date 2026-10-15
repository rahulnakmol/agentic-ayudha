// Package seats implements WP5.1: the generated parts of the twelve seat
// charters and of the AGENTS.md RACI section, and `aayu seat add|remove|list`,
// which pair a seat with a human (config, CODEOWNERS, onboarding issue) and
// switch the WP0.1 solo-mode policy when a second human arrives.
package seats

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/hosts"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// AgentsSectionID is the id of the RACI section in AGENTS.md.
const AgentsSectionID = "raci"

func code(s string) string { return "`" + s + "`" }

func codes(xs []string) string {
	out := make([]string, len(xs))
	for i, x := range xs {
		out[i] = code(x)
	}
	return strings.Join(out, ", ")
}

// AgentsSection is the seat RACI digest and the twins' voices rendered into
// AGENTS.md: by the generator on aayudha itself, and by `aayu seat` in a
// product repository.
func AgentsSection(personas map[verbs.SeatID]Persona) hosts.AgentsSection {
	var b strings.Builder
	b.WriteString(`## Seats and RACI (aayudha)

Twelve seats share the work. Each verb has exactly one Accountable seat; that seat's agent twin executes it in the
verb's model lane, and the seat that owns its thresholds or policy may differ (` + "`verify`" + ` runs against the Quality
Architect's floors). Each layer has exactly one owner seat. A seat runs solo (agent twin only) or paired with a human
(` + "`[seats.<slug>]`" + `, set by ` + "`aayu seat add <slug> --human <login>`" + `; ` + "`aayu seat list`" + ` shows them). Act as the Accountable
seat's twin, consult the Consulted seats before acting, and leave paired humans' review to CODEOWNERS.

| Seat | Twin | Layers owned | Accountable for |
|---|---|---|---|
`)
	for _, s := range verbs.Seats {
		var layers []string
		for _, l := range verbs.LayersOwned(s.ID) {
			layers = append(layers, string(l.ID))
		}
		var owns []string
		for _, c := range append(slices.Clone(verbs.Verbs), verbs.Plumbing...) {
			if c.Seat == s.ID {
				owns = append(owns, c.Name)
			}
		}
		l := "—"
		if len(layers) > 0 {
			l = strings.Join(layers, ", ")
		}
		twin := "—"
		if p, ok := personas[s.ID]; ok {
			twin = p.Name
		}
		fmt.Fprintf(&b, "| %s (%s) | %s | %s | %s |\n", s.Name, code(string(s.ID)), twin, l, codes(owns))
	}
	b.WriteString(agentsVoices(personas))
	return hosts.AgentsSection{ID: AgentsSectionID, Body: b.String()}
}

// CharterRegion is the generated RACI region of seats/<id>/CHARTER.md.
func CharterRegion(id verbs.SeatID) string {
	var b strings.Builder
	b.WriteString("## RACI (generated)\n\n")
	layers := verbs.LayersOwned(id)
	if len(layers) == 0 {
		b.WriteString("Layers owned: none.\n")
	} else {
		b.WriteString("| Layer owned | Name | Paths |\n|---|---|---|\n")
		for _, l := range layers {
			fmt.Fprintf(&b, "| %s | %s | %s |\n", l.ID, l.Name, codes(l.Paths))
		}
	}
	b.WriteString("\n")
	var rows []string
	for _, c := range verbs.Verbs {
		var roles []string
		if c.Seat == id {
			roles = append(roles, fmt.Sprintf("Accountable; the twin executes it (%s lane)", c.Lane))
		}
		if verbs.PolicyOwner(c) == id {
			roles = append(roles, "owns thresholds and policy")
		}
		r := verbs.RACI[c.Name]
		if slices.Contains(r.Consulted, id) {
			roles = append(roles, "Consulted")
		}
		if slices.Contains(r.Informed, id) {
			roles = append(roles, "Informed")
		}
		if len(roles) > 0 {
			rows = append(rows, fmt.Sprintf("| %s | %s |\n", code(c.Name), strings.Join(roles, "; ")))
		}
	}
	if len(rows) == 0 {
		b.WriteString("Verbs: none.\n")
	} else {
		b.WriteString("| Verb | Role |\n|---|---|\n" + strings.Join(rows, ""))
	}
	var plumbing []string
	for _, c := range verbs.Plumbing {
		if c.Seat == id {
			plumbing = append(plumbing, c.Name)
		}
	}
	if len(plumbing) > 0 {
		b.WriteString("\nPlumbing commands accountable for: " + codes(plumbing) + ".\n")
	}
	return b.String()
}

// TwinOwner is the Technical Owner text for a seat's agent twin.
func TwinOwner(id verbs.SeatID) string { return "the " + verbs.SeatName(id) + " twin" }

var loginRE = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9]|-[A-Za-z0-9]){0,38}$`)

// ValidLogin reports whether s is a syntactically valid GitHub user login.
func ValidLogin(s string) bool { return len(s) <= 39 && loginRE.MatchString(s) }

// ---- CODEOWNERS ------------------------------------------------------------

// Markers of the CODEOWNERS block `aayu seat` owns. Lines outside it are never
// changed. The block sits at the end, so its lines win (last match wins).
const (
	BlockBegin = "# BEGIN aayudha:seats (written by `aayu seat`; edit seats with `aayu seat add|remove`, not by hand)"
	BlockEnd   = "# END aayudha:seats"
)

// Pattern turns a layer glob into a root-anchored CODEOWNERS pattern: "dir/**"
// becomes "/dir/" and a literal path stays literal, so the protected-paths gate
// (which reads only root-anchored literal patterns) can evaluate it.
func Pattern(glob string) string {
	if d, ok := strings.CutSuffix(glob, "/**"); ok && !strings.ContainsAny(d, "*?[") {
		return "/" + d + "/"
	}
	return "/" + strings.TrimPrefix(glob, "/")
}

// splitBlock returns CODEOWNERS text without the aayudha block.
func splitBlock(text string) string {
	lines := strings.Split(text, "\n")
	var out []string
	in := false
	for _, l := range lines {
		switch {
		case strings.HasPrefix(l, "# BEGIN aayudha:seats"):
			in = true
		case in && strings.TrimSpace(l) == BlockEnd:
			in = false
		case !in:
			out = append(out, l)
		}
	}
	return strings.TrimRight(strings.Join(out, "\n"), "\n")
}

// person reports whether a CODEOWNERS owner is a user login (not a team or an
// email address) and not a bot login.
func person(owner string, bots []string) bool {
	return owner != "" && !strings.ContainsAny(owner, "/@") &&
		!slices.ContainsFunc(bots, func(b string) bool { return strings.EqualFold(b, owner) })
}

func addOwner(owners []string, o string) []string {
	if slices.ContainsFunc(owners, func(x string) bool { return strings.EqualFold(x, o) }) {
		return owners
	}
	return append(owners, o)
}

// Humans returns the distinct human logins a CODEOWNERS text names, in order of
// first appearance. The solo-mode policy counts them (WP0.1).
func Humans(codeowners string, bots []string) []string {
	var out []string
	for _, r := range identity.ParseCodeowners(codeowners) {
		for _, o := range r.Owners {
			if person(o, bots) {
				out = addOwner(out, o)
			}
		}
	}
	return out
}

// RenderCodeowners returns existing with the aayudha block rebuilt from the
// paired seats (seat → human login):
//
//   - each glob of a layer whose owner seat is paired gets a line owned by the
//     seat's human and the owners of the hand-written lines;
//   - with two or more humans (solo mode off), each hand-written line is
//     re-declared with the paired humans of the layers that cover it, and any
//     line that would still name fewer than two humans gets every human, because
//     native CODEOWNER review never lets an author approve their own PR: a path
//     with one human owner could not be changed by that human.
//
// Hand-written lines are never edited. Rendering is deterministic, and the same
// seats render the same block, so `seat remove` reverses `seat add` exactly.
func RenderCodeowners(existing string, paired map[verbs.SeatID]string, bots []string) string {
	hand := splitBlock(existing)
	rules := identity.ParseCodeowners(hand)
	var handOwners []string
	for _, r := range rules {
		for _, o := range r.Owners {
			handOwners = addOwner(handOwners, o)
		}
	}
	humans := Humans(hand, bots)
	for _, s := range verbs.Seats {
		if h := paired[s.ID]; person(h, bots) {
			humans = addOwner(humans, h)
		}
	}
	multi := len(humans) >= 2
	atLeastTwo := func(owners []string) []string {
		n := 0
		for _, o := range owners {
			if person(o, bots) {
				n++
			}
		}
		if multi && n < 2 {
			for _, h := range humans {
				owners = addOwner(owners, h)
			}
		}
		return owners
	}
	var lines []string
	line := func(pattern string, owners []string) {
		lines = append(lines, pattern+"  @"+strings.Join(owners, " @"))
	}
	for _, l := range verbs.LayerTable {
		h := paired[l.Owner]
		if h == "" {
			continue
		}
		for _, g := range l.Paths {
			owners := []string{h}
			for _, o := range handOwners {
				owners = addOwner(owners, o)
			}
			line(Pattern(g), atLeastTwo(owners))
		}
	}
	if multi {
		for _, r := range rules {
			owners := slices.Clone(r.Owners)
			for _, l := range verbs.LayerTable {
				if h := paired[l.Owner]; h != "" && slices.ContainsFunc(l.Paths, func(g string) bool { return covers(Pattern(g), r.Pattern) }) {
					owners = addOwner(owners, h)
				}
			}
			if owners = atLeastTwo(owners); len(owners) != len(r.Owners) {
				line(r.Pattern, owners)
			}
		}
	}
	if len(lines) == 0 {
		if hand == "" {
			return ""
		}
		return hand + "\n"
	}
	head := hand
	if head != "" {
		head += "\n\n"
	}
	return head + BlockBegin + "\n" + strings.Join(lines, "\n") + "\n" + BlockEnd + "\n"
}

// covers reports whether layer pattern lp covers CODEOWNERS pattern p.
func covers(lp, p string) bool {
	return lp == p || (strings.HasSuffix(lp, "/") && strings.HasPrefix(p, lp))
}

// ---- .aayudha/config.toml --------------------------------------------------

// SetSeat returns the config text with [seats.<seat>] set to paired with human,
// or, when human is empty, returned to solo: the table is removed when it holds
// nothing else, otherwise mode becomes "solo" and human is dropped. Other keys
// and comments in the table are kept.
func SetSeat(text string, seat verbs.SeatID, human string) string {
	header := "[seats." + string(seat) + "]"
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if text == "" {
		lines = nil
	}
	start := slices.IndexFunc(lines, func(l string) bool { return strings.TrimSpace(l) == header })
	if start < 0 {
		if human == "" {
			return text
		}
		if len(lines) == 0 {
			lines = []string{"version = 1"}
		}
		return strings.Join(lines, "\n") + "\n\n" + header + "\nmode = \"paired\"\nhuman = \"" + human + "\"\n"
	}
	end := start + 1
	for end < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[end]), "[") {
		end++
	}
	var keep []string
	for _, l := range lines[start+1 : end] {
		k, _, _ := strings.Cut(strings.TrimSpace(l), "=")
		if k = strings.TrimSpace(k); k == "mode" || k == "human" {
			continue
		}
		keep = append(keep, l)
	}
	for len(keep) > 0 && strings.TrimSpace(keep[len(keep)-1]) == "" {
		keep = keep[:len(keep)-1]
	}
	var table []string
	switch {
	case human != "":
		table = append([]string{header, `mode = "paired"`, `human = "` + human + `"`}, keep...)
	case len(keep) > 0:
		table = append([]string{header, `mode = "solo"`}, keep...)
	}
	before := lines[:start]
	after := lines[end:]
	if table == nil {
		for len(before) > 0 && strings.TrimSpace(before[len(before)-1]) == "" {
			before = before[:len(before)-1]
		}
	} else if len(after) > 0 {
		table = append(table, "")
	}
	out := append(append(slices.Clone(before), table...), after...)
	return strings.TrimRight(strings.Join(out, "\n"), "\n") + "\n"
}

// ---- The solo-mode ruleset (WP0.1, Appendix B D2) --------------------------

// The ruleset that carries the solo-mode policy, its gate check, and the
// GitHub Actions app that posts the check.
const (
	GateRuleset    = "protected-paths-codeowner-review"
	GateCheck      = "protected-paths"
	actionsAppID   = 15368
	adminRoleActor = 5 // the repository admin role as a ruleset bypass actor
)

func rules(rs map[string]any) []map[string]any {
	raw, _ := rs["rules"].([]any)
	var out []map[string]any
	for _, r := range raw {
		if m, ok := r.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func params(r map[string]any) map[string]any {
	p, _ := r["parameters"].(map[string]any)
	if p == nil {
		p = map[string]any{}
		r["parameters"] = p
	}
	return p
}

// TwoHuman reports whether a solo-mode ruleset has been switched to
// second-human review: at least one approval is required.
func TwoHuman(rs map[string]any) bool {
	for _, r := range rules(rs) {
		if r["type"] == "pull_request" {
			n, _ := params(r)["required_approving_review_count"].(float64)
			return n >= 1
		}
	}
	return false
}

// SwitchRuleset returns the PUT body that moves the solo-mode ruleset to
// second-human review (on) or back to solo mode (off). On: one approval with
// native CODEOWNER review, no protected-paths gate check, no bypass actors.
// Off: no approval, CODEOWNER review kept on, the gate check required, and the
// repository admin role as the only bypass, in pull-request mode (D2).
func SwitchRuleset(rs map[string]any, on bool) map[string]any {
	var out []any
	sawPR, sawChecks := false, false
	for _, r := range rules(rs) {
		switch r["type"] {
		case "pull_request":
			sawPR = true
			p := params(r)
			p["required_approving_review_count"] = map[bool]int{true: 1, false: 0}[on]
			p["require_code_owner_review"] = true
		case "required_status_checks":
			sawChecks = true
			p := params(r)
			raw, _ := p["required_status_checks"].([]any)
			var checks []any
			for _, c := range raw {
				if m, _ := c.(map[string]any); m["context"] != GateCheck {
					checks = append(checks, c)
				}
			}
			if !on {
				checks = append(checks, map[string]any{"context": GateCheck, "integration_id": actionsAppID})
			}
			if len(checks) == 0 {
				continue
			}
			p["required_status_checks"] = checks
		}
		out = append(out, r)
	}
	if !sawPR {
		out = append(out, map[string]any{"type": "pull_request", "parameters": map[string]any{
			"required_approving_review_count": map[bool]int{true: 1, false: 0}[on], "require_code_owner_review": true,
			"dismiss_stale_reviews_on_push": true, "require_last_push_approval": false, "required_review_thread_resolution": false,
		}})
	}
	if !sawChecks && !on {
		out = append(out, map[string]any{"type": "required_status_checks", "parameters": map[string]any{
			"strict_required_status_checks_policy": false, "do_not_enforce_on_create": false,
			"required_status_checks": []any{map[string]any{"context": GateCheck, "integration_id": actionsAppID}},
		}})
	}
	bypass := []any{}
	if !on {
		bypass = []any{map[string]any{"actor_id": adminRoleActor, "actor_type": "RepositoryRole", "bypass_mode": "pull_request"}}
	}
	body := map[string]any{"rules": out, "bypass_actors": bypass}
	for _, k := range []string{"name", "target", "enforcement", "conditions"} {
		if v, ok := rs[k]; ok {
			body[k] = v
		}
	}
	return body
}
