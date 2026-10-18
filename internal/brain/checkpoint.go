package brain

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Records are the repository's own records a checkpoint reads. Every field is
// a link or a one-line fact: the brain never copies an issue's body,
// criteria, status, or labels.
type Records struct {
	Repo      string
	Inception string // the date the repository was created
	// Issues are the work items, by number: type and parent only, and the
	// title, which a link line may show.
	Issues   map[int]Issue
	Tags     []Tag
	Rungs    []Rung     // ledger releases/<issue>/<rolling|main>.json
	Baseline []Baseline // ledger releases/<version>/baseline-<n>.json, newest per version
	Drift    []Drift    // ledger releases/<version>/drift.json
	Seats    []Seat
	Approver string // [autonomy] approver: the person whose review counts
}

// Issue is a work item as a link target.
type Issue struct {
	Number int
	Type   string // goal, initiative, feature, story, bug, decision, release, ...
	Title  string
	Parent int
	Closed string // the date it closed, or ""
}

// Tag is a published release tag.
type Tag struct {
	Name string
	Date string
	URL  string
}

// Rung is one promotion record on the ledger.
type Rung struct {
	Path    string
	Release int    // the Release issue
	Version string // vX.Y.Z when the promotion belongs to one
	Target  string // rolling or main
	Tag     string
	Date    string // the promotion date, YYYY-MM-DD
	Stories []int
}

// Baseline is the newest release-plan baseline of a version.
type Baseline struct {
	Path         string
	N            int
	Version      string
	ReleaseIssue int
	BaselinedAt  string
	BaselinedBy  string
	BudgetPct    float64
	Items        []PlanItem
}

// PlanItem is one planned issue.
type PlanItem struct {
	Number int
	Type   string
	Title  string
	Effort string
	Parent int
}

// Drift is the change of a release plan since its baseline.
type Drift struct {
	Path      string
	Version   string
	Baseline  int
	BudgetPct float64
	DriftPct  float64
	Decision  int // the Decision issue when the drift went over the budget
	Added     []PlanItem
	Removed   []PlanItem
	Resized   []Resize
}

// Resize is a planned item whose effort changed. FromType is its type at the
// baseline when the type changed too (story to initiative); Type is the current one.
type Resize struct {
	PlanItem
	From, To, FromType string
}

// Seat is one seat of the pod.
type Seat struct {
	ID      string
	Name    string
	Twin    string
	Purpose string
	Human   string // paired human's login, or ""
	Verbs   []string
}

// Checkpoint is the result of a checkpoint: the files to write and the notes
// whose generated facts changed after a person reviewed them.
type Checkpoint struct {
	Files  map[string][]byte `json:"-"`
	Paths  []string          `json:"changed"`
	Review []string          `json:"needs_review"`
}

var versionRE = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

// Run refreshes the vault from r on today's date. It creates the notes the
// records imply (one per release, seat, and person), refreshes every marked
// generated block, and leaves all other text as a person wrote it. It returns
// only the files that changed, so a second run with the same records writes
// nothing.
func Run(files map[string][]byte, r Records, today string, public bool) (Checkpoint, error) {
	out := maps.Clone(files)
	if _, ok := out[HomePath]; !ok {
		for p, b := range Skeleton(r.Repo, today, public) {
			if _, ok := out[p]; !ok { // a vault without Home keeps every file it has
				out[p] = b
			}
		}
	}
	out[MarkerPath] = []byte(Marker) // a vault from before the marker gets it
	c := &cp{files: out, r: r, today: today, v: Load(out)}
	c.releases()
	c.seats()
	c.v = Load(c.files)
	c.issueBlocks()
	c.timeline()
	c.v = Load(c.files)
	c.mocs()
	c.v = Load(c.files)
	c.home()
	res := Checkpoint{Files: map[string][]byte{}}
	for p, b := range c.files {
		if old, ok := files[p]; !ok || string(old) != string(b) {
			res.Files[p] = b
			res.Paths = append(res.Paths, p)
		}
	}
	slices.Sort(res.Paths)
	res.Review = c.reviewList()
	return res, c.err
}

type cp struct {
	files map[string][]byte
	r     Records
	today string
	v     *Vault
	err   error
}

