package survey

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// Claim bases: how a claim is known.
const (
	BasisObserved  = "observed"  // the code shows it (cited)
	BasisStated    = "stated"    // a document or a comment in the repository says it (cited)
	BasisInferred  = "inferred"  // reasoned from cited code, with the reasoning written down
	BasisConfirmed = "confirmed" // the person said it in the interview (an answered question, quoted)
	BasisUnknown   = "unknown"   // nobody knows yet: a question asks it
)

// Bases lists the claim bases in order of strength.
var Bases = []string{BasisObserved, BasisStated, BasisInferred, BasisConfirmed, BasisUnknown}

// Topics are the subjects a claim may have at each level.
var Topics = map[string][]string{
	"engineering": {"architecture", "modules", "data", "interfaces", "build", "dependencies", "tests", "risks", "quality", "operations"},
	"capability":  {"capability"},
	"product":     {"users", "jobs", "outcomes", "value", "constraints", "context", "competition"},
}

// Draft is the survey the host's model writes from the index, module by
// module, then rolls up: claims at three levels, the questions the code cannot
// answer (and the person's answers), and the modernisation path.
type Draft struct {
	Product       string     `json:"product"`
	Request       string     `json:"request"` // the person's own words asking for the survey
	Commit        string     `json:"commit,omitempty"`
	Summary       string     `json:"summary"`
	Engineering   []Claim    `json:"engineering"`
	Capabilities  []Claim    `json:"capabilities"`
	ProductClaims []Claim    `json:"product_claims"`
	Questions     []Question `json:"questions"`
	// Traps are the trap rules' questions asked of the code (references/traps.md).
	Traps []Trap `json:"traps,omitempty"`
	// DoNotCarry lists what the modern version deliberately does not reproduce.
	DoNotCarry []Carry `json:"do_not_carry,omitempty"`
	// Compatibility is the contract: every interface others depend on, at the
	// level the person decides.
	Compatibility []Interface    `json:"compatibility,omitempty"`
	Modernisation *Modernisation `json:"modernisation,omitempty"`
}

// Claim is one statement about the software, with how it is known.
type Claim struct {
	ID        string   `json:"id"`              // E1, C1, P1
	Topic     string   `json:"topic,omitempty"` // Topics[level]
	Name      string   `json:"name,omitempty"`  // a capability's name
	Text      string   `json:"claim"`
	Basis     string   `json:"basis"`
	Cites     []string `json:"cites,omitempty"`     // fact IDs or path:line
	Reasoning string   `json:"reasoning,omitempty"` // inferred: why the cites support it
	Answer    string   `json:"answer,omitempty"`    // confirmed: the answered question's ID
	Question  string   `json:"question,omitempty"`  // unknown: the question that asks it
}

// Question is a gap the code cannot close, for the interview.
type Question struct {
	ID      string   `json:"id"` // Q1
	Text    string   `json:"question"`
	Why     string   `json:"why"`               // what it changes
	Default string   `json:"default,omitempty"` // the default offered, with its reason
	Affects []string `json:"affects,omitempty"` // claim IDs
	Status  string   `json:"status"`            // open, answered, deferred
	Answer  string   `json:"answer,omitempty"`  // answered: the person's exact words
	Reason  string   `json:"reason,omitempty"`  // deferred: why it can wait
}

// Modernisation is the path to a modern version, in steps that each prove
// parity with the legacy build through characterisation case sets.
type Modernisation struct {
	Target       string    `json:"target"`        // what to build: "Rust", "PHP 8 with types", …
	TargetSource string    `json:"target_source"` // "request" (the target is in the person's words) or an answered question's ID
	Strategy     string    `json:"strategy"`
	Cites        []string  `json:"cites,omitempty"`
	Steps        []Step    `json:"steps"`
	CaseSets     []CaseSet `json:"case_sets"`
	Risks        []Claim   `json:"risks,omitempty"`
	// Suites are the legacy project's own tests, run against the new version.
	Suites      []Suite      `json:"suites,omitempty"`
	Migrations  []Migration  `json:"migrations,omitempty"`
	Coexistence *Coexistence `json:"coexistence,omitempty"`
	Shims       []Shim       `json:"shims,omitempty"`
}

