package survey

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// Backlog is the backlog the host's model drafts from a survey: one Goal,
// Initiatives, Features, Stories, and Bugs, each resting on survey claims.
type Backlog struct {
	Product       string   `json:"product"`
	Items         []Item   `json:"items"`
	OpenQuestions []string `json:"open_questions,omitempty"`
}

// Item is one work item of the backlog, keyed so a re-run finds it again.
type Item struct {
	Key       string            `json:"key"` // lower-case slug, unique in the backlog
	Type      string            `json:"type"`
	Parent    string            `json:"parent,omitempty"` // the parent item's key
	Title     string            `json:"title"`
	Fields    map[string]string `json:"fields"`
	Labels    []string          `json:"labels,omitempty"`
	Basis     []string          `json:"basis"`                // survey claim, question, step, or case set IDs, or index citations
	Step      string            `json:"step,omitempty"`       // a modernisation step (M1) the item delivers
	Parity    []string          `json:"parity,omitempty"`     // case sets (G1) or suites (S1) its Completion Goal proves
	DependsOn []string          `json:"depends_on,omitempty"` // keys of items that must merge first
	// Contract names the compatibility interfaces (K1) the item keeps; an
	// exact one is closed only by the evidence the contract names.
	Contract []string `json:"contract,omitempty"`
	// Avoids names the do-not-carry entries (N1) in the code the item ports.
	Avoids []string `json:"avoids,omitempty"`
	// Constrains (a Decision only) lists the keys of the items it constrains.
	Constrains []string `json:"constrains,omitempty"`
}

// BacklogTypes are the work-item types a survey backlog files.
var BacklogTypes = []string{"goal", "initiative", "feature", "story", "bug", "decision"}

// stageFor is the stage each type must pass before it is filed: a Story or
// Bug is executable (Ready, with its Completion Goal), a Feature has its
// problem and scope, and a Goal or Initiative is created with its outcome,
// since its baseline comes from `measure` later, never from a guess.
func stageFor(typ string) workitems.Stage {
	switch typ {
	case "story", "bug", "feature", "decision":
		return workitems.StageReady
	}
	return workitems.StageCreate
}

// ItemReport is one item's check.
type ItemReport struct {
	Key      string   `json:"key"`
	Type     string   `json:"type"`
	Title    string   `json:"title"`
	Problems []string `json:"problems"`
}

// BacklogReport is the check of a backlog draft.
type BacklogReport struct {
	Pass     bool           `json:"pass"`
	Items    []ItemReport   `json:"items"`
	Problems []string       `json:"problems"`
	Order    []string       `json:"order"` // filing order: parents and dependencies first
	Counts   map[string]int `json:"counts"`
	Digest   string         `json:"digest"` // of the rendered bodies, titles, labels, and links: what the person reviews
}

var keyRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

// ParseBacklog decodes a backlog draft strictly.
func ParseBacklog(b []byte) (Backlog, error) {
	var bl Backlog
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	err := dec.Decode(&bl)
	return bl, err
}

