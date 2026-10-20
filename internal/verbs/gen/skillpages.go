package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// guideSkillsDir holds one reader page per verb (docs/guide/skills/<verb>.md)
// and the index of them all (README.md). The regions between the BEGIN/END
// GENERATED markers come from the registry; the rest of each page is written
// by hand. A verb without a page gets one, with its generated regions and the
// headings to write under; a page for a verb that no longer exists is
// refused, because its prose is hand-authored.
const guideSkillsDir = "docs/guide/skills"

// skillPageRegions are the generated regions of a skill page, in page order.
var skillPageRegions = []string{"skill-card", "skill-usage", "skill-gates", "skill-human"}

func genSkillPages() error {
	want := map[string]bool{"README.md": true}
	for _, c := range verbs.Verbs {
		name := c.Name + ".md"
		want[name] = true
		path := filepath.Join(guideSkillsDir, name)
		b, err := os.ReadFile(path)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			b = []byte(skillPageStub(c))
		case err != nil:
			return err
		}
		s := string(b)
		for _, region := range skillPageRegions {
			if s, err = replaceRegion(path, s, region, skillRegion(c, region)); err != nil {
				return err
			}
		}
		if err := writeFile(path, []byte(s)); err != nil {
			return err
		}
	}
	if err := genSkillIndex(); err != nil {
		return err
	}
	entries, err := os.ReadDir(guideSkillsDir)
	if err != nil {
		return err
	}
	var stale []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") && !want[e.Name()] {
			stale = append(stale, filepath.ToSlash(filepath.Join(guideSkillsDir, e.Name())))
		}
	}
	if len(stale) > 0 {
		return fmt.Errorf("skill pages for verbs the registry does not have (move or delete them): %s", strings.Join(stale, ", "))
	}
	return nil
}

// skillPageStub is a new verb's page: its generated regions under the
// headings every skill page has, with the hand-written parts to fill in.
func skillPageStub(c verbs.Command) string {
	region := func(name string) string {
		return "<!-- BEGIN GENERATED: " + name + " -->\n<!-- END GENERATED: " + name + " -->\n"
	}
	return "# `" + c.Name + "`\n\n" + c.Reader + "\n\n" + region("skill-card") +
		"\n## When to use it\n\n" + c.Reader + "\n" +
		"\n## Run it\n\n" + region("skill-usage") +
		"\n## What you get back\n\n" + strings.Join(c.Outputs, ", ") + ".\n" +
		"\n## Gates\n\n" + region("skill-gates") +
		"\n## What only a person does\n\n" + region("skill-human") +
		"\n## What's next\n\nSee [every skill](README.md).\n"
}

// planRef matches a parenthesised build-plan reference in registry text, such
// as "(WP5.1b)" or "(Appendix B D2)": reader pages name no work packages or
// decision-log rows (docs/guide/AGENTS.md).
var planRef = regexp.MustCompile(` \((?:WP[0-9][^()]*|Appendix B [^()]*)\)`)

// readerText is registry text for a reader page: plan references dropped and
// angle brackets outside code spans escaped, so "<feature>" is not read as HTML.
func readerText(s string) string {
	parts := strings.Split(strings.TrimSpace(planRef.ReplaceAllString(s, "")), "`")
	for i := 0; i < len(parts); i += 2 {
		parts[i] = strings.ReplaceAll(parts[i], "<", `\<`)
	}
	return strings.Join(parts, "`")
}

func skillRegion(c verbs.Command, region string) string {
	switch region {
	case "skill-card":
		return skillCard(c)
	case "skill-usage":
		return skillUsage(c)
	case "skill-gates":
		return skillGates(c)
	default:
		return skillHuman(c)
	}
}

// seatWithTwin is "Product Strategist (Arjun)", linked to the seat's charter.
func seatWithTwin(id verbs.SeatID) string {
	return fmt.Sprintf("[%s](../../../seats/%s/CHARTER.md) (%s)", verbs.SeatName(id), id, personas[id].Name)
}

func seatList(ids []verbs.SeatID) string {
	if len(ids) == 0 {
		return "none"
	}
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = seatWithTwin(id)
	}
	return strings.Join(out, ", ")
}

