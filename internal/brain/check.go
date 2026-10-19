package brain

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/brief"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// Finding is one problem in the vault.
type Finding struct {
	Path    string `json:"path"`
	Rule    string `json:"rule"`
	Message string `json:"message"`
}

func (f Finding) String() string { return fmt.Sprintf("%s: %s: %s", f.Path, f.Rule, f.Message) }

// Report is the result of Check: errors fail the check; warnings do not.
type Report struct {
	Notes    int       `json:"notes"`
	Errors   []Finding `json:"errors"`
	Warnings []Finding `json:"warnings"`
}

// OK reports whether the vault passed.
func (r Report) OK() bool { return len(r.Errors) == 0 }

// Vault is the parsed vault: every note by path, and link resolution.
type Vault struct {
	Files  map[string][]byte
	Notes  map[string]*Note // Markdown notes outside Templates/, by path
	bad    map[string]error // notes whose frontmatter did not parse
	names  map[string][]string
	unsafe []string // notes whose file name breaks a wikilink: no generated link names them
}

// isTemplate reports whether p is an Obsidian template (outside the checks).
func isTemplate(p string) bool { return strings.HasPrefix(p, "Templates/") }

// Load parses the vault's files.
func Load(files map[string][]byte) *Vault {
	v := &Vault{Files: files, Notes: map[string]*Note{}, bad: map[string]error{}, names: map[string][]string{}}
	for p, b := range files {
		if strings.HasPrefix(p, ".") || strings.Contains(p, "/.") {
			continue
		}
		name := path.Base(p)
		if strings.HasSuffix(p, ".md") {
			name = Base(p)
		}
		v.names[strings.ToLower(name)] = append(v.names[strings.ToLower(name)], p)
		if !strings.HasSuffix(p, ".md") || isTemplate(p) {
			continue
		}
		if b := Base(p); unsafeName.MatchString(b) || b == "" || strings.TrimSpace(b) != b { // [[x]] ![t](u)]] would close the link; [[ A]] reaches A
			v.unsafe = append(v.unsafe, p)
			continue
		}
		n, err := Parse(p, b)
		v.Notes[p] = n
		if err != nil {
			v.bad[p] = err
		}
	}
	return v
}

// Resolve returns the path a wikilink target names, or "".
func (v *Vault) Resolve(target string) string {
	t, _, _ := strings.Cut(target, "|")
	t, _, _ = strings.Cut(t, "#")
	t = strings.TrimSpace(t)
	if t == "" {
		return "."
	}
	if strings.Contains(t, "/") {
		for _, c := range []string{t, t + ".md"} {
			if _, ok := v.Files[c]; ok {
				return c
			}
		}
		return ""
	}
	ps := v.names[strings.ToLower(strings.TrimSuffix(t, ".md"))]
	if len(ps) == 0 {
		ps = v.names[strings.ToLower(t)]
	}
	if len(ps) == 1 {
		return ps[0]
	}
	return ""
}

var (
	wikilinkRE = regexp.MustCompile(`!?\[\[([^\[\]]+)\]\]`)
	commentRE  = regexp.MustCompile(`(?s)<!--.*?-->`)
	// loginRE is a GitHub login: letters, digits, and single hyphens between them, at most 39 characters.
	loginRE = regexp.MustCompile(`^@?[A-Za-z0-9]+(?:-[A-Za-z0-9]+)*$`)
	// sourceRE is the evidence rule's citation forms.
	sourceRE = regexp.MustCompile(`^(?:(?:issue|PR|Decision|Goal|Initiative|Feature|Story|Bug|Release|Feedback|Experiment|Incident) #\d+|tag \S+|ledger \S+|file \S+|commit [0-9a-f]{7,40}|https://\S+)$`)
	// Copy rule: a pasted Completion Goal criterion, or an issue label.
	criterionRE = regexp.MustCompile(`(?m)^\s*\d+\.\s*\[(?:screenshot|recording|sr-transcript|api-trace|metric)\]`)
	labelRE     = regexp.MustCompile(`\b(?:status|type|priority|effort|risk|health|horizon):[A-Za-z0-9][A-Za-z0-9-]*\b`)
	urlRE       = regexp.MustCompile(`https?://[^\s)>\]]+`)
)

// links returns the wikilink targets of text outside code.
func links(text string) []string {
	text = maskCode(stripComments(text))
	var out []string
	for _, m := range wikilinkRE.FindAllStringSubmatch(text, -1) {
		out = append(out, strings.ReplaceAll(m[1], `\|`, "|")) // a pipe escaped inside a table
	}
	return out
}

// issueNum reads an issues entry: a positive number, with or without #.
func issueNum(s string) (int, bool) {
	n, err := strconv.Atoi(strings.TrimPrefix(strings.TrimSpace(s), "#"))
	return n, err == nil && n > 0
}