// CheckBacklog validates a backlog against the work-item schema, the survey
// draft, and the index: every item rests on something the survey knows (an
// unknown never backs an item), the tree is legal, and a modernisation Story
// proves parity with the case sets its step names.
func CheckBacklog(ix *Index, d Draft, bl Backlog) BacklogReport {
	r := BacklogReport{Counts: map[string]int{}, Problems: []string{}}
	add := func(format string, args ...any) { r.Problems = append(r.Problems, fmt.Sprintf(format, args...)) }
	claims := d.Claims()
	questions := map[string]Question{}
	for _, q := range d.Questions {
		questions[q.ID] = q
	}
	steps, sets := map[string]Step{}, map[string]bool{}
	if m := d.Modernisation; m != nil {
		for _, s := range m.Steps {
			steps[s.ID] = s
		}
		for _, g := range m.CaseSets {
			sets[g.ID] = true
		}
		for _, su := range m.Suites {
			sets[su.ID] = true
		}
	}
	carries, ifaces := map[string]Carry{}, map[string]Interface{}
	for _, n := range d.DoNotCarry {
		carries[n.ID] = n
	}
	for _, k := range d.Compatibility {
		ifaces[k.ID] = k
	}
	decisions := map[string]string{} // do-not-carry ID → the key of its Decision
	for _, it := range bl.Items {
		if it.Type == "decision" {
			for _, ref := range it.Basis {
				if _, ok := carries[ref]; ok {
					decisions[ref] = it.Key
				}
			}
		}
	}
	closed := map[string]bool{} // exact interfaces a Story or Bug keeps
	byKey := map[string]Item{}
	for _, it := range bl.Items {
		if _, dup := byKey[it.Key]; dup {
			add("item %s: duplicate key", it.Key)
		}
		byKey[it.Key] = it
	}
	goals := 0
	for _, it := range bl.Items {
		ir := ItemReport{Key: it.Key, Type: it.Type, Title: it.Title, Problems: []string{}}
		bad := func(format string, args ...any) { ir.Problems = append(ir.Problems, fmt.Sprintf(format, args...)) }
		if !keyRE.MatchString(it.Key) {
			bad("key %q: lower-case letters, digits, and dashes", it.Key)
		}
		if !slices.Contains(BacklogTypes, it.Type) {
			bad("type %q: a survey backlog files %s", it.Type, strings.Join(BacklogTypes, ", "))
			r.Items = append(r.Items, ir)
			continue
		}
		r.Counts[it.Type]++
		if it.Type == "goal" {
			goals++
		}
		t, _ := workitems.FindType(it.Type)
		wi := workitems.Item{Type: it.Type, Title: it.Title, Fields: it.Fields, Labels: it.Labels}
		switch p, ok := byKey[it.Parent]; {
		case it.Parent == "":
		case !ok:
			bad("parent %q is not an item of this backlog", it.Parent)
		default:
			wi.Parent = &workitems.Ref{Number: 1, Type: p.Type}
		}
		if it.Parent == "" && len(t.Parents) > 0 && it.Type != "story" && it.Type != "bug" {
			bad("a %s needs a parent %s", t.Name, strings.Join(t.Parents, " or "))
		}
		if it.Type == "story" || it.Type == "bug" {
			if it.Parent == "" {
				bad("file it under the Feature it delivers (parent)")
			}
		}
		for _, p := range workitems.Validate(wi, stageFor(it.Type)) {
			if p.Field == "parent" {
				if wi.Parent != nil { // the placeholder number would mislead: name the key
					bad("parent %s is a %s; a %s's parent must be a %s", it.Parent, wi.Parent.Type, t.Name, strings.Join(t.Parents, " or "))
				}
				continue
			}
			bad("%s", p.String())
		}
		// Basis: what the survey knows, never an unknown.
		var levels []string
		if len(it.Basis) == 0 {
			bad("basis: cite the survey claims (E…, C…, P…), answered questions, or index facts it rests on")
		}
		marker := false
		for _, ref := range it.Basis {
			switch c, isClaim := claims[ref]; {
			case isClaim && c.Basis == BasisUnknown:
				bad("basis %s is unknown in the survey (%s asks it): an unknown cannot back an item; ask, record the answer, and cite the confirmed claim", ref, c.Question)
			case isClaim:
				levels = append(levels, levelOf(d, ref)+":"+c.Basis+":"+c.Topic)
			case questions[ref].ID != "":
				if questions[ref].Status != "answered" {
					bad("basis %s is a question that is not answered yet", ref)
				} else {
					levels = append(levels, "answer")
				}
			case steps[ref].ID != "" || sets[ref]:
				levels = append(levels, "modernisation")
			case carries[ref].ID != "":
				levels = append(levels, "carry:"+carries[ref].Status)
			case ifaces[ref].ID != "":
				levels = append(levels, "interface")
			default:
				if err := ix.Resolve(ref); err != nil {
					bad("basis %v", err)
				} else if strings.HasPrefix(ref, "marker:") {
					marker = true
				}
			}
		}
		has := func(pred func(string) bool) bool { return slices.ContainsFunc(levels, pred) }
		switch it.Type {
		case "goal":
			if !has(func(l string) bool {
				return l == "answer" || strings.HasPrefix(l, "product:stated") || strings.HasPrefix(l, "product:confirmed")
			}) {
				bad("basis: a Goal rests on a product claim the person confirmed or a document states (code alone never says what the product is for)")
			}
		case "feature", "story":
			if !has(func(l string) bool {
				return strings.HasPrefix(l, "engineering:") || strings.HasPrefix(l, "capability:")
			}) {
				bad("basis: a %s rests on at least one engineering or capability claim", t.Name)
			}
		case "bug":
			if !marker && !has(func(l string) bool { return strings.HasSuffix(l, ":risks") && !strings.Contains(l, ":inferred:") }) {
				bad("basis: a Bug rests on a known-issue marker (marker:…) or an observed or stated risk claim")
			}
		case "decision":
			// The one item a pending answer may back: a Decision is how the
			// person decides a do-not-carry entry, and it constrains the items
			// that port its code until it is accepted.
			if !has(func(l string) bool { return l == "carry:proposed" || l == "carry:confirmed" }) {
				bad("basis: a survey Decision decides a do-not-carry entry (N…) that is proposed or confirmed")
			}
			if len(it.Constrains) == 0 {
				bad("constrains: list the items whose code holds what this Decision keeps out")
			}
		}
		if it.Type != "decision" && len(it.Constrains) > 0 {
			bad("constrains: only a Decision constrains other items")
		}
		for _, c := range it.Constrains {
			if p, ok := byKey[c]; !ok || !slices.Contains([]string{"goal", "initiative", "feature", "story", "bug"}, p.Type) {
				bad("constrains %q is not a Goal, Initiative, Feature, Story, or Bug of this backlog", c)
			}
		}
		// What the item's step keeps out, and the contract it keeps.
		if st, ok := steps[it.Step]; ok {
			for _, n := range st.Avoids {
				if carries[n].Status != "rejected" && !slices.Contains(it.Avoids, n) {
					bad("avoids: step %s does not carry over %s; list it so the Story is constrained by its Decision", it.Step, n)
				}
			}
		}
		for _, n := range it.Avoids {
			c, ok := carries[n]
			switch {
			case !ok:
				bad("avoids %s is not on the survey's do-not-carry list", n)
			case c.Status == "rejected":
				bad("avoids %s, which the person decided to keep (%s)", n, c.Question)
			case decisions[n] == "":
				bad("avoids %s, but no Decision item decides it: add one with basis %s that constrains this item", n, n)
			case !slices.Contains(byKey[decisions[n]].Constrains, it.Key):
				bad("avoids %s: its Decision %s must constrain this item", n, decisions[n])
			}
		}
		for _, kid := range it.Contract {
			k, ok := ifaces[kid]
			switch {
			case !ok:
				bad("contract %s is not an interface of the survey's compatibility contract", kid)
				continue
			case k.Level == "undecided":
				bad("contract %s: its level is undecided (%s asks); no item can claim to keep it until the person decides", kid, k.DecidedBy)
				continue
			case k.Level != "exact":
				continue
			}
			// Must match: only the contract's own evidence closes it.
			for _, p := range k.Proof {
				if strings.HasPrefix(p, "check:") {
					if !strings.Contains(it.Fields[workitems.FieldCriteria], p) {
						bad("contract %s must match exactly: the Completion Goal must replay %s", kid, p)
					}
				} else if !slices.Contains(it.Parity, p) {
					bad("contract %s must match exactly: parity must include %s, the evidence the contract names", kid, p)
				}
			}
			if it.Type == "story" || it.Type == "bug" {
				closed[kid] = true
			}
		}
		// A modernisation item proves parity in its Completion Goal.
		if it.Step != "" {
			s, ok := steps[it.Step]
			if !ok {
				bad("step %s is not a step of the survey's modernisation path", it.Step)
			}
			if (it.Type == "story" || it.Type == "bug") && len(it.Parity) == 0 {
				bad("parity: a modernisation %s names the case sets its Completion Goal proves (step %s names %s)", t.Name, it.Step, strings.Join(s.Parity, ", "))
			}
		}
		for _, g := range it.Parity {
			if !sets[g] {
				bad("parity %s is not a case set of the survey", g)
			} else if !strings.Contains(it.Fields[workitems.FieldCriteria], g) {
				bad("parity: the Completion Goal must name case set %s in the criterion that proves it", g)
			}
		}
		for _, dep := range it.DependsOn {
			if p, ok := byKey[dep]; !ok {
				bad("depends_on %q is not an item of this backlog", dep)
			} else if !slices.Contains([]string{"feature", "story", "bug"}, p.Type) || !slices.Contains(t.Links, "depends-on") {
				bad("depends_on %s: only a Feature, Story, or Bug depends on another Feature, Story, or Bug", dep)
			}
		}
		r.Items = append(r.Items, ir)
	}
	if goals > 1 {
		add("a survey backlog has one Goal; it has %d", goals)
	}
	ported := slices.ContainsFunc(bl.Items, func(it Item) bool { return it.Step != "" })
	for _, k := range d.Compatibility {
		if ported && k.Level == "exact" && !closed[k.ID] {
			add("interface %s must match exactly, but no Story or Bug keeps it (contract): the modernisation would close without proving it", k.ID)
		}
	}
	if len(bl.Items) == 0 {
		add("no items")
	}
	order, cyc := fileOrder(bl.Items, byKey)
	if cyc != "" {
		add("parents and dependencies form a cycle: %s", cyc)
	}
	r.Order = order
	r.Pass = len(r.Problems) == 0
	for _, ir := range r.Items {
		if len(ir.Problems) > 0 {
			r.Pass = false
		}
	}
	r.Digest = Digest(bl, order)
	return r
}