// Step is one stage of the modernisation, in dependency order.
type Step struct {
	ID        string   `json:"id"` // M1
	Title     string   `json:"title"`
	Modules   []string `json:"modules,omitempty"` // mod: IDs it replaces
	Claims    []string `json:"claims,omitempty"`  // the claims it rests on
	DependsOn []string `json:"depends_on,omitempty"`
	Parity    []string `json:"parity"`           // case set or suite IDs that prove the step
	Avoids    []string `json:"avoids,omitempty"` // do-not-carry entries (N1) in the code it replaces
}

// CaseSet is a group of characterisation cases: inputs run on the legacy
// build, whose outputs become golden files the new build must reproduce.
type CaseSet struct {
	ID      string   `json:"id"` // G1
	Title   string   `json:"title"`
	From    []string `json:"from"`              // where the inputs come from: existing tests, entry points (cites)
	Compare string   `json:"compare"`           // exact, or normalised with what is normalised
	Cases   string   `json:"cases,omitempty"`   // the cases file (--golden), once written
	Exclude []string `json:"exclude,omitempty"` // behaviour deliberately not kept, each naming the question or claim that decided it
}

// Coverage says how much of the index the draft cites.
type Coverage struct {
	Modules       int      `json:"modules"`
	ModulesCited  int      `json:"modules_cited"`
	Entries       int      `json:"entries"`
	EntriesCited  int      `json:"entries_cited"`
	Markers       int      `json:"markers"`
	MarkersCited  int      `json:"markers_cited"`
	UncitedModule []string `json:"uncited_modules,omitempty"`
	UncitedMarker []string `json:"uncited_markers,omitempty"`
	Traps         int      `json:"traps"`
	TrapsChecked  int      `json:"traps_checked"` // trap facts a trap finding or claim cites
	UncheckedTrap []string `json:"unchecked_traps,omitempty"`
	OpenQuestions int      `json:"open_questions"`
	Unknowns      int      `json:"unknown_claims"`
}

// Report is the check of a draft.
type Report struct {
	Pass     bool     `json:"pass"`
	Problems []string `json:"problems"`
	Coverage Coverage `json:"coverage"`
}

var (
	claimID = map[string]*regexp.Regexp{"engineering": regexp.MustCompile(`^E[1-9][0-9]*$`), "capability": regexp.MustCompile(`^C[1-9][0-9]*$`), "product": regexp.MustCompile(`^P[1-9][0-9]*$`), "trap": tID}
	qID     = regexp.MustCompile(`^Q[1-9][0-9]*$`)
	mID     = regexp.MustCompile(`^M[1-9][0-9]*$`)
	gID     = regexp.MustCompile(`^G[1-9][0-9]*$`)
	riskID  = regexp.MustCompile(`^R[1-9][0-9]*$`)
)

// ParseDraft decodes a draft strictly: an unknown key is an error, so a
// misspelt field never silently drops a claim.
func ParseDraft(b []byte) (Draft, error) {
	var d Draft
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	err := dec.Decode(&d)
	return d, err
}

// Claims lists every claim of the draft with its level, risks included.
func (d Draft) Claims() map[string]Claim {
	out := map[string]Claim{}
	for _, l := range [][]Claim{d.Engineering, d.Capabilities, d.ProductClaims} {
		for _, c := range l {
			out[c.ID] = c
		}
	}
	if d.Modernisation != nil {
		for _, c := range d.Modernisation.Risks {
			out[c.ID] = c
		}
	}
	for _, t := range d.Traps {
		out[t.ID] = t.Claim
	}
	return out
}