func isLogin(s string) bool { return loginRE.MatchString(s) && len(strings.TrimPrefix(s, "@")) <= 39 }

// IsVault reports whether files are an aayudha brain: the aayudha marker, or
// a Home note of type home. A branch named brain that has neither is someone
// else's, and aayu writes nothing to it.
func IsVault(files map[string][]byte) bool {
	if string(files[MarkerPath]) == Marker {
		return true
	}
	n, err := Parse(HomePath, files[HomePath])
	return err == nil && n.Str("type") == "home"
}

// stripComments removes the HTML comments outside code: a `<!--` or `-->`
// inside a code span or block is text, so it cannot hide what follows it.
func stripComments(s string) string {
	masked := maskCode(s)
	var b strings.Builder
	last := 0
	for _, m := range commentRE.FindAllStringIndex(masked, -1) {
		b.WriteString(s[last:m[0]])
		last = m[1]
	}
	b.WriteString(s[last:])
	return b.String()
}

// isDate reports whether s is a real calendar date written YYYY-MM-DD.
func isDate(s string) bool {
	_, err := time.Parse(time.DateOnly, s)
	return err == nil
}

// isNote reports whether a resolved path is a note of the vault, not "",
// a same-note anchor ("."), a template, or an attachment.
func (v *Vault) isNote(p string) bool { return v.Notes[p] != nil }

// linkTarget strips the [[ ]] of a frontmatter link value; "" if it is not one.
func linkTarget(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "[[") || !strings.HasSuffix(s, "]]") {
		return ""
	}
	return s[2 : len(s)-2]
}

// linkKeys are the frontmatter keys that hold wikilinks.
var linkKeys = []string{"up", "direction", "objectives", "objections", "supersedes", "superseded_by", "objects_to"}

// MaxSentenceWords is the STE limit the brain holds human text to: the
// repository's shared rule (internal/brief, ASD-STE100 rule 5.1).
const MaxSentenceWords = brief.MaxSTEWords

// prose is what a person wrote: the body without generated blocks and
// comments, with wikilinks replaced by the text they show. brief.Parse skips
// its code; the copy checks mask it (maskCode).
func prose(body string) string {
	s := stripComments(StripBlocks(body))
	return wikilinkRE.ReplaceAllStringFunc(s, func(m string) string {
		t := wikilinkRE.FindStringSubmatch(m)[1]
		if _, alias, ok := strings.Cut(t, "|"); ok {
			return alias
		}
		t, _, _ = strings.Cut(t, "#")
		return path.Base(t)
	})
}