func levelOf(d Draft, id string) string {
	for level, l := range map[string][]Claim{"engineering": d.Engineering, "capability": d.Capabilities, "product": d.ProductClaims} {
		for _, c := range l {
			if c.ID == id {
				return level
			}
		}
	}
	return "engineering" // a modernisation risk
}

// fileOrder lists the keys parents first, then dependencies first, keeping
// the draft's order otherwise.
func fileOrder(items []Item, byKey map[string]Item) ([]string, string) {
	state := map[string]int{}
	var order, path []string
	var visit func(string) string
	visit = func(k string) string {
		switch state[k] {
		case 1:
			return strings.Join(append(path, k), " → ")
		case 2:
			return ""
		}
		it, ok := byKey[k]
		if !ok {
			return ""
		}
		state[k] = 1
		path = append(path, k)
		for _, dep := range slices.Concat([]string{it.Parent}, it.DependsOn, it.Constrains) {
			if dep == "" {
				continue
			}
			if c := visit(dep); c != "" {
				return c
			}
		}
		path = path[:len(path)-1]
		state[k] = 2
		order = append(order, k)
		return ""
	}
	for _, it := range items {
		if c := visit(it.Key); c != "" {
			return order, c
		}
	}
	return order, ""
}

// KeyMarker is the hidden line at the top of a filed issue's body that names its
// survey key; the parser ignores text before the first section.
func KeyMarker(product, key string) string {
	return fmt.Sprintf("<!-- aayudha:survey product=%s key=%s -->", slug(product), key)
}