// Check validates a draft against the index: every claim has a basis its
// evidence supports, every citation resolves, product claims are never
// "observed" (code shows what software does, not who uses it or why), and
// every unknown has a question. It also measures how much of the index the
// draft cites.
func Check(ix *Index, d Draft) Report {
	var ps []string
	add := func(format string, args ...any) { ps = append(ps, fmt.Sprintf(format, args...)) }
	if strings.TrimSpace(d.Product) == "" {
		add("product: name the product")
	}
	if strings.TrimSpace(d.Request) == "" {
		add("request: quote the person's words asking for the survey")
	}
	if strings.TrimSpace(d.Summary) == "" {
		add("summary: write the one-paragraph summary")
	}
	if d.Commit != "" && ix.Commit != "" && !strings.HasPrefix(ix.Commit, d.Commit) {
		add("commit: the draft was written against %s; the index is at %s (run `aayu survey` again, or update the draft)", d.Commit, short(ix.Commit))
	}
	questions := map[string]Question{}
	for _, q := range d.Questions {
		switch {
		case !qID.MatchString(q.ID):
			add("question %q: IDs are Q1, Q2, …", q.ID)
		case questions[q.ID].ID != "":
			add("question %s: duplicate ID", q.ID)
		}
		questions[q.ID] = q
		if strings.TrimSpace(q.Text) == "" || strings.TrimSpace(q.Why) == "" {
			add("question %s: needs the question and why it matters", q.ID)
		}
		switch q.Status {
		case "open":
		case "answered":
			if strings.TrimSpace(q.Answer) == "" {
				add("question %s: answered, but no answer: quote the person's exact words", q.ID)
			}
		case "deferred":
			if strings.TrimSpace(q.Reason) == "" {
				add("question %s: deferred, but no reason it can wait", q.ID)
			}
		default:
			add("question %s: status %q; want open, answered, or deferred", q.ID, q.Status)
		}
	}
	seen := map[string]bool{}
	cited := map[string]bool{} // files cited
	citedIDs := map[string]bool{}
	checkCites := func(where string, cites []string) {
		for _, c := range cites {
			if err := ix.Resolve(c); err != nil {
				add("%s: cite %v", where, err)
				continue
			}
			citedIDs[c] = true
			if f := citeFile(ix, c); f != "" {
				cited[f] = true
			}
		}
	}
	unknowns := 0
	claim := func(level string, c Claim) {
		where := "claim " + c.ID
		switch {
		case !claimID[level].MatchString(c.ID) && (level != "engineering" || !riskID.MatchString(c.ID)):
			add("%s %q: IDs at this level are %s1, %s2, …", level, c.ID, strings.ToUpper(level[:1]), strings.ToUpper(level[:1]))
		case seen[c.ID]:
			add("%s: duplicate ID", where)
		}
		seen[c.ID] = true
		if strings.TrimSpace(c.Text) == "" {
			add("%s: empty claim", where)
		}
		if level == "capability" && strings.TrimSpace(c.Name) == "" {
			add("%s: a capability needs a name", where)
		}
		if level != "capability" && level != "trap" && !slices.Contains(Topics[level], c.Topic) {
			add("%s: topic %q; want one of %s", where, c.Topic, strings.Join(Topics[level], ", "))
		}
		checkCites(where, c.Cites)
		switch c.Basis {
		case BasisObserved, BasisStated, BasisInferred:
			if len(c.Cites) == 0 {
				add("%s: %s needs at least one citation (a fact ID or path:line); with none, mark it unknown and ask", where, c.Basis)
			}
			if c.Basis == BasisInferred && strings.TrimSpace(c.Reasoning) == "" {
				add("%s: inferred needs its reasoning: why the cited code supports the claim", where)
			}
			if c.Basis == BasisObserved && level == "product" {
				add("%s: a product claim cannot be observed in code (who uses it, for what, and why are not in the code): make it stated (a document says so), inferred (with reasoning), confirmed (the person said so), or unknown", where)
			}
		case BasisConfirmed:
			q, ok := questions[c.Answer]
			if !ok || q.Status != "answered" {
				add("%s: confirmed needs answer = the ID of an answered question (got %q)", where, c.Answer)
			}
		case BasisUnknown:
			unknowns++
			if _, ok := questions[c.Question]; !ok {
				add("%s: unknown needs question = the ID of a question that asks it (got %q)", where, c.Question)
			}
		default:
			add("%s: basis %q; want one of %s", where, c.Basis, strings.Join(Bases, ", "))
		}
	}
	if len(d.Engineering) == 0 {
		add("engineering: no claims")
	}
	if len(d.Capabilities) == 0 {
		add("capabilities: no capability claims (the capability map)")
	}
	if len(d.ProductClaims) == 0 {
		add("product_claims: none; a product claim the code cannot support is unknown, with a question")
	}
	for _, c := range d.Engineering {
		claim("engineering", c)
	}
	for _, c := range d.Capabilities {
		claim("capability", c)
	}
	for _, c := range d.ProductClaims {
		claim("product", c)
	}
	checkContract(ix, d, questions, add, checkCites, claim)
	planned := map[string]bool{} // modernisation steps and case sets a question may also affect
	if m := d.Modernisation; m != nil {
		for _, st := range m.Steps {
			planned[st.ID] = true
		}
		for _, g := range m.CaseSets {
			planned[g.ID] = true
		}
	}
	for _, q := range d.Questions {
		for _, a := range q.Affects {
			if !seen[a] && !planned[a] && !contractIDs(d)[a] {
				add("question %s: affects %s, which is not a claim, a modernisation step, a case set, a do-not-carry entry, or an interface", q.ID, a)
			}
		}
	}
	if m := d.Modernisation; m != nil {
		checkModernisation(ix, d, m, questions, seen, add, checkCites, claim)
	}
	cov := coverage(ix, cited, citedIDs)
	for _, q := range d.Questions {
		if q.Status == "open" {
			cov.OpenQuestions++
		}
	}
	cov.Unknowns = unknowns
	if ps == nil {
		ps = []string{}
	}
	return Report{Pass: len(ps) == 0, Problems: ps, Coverage: cov}
}