// Index refreshes only the maps of content and Home from the vault's own
// notes, with no records: init runs it so a new workstream appears in the
// root graph at once. It returns the files that changed.
func Index(files map[string][]byte, today string) map[string][]byte {
	c := &cp{files: maps.Clone(files), today: today, v: Load(files)}
	c.mocs()
	c.v = Load(c.files)
	c.home()
	out := map[string][]byte{}
	for p, b := range c.files {
		if old, ok := files[p]; !ok || string(old) != string(b) {
			out[p] = b
		}
	}
	return out
}

// upsert creates the note at p (from create) when it is absent, then sets the
// generated blocks and the generated frontmatter keys. It bumps `updated`
// only when a generated fact changed.
func (c *cp) upsert(p string, create func() *Note, meta map[string]any, blocks map[string]string) {
	var n *Note
	b, exists := c.files[p]
	if exists {
		var err error
		if n, err = Parse(p, b); err != nil {
			c.err = fmt.Errorf("%s: %w; fix it and run the checkpoint again", p, err)
			return
		}
	} else if create != nil {
		n = create()
	} else {
		return // a person removed the note; a checkpoint does not bring it back
	}
	before := string(n.Render())
	for _, k := range slices.Sorted(maps.Keys(meta)) {
		v := meta[k]
		if k == "sources" { // a person's added sources stay
			v = union(n.List("sources"), v.([]string))
		}
		if !sameValue(n, k, v) {
			n.Set(k, v)
		}
	}
	for _, k := range slices.SortedFunc(maps.Keys(blocks), byBlockOrder) {
		n.Body = SetBlock(n.Body, k, blocks[k])
	}
	if exists && string(n.Render()) == before {
		return // nothing changed: keep the bytes a person wrote, comments and all
	}
	n.Set("updated", c.today)
	c.files[p] = n.Render()
}

// sameValue reports whether key already holds v, whatever its YAML style.
func sameValue(n *Note, key string, v any) bool {
	if !n.Has(key) {
		return false
	}
	switch v := v.(type) {
	case []string:
		return slices.Equal(n.List(key), v)
	case string:
		return n.Str(key) == v
	case int:
		return n.Str(key) == strconv.Itoa(v)
	}
	return false
}

// blockOrder is the order in which a checkpoint appends the generated blocks
// a note lacks, so a release reads Record, Plan, Changes, Delivered.
var blockOrder = []string{"record", "plan", "changes", "delivered", "history", "facts", "roles", "events", "notes", "workstreams", "releases", "review"}

func byBlockOrder(a, b string) int {
	ia, ib := slices.Index(blockOrder, a), slices.Index(blockOrder, b)
	if ia < 0 || ib < 0 || ia == ib {
		return strings.Compare(a, b)
	}
	return ia - ib
}

