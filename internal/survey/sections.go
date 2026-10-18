package survey

import (
	"bytes"
	"cmp"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Sections are the parts a survey is written in, in order: each is its own
// model call with its own output cap, checked and saved as soon as it
// returns, so a truncated or failed section is redone alone and an error
// never loses the sections before it. Keys are the draft's top-level keys
// each section owns.
var Sections = []struct {
	Name string
	Keys []string
}{
	{"engineering", []string{"product", "request", "commit", "engineering", "traps"}},
	{"capabilities", []string{"capabilities"}},
	{"product", []string{"product_claims"}},
	{"modernisation", []string{"do_not_carry", "compatibility", "modernisation"}},
	{"questions", []string{"questions", "summary", "assign"}},
}

// Pending is the question a section names for a gap before the questions
// section is written: an unknown claim's question, a proposed do-not-carry
// entry's, or an undecided interface's. The questions section maps each
// pending item to a question (Draft.Assign).
const Pending = "?"

// SectionNames lists the sections in order.
func SectionNames() []string {
	var out []string
	for _, s := range Sections {
		out = append(out, s.Name)
	}
	return out
}

func sectionOfKey(k string) string {
	for _, s := range Sections {
		if slices.Contains(s.Keys, k) {
			return s.Name
		}
	}
	return ""
}

// SplitDraft splits a draft, whole or partial, into its sections' JSON
// objects. A key no section owns is an error, as in ParseDraft.
func SplitDraft(b []byte) (map[string]json.RawMessage, error) {
	var top map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(&top); err != nil {
		return nil, err
	}
	parts := map[string]map[string]json.RawMessage{}
	for k, v := range top {
		s := sectionOfKey(k)
		if s == "" {
			return nil, fmt.Errorf("json: unknown field %q", k)
		}
		if parts[s] == nil {
			parts[s] = map[string]json.RawMessage{}
		}
		parts[s][k] = v
	}
	out := map[string]json.RawMessage{}
	for s, kv := range parts {
		b, err := marshalOrdered(s, kv)
		if err != nil {
			return nil, err
		}
		out[s] = b
	}
	return out, nil
}

// marshalOrdered writes a section's keys in the section's key order.
func marshalOrdered(section string, kv map[string]json.RawMessage) (json.RawMessage, error) {
	var buf bytes.Buffer
	buf.WriteString("{")
	n := 0
	for _, s := range Sections {
		if s.Name != section {
			continue
		}
		for _, k := range s.Keys {
			v, ok := kv[k]
			if !ok {
				continue
			}
			if n > 0 {
				buf.WriteString(",")
			}
			kb, _ := json.Marshal(k)
			buf.Write(kb)
			buf.WriteString(":")
			buf.Write(v)
			n++
		}
	}
	buf.WriteString("}")
	var out bytes.Buffer
	if err := json.Indent(&out, buf.Bytes(), "", "  "); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// MergeSections joins sections into one draft, parsed strictly, and applies
// the questions section's assignments to the pending items.
func MergeSections(parts map[string]json.RawMessage) (Draft, []string, error) {
	all := map[string]json.RawMessage{}
	for _, name := range SectionNames() {
		b, ok := parts[name]
		if !ok {
			continue
		}
		var kv map[string]json.RawMessage
		if err := json.Unmarshal(b, &kv); err != nil {
			return Draft{}, nil, fmt.Errorf("section %s: %w", name, err)
		}
		for k, v := range kv {
			if s := sectionOfKey(k); s != name {
				return Draft{}, nil, fmt.Errorf("section %s: %q belongs to the %s section", name, k, cmp.Or(s, "no"))
			}
			all[k] = v
		}
	}
	b, err := json.Marshal(all)
	if err != nil {
		return Draft{}, nil, err
	}
	d, err := ParseDraft(b)
	if err != nil {
		return Draft{}, nil, err
	}
	problems := applyAssign(&d)
	return d, problems, nil
}

// Parseable splits sections into those that parse on their own and those
// that do not, with each one's error, so a malformed section never stops the
// sections beside it being checked and saved.
func Parseable(parts map[string]json.RawMessage) (good map[string]json.RawMessage, bad map[string]error) {
	good, bad = map[string]json.RawMessage{}, map[string]error{}
	for name, b := range parts {
		if _, _, err := MergeSections(map[string]json.RawMessage{name: b}); err != nil {
			if !strings.HasPrefix(err.Error(), "section ") {
				err = fmt.Errorf("section %s: %w", name, err)
			}
			bad[name] = err
		} else {
			good[name] = b
		}
	}
	return good, bad
}

// applyAssign gives each pending item the question the questions section
// assigns it, and reports assignments to items that are not pending or to
// questions that do not exist.
func applyAssign(d *Draft) []string {
	if len(d.Assign) == 0 {
		return nil
	}
	var ps []string
	qs := map[string]bool{}
	for _, q := range d.Questions {
		qs[q.ID] = true
	}
	used := map[string]bool{}
	set := func(id string, field *string) {
		q, ok := d.Assign[id]
		if !ok || *field != Pending {
			return
		}
		used[id] = true
		*field = q
	}
	for _, l := range []*[]Claim{&d.Engineering, &d.Capabilities, &d.ProductClaims} {
		for k := range *l {
			set((*l)[k].ID, &(*l)[k].Question)
		}
	}
	if d.Modernisation != nil {
		for k := range d.Modernisation.Risks {
			set(d.Modernisation.Risks[k].ID, &d.Modernisation.Risks[k].Question)
		}
	}
	for k := range d.Traps {
		set(d.Traps[k].ID, &d.Traps[k].Question)
	}
	for k := range d.DoNotCarry {
		set(d.DoNotCarry[k].ID, &d.DoNotCarry[k].Question)
	}
	for k := range d.Compatibility {
		set(d.Compatibility[k].ID, &d.Compatibility[k].DecidedBy)
	}
	for _, id := range sortedKeys(d.Assign) {
		switch q := d.Assign[id]; {
		case !used[id]:
			ps = append(ps, fmt.Sprintf("question assign: %s is not an item waiting for a question (its question is not %q)", id, Pending))
		case !qs[q]:
			ps = append(ps, fmt.Sprintf("question assign: %s is assigned %s, which is not a question", id, q))
		}
	}
	d.Assign = nil
	return ps
}

// SectionReport is the check of a survey written in sections.
type SectionReport struct {
	Report
	// Sections maps each section to its problems; a section with none passed.
	Sections map[string][]string `json:"sections"`
	Missing  []string            `json:"missing,omitempty"`
}

// CheckSections checks the sections present. Every floor of Check runs on
// the merged draft; while sections are missing, a problem that belongs to a
// missing section is left for it, and the pending questions of present
// sections stand for the questions section to come. With every section
// present the result is Check's, attributed to sections.
func CheckSections(ix *Index, parts map[string]json.RawMessage) (Draft, SectionReport, error) {
	d, assign, err := MergeSections(parts)
	if err != nil {
		return Draft{}, SectionReport{}, err
	}
	var missing []string
	for _, s := range SectionNames() {
		if _, ok := parts[s]; !ok {
			missing = append(missing, s)
		}
	}
	check := d
	if slices.Contains(missing, "questions") {
		check = withPendingQuestions(d)
	}
	r := Check(ix, check)
	r.Problems = append(assign, r.Problems...)
	sr := SectionReport{Sections: map[string][]string{}, Missing: missing}
	ids := map[string]bool{}
	if all, err := json.Marshal(d); err == nil {
		for _, m := range draftID.FindAllStringSubmatch(string(all), -1) {
			ids[m[1]] = true
		}
	}
	var kept []string
	for _, p := range r.Problems {
		s := SectionOf(d, p)
		if slices.Contains(missing, s) || dangling(p, ids, missing) {
			continue
		}
		kept = append(kept, p)
		sr.Sections[s] = append(sr.Sections[s], p)
	}
	for _, s := range SectionNames() {
		if _, ok := parts[s]; ok && sr.Sections[s] == nil {
			sr.Sections[s] = []string{}
		}
	}
	if kept == nil {
		kept = []string{}
	}
	r.Problems, r.Pass = kept, len(kept) == 0 && len(missing) == 0
	sr.Report = r
	return d, sr, nil
}

// withPendingQuestions gives every pending item its own open question, so
// the floors that need a question see one until the questions section maps
// them.
func withPendingQuestions(d Draft) Draft {
	n := 900
	next := func(field *string) {
		if *field == Pending {
			n++
			id := fmt.Sprintf("Q%d", n)
			d.Questions = append(d.Questions, Question{ID: id, Text: "pending", Why: "pending", Status: "open"})
			*field = id
		}
	}
	cp := func(l []Claim) []Claim { return append([]Claim(nil), l...) }
	d.Engineering, d.Capabilities, d.ProductClaims = cp(d.Engineering), cp(d.Capabilities), cp(d.ProductClaims)
	for _, l := range [][]Claim{d.Engineering, d.Capabilities, d.ProductClaims} {
		for k := range l {
			next(&l[k].Question)
		}
	}
	d.Traps = append([]Trap(nil), d.Traps...)
	for k := range d.Traps {
		next(&d.Traps[k].Question)
	}
	d.DoNotCarry = append([]Carry(nil), d.DoNotCarry...)
	for k := range d.DoNotCarry {
		next(&d.DoNotCarry[k].Question)
	}
	d.Compatibility = append([]Interface(nil), d.Compatibility...)
	for k := range d.Compatibility {
		next(&d.Compatibility[k].DecidedBy)
	}
	if d.Modernisation != nil {
		m := *d.Modernisation
		m.Risks = cp(m.Risks)
		for k := range m.Risks {
			next(&m.Risks[k].Question)
		}
		d.Modernisation = &m
	}
	return d
}

// dangling reports whether a problem waits for a missing section: every item
// it names that the draft lacks has the ID of an item only a missing section
// writes (Q7 before the questions section). A reference no missing section
// can supply (ZZ1, or E9 once engineering is in) is reported at once.
func dangling(problem string, ids map[string]bool, missing []string) bool {
	absent := 0
	for _, m := range itemRef.FindAllString(problem, -1) {
		if ids[m] {
			continue
		}
		s, ok := idSections[strings.TrimRight(m, "0123456789")]
		if !ok || !slices.Contains(missing, s) {
			return false
		}
		absent++
	}
	return absent > 0
}

// idSections maps an item ID's prefix (references/formats.md) to the section
// that writes it.
var idSections = map[string]string{
	"E": "engineering", "T": "engineering", "C": "capabilities", "P": "product", "Q": "questions",
	"N": "modernisation", "K": "modernisation", "M": "modernisation", "G": "modernisation", "R": "modernisation",
	"S": "modernisation", "MG": "modernisation", "SH": "modernisation",
}

var (
	draftID   = regexp.MustCompile(`"id":"([^"]+)"`)
	itemRef   = regexp.MustCompile(`\b[A-Z]{1,2}[0-9]+\b`)
	problemID = regexp.MustCompile(`^[a-z][a-z -]*?\s"?([A-Z]{1,2}[0-9]+)\b`)
	// sectionPrefix attributes a problem by how it starts when it names no item.
	sectionPrefix = []struct{ prefix, section string }{
		{"product:", "engineering"}, {"request:", "engineering"}, {"commit:", "engineering"}, {"engineering", "engineering"},
		{"trap", "engineering"}, {"capabilities", "capabilities"}, {"capability", "capabilities"}, {"product_claims", "product"},
		{"product", "product"}, {"questions", "questions"}, {"question", "questions"}, {"summary", "questions"},
	}
)

// SectionOf names the section a problem belongs to: the section of the item
// it names (claim E3, question Q2, case set G1), or, for a problem about a
// whole list or field, the section that owns it. Everything about the
// modernisation, its contract, and do-not-carry entries is the
// modernisation section's.
func SectionOf(d Draft, problem string) string {
	if m := problemID.FindStringSubmatch(problem); m != nil {
		if s := itemSection(d, m[1]); s != "" {
			return s
		}
	}
	for _, p := range sectionPrefix {
		if strings.HasPrefix(problem, p.prefix) {
			return p.section
		}
	}
	return "modernisation"
}

func itemSection(d Draft, id string) string {
	has := func(l []Claim) bool { return slices.ContainsFunc(l, func(c Claim) bool { return c.ID == id }) }
	switch {
	case has(d.Engineering) || slices.ContainsFunc(d.Traps, func(t Trap) bool { return t.ID == id }):
		return "engineering"
	case has(d.Capabilities):
		return "capabilities"
	case has(d.ProductClaims):
		return "product"
	case slices.ContainsFunc(d.Questions, func(q Question) bool { return q.ID == id }):
		return "questions"
	case d.Modernisation != nil && has(d.Modernisation.Risks),
		slices.ContainsFunc(d.DoNotCarry, func(n Carry) bool { return n.ID == id }),
		slices.ContainsFunc(d.Compatibility, func(k Interface) bool { return k.ID == id }):
		return "modernisation"
	}
	return ""
}

// SavedSection is one section on disk: what was written, against which
// index, and what the floors found, saved before the next section starts.
type SavedSection struct {
	Section  string          `json:"section"`
	Commit   string          `json:"index_commit,omitempty"`
	Digest   string          `json:"index_digest,omitempty"`
	SavedAt  string          `json:"saved_at"`
	Pass     bool            `json:"pass"`
	Problems []string        `json:"problems"`
	Content  json.RawMessage `json:"content"`
	// MAC signs the rest with the user's survey key, kept outside every
	// checkout: a section file the surveyed checkout ships (a source archive
	// can hold .aayudha/survey/sections) is never resumed as earlier work.
	MAC string `json:"mac,omitempty"`
}

// mac is s's signature under key: HMAC-SHA256 of s without its MAC.
func (s SavedSection) mac(key []byte) string {
	s.MAC = ""
	b, _ := json.Marshal(s)
	h := hmac.New(sha256.New, key)
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}

// SaveSection writes one section to dir/<section>.json, signed with key,
// replacing a file atomically so an interrupted write never leaves half a
// section.
func SaveSection(dir string, key []byte, s SavedSection) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if s.SavedAt == "" {
		s.SavedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if s.Problems == nil {
		s.Problems = []string{}
	}
	s.MAC = s.mac(key)
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, "."+s.Section+".json.tmp")
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, s.Section+".json"))
}

// LoadSections reads the saved sections of dir. A section saved against
// another index (its commit or its content digest differs), or not signed
// with key, is stale: it is returned in stale, not in saved.
func LoadSections(dir string, key []byte, commit, digest string) (saved map[string]SavedSection, stale []string, err error) {
	saved = map[string]SavedSection{}
	for _, name := range SectionNames() {
		b, err := os.ReadFile(filepath.Join(dir, name+".json"))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		var s SavedSection
		if err := json.Unmarshal(b, &s); err != nil {
			return nil, nil, fmt.Errorf("%s: %w", filepath.Join(dir, name+".json"), err)
		}
		if s.Commit != commit || s.Digest != digest || !hmac.Equal([]byte(s.MAC), []byte(s.mac(key))) {
			stale = append(stale, name)
			continue
		}
		saved[name] = s
	}
	return saved, stale, nil
}