var markerRE = regexp.MustCompile(`<!-- aayudha:survey product=(\S+) key=(\S+) -->`)

// ParseMarker reads a filed issue's survey key, if it has one for product.
func ParseMarker(product, body string) (string, bool) {
	m := markerRE.FindStringSubmatch(body)
	if m == nil || m[1] != slug(product) {
		return "", false
	}
	return m[2], true
}

// Label is the label every filed item of a product's survey carries, so a
// re-run can list what is already filed and a person can filter on it.
func Label(product string) string { return "survey:" + slug(product) }

func slug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// Body is the issue body an item is filed with: the survey marker, the basis
// it rests on (for the reader; the parser skips it), then the sections.
// Links (depends-on) are added at filing time with the issue numbers.
func Body(product string, it Item, links string) string {
	t, _ := workitems.FindType(it.Type)
	fields := map[string]string{}
	for k, v := range it.Fields {
		fields[k] = v
	}
	if links != "" {
		fields[workitems.FieldLinks] = links
	}
	head := KeyMarker(product, it.Key) + "\n\n> From the survey of " + product + ": rests on " + strings.Join(it.Basis, ", ")
	if len(it.Parity) > 0 {
		head += "; proves parity with " + strings.Join(it.Parity, ", ")
	}
	if len(it.Contract) > 0 {
		head += "; keeps compatibility interfaces " + strings.Join(it.Contract, ", ")
	}
	if len(it.Avoids) > 0 {
		head += "; does not carry over " + strings.Join(it.Avoids, ", ")
	}
	return head + ".\n\n" + workitems.Render(t, fields)
}