func stageLink(id verbs.StageID) string {
	for _, st := range verbs.Stages {
		if st.ID == id {
			return fmt.Sprintf("[%s](../%s)", st.Title, stagePages[id])
		}
	}
	return string(id)
}

func layersText(l verbs.Layers) string {
	switch l.Scope {
	case verbs.ScopeAll:
		return "[every layer](../concepts-layers.md)"
	case verbs.ScopeSome:
		names := make([]string, len(l.IDs))
		for i, id := range l.IDs {
			ly, _ := verbs.FindLayer(id)
			names[i] = fmt.Sprintf("%s %s", id, ly.Name)
		}
		return "[" + strings.Join(names, ", ") + "](../concepts-layers.md)"
	default:
		return "none: it changes no product code"
	}
}

func modeText(c verbs.Command) string {
	var s string
	switch c.Mode {
	case verbs.ModeNone:
		s = "no model: it is deterministic"
	case verbs.ModeHost:
		s = "your agent's own model, inside the skill"
	case verbs.ModeDirect:
		s = "aayu calls a model itself"
	case verbs.ModeHybrid:
		s = "your agent's model, plus a direct pass by aayu"
	}
	if c.ModeNote != "" {
		s += " (" + c.ModeNote + ")"
	}
	return s
}

func identityText(id verbs.Identity) string {
	if id == verbs.IdentityHuman {
		return "a person's own GitHub identity"
	}
	return "the bot identity (a GitHub App); without one, your identity, tagged as human fallback"
}

func surfacesText(c verbs.Command) string {
	var out []string
	if c.Has(verbs.SurfaceCLI) {
		out = append(out, "the `aayu` command")
	}
	if c.Has(verbs.SurfaceSkill) {
		out = append(out, "a skill in your agent")
	}
	if c.Has(verbs.SurfaceMCP) {
		switch c.Exposure {
		case verbs.ExposeRead:
			out = append(out, "an MCP tool (read-only unless `[mcp] expose_writes` is on)")
		case verbs.ExposeWrite:
			out = append(out, "an MCP tool (only with `[mcp] expose_writes` on)")
		}
	}
	return strings.Join(out, ", ")
}

func skillCard(c verbs.Command) string {
	roles := verbs.RACI[c.Name]
	var b strings.Builder
	b.WriteString("| Field | Value |\n|---|---|\n")
	row := func(k, v string) { fmt.Fprintf(&b, "| %s | %s |\n", k, cell(v)) }
	row("In plain words", c.Reader)
	row("Journey stage", stageLink(c.Stage))
	row("Accountable seat", seatWithTwin(c.Seat))
	if c.Owns != "" {
		row("Owns the thresholds", seatWithTwin(c.Owns))
	}
	lane := fmt.Sprintf("[`%s`](../how-models.md)", c.Lane)
	row("Does the work", fmt.Sprintf("%s, the Accountable seat's twin, in the %s model lane, with the seat's paired human when it has one", personas[c.Seat].Name, lane))
	row("Consulted", seatList(roles.Consulted))
	row("Informed", seatList(roles.Informed))
	row("Layers it can touch", layersText(c.Layers))
	row("Who reasons", modeText(c))
	row("Writes as", identityText(c.Identity))
	row("Where it runs", surfacesText(c))
	if c.Has(verbs.SurfaceSkill) {
		row("The skill your agent reads", fmt.Sprintf("[skills/%s/SKILL.md](../../../skills/%s/SKILL.md)", c.Name, c.Name))
	}
	return b.String()
}