// Check holds the vault to its rules: valid frontmatter, cited sources, links
// that resolve (across workstreams too), unique note names, an up link on
// every note, two-way supersede and objection links, workstream indexes,
// STE sentence length, and no text copied from an issue. now dates the
// review warnings.
func Check(files map[string][]byte) Report {
	v := Load(files)
	var r Report
	fail := func(p, rule, f string, a ...any) {
		r.Errors = append(r.Errors, Finding{Path: p, Rule: rule, Message: fmt.Sprintf(f, a...)})
	}
	warn := func(p, rule, f string, a ...any) {
		r.Warnings = append(r.Warnings, Finding{Path: p, Rule: rule, Message: fmt.Sprintf(f, a...)})
	}
	if _, ok := files[HomePath]; !ok {
		fail(HomePath, "home", "the vault has no Home note; run aayu brain init")
	}
	for name, ps := range v.names {
		slices.Sort(ps)
		if len(ps) > 1 && strings.HasSuffix(ps[0], ".md") {
			fail(ps[0], "unique-name", "%d notes share the name %q (%s): a wikilink to it is ambiguous", len(ps), name, strings.Join(ps, ", "))
		}
	}
	for _, p := range v.unsafe {
		fail(p, "name", "the file name %q breaks a wikilink (a character of [ ] # ^ | \\ : * ? \" < >, or space at an end): rename the note", Base(p))
	}
	ids := map[string]string{}
	for _, p := range slices.Sorted(mapKeys(v.Notes)) {
		n := v.Notes[p]
		r.Notes++
		if err := v.bad[p]; err != nil {
			fail(p, "frontmatter", "%v", err)
			continue
		}
		typ := n.Str("type")
		t, ok := FindType(typ)
		if !ok {
			fail(p, "frontmatter", "type %q is not a note type", typ)
			continue
		}
		id := n.Str("id")
		switch {
		case id == "":
			fail(p, "frontmatter", "no id")
		case ids[id] != "":
			fail(p, "frontmatter", "id %q is also the id of %s", id, ids[id])
		default:
			ids[id] = p
		}
		if n.Str("title") == "" {
			fail(p, "frontmatter", "no title")
		}
		if o := n.Str("owner"); o == "" {
			fail(p, "frontmatter", "no owner seat")
		} else if _, ok := verbs.FindSeat(verbs.SeatID(o)); !ok {
			fail(p, "frontmatter", "owner %q is not a seat slug (a seat outlives a person; name people under people)", o)
		}
		for _, s := range n.List("issues") {
			if _, ok := issueNum(s); !ok {
				fail(p, "frontmatter", "issues: %q is not an issue number", s)
			}
		}
		for _, l := range n.List("people") {
			if !isLogin(l) {
				fail(p, "frontmatter", "people: %q is not a GitHub login", l)
			}
		}
		if s := n.Str("status"); !slices.Contains(t.Statuses, s) {
			fail(p, "frontmatter", "status %q is not one of %s", s, strings.Join(t.Statuses, ", "))
		}
		for _, k := range []string{"updated", "reviewed"} {
			if d := n.Str(k); (k == "updated" || n.Has(k)) && !isDate(d) {
				fail(p, "frontmatter", "%s %q is not a YYYY-MM-DD date", k, d)
			}
		}
		if !slices.Contains(n.List("tags"), "type/"+typ) {
			fail(p, "frontmatter", "tags must include type/%s (the graph colours notes by it)", typ)
		}
		if typ != "home" && typ != "moc" {
			src := n.List("sources")
			if len(src) == 0 {
				fail(p, "sources", "no sources: cite the issue, PR, Decision, tag, ledger path, or file each claim comes from")
			}
			for _, s := range src {
				if !sourceRE.MatchString(s) {
					fail(p, "sources", "source %q is not a citation (issue #n, PR #n, Decision #n, tag vX.Y.Z, ledger <path>, file <path>, commit <sha>, or an https URL)", s)
				}
			}
		}
		if typ != "home" {
			up := linkTarget(n.Str("up"))
			switch {
			case up == "":
				fail(p, "orphan", "no up link: link the note up to its map of content")
			case !v.isNote(v.Resolve(up)) || v.Resolve(up) == p:
				fail(p, "orphan", "up link [[%s]] does not resolve to another note", up)
			}
		}
		for _, k := range linkKeys {
			for _, val := range n.List(k) {
				if lt := linkTarget(val); lt == "" {
					fail(p, "link", "%s: %q is not a [[wikilink]]", k, val)
				} else if q := v.Resolve(lt); !v.isNote(q) || q == p { // the note itself, a template, or an attachment links no other note
					fail(p, "link", "%s: [[%s]] does not resolve to another note", k, lt)
				} else if want := target[k]; want != "" && v.Notes[q].typ() != want {
					fail(p, "link", "%s: [[%s]] has type %s; %s links a note of type %s", k, lt, v.Notes[q].typ(), k, want)
				}
			}
		}
		for _, l := range links(n.Body) {
			if v.Resolve(l) == "" {
				fail(p, "link", "[[%s]] does not resolve", l)
			}
		}
		if k := dupBlock(n.Body); k != "" {
			fail(p, "block", "two generated %s blocks: a checkpoint refreshes one, so delete the other", k)
		}
		human := prose(n.Body)
		units := proseUnits(human)
		for _, s := range brief.Sentences(units) {
			if len(s.Words) > MaxSentenceWords {
				fail(p, "ste", "a sentence has %d words (at most %d): %q", len(s.Words), MaxSentenceWords, clip(s.Text))
			}
		}
		masked := maskCode(human)
		if m := criterionRE.FindString(masked); m != "" {
			fail(p, "copy", "a Completion Goal criterion is pasted from an issue (%q): link the issue instead", strings.TrimSpace(m))
		}
		if m := labelRE.FindString(urlRE.ReplaceAllString(masked, "")); m != "" {
			fail(p, "copy", "an issue label (%s) is copied into the note: the issue holds status and labels; link it instead", m)
		}
		if rv := n.Str("reviewed"); rv != "" && n.Str("updated") > rv {
			warn(p, "review", "updated %s after a person last reviewed it on %s", n.Str("updated"), rv)
		}
		if typ == "objection" {
			if st := n.Str("status"); st != "open" && strings.TrimSpace(n.Str("resolution")) == "" {
				fail(p, "objection", "status %s needs a resolution: say why it was answered, accepted, or set aside", st)
			}
			if rb := n.Str("raised_by"); rb == "" {
				fail(p, "objection", "no raised_by: name who raised it, by GitHub login or seat slug")
			} else if _, seat := verbs.FindSeat(verbs.SeatID(rb)); !seat && !isLogin(rb) {
				fail(p, "objection", "raised_by %q is neither a seat slug nor a GitHub login", rb)
			}
			if lt := linkTarget(n.Str("objects_to")); lt == "" {
				fail(p, "objection", "no objects_to link: name the note this objection is against")
			}
		}
	}
	r.Errors = append(r.Errors, v.twoWay()...)
	r.Errors = append(r.Errors, v.workstreams()...)
	sortFindings(r.Errors)
	sortFindings(r.Warnings)
	return r
}