// Digest identifies the exact backlog the person reviewed: every item's key,
// type, parent, links, title, labels, and body, in filing order.
func Digest(bl Backlog, order []string) string {
	byKey := map[string]Item{}
	for _, it := range bl.Items {
		byKey[it.Key] = it
	}
	h := sha256.New()
	for _, k := range order {
		it := byKey[k]
		fmt.Fprintf(h, "%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00", it.Key, it.Type, it.Parent, strings.Join(it.DependsOn, ","), it.Title, strings.Join(it.Labels, ","), Body(bl.Product, it, ""))
		if len(it.Constrains) > 0 { // added only when present, so a digest without one is unchanged
			fmt.Fprintf(h, "constrains\x00%s\x00", strings.Join(it.Constrains, ","))
		}
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// RenderBacklog writes backlog.md: the review copy of exactly what will be
// filed, in filing order, with each item's problems while it has any.
func RenderBacklog(bl Backlog, r BacklogReport, draftFile, backlogFile string) string {
	var b strings.Builder
	p := func(format string, args ...any) { fmt.Fprintf(&b, format, args...) }
	p("# Backlog draft: %s\n\n", oneLine(bl.Product))
	var counts []string
	for _, t := range BacklogTypes {
		if n := r.Counts[t]; n > 0 {
			counts = append(counts, fmt.Sprintf("%d %s", n, t))
		}
	}
	p("Nothing here is filed yet. Review every item below: it is the exact issue that will be filed (%s).\n\n", strings.Join(counts, ", "))
	if r.Pass {
		p("Review digest `%s`. To file it after review:\n\n```sh\naayu survey --backlog %s --draft %s --create --reviewed %s\n```\n\n", r.Digest, backlogFile, draftFile, r.Digest)
	} else {
		p("This draft does not pass the check yet; it cannot be filed.\n\n")
		for _, x := range r.Problems {
			p("- %s\n", oneLine(x))
		}
		p("\n")
	}
	if len(bl.OpenQuestions) > 0 {
		p("## Open questions\n\n")
		for _, q := range bl.OpenQuestions {
			p("- %s\n", oneLine(q))
		}
		p("\n")
	}
	byKey := map[string]Item{}
	for _, it := range bl.Items {
		byKey[it.Key] = it
	}
	reports := map[string]ItemReport{}
	for _, ir := range r.Items {
		reports[ir.Key] = ir
	}
	order := r.Order
	if len(order) == 0 {
		for _, it := range bl.Items {
			order = append(order, it.Key)
		}
	}
	for k, key := range order {
		it := byKey[key]
		name := it.Type
		if t, ok := workitems.FindType(it.Type); ok {
			name = t.Name
		}
		p("## %d. %s: %s\n\n", k+1, name, oneLine(it.Title))
		meta := []string{"key `" + it.Key + "`"}
		if it.Parent != "" {
			meta = append(meta, "under `"+it.Parent+"`")
		}
		if len(it.DependsOn) > 0 {
			meta = append(meta, "after "+citeText(it.DependsOn))
		}
		if len(it.Constrains) > 0 {
			meta = append(meta, "constrains "+citeText(it.Constrains))
		}
		if len(it.Labels) > 0 {
			meta = append(meta, "labels "+citeText(it.Labels))
		}
		p("%s.\n\n", strings.Join(meta, ", "))
		for _, x := range reports[key].Problems {
			p("- **Problem:** %s\n", oneLine(x))
		}
		if len(reports[key].Problems) > 0 {
			p("\n")
		}
		body := Body(bl.Product, it, "")
		p("````markdown\n%s````\n\n", body)
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}