func skillUsage(c verbs.Command) string {
	parts := []string{"aayu", c.Name}
	for _, in := range c.Inputs {
		if !in.Positional {
			continue
		}
		if in.Required {
			parts = append(parts, "<"+in.Name+">")
		} else {
			parts = append(parts, "[<"+in.Name+">]")
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "```sh\n%s\n```\n", strings.Join(parts, " "))
	if len(c.Inputs) == 0 {
		return b.String()
	}
	b.WriteString("\n| Input | Required | What it does |\n|---|---|---|\n")
	for _, in := range c.Inputs {
		name := "--" + in.Name
		if in.Positional {
			name = "<" + in.Name + ">"
		}
		req := "no"
		if in.Required {
			req = "yes"
		}
		desc := readerText(in.Description)
		if len(in.Enum) > 0 {
			desc += " One of: `" + strings.Join(in.Enum, "`, `") + "`."
		}
		if in.HumanOnly {
			desc += " **Person only.**"
		}
		fmt.Fprintf(&b, "| `%s` | %s | %s |\n", name, req, cell(desc))
	}
	return b.String()
}

func skillGates(c verbs.Command) string {
	if len(c.Gates) == 0 {
		return "It checks no gate before it acts.\n"
	}
	var b strings.Builder
	b.WriteString("It checks these before it acts, and stops when one fails:\n\n")
	for _, id := range c.Gates {
		g, _ := verbs.FindGate(id)
		fmt.Fprintf(&b, "- **%s:** %s\n", id, readerText(g.Summary))
	}
	return b.String()
}

// humanGates are the gates that only a person can pass, with what that means for the reader.
var humanGates = map[verbs.GateID]string{
	verbs.GateHumanPass:     "A person approves the pull request. Working alone, you record a solo-mode bypass instead; it is never counted as an approval.",
	verbs.GateHumanIdentity: "It runs only under a person's identity, never the bot's.",
	verbs.GateTTY:           "It asks a person to confirm in a terminal, and refuses agents.",
	verbs.GateEnvironment:   "A person approves the GitHub Environment before anything runs in production.",
	verbs.GateCodeowner:     "A person who owns the changed paths (a CODEOWNER) reviews the change.",
	verbs.GateAuthority:     "Only a person who holds `[decisions] authority` for its type or layer accepts it.",
	verbs.GateScopeBudget:   "Once a person accepts a release's baseline, a change beyond the budget waits for a person: they approve it through GitHub's device code, against an accepted Decision.",
	verbs.GateStableRequest: "A stable release to main waits for a person's request: their comment on the Release issue, or their yes at a terminal.",
}

func skillHuman(c verbs.Command) string {
	var items []string
	if c.Identity == verbs.IdentityHuman {
		items = append(items, "It writes under a person's own GitHub identity.")
	}
	if c.Exposure == verbs.ExposeNever {
		items = append(items, "It is never offered to agents as an MCP tool.")
	}
	for _, id := range c.Gates {
		if s, ok := humanGates[id]; ok {
			items = append(items, s)
		}
	}
	for _, in := range c.Inputs {
		if in.HumanOnly {
			name := "--" + in.Name
			if in.Positional {
				name = "<" + in.Name + ">"
			}
			items = append(items, fmt.Sprintf("`%s`: %s", name, readerText(in.Description)))
		}
	}
	if len(items) == 0 {
		return "The registry reserves no step of this verb for a person.\n"
	}
	var b strings.Builder
	for _, it := range items {
		fmt.Fprintf(&b, "- %s\n", it)
	}
	return b.String()
}

// genSkillIndex writes the index region of docs/guide/skills/README.md: every
// verb by journey stage, with its Accountable seat and twin.
func genSkillIndex() error {
	path := filepath.Join(guideSkillsDir, "README.md")
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var t strings.Builder
	t.WriteString("| Stage | Skill | What it does | Accountable seat (twin) |\n|---|---|---|---|\n")
	for _, st := range verbs.Stages {
		first := true
		for _, c := range verbs.Verbs {
			if c.Stage != st.ID {
				continue
			}
			stage := ""
			if first {
				stage, first = fmt.Sprintf("[%s](../%s)", st.Title, stagePages[st.ID]), false
			}
			fmt.Fprintf(&t, "| %s | [`%s`](%s.md) | %s | %s |\n", stage, c.Name, c.Name, cell(c.Reader),
				fmt.Sprintf("%s (%s)", verbs.SeatName(c.Seat), personas[c.Seat].Name))
		}
	}
	s, err := replaceRegion(path, string(b), "skill-index", t.String())
	if err != nil {
		return err
	}
	return writeFile(path, []byte(s))
}
