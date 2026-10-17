package survey

import (
	"regexp"
	"slices"
	"strings"
)

// Trap is a trap finding: one rule's question (traps.md) asked of the code,
// with what was found and how it is known, like a claim.
type Trap struct {
	Claim
	Rule  string `json:"rule"`  // a trap rule (TrapRules), or "stack" for a per-stack question no rule matches
	Check string `json:"check"` // the question asked of the code
}

// Carry is an entry of the do-not-carry list: a technical feature of the
// legacy code the modern version should deliberately not reproduce. The person
// decides each in the interview; a confirmed one becomes a Decision the
// modernisation Stories that touch its code reference.
type Carry struct {
	ID          string   `json:"id"` // N1
	Feature     string   `json:"feature"`
	Cites       []string `json:"cites"`
	Traps       []string `json:"traps,omitempty"` // trap findings (T1) that found it
	Risk        string   `json:"risk"`            // what copying it would cost
	Replacement string   `json:"replacement"`
	Question    string   `json:"question"` // the interview question that decides it
	Status      string   `json:"status"`   // proposed (the question is open), confirmed, or rejected (the person keeps it)
}

// Interface is an entry of the compatibility contract: something others
// depend on, at the level the person decides.
type Interface struct {
	ID          string   `json:"id"` // K1
	Kind        string   `json:"kind"`
	Name        string   `json:"name"`
	Cites       []string `json:"cites"`
	Consumers   string   `json:"consumers,omitempty"` // who depends on it, as far as known
	Level       string   `json:"level"`               // exact, differ, drop, or undecided
	DecidedBy   string   `json:"decided_by"`          // "request" or an answered question; while undecided, the open question that will decide
	Quote       string   `json:"quote,omitempty"`     // with decided_by "request": the request's own words that decide the level
	Difference  string   `json:"difference,omitempty"`
	Deprecation string   `json:"deprecation,omitempty"`
	Proof       []string `json:"proof,omitempty"` // case sets (G1), suites (S1), or behaviour checks replaying a user journey (check:<product>/<feature>.<check>, a verify feature map)
}

// Suite is a legacy test suite reused as the compatibility reference: run
// against the new implementation, it scores pass counts per area.
type Suite struct {
	ID        string   `json:"id"` // S1
	Title     string   `json:"title"`
	From      []string `json:"from"`
	Cases     string   `json:"cases,omitempty"` // the cases file `aayu survey --compat` runs, once written
	Areas     []string `json:"areas"`
	Unfixable []string `json:"unfixable,omitempty"` // "test: reason": tests that can never pass for a structural reason, left out of the score
}

// Migration moves users' data or configuration to the new version, with a
// tested way back.
type Migration struct {
	ID       string   `json:"id"` // MG1
	What     string   `json:"what"`
	Cites    []string `json:"cites"`
	Rollback string   `json:"rollback"`
	Proof    []string `json:"proof"` // case sets or suites that prove the migration and its rollback
}

// Coexistence is how the old and new versions run together before cut-over.
type Coexistence struct {
	Mode     string   `json:"mode"` // shadow, side-by-side, or switch
	How      string   `json:"how"`
	Compare  []string `json:"compare"` // case sets or suites run on both, differences reported before cut-over
	CutOver  string   `json:"cut_over"`
	Rollback string   `json:"rollback"`
}

// Shim keeps a behaviour only for old users, with a plan to remove it.
type Shim struct {
	ID        string `json:"id"` // SH1
	Interface string `json:"interface"`
	Behaviour string `json:"behaviour"`
	Mechanism string `json:"mechanism"` // the flag, variable, or mode that turns it on
	Removal   string `json:"removal"`
}

// InterfaceKinds are the kinds of interface a compatibility contract names.
// The ui- kinds and os-integration are a desktop application's user-visible
// behaviour, proved by user journeys rather than stdout.
var InterfaceKinds = []string{"cli", "exit-code", "stdout", "stderr", "env", "locale", "file-format", "config", "plugin",
	"protocol", "api", "data-store", "ui-form", "ui-shortcut", "ui-print", "ui-clipboard", "ui-file-association", "os-integration", "com"}

// ContractLevels are the levels the person decides per interface.
var ContractLevels = []string{"exact", "differ", "drop", "undecided"}

var (
	tID       = regexp.MustCompile(`^T[1-9][0-9]*$`)
	nID       = regexp.MustCompile(`^N[1-9][0-9]*$`)
	kID       = regexp.MustCompile(`^K[1-9][0-9]*$`)
	sID       = regexp.MustCompile(`^S[1-9][0-9]*$`)
	mgID      = regexp.MustCompile(`^MG[1-9][0-9]*$`)
	shID      = regexp.MustCompile(`^SH[1-9][0-9]*$`)
	journeyRE = regexp.MustCompile(`^check:[a-z0-9][a-z0-9-]*/[a-z0-9][a-z0-9-]*\.[a-z0-9][a-z0-9-]*$`)
)