func checkModernisation(ix *Index, d Draft, m *Modernisation, questions map[string]Question, claims map[string]bool,
	add func(string, ...any), checkCites func(string, []string), claim func(string, Claim)) {
	if strings.TrimSpace(m.Target) == "" || strings.TrimSpace(m.Strategy) == "" {
		add("modernisation: needs a target and a strategy")
	}
	switch q, ok := questions[m.TargetSource]; {
	case m.TargetSource == "request":
		if !strings.Contains(strings.ToLower(d.Request), strings.ToLower(firstWord(m.Target))) {
			add("modernisation: target_source is the request, but the request does not name %q; ask the person (a question) instead", m.Target)
		}
	case !ok || q.Status != "answered":
		add("modernisation: target_source must be \"request\" or an answered question's ID (got %q): the person chooses the target", m.TargetSource)
	}
	checkCites("modernisation", m.Cites)
	for _, r := range m.Risks {
		if !riskID.MatchString(r.ID) {
			add("modernisation risk %q: IDs are R1, R2, …", r.ID)
		}
		r.Topic = "risks"
		claim("engineering", r)
	}
	sets := map[string]bool{}
	for _, g := range m.CaseSets {
		if !gID.MatchString(g.ID) || sets[g.ID] {
			add("case set %q: IDs are G1, G2, …, each once", g.ID)
		}
		sets[g.ID] = true
		if strings.TrimSpace(g.Title) == "" || strings.TrimSpace(g.Compare) == "" {
			add("case set %s: needs a title and how outputs compare (exact, or normalised and how)", g.ID)
		}
		if len(g.From) == 0 {
			add("case set %s: from: cite where its inputs come from (existing tests, entry points)", g.ID)
		}
		checkCites("case set "+g.ID, g.From)
		for _, x := range g.Exclude {
			if !refersTo(x, questions, claims) {
				add("case set %s: exclusion %q must name the question or claim that decided it (Q3, E4)", g.ID, x)
			}
		}
	}
	steps := map[string]Step{}
	for _, s := range m.Steps {
		if !mID.MatchString(s.ID) || steps[s.ID].ID != "" {
			add("step %q: IDs are M1, M2, …, each once", s.ID)
		}
		steps[s.ID] = s
		if strings.TrimSpace(s.Title) == "" {
			add("step %s: needs a title", s.ID)
		}
		for _, mod := range s.Modules {
			if ix.Resolve(mod) != nil || !strings.HasPrefix(mod, "mod:") {
				add("step %s: %q is not a module of the index", s.ID, mod)
			}
		}
		for _, c := range s.Claims {
			if !claims[c] {
				add("step %s: rests on %s, which is not a claim", s.ID, c)
			}
		}
		if len(s.Parity) == 0 {
			add("step %s: name the case sets that prove it keeps the legacy behaviour (parity)", s.ID)
		}
		for _, g := range s.Parity {
			if !sets[g] && !slices.ContainsFunc(m.Suites, func(x Suite) bool { return x.ID == g }) {
				add("step %s: parity %s is not a case set or a suite", s.ID, g)
			}
		}
	}
	if len(m.Steps) == 0 {
		add("modernisation: no steps")
	}
	for _, s := range m.Steps {
		for _, dep := range s.DependsOn {
			if _, ok := steps[dep]; !ok {
				add("step %s: depends on %s, which is not a step", s.ID, dep)
			}
		}
	}
	if cyc := cycle(m.Steps); cyc != "" {
		add("modernisation: steps depend on each other in a cycle: %s", cyc)
	}
}

