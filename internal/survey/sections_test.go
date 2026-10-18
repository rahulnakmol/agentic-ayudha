package survey

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func fixtureSections(t *testing.T) map[string]json.RawMessage {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "draft.json"))
	if err != nil {
		t.Fatal(err)
	}
	parts, err := SplitDraft(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != len(Sections) {
		t.Fatalf("sections = %d; the fixture draft has all five", len(parts))
	}
	return parts
}

func only(parts map[string]json.RawMessage, names ...string) map[string]json.RawMessage {
	out := map[string]json.RawMessage{}
	for _, n := range names {
		out[n] = parts[n]
	}
	return out
}

// edit rewrites one section's JSON.
func edit(t *testing.T, parts map[string]json.RawMessage, name string, f func(d *Draft)) {
	t.Helper()
	var d Draft
	if err := json.Unmarshal(parts[name], &d); err != nil {
		t.Fatal(err)
	}
	f(&d)
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	p, err := SplitDraft(b)
	if err != nil {
		t.Fatal(err)
	}
	parts[name] = p[name]
}

// A survey written in sections is checked as each arrives: the floors run on
// the merged draft, a missing section's problems wait for it, a gap waits for
// the questions section as "?", and the whole is Check's verdict exactly.
func TestCheckSections(t *testing.T) {
	ix := buildFixture(t, 0)
	parts := fixtureSections(t)

	// Whole: the same verdict and problems as Check on the draft.
	_, whole, err := CheckSections(ix, parts)
	if err != nil {
		t.Fatal(err)
	}
	if want := Check(ix, fixtureDraft(t)); !whole.Pass || whole.Pass != want.Pass || len(whole.Missing) != 0 {
		t.Fatalf("whole: %+v; Check: %v", whole, want.Problems)
	}

	// Engineering alone passes: the empty capability map, product, contract, and
	// questions are the missing sections' to write.
	_, r, err := CheckSections(ix, only(parts, "engineering"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Pass || len(r.Sections["engineering"]) != 0 || len(r.Missing) != 4 || len(r.Problems) != 0 {
		t.Errorf("engineering alone: %+v", r)
	}

	// A reference that only a missing section can supply waits for it (Q7, before
	// the questions section); one no section can supply is reported at once.
	for q, want := range map[string]bool{"Q7": false, "ZZ1": true} {
		eng := only(parts, "engineering")
		edit(t, eng, "engineering", func(d *Draft) { d.Engineering[0].Basis, d.Engineering[0].Question = "unknown", q })
		_, r, err := CheckSections(ix, eng)
		if got := slices.ContainsFunc(r.Sections["engineering"], func(p string) bool { return strings.Contains(p, q) }); err != nil || got != want {
			t.Errorf("engineering claim asking %s: reported %v, want %v: %v", q, got, want, r.Sections["engineering"])
		}
	}

	// A product claim left pending ("?") passes before the questions section.
	edit(t, parts, "product", func(d *Draft) { d.ProductClaims[0].Question = Pending })
	if _, r, _ := CheckSections(ix, only(parts, "engineering", "capabilities", "product")); len(r.Problems) != 0 {
		t.Errorf("pending gap before the questions section: %v", r.Problems)
	}
	// With the questions section, the pending gap needs an assignment.
	if _, r, _ := CheckSections(ix, parts); r.Pass || !slices.ContainsFunc(r.Sections["product"], func(p string) bool { return strings.Contains(p, `claim P1: unknown needs question`) }) {
		t.Errorf("unassigned gap: %+v", r.Sections)
	}
	edit(t, parts, "questions", func(d *Draft) { d.Assign = map[string]string{"P1": "Q1"} })
	d, r, err := CheckSections(ix, parts)
	if err != nil || !r.Pass || d.ProductClaims[0].Question != "Q1" || d.Assign != nil {
		t.Errorf("assigned gap: %v %+v; P1 asks %q", err, r.Sections, d.ProductClaims[0].Question)
	}
	// Assigning what is not pending, or to a question that does not exist, fails the questions section.
	edit(t, parts, "questions", func(d *Draft) { d.Assign = map[string]string{"P1": "Q1", "E1": "Q2", "N1": "Q9"} })
	edit(t, parts, "modernisation", func(d *Draft) { d.DoNotCarry[0].Question = Pending })
	if _, r, _ := CheckSections(ix, parts); r.Pass || len(r.Sections["questions"]) != 2 ||
		!strings.Contains(strings.Join(r.Sections["questions"], "\n"), "E1 is not an item waiting") ||
		!strings.Contains(strings.Join(r.Sections["questions"], "\n"), "N1 is assigned Q9, which is not a question") {
		t.Errorf("bad assignments: %+v", r.Sections)
	}

	// A problem is the section's that owns the item it names.
	parts = fixtureSections(t)
	edit(t, parts, "capabilities", func(d *Draft) { d.Capabilities[0].Cites = []string{"src/main.cpp:400"} })
	edit(t, parts, "modernisation", func(d *Draft) { d.Modernisation.Steps[0].Parity = nil })
	if _, r, _ := CheckSections(ix, parts); len(r.Sections["capabilities"]) == 0 || len(r.Sections["modernisation"]) != 1 || len(r.Sections["engineering"]) != 0 {
		t.Errorf("attribution: %+v", r.Sections)
	}

	// A key no section owns is refused, as in ParseDraft.
	if _, err := SplitDraft([]byte(`{"product": "x", "claims": []}`)); err == nil {
		t.Error("an unknown key must fail")
	}
}

func TestSectionOf(t *testing.T) {
	d := fixtureDraft(t)
	for p, want := range map[string]string{
		"claim E2: observed needs at least one citation":                "engineering",
		`capability "E9": IDs at this level are C1, C2, …`:              "capabilities",
		"claim P1: unknown needs question":                              "product",
		"trap T1: check: write the question you asked of the code":      "engineering",
		"question Q1: answered, but no answer":                          "questions",
		"questions: 6 open; the interview asks at most 5 a round":       "questions",
		"question assign: E1 is not an item waiting for a question":     "questions",
		"summary: write the one-paragraph summary":                      "questions",
		"product: name the product":                                     "engineering",
		"product_claims: none; a product claim the code cannot support": "product",
		"capabilities: no capability covers the entry point(s) a.h:3":   "capabilities",
		"step M1: name the case sets":                                   "modernisation",
		"case set G1: tests/a.js:1 calls Math.sqrt":                     "modernisation",
		"interface K1: Q1 is not answered":                              "modernisation",
		"do-not-carry N1: Q2 is not answered":                           "modernisation",
		"modernisation: needs a target and a strategy":                  "modernisation",
	} {
		if got := SectionOf(d, p); got != want {
			t.Errorf("SectionOf(%q) = %s, want %s", p, got, want)
		}
	}
}

// Each section is saved atomically with its findings and signed with the
// user's key; one saved against another index commit, or signed with another
// key (a section file the surveyed checkout ships), is stale and not resumed.
func TestSaveSections(t *testing.T) {
	dir := t.TempDir()
	parts := fixtureSections(t)
	key, other := []byte("0123456789abcdef0123456789abcdef"), []byte("another user's key, 32 bytes...")
	if err := SaveSection(dir, key, SavedSection{Section: "engineering", Commit: "abc", Pass: true, Content: parts["engineering"]}); err != nil {
		t.Fatal(err)
	}
	if err := SaveSection(dir, key, SavedSection{Section: "product", Commit: "old", Problems: []string{"claim P1: …"}, Content: parts["product"]}); err != nil {
		t.Fatal(err)
	}
	if err := SaveSection(dir, other, SavedSection{Section: "capabilities", Commit: "abc", Pass: true, Content: parts["capabilities"]}); err != nil {
		t.Fatal(err)
	}
	saved, stale, err := LoadSections(dir, key, "abc", "")
	if err != nil || len(saved) != 1 || !slices.Equal(stale, []string{"capabilities", "product"}) || compact(saved["engineering"].Content) != compact(parts["engineering"]) {
		t.Errorf("saved %v, stale %v, err %v", saved, stale, err)
	}
	// A signed section whose content is edited no longer verifies.
	b, err := os.ReadFile(filepath.Join(dir, "engineering.json"))
	if err != nil {
		t.Fatal(err)
	}
	var s SavedSection
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	s.Content = parts["product"]
	b, _ = json.Marshal(s)
	if err := os.WriteFile(filepath.Join(dir, "engineering.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	if saved, _, _ := LoadSections(dir, key, "abc", ""); len(saved) != 0 {
		t.Errorf("an edited section was resumed: %v", saved)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, ".*.tmp")); len(left) != 0 {
		t.Errorf("temporary files left: %v", left)
	}
}

func compact(b []byte) string {
	var out bytes.Buffer
	if json.Compact(&out, b) != nil {
		return string(b)
	}
	return out.String()
}

// A saved section is stale after any edit, a same-size edit to a file the
// scanner skips (binary or too large) included: the digest hashes its bytes.
func TestDigestSkippedFiles(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("main.c", "int main(void) { return 0; }\n")
	write("table.bin", "\x00\x01aaaa")
	digest := func() string {
		t.Helper()
		ix, err := Build(context.Background(), dir, Options{Git: noGit})
		if err != nil {
			t.Fatal(err)
		}
		if !slices.ContainsFunc(ix.Files, func(f File) bool { return f.Path == "table.bin" }) {
			t.Fatalf("files = %v; want table.bin indexed", ix.Files)
		}
		return ix.Digest
	}
	before := digest()
	write("table.bin", "\x00\x01bbbb")
	if digest() == before {
		t.Fatal("a same-size edit to a binary file left the digest unchanged")
	}
}