func uiKind(k string) bool { return strings.HasPrefix(k, "ui-") || k == "os-integration" }

// exactCompare reports whether a case set compares outputs byte for byte.
func exactCompare(compare string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(compare)), "exact")
}

// normalSpace lower-cases s and collapses its white space, so a quote matches
// its request across line breaks.
func normalSpace(s string) string { return strings.Join(strings.Fields(strings.ToLower(s)), " ") }

// checkContract checks the trap findings, the do-not-carry list, the
// compatibility contract, and the modernisation's suites, migrations,
// coexistence, and shims.
func checkContract(ix *Index, d Draft, questions map[string]Question, add func(string, ...any), checkCites func(string, []string), claim func(string, Claim)) {
	rules := TrapRules()
	traps := map[string]bool{}
	for _, t := range d.Traps {
		claim("trap", t.Claim)
		traps[t.ID] = true
		if t.Rule != "stack" && !slices.Contains(rules, t.Rule) {
			add("trap %s: rule %q is not a trap rule (references/traps.md) or \"stack\"", t.ID, t.Rule)
		}
		if strings.TrimSpace(t.Check) == "" {
			add("trap %s: check: write the question you asked of the code", t.ID)
		}
	}
	m := d.Modernisation
	sets, suites := map[string]CaseSet{}, map[string]bool{}
	if m != nil {
		for _, g := range m.CaseSets {
			sets[g.ID] = g
		}
		for _, s := range m.Suites {
			suites[s.ID] = true
		}
	}
	decided := func(where, q string, wantOpen bool) {
		switch qq, ok := questions[q]; {
		case !ok:
			add("%s: name the question that decides it (got %q)", where, q)
		case wantOpen && qq.Status == "answered":
			add("%s: %s is answered; record what the person decided", where, q)
		case !wantOpen && qq.Status != "answered":
			add("%s: %s is not answered: only the person decides this; leave it open until they do", where, q)
		}
	}
	for _, n := range d.DoNotCarry {
		where := "do-not-carry " + n.ID
		if !nID.MatchString(n.ID) {
			add("do-not-carry %q: IDs are N1, N2, …", n.ID)
		}
		if strings.TrimSpace(n.Feature) == "" || strings.TrimSpace(n.Risk) == "" || strings.TrimSpace(n.Replacement) == "" {
			add("%s: needs the feature, the risk of copying it, and its replacement", where)
		}
		if len(n.Cites) == 0 {
			add("%s: cite the code that shows it", where)
		}
		checkCites(where, n.Cites)
		for _, t := range n.Traps {
			if !traps[t] {
				add("%s: trap %s is not a trap finding", where, t)
			}
		}
		switch n.Status {
		case "proposed":
			decided(where, n.Question, true)
		case "confirmed", "rejected":
			decided(where, n.Question, false)
		default:
			add("%s: status %q; want proposed (its question is open), confirmed, or rejected", where, n.Status)
		}
	}
	for _, k := range d.Compatibility {
		where := "interface " + k.ID
		if !kID.MatchString(k.ID) {
			add("interface %q: IDs are K1, K2, …", k.ID)
		}
		if !slices.Contains(InterfaceKinds, k.Kind) {
			add("%s: kind %q; want one of %s", where, k.Kind, strings.Join(InterfaceKinds, ", "))
		}
		if strings.TrimSpace(k.Name) == "" || len(k.Cites) == 0 {
			add("%s: needs a name and a citation of where the code provides it", where)
		}
		checkCites(where, k.Cites)
		switch k.Level {
		case "undecided":
			decided(where, k.DecidedBy, true)
		case "exact", "differ", "drop":
			if k.DecidedBy != "request" {
				decided(where, k.DecidedBy, false)
			} else if q := normalSpace(k.Quote); q == "" || !strings.Contains(normalSpace(d.Request), q) {
				add("%s: decided by the request: quote the request's words that decide its level (quote), exactly as they appear in the request", where)
			}
		default:
			add("%s: level %q; want exact, differ, drop, or undecided", where, k.Level)
		}
		if k.Level == "differ" && strings.TrimSpace(k.Difference) == "" {
			add("%s: may differ, but difference does not say how and why", where)
		}
		if k.Level == "drop" && strings.TrimSpace(k.Deprecation) == "" {
			add("%s: dropped, but deprecation does not give the path (warning, release, removal) for its users", where)
		}
		journeys := 0
		for _, p := range k.Proof {
			switch g, isSet := sets[p]; {
			case isSet:
				if k.Level == "exact" && !exactCompare(g.Compare) {
					add("%s must match exactly, but case set %s compares %q: an exact interface needs exact evidence", where, p, g.Compare)
				}
				if k.Level == "exact" && strings.TrimSpace(g.Reference) == "" {
					add("%s must match exactly, but case set %s names no pinned reference build: give the commit, the toolchain and its version, and the build flags its goldens come from (reference), since another build can print other results", where, p)
				}
			case suites[p]:
			case journeyRE.MatchString(p):
				journeys++
			default:
				add("%s: proof %q is not a case set, a suite, or a feature-map check (check:<product>/<feature>.<check>)", where, p)
			}
		}
		if k.Level == "exact" && len(k.Proof) == 0 {
			add("%s must match exactly: name the case sets, suites, or journeys that prove it", where)
		}
		if k.Level == "exact" && uiKind(k.Kind) && journeys == 0 {
			add("%s is user-visible (%s): prove it with a user journey replayed on both versions (a feature-map check, check:…), not only stdout", where, k.Kind)
		}
	}
	if m == nil {
		return
	}
	if len(d.Compatibility) == 0 {
		add("modernisation: list the compatibility contract (compatibility): every interface others depend on, each with the level the person decides")
	}
	carried := map[string]Carry{}
	for _, n := range d.DoNotCarry {
		carried[n.ID] = n
	}
	modOf := map[string]string{}
	for _, md := range ix.Modules {
		for _, f := range md.Files {
			modOf[f] = md.ID
		}
	}
	for _, st := range m.Steps {
		for _, a := range st.Avoids {
			if _, ok := carried[a]; !ok {
				add("step %s: avoids %s, which is not on the do-not-carry list", st.ID, a)
			}
		}
		for _, n := range d.DoNotCarry {
			if n.Status == "rejected" || slices.Contains(st.Avoids, n.ID) {
				continue
			}
			for _, c := range n.Cites {
				if mod := modOf[citeFile(ix, c)]; mod != "" && slices.Contains(st.Modules, mod) {
					add("step %s replaces %s, where do-not-carry %s is (%s): list it in avoids so the port does not copy it", st.ID, mod, n.ID, c)
					break
				}
			}
		}
	}
	for _, s := range m.Suites {
		where := "suite " + s.ID
		if !sID.MatchString(s.ID) {
			add("suite %q: IDs are S1, S2, …", s.ID)
		}
		if strings.TrimSpace(s.Title) == "" || len(s.From) == 0 || len(s.Areas) == 0 {
			add("%s: needs a title, where the legacy tests are (from), and the areas its score is counted in", where)
		}
		checkCites(where, s.From)
		for _, u := range s.Unfixable {
			if _, reason, ok := strings.Cut(u, ":"); !ok || strings.TrimSpace(reason) == "" {
				add("%s: unfixable %q needs its reason (\"test: reason\")", where, u)
			}
		}
	}
	proofs := func(where string, ps []string) {
		if len(ps) == 0 {
			add("%s: name the case sets or suites that prove it", where)
		}
		for _, p := range ps {
			if _, ok := sets[p]; !ok && !suites[p] {
				add("%s: %s is not a case set or a suite", where, p)
			}
		}
	}
	for _, mg := range m.Migrations {
		where := "migration " + mg.ID
		if !mgID.MatchString(mg.ID) {
			add("migration %q: IDs are MG1, MG2, …", mg.ID)
		}
		if strings.TrimSpace(mg.What) == "" || len(mg.Cites) == 0 {
			add("%s: say what moves and cite where the legacy code reads or writes it", where)
		}
		if strings.TrimSpace(mg.Rollback) == "" {
			add("%s: give the tested way back (rollback)", where)
		}
		checkCites(where, mg.Cites)
		proofs(where, mg.Proof)
	}
	if c := m.Coexistence; c != nil {
		if !slices.Contains([]string{"shadow", "side-by-side", "switch"}, c.Mode) {
			add("coexistence: mode %q; want shadow, side-by-side, or switch", c.Mode)
		}
		if strings.TrimSpace(c.How) == "" || strings.TrimSpace(c.CutOver) == "" || strings.TrimSpace(c.Rollback) == "" {
			add("coexistence: needs how both run, the cut-over criteria, and the rollback")
		}
		proofs("coexistence", c.Compare)
	}
	ks := map[string]Interface{}
	for _, k := range d.Compatibility {
		ks[k.ID] = k
	}
	for _, sh := range m.Shims {
		where := "shim " + sh.ID
		if !shID.MatchString(sh.ID) {
			add("shim %q: IDs are SH1, SH2, …", sh.ID)
		}
		if _, ok := ks[sh.Interface]; !ok {
			add("%s: interface %q is not in the compatibility contract", where, sh.Interface)
		}
		if strings.TrimSpace(sh.Behaviour) == "" || strings.TrimSpace(sh.Mechanism) == "" || strings.TrimSpace(sh.Removal) == "" {
			add("%s: needs the behaviour kept, how it is turned on, and the plan to remove it", where)
		}
	}
}