func refersTo(s string, questions map[string]Question, claims map[string]bool) bool {
	for _, w := range regexp.MustCompile(`\b[QECPRTNK][1-9][0-9]*\b`).FindAllString(s, -1) {
		if _, ok := questions[w]; ok || claims[w] {
			return true
		}
	}
	return false
}

func cycle(steps []Step) string {
	deps := map[string][]string{}
	for _, s := range steps {
		deps[s.ID] = s.DependsOn
	}
	state := map[string]int{}
	var path []string
	var visit func(string) string
	visit = func(n string) string {
		switch state[n] {
		case 1:
			return strings.Join(append(path, n), " → ")
		case 2:
			return ""
		}
		state[n] = 1
		path = append(path, n)
		for _, d := range deps[n] {
			if c := visit(d); c != "" {
				return c
			}
		}
		path = path[:len(path)-1]
		state[n] = 2
		return ""
	}
	for _, s := range steps {
		if c := visit(s.ID); c != "" {
			return c
		}
	}
	return ""
}

func firstWord(s string) string {
	if f := strings.Fields(s); len(f) > 0 {
		return strings.Trim(f[0], ".,;:()")
	}
	return s
}

func short(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

// citeFile is the checkout file a resolved citation points into, or "".
func citeFile(ix *Index, c string) string {
	if f := fileOf(c); f != "" {
		return f
	}
	if strings.HasPrefix(c, "mod:") || strings.HasPrefix(c, "build:") || strings.HasPrefix(c, "dep:") || strings.HasPrefix(c, "probe:") {
		return ""
	}
	for _, pre := range []string{"entry:", "io:", "trap:", "marker:", "test:", "doc:"} {
		if strings.HasPrefix(c, pre) {
			c = c[len(pre):]
			if pre == "io:" || pre == "trap:" {
				if _, rest, ok := strings.Cut(c, ":"); ok {
					c = rest
				}
			}
			break
		}
	}
	if ix.HasFile(c) {
		return c
	}
	if p, _ := splitAt(c); ix.HasFile(p) {
		return p
	}
	if j := strings.LastIndex(c, ":"); j >= 0 && ix.HasFile(c[:j]) {
		return c[:j]
	}
	return ""
}

func coverage(ix *Index, files, ids map[string]bool) Coverage {
	var cov Coverage
	for _, m := range ix.Modules {
		cov.Modules++
		hit := ids[m.ID]
		for _, f := range m.Files {
			hit = hit || files[f]
		}
		if hit {
			cov.ModulesCited++
		} else {
			cov.UncitedModule = append(cov.UncitedModule, m.ID)
		}
	}
	for _, e := range ix.Entries {
		cov.Entries++
		if ids[e.ID] || ids[e.At] || citesSymbolAt(ix, ids, e.At) {
			cov.EntriesCited++
		}
	}
	for _, mk := range ix.Markers {
		cov.Markers++
		if ids[mk.ID] || ids[mk.At] {
			cov.MarkersCited++
		} else {
			cov.UncitedMarker = append(cov.UncitedMarker, mk.ID)
		}
	}
	for _, t := range ix.Traps {
		cov.Traps++
		if ids[t.ID] {
			cov.TrapsChecked++
		} else {
			cov.UncheckedTrap = append(cov.UncheckedTrap, t.ID)
		}
	}
	sort.Strings(cov.UncitedModule)
	return cov
}

// citesSymbolAt reports whether the draft cites the symbol defined at a location.
func citesSymbolAt(ix *Index, ids map[string]bool, at string) bool {
	for _, s := range ix.Symbols {
		if s.At == at && ids[s.ID] {
			return true
		}
	}
	return false
}

// contractIDs are the do-not-carry entries and interfaces of a draft.
func contractIDs(d Draft) map[string]bool {
	out := map[string]bool{}
	for _, n := range d.DoNotCarry {
		out[n.ID] = true
	}
	for _, k := range d.Compatibility {
		out[k.ID] = true
	}
	return out
}