func union(a, b []string) []string {
	out := slices.Clone(a)
	for _, s := range b {
		if !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return nonNil(out)
}

// noteFor returns the note path whose `issues` list names issue n (an
// Objective for a Goal or Initiative, a Capability for a Feature).
func (c *cp) notesFor(n int) []string {
	var out []string
	for _, p := range slices.Sorted(mapKeys(c.v.Notes)) {
		for _, s := range c.v.Notes[p].List("issues") {
			if num, ok := issueNum(s); ok && num == n {
				out = append(out, p)
			}
		}
	}
	return out
}

func (c *cp) issueLine(n int) string {
	is, ok := c.r.Issues[n]
	if !ok {
		return fmt.Sprintf("#%d", n)
	}
	s := fmt.Sprintf("#%d %s", n, typeLabel(is.Type))
	if is.Title != "" {
		s += ": " + Fact(is.Title)
	}
	return s
}

func typeLabel(t string) string {
	if t == "" {
		return "issue"
	}
	return strings.ToUpper(t[:1]) + t[1:]
}

// ancestors returns n's parent chain, nearest first.
func (c *cp) ancestors(n int) []int {
	var out []int
	for seen := map[int]bool{n: true}; ; {
		p := c.r.Issues[n].Parent
		if p == 0 || seen[p] {
			return out
		}
		seen[p] = true
		out = append(out, p)
		n = p
	}
}

// delivered maps each version to the stories its promotions shipped.
func (c *cp) delivered() map[string][]int {
	out := map[string][]int{}
	for _, g := range c.r.Rungs {
		v := g.Version
		if !versionRE.MatchString(v) {
			v = g.Tag
		}
		for _, s := range g.Stories {
			if !slices.Contains(out[v], s) {
				out[v] = append(out[v], s)
			}
		}
	}
	for _, s := range out {
		slices.Sort(s)
	}
	return out
}

// CompareVersions orders vX.Y.Z strings by number, not by text.
func CompareVersions(a, b string) int {
	pa, pb := strings.Split(strings.Trim(a, "-[]v "), "."), strings.Split(strings.Trim(b, "-[]v "), ".")
	for i := 0; i < len(pa) && i < len(pb); i++ {
		x, _ := strconv.Atoi(pa[i])
		y, _ := strconv.Atoi(pb[i])
		if x != y {
			return x - y
		}
	}
	return strings.Compare(a, b)
}

func releasePath(version string) string { return "Releases/" + version + ".md" }

// releases creates and refreshes one note per version: the plan from the
// newest baseline, the changes since it, and what shipped.
func (c *cp) releases() {
	versions := map[string]bool{}
	for _, t := range c.r.Tags {
		if versionRE.MatchString(t.Name) {
			versions[t.Name] = true
		}
	}
	for _, g := range c.r.Rungs {
		if versionRE.MatchString(g.Version) {
			versions[g.Version] = true
		}
	}
	for _, b := range c.r.Baseline {
		versions[b.Version] = true
	}
	del := c.delivered()
	t, _ := FindType("release")
	for _, ver := range slices.SortedFunc(maps.Keys(versions), CompareVersions) {
		var src []string
		var rec []string
		status := "planned"
		var issue int
		for _, tg := range c.r.Tags {
			if tg.Name == ver {
				status = "released"
				src = append(src, "tag "+ver)
				rec = append(rec, fmt.Sprintf("- Tagged %s on %s ([GitHub release](%s)).", ver, tg.Date, tg.URL))
			}
		}
		for _, g := range c.r.Rungs {
			if g.Version == ver {
				if g.Target == "main" {
					status = "released" // the main rung proves it, with or without a GitHub Release
				}
				issue = g.Release
				src = append(src, "issue #"+strconv.Itoa(g.Release), "ledger "+g.Path)
				rec = append(rec, fmt.Sprintf("- Promoted to %s as %s by Release #%d (ledger `%s`).", g.Target, g.Tag, g.Release, g.Path))
			}
		}
		blocks := map[string]string{"record": strings.Join(rec, "\n")}
		var bl *Baseline
		for i := range c.r.Baseline {
			if c.r.Baseline[i].Version == ver {
				bl = &c.r.Baseline[i]
			}
		}
		if bl != nil {
			issue = bl.ReleaseIssue
			src = append(src, "issue #"+strconv.Itoa(bl.ReleaseIssue), "ledger "+bl.Path)
			blocks["plan"] = c.plan(*bl)
		}
		for _, d := range c.r.Drift {
			if d.Version == ver {
				src = append(src, "ledger "+d.Path)
				blocks["changes"] = c.changes(d)
			}
		}
		if s := del[ver]; len(s) > 0 {
			blocks["delivered"] = c.deliveredBlock(s)
		}
		p := releasePath(ver)
		meta := map[string]any{"sources": union(nil, src), "version": ver}
		if issue != 0 {
			meta["release_issue"] = issue
		}
		if old, ok := c.files[p]; ok && status == "released" {
			if n, err := Parse(p, old); err == nil && n.Str("status") == "planned" {
				meta["status"] = "released" // a person's later status (archived) stays
			}
		}
		c.upsert(p, func() *Note {
			n := newNote(t, "release-"+ver, ver, c.today, nil, "[[Releases]]")
			n.Set("status", status)
			n.Body = "# " + ver + "\n\n## Intent\n\n## Objectives served\n\n## Out of scope\n\n## Outcome\n\n## Record\n\n" +
				Block("record", "") + "\n\n## Plan\n\n" + Block("plan", "") + "\n\n## Changes since baseline\n\n" + Block("changes", "") +
				"\n\n## Delivered\n\n" + Block("delivered", "") + "\n"
			return n
		}, meta, blocks)
	}
}

// group renders issue lines grouped under their Initiative and Feature, each
// group linked to the Objective or Capability note that names it.
func (c *cp) group(items []PlanItem) string {
	type key struct{ init, feat int }
	groups := map[key][]PlanItem{}
	for _, it := range items {
		var k key
		chain := append([]int{it.Number}, c.ancestors(it.Number)...)
		if it.Parent != 0 && (len(chain) < 2 || chain[1] != it.Parent) {
			chain = append([]int{it.Number, it.Parent}, c.ancestors(it.Parent)...)
		}
		for _, n := range chain[1:] {
			switch c.r.Issues[n].Type {
			case "feature":
				if k.feat == 0 {
					k.feat = n
				}
			case "initiative":
				if k.init == 0 {
					k.init = n
				}
			}
		}
		groups[k] = append(groups[k], it)
	}
	keys := slices.SortedFunc(maps.Keys(groups), func(a, b key) int {
		if a.init != b.init {
			return a.init - b.init
		}
		return a.feat - b.feat
	})
	var b strings.Builder
	for _, k := range keys {
		var head []string
		for _, n := range []int{k.init, k.feat} {
			if n == 0 {
				continue
			}
			h := c.issueLine(n)
			for _, p := range c.notesFor(n) {
				h += " " + Link(p)
			}
			head = append(head, h)
		}
		if len(head) == 0 {
			head = []string{"No Initiative or Feature"}
		}
		fmt.Fprintf(&b, "- %s\n", strings.Join(head, " / "))
		for _, it := range groups[k] {
			line := c.issueLine(it.Number)
			if _, ok := c.r.Issues[it.Number]; !ok {
				line = fmt.Sprintf("#%d %s", it.Number, typeLabel(Fact(it.Type)))
				if it.Title != "" {
					line += ": " + Fact(it.Title)
				}
			}
			if it.Effort != "" {
				line += " (effort " + Fact(it.Effort) + ")"
			}
			fmt.Fprintf(&b, "  - %s\n", line)
		}
	}
	return b.String()
}

func (c *cp) plan(bl Baseline) string {
	by := ""
	if bl.BaselinedBy != "" {
		by = " by @" + strings.TrimPrefix(bl.BaselinedBy, "@")
	}
	head := fmt.Sprintf("Baseline %d of Release #%d, taken %s%s (ledger `%s`). Change budget: %s%%.\n\n", bl.N, bl.ReleaseIssue, bl.BaselinedAt, by, bl.Path, pct(bl.BudgetPct))
	return head + c.group(bl.Items)
}

func pct(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

func (c *cp) changes(d Drift) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Drift since baseline %d: %s%% of a %s%% budget (ledger `%s`).", d.Baseline, pct(d.DriftPct), pct(d.BudgetPct), d.Path)
	if d.Decision != 0 {
		fmt.Fprintf(&b, " Over the budget: Decision #%d.", d.Decision)
		for _, p := range c.notesFor(d.Decision) {
			b.WriteString(" " + Link(p))
		}
	}
	b.WriteString("\n")
	for _, part := range []struct {
		name  string
		items []PlanItem
	}{{"Added", d.Added}, {"Removed", d.Removed}} {
		if len(part.items) > 0 {
			fmt.Fprintf(&b, "\n%s:\n\n%s", part.name, c.group(part.items))
		}
	}
	if len(d.Resized) > 0 {
		b.WriteString("\nResized:\n\n")
		for _, rz := range d.Resized {
			fmt.Fprintf(&b, "- %s: effort %s to %s", c.issueLine(rz.Number), Fact(rz.From), Fact(rz.To))
			if rz.FromType != "" {
				fmt.Fprintf(&b, "; type %s to %s", typeLabel(Fact(rz.FromType)), typeLabel(Fact(rz.Type)))
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (c *cp) deliveredBlock(stories []int) string {
	items := make([]PlanItem, 0, len(stories))
	for _, s := range stories {
		items = append(items, PlanItem{Number: s, Type: c.r.Issues[s].Type, Title: c.r.Issues[s].Title})
	}
	return c.group(items)
}

// issueBlocks refreshes the record block of every note that lists issues: the
// issue links, and for Objectives and Capabilities the releases that
// delivered work under them ("Delivered in v0.2.0 by #321, #340").
func (c *cp) issueBlocks() {
	del := c.delivered()
	for _, p := range slices.Sorted(mapKeys(c.v.Notes)) {
		n := c.v.Notes[p]
		var nums []int
		for _, s := range n.List("issues") {
			num, ok := issueNum(s)
			if !ok {
				c.err = fmt.Errorf("%s: issues: %q is not an issue number; fix it and run the checkpoint again", p, s)
				continue
			}
			nums = append(nums, num)
		}
		if t, _ := FindType(n.Str("type")); len(nums) == 0 && slices.Contains(t.Keys, "issues") {
			// the last issue was removed: its generated facts go too
			clear := map[string]string{}
			for k, b := range Blocks(n.Body) {
				if (k == "record" || k == "history") && b != "" {
					clear[k] = ""
				}
			}
			if len(clear) > 0 {
				c.upsert(p, nil, nil, clear)
			}
			continue
		}
		if len(nums) == 0 {
			continue
		}
		var lines []string
		for _, num := range nums {
			lines = append(lines, "- Recorded in "+c.issueLine(num)+".")
		}
		var hist []string
		for _, ver := range slices.SortedFunc(maps.Keys(del), CompareVersions) {
			var by []string
			for _, s := range del[ver] {
				anc := c.ancestors(s)
				for _, num := range nums {
					if slices.Contains(anc, num) || s == num {
						by = append(by, "#"+strconv.Itoa(s))
						break
					}
				}
			}
			if len(by) > 0 {
				link := ver
				if _, ok := c.files[releasePath(ver)]; ok {
					link = "[[" + ver + "]]"
				}
				hist = append(hist, fmt.Sprintf("- Delivered in %s by %s.", link, strings.Join(by, ", ")))
			}
		}
		blocks := map[string]string{"record": strings.Join(lines, "\n")}
		if t := n.Str("type"); t == "objective" || t == "capability" {
			blocks["history"] = strings.Join(hist, "\n")
		}
		c.upsert(p, nil, nil, blocks)
	}
}

// seats writes one note per seat and one per named person. A seat owns
// notes, so its knowledge stays when a person leaves.
func (c *cp) seats() {
	st, _ := FindType("seat")
	pt, _ := FindType("person")
	people := map[string][]string{} // by lower-case login: GitHub logins ignore case
	if c.r.Approver != "" {
		people[strings.ToLower(c.r.Approver)] = append(people[strings.ToLower(c.r.Approver)], "the approver ([autonomy] approver)")
	}
	for _, s := range c.r.Seats {
		facts := []string{
			fmt.Sprintf("- Twin: %s. %s", s.Twin, s.Purpose),
			"- Accountable for: " + strings.Join(codes(s.Verbs), ", ") + ".",
		}
		if s.Human != "" {
			facts = append(facts, "- Paired with [["+strings.ToLower(s.Human)+"]].")
			people[strings.ToLower(s.Human)] = append(people[strings.ToLower(s.Human)], "[["+s.Name+"]]")
		} else {
			facts = append(facts, "- Solo: the twin runs it alone.")
		}
		if len(s.Verbs) == 0 {
			facts[1] = "- Accountable for no verb."
		}
		p := "Seats/" + s.Name + ".md"
		c.upsert(p, func() *Note {
			n := newNote(st, "seat-"+s.ID, s.Name, c.today, nil, "[[Seats]]")
			n.Set("status", "current")
			n.Body = "# " + s.Name + "\n\n## Facts\n\n" + Block("facts", "") + "\n\n## How this seat works here\n"
			return n
		}, map[string]any{"sources": []string{"file seats/" + s.ID + "/persona.toml", "file internal/verbs/registry.go"}},
			map[string]string{"facts": strings.Join(facts, "\n")})
	}
	// A person who no longer holds a role keeps their note; its generated
	// roles say so, rather than a role they lost. A login matches its note
	// whatever the case of either (People/Alice.md is alice's note).
	notes := map[string]string{}
	for _, p := range slices.Sorted(mapKeys(c.v.Notes)) {
		n := c.v.Notes[p]
		if n.Str("type") != "person" {
			continue
		}
		login := strings.ToLower(Base(p))
		notes[login] = p
		if people[login] == nil && Blocks(n.Body)["roles"] != "" {
			c.upsert(p, nil, nil, map[string]string{"roles": "- No role in the current records."})
		}
	}
	for _, login := range slices.Sorted(maps.Keys(people)) {
		p := notes[login]
		if p == "" {
			p = "People/" + login + ".md"
		}
		c.upsert(p, func() *Note {
			n := newNote(pt, "person-"+login, login, c.today, nil, "[[People]]")
			n.Set("people", []string{login})
			n.Body = "# " + login + "\n\n## Roles\n\n" + Block("roles", "") + "\n\n## Context\n"
			return n
		}, map[string]any{"sources": []string{"file .aayudha/config.toml"}},
			map[string]string{"roles": "- " + strings.Join(people[login], "\n- ")})
	}
}

func codes(vs []string) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = "`" + v + "`"
	}
	return out
}

// timeline writes the dated lines: inception, each release, and each
// decision note's recorded issue.
func (c *cp) timeline() {
	type ev struct{ date, line string }
	var evs []ev
	if c.r.Inception != "" {
		evs = append(evs, ev{c.r.Inception, "Inception: the repository " + c.r.Repo + " is created."})
	}
	tagged := map[string]bool{}
	for _, t := range c.r.Tags {
		if versionRE.MatchString(t.Name) {
			tagged[t.Name] = true
			evs = append(evs, ev{t.Date, "[[" + t.Name + "]] is released (tag " + t.Name + ")."})
		}
	}
	for _, g := range c.r.Rungs { // a version on main without a GitHub Release
		if g.Target == "main" && versionRE.MatchString(g.Version) && !tagged[g.Version] && g.Date != "" {
			tagged[g.Version] = true
			evs = append(evs, ev{g.Date, fmt.Sprintf("[[%s]] is released (promoted to main by Release #%d).", g.Version, g.Release)})
		}
	}
	for _, p := range slices.Sorted(mapKeys(c.v.Notes)) {
		n := c.v.Notes[p]
		if n.Str("type") != "decision" {
			continue
		}
		for _, s := range n.List("issues") {
			num, ok := issueNum(s)
			if !ok || c.r.Issues[num].Closed == "" {
				continue
			}
			evs = append(evs, ev{c.r.Issues[num].Closed, Link(p) + " is decided in #" + strconv.Itoa(num) + "."})
		}
	}
	slices.SortStableFunc(evs, func(a, b ev) int { return strings.Compare(a.date+a.line, b.date+b.line) })
	var lines []string
	for _, e := range evs {
		lines = append(lines, "- "+e.date+": "+e.line)
	}
	src := []string{}
	for _, t := range c.r.Tags {
		if versionRE.MatchString(t.Name) {
			src = append(src, "tag "+t.Name)
		}
	}
	if c.r.Repo != "" {
		src = append(src, "https://github.com/"+c.r.Repo)
	}
	c.upsert(TimelinePath, nil, map[string]any{"sources": src}, map[string]string{"events": strings.Join(lines, "\n")})
}

// mocs refreshes every map of content: the notes that link up to it.
func (c *cp) mocs() {
	children := map[string][]string{}
	for _, p := range slices.Sorted(mapKeys(c.v.Notes)) {
		if up := c.v.Resolve(linkTarget(c.v.Notes[p].Str("up"))); up != "" {
			children[up] = append(children[up], p)
		}
	}
	for _, p := range slices.Sorted(mapKeys(c.v.Notes)) {
		n := c.v.Notes[p]
		if n.Str("type") != "moc" {
			continue
		}
		var lines []string
		for _, ch := range children[p] {
			line := "- " + Link(ch)
			if t := c.v.Notes[ch].Str("title"); t != "" && t != Base(ch) {
				line += ": " + t
			}
			lines = append(lines, line)
		}
		c.upsert(p, nil, nil, map[string]string{"notes": strings.Join(lines, "\n")})
	}
}

func (c *cp) reviewList() []string {
	var out []string
	v := Load(c.files)
	for _, p := range slices.Sorted(mapKeys(v.Notes)) {
		n := v.Notes[p]
		if rv := n.Str("reviewed"); p != HomePath && rv != "" && n.Str("updated") > rv {
			out = append(out, p)
		}
	}
	return out
}

// home refreshes Home: the workstreams, the releases, and the notes that
// need review.
func (c *cp) home() {
	var ws, rel []string
	for _, p := range slices.Sorted(mapKeys(c.v.Notes)) {
		switch c.v.Notes[p].Str("type") {
		case "workstream":
			ws = append(ws, "- "+Link(p))
		case "release":
			rel = append(rel, "- "+Link(p))
		}
	}
	slices.SortFunc(rel, func(a, b string) int { return CompareVersions(b, a) })
	var rv []string
	for _, p := range c.reviewList() {
		rv = append(rv, "- "+Link(p))
	}
	c.upsert(HomePath, nil, nil, map[string]string{"workstreams": strings.Join(ws, "\n"), "releases": strings.Join(rel, "\n"), "review": strings.Join(rv, "\n")})
}