func proseUnits(s string) []brief.Unit {
	d, err := brief.Parse([]byte(s), "")
	if err != nil {
		return nil
	}
	return d.Units
}

func clip(s string) string {
	if r := []rune(s); len(r) > 80 {
		return string(r[:77]) + "..."
	}
	return s
}

// twoWay checks the links that must point both ways: supersedes and
// superseded_by between decisions, and objects_to and objections between an
// objection and the note it is against.
func (v *Vault) twoWay() []Finding {
	var out []Finding
	has := func(p, key, back string) bool {
		n := v.Notes[p]
		if n == nil {
			return false
		}
		for _, val := range n.List(key) {
			if v.Resolve(linkTarget(val)) == back {
				return true
			}
		}
		return false
	}
	pairs := [][2]string{{"supersedes", "superseded_by"}, {"superseded_by", "supersedes"}, {"objects_to", "objections"}, {"objections", "objects_to"}}
	for _, p := range slices.Sorted(mapKeys(v.Notes)) {
		n := v.Notes[p]
		for _, kp := range pairs {
			for _, val := range n.List(kp[0]) {
				q := v.Resolve(linkTarget(val))
				if q == "" || q == "." {
					continue // reported as a broken link
				}
				from, to := n.Str("type"), v.Notes[q].typ()
				if want := relation[kp[0]]; !slices.Contains(want[0], from) || !slices.Contains(want[1], to) {
					out = append(out, Finding{Path: p, Rule: "two-way", Message: fmt.Sprintf("%s: [[%s]] has type %s, and this note has type %s; %s runs from type %s to type %s", kp[0], Base(q), to, from, kp[0], strings.Join(want[0], " or "), strings.Join(want[1], " or "))})
					continue
				}
				if !has(q, kp[1], p) {
					out = append(out, Finding{Path: p, Rule: "two-way", Message: fmt.Sprintf("%s: [[%s]] does not link back under %s", kp[0], Base(q), kp[1])})
				}
			}
		}
	}
	return out
}

// objectable are the note types an objection may be against.
var objectable = []string{"direction", "principle", "objective", "decision"}

// relation holds, for each two-way key, the note types it may run from and to.
// target is the note type a one-way relationship field links to.
var target = map[string]string{"direction": "direction", "objectives": "objective"}

var relation = map[string][2][]string{
	"supersedes":    {{"decision"}, {"decision"}},
	"superseded_by": {{"decision"}, {"decision"}},
	"objects_to":    {{"objection"}, objectable},
	"objections":    {objectable, {"objection"}},
}

func (n *Note) typ() string {
	if n == nil {
		return ""
	}
	return n.Str("type")
}

// workstreams checks that each workstream folder has its index, and that the
// index links up out of the workstream into the root vault.
func (v *Vault) workstreams() []Finding {
	var out []Finding
	seen := map[string]bool{}
	for p := range v.Files {
		if ws := workstreamOf(p); ws != "" {
			seen[ws] = true
		}
	}
	for _, ws := range slices.Sorted(mapKeys(seen)) {
		idx := WorkstreamIndex(ws)
		n := v.Notes[idx]
		if n == nil || n.Str("type") != "workstream" {
			out = append(out, Finding{Path: "Workstreams/" + ws, Rule: "workstream", Message: "no index note " + idx + " of type workstream; run aayu brain init --workstream " + ws})
			continue
		}
		if up := v.Resolve(linkTarget(n.Str("up"))); up != "" && workstreamOf(up) == ws {
			out = append(out, Finding{Path: idx, Rule: "workstream", Message: "the index must link up into the root vault (for example [[Workstreams]]), not inside its own workstream"})
		}
		if _, ok := v.Files[WorkstreamMOC(ws)]; !ok {
			out = append(out, Finding{Path: "Workstreams/" + ws, Rule: "workstream", Message: "no map of content " + WorkstreamMOC(ws)})
		}
	}
	return out
}

func sortFindings(fs []Finding) {
	slices.SortFunc(fs, func(a, b Finding) int { return strings.Compare(a.String(), b.String()) })
}

func mapKeys[M ~map[string]V, V any](m M) func(func(string) bool) {
	return func(yield func(string) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}

// Today formats t as a frontmatter date.
func Today(t time.Time) string { return t.UTC().Format(DateLayout) }
