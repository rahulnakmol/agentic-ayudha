package tell

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/design"
)

// design/brand.yaml as grill brand (WP5.3) writes it sets the tone; keys it
// leaves out keep their defaults, and its avoid list replaces the default one.
func TestLoadBrand(t *testing.T) {
	root := t.TempDir()
	if b, err := LoadBrand(""); err != nil || !reflect.DeepEqual(b, Default) {
		t.Fatalf("outside a checkout: %+v %v", b, err)
	}
	if b, err := LoadBrand(root); err != nil || !reflect.DeepEqual(b, Default) {
		t.Fatalf("no brand.yaml: %+v %v", b, err)
	}
	_ = os.MkdirAll(filepath.Join(root, "design"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "design", "brand.yaml"), []byte("version: 1\nname: Benefits QA\nvoice:\n  tone: [warm]\n  avoid: [simply, just, exclamation marks]\npalette:\n  primary: '#1f5bd1'\npairs:\n  - {fg: primary, bg: surface, use: ui}\n"), 0o644)
	b, err := LoadBrand(root)
	if err != nil || b.Source != design.BrandPath || !reflect.DeepEqual(b.Tone, []string{"warm"}) || b.Voice != "Write as Benefits QA. "+Default.Voice ||
		!reflect.DeepEqual(b.Avoid, []string{"simply", "just", "exclamation marks"}) {
		t.Fatalf("brand: %+v %v", b, err)
	}
	_, probs := Check("Simply answer three questions.\nIt is seamless.\n", "o/r", nil, b, UTM{})
	if len(probs) != 1 || probs[0].Line != 1 || !strings.Contains(probs[0].Reason, `"Simply" is on the brand's avoid list (design/brand.yaml)`) {
		t.Fatalf("problems: %+v", probs)
	}
	_ = os.WriteFile(filepath.Join(root, "design", "brand.yaml"), []byte("voice:\n  tone: {a: b}\n"), 0o644)
	if _, err := LoadBrand(root); err == nil {
		t.Fatal("a tone that is not a list was accepted")
	}
}

// A URL the author already tagged keeps its tags; others gain all three after
// any existing query; non-http links are not tagged.
func TestTag(t *testing.T) {
	u := UTM{Source: "r", Medium: "launch", Campaign: "v1.0.0"}
	for _, tt := range []struct{ in, want string }{
		{"https://example.com/a?utm_source=x", "https://example.com/a?utm_source=x"},
		{"https://example.com/a?x=1#top", "https://example.com/a?x=1&utm_campaign=v1.0.0&utm_medium=launch&utm_source=r#top"},
		{"mailto:a@example.com", "mailto:a@example.com"},
	} {
		if got, _ := Tag(tt.in, u); got != tt.want {
			t.Errorf("Tag(%s) = %s, want %s", tt.in, got, tt.want)
		}
	}
}

// Release notes link only proven Features and count the rest.
func TestReleaseNotes(t *testing.T) {
	proven := map[int]Feature{7: {Number: 7, Title: "Export", URL: "https://github.com/o/r/issues/7"}}
	got := ReleaseNotes("Rolling snapshot 20261012T0900Z", proven, []Shipped{
		{Number: 12, Title: "CSV export", Feature: 7}, {Number: 11, Title: "Export button", Feature: 7}, {Number: 13, Title: "Bulk upload", Feature: 9}, {Number: 14, Title: "Chore"},
	})
	want := "## Rolling snapshot 20261012T0900Z\n\n### Export ([#7](https://github.com/o/r/issues/7))\n\n- Export button\n- CSV export\n\n" +
		"2 more change(s) shipped under Features without authoritative packs; they are not described here.\n"
	if got != want {
		t.Fatalf("notes:\n%s\nwant:\n%s", got, want)
	}
	if _, probs := Check(got, "o/r", proven, Default, UTM{}); len(probs) != 0 {
		t.Fatalf("release notes fail their own claims check: %+v", probs)
	}
	if got := ReleaseNotes("v0.1.0", nil, []Shipped{{Number: 12, Title: "CSV export", Feature: 7}}); strings.Contains(got, "CSV export") || !strings.Contains(got, "claim nothing") {
		t.Fatalf("notes without a proven Feature:\n%s", got)
	}
}

// Only what renders as code is skipped: a fence closes on its own character,
// at least as long, with nothing after it; a backtick run with a backtick in
// its info string opens nothing; an inline code span closes only on a run of
// the same length.
func TestCheckFences(t *testing.T) {
	for _, tt := range []struct {
		name, md string
		claims   bool
	}{
		{"mixed delimiters", "```text\n~~~\n```\nSee #9.\n", true},
		{"shorter closing run", "````\n```\nSee #9.\n````\n", false},
		{"closing run with text", "```\n``` not a close\nSee #9.\n```\nSee #9.\n", true},
		{"backtick in the info string", "``` a`b\nSee #9.\n", true},
		{"unclosed fence", "~~~\nSee #9.\n", false},
		{"code span", "Cite issues as `[#9]`.\n", false},
		{"code span holding a shorter run", "``a ` #9`` is code.\n", false},
		{"unmatched backtick run", "``a` #9\n", true},
		{"after a code span", "`a` #9\n", true},
	} {
		_, probs := Check(tt.md, "o/r", nil, Default, UTM{})
		if got := len(probs) > 0; got != tt.claims {
			t.Errorf("%s: claim found %t, want %t (%+v)", tt.name, got, tt.claims, probs)
		}
	}
}

// Every way a Markdown line can point at an issue or pull request of the
// repository is a claim; another repository's issues are not.
func TestCheckRefs(t *testing.T) {
	for _, tt := range []struct {
		line string
		want []int
	}{
		{"See #4 and o/r#5.", []int{4, 5}},
		{"[a](https://github.com/O/R/issues/6) [b](/o/r/pull/7) [c](../../issues/8) [d](pulls/9)", []int{6, 7, 8, 9}},
		{"[x](https://github.com/other/repo/issues/9), other/repo#3, a&#39;b, /o/r/issues-closed/2", nil},
	} {
		_, probs := Check(tt.line, "o/r", nil, Default, UTM{})
		var got []int
		for _, p := range probs {
			got = append(got, p.Ref)
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: claims %v, want %v", tt.line, got, tt.want)
		}
	}
}

// A URL in a code span is shown, not followed, so it keeps its text.
func TestCheckLeavesCodeSpanLinks(t *testing.T) {
	d, _ := Check("Run `curl https://a.example/x` or see https://b.example/y.", "o/r", nil, Default, UTM{Source: "r", Medium: "docs", Campaign: "c"})
	if want := "Run `curl https://a.example/x` or see https://b.example/y?utm_campaign=c&utm_medium=docs&utm_source=r."; d.Copy != want || d.Tagged != 1 {
		t.Fatalf("copy %q (%d tagged), want %q", d.Copy, d.Tagged, want)
	}
}

// A host's draft (D-7.5b): no superlative, no exclamation mark when the brand
// avoids them, no sentence naming an unproven Feature even unlinked, a named
// proven Feature linked at least once, and announcing kinds link one.
func TestCheckDraft(t *testing.T) {
	proven := map[int]Feature{7: {Number: 7, Title: "Export filtered claims", URL: "https://github.com/o/r/issues/7"}}
	unproven := []Feature{{Number: 20, Title: "Team workspaces"}}
	quiet := Brand{Avoid: []string{"exclamation marks"}, Source: "design/brand.yaml"}
	for _, tt := range []struct {
		name, kind, md string
		b              Brand
		want           []string // problem reasons, in order
	}{
		{"clean launch", "launch", "Export the claims you filtered as a CSV file (#7). It works at least as well with 12,480 claims; the most important step is the filter.\n\n![Export](img/x.png)\n\n```\nthe best code\n```\n", quiet, nil},
		{"superlatives", "guide", "The fastest way, and the `best` flag.\nOur most reliable export.\n", Default, []string{`"fastest" is a superlative`, `"most reliable" is a superlative`}},
		{"exclamation marks", "guide", "Try it now!\n", quiet, []string{"an exclamation mark: the brand avoids them (design/brand.yaml)"}},
		{"exclamation marks allowed", "guide", "Try it now!\n", Default, nil},
		{"alert markers are not exclamation marks", "guide", "> [!NOTE]\n> Try it now.\n\n> [!WARNING]\n> Try it now!\n\n> [!TIP] Try it now.\n\n> > [!NOTE]\n\n> Plain text.\n> [!CAUTION]\n\n> Plain text,\nlazy line.\n> [!NOTE]\n\nIntro.\n> [!IMPORTANT]\n> Try it now.\n\n- A list item.\n\n  > [!TIP]\n\n> `x` [!NOTE]\n\n<pre>\n\n> [!NOTE]\n</pre>\n\n<div>\n> [!TIP]\n\n> [!NOTE]\n", quiet, []string{"an exclamation mark", "an exclamation mark", "an exclamation mark", "an exclamation mark", "an exclamation mark", "an exclamation mark", "an exclamation mark", "an exclamation mark", "an exclamation mark"}},
		{"unproven named", "guide", "Teams will share a workspace soon. A team has one export space.\n", Default, []string{`names #20 "Team workspaces"`}},
		{"unproven words in separate sentences", "guide", "Your team exports claims. Each workspace keeps its own.\n", Default, nil},
		{"unproven words far apart", "guide", "Your team can export the filtered claims to one shared workspace.\n\nSee #7.\n", Default, nil},
		{"proven named, never linked", "guide", "You can export the claims you have filtered.\n", Default, []string{`names #7 "Export filtered claims" without linking it`}},
		{"proven named, linked later", "guide", "You can export the claims you have filtered.\n\nSee its proof: #7.\n", Default, nil},
		{"launch links nothing", "launch", "Reviewing claims is calmer now.\n", Default, []string{"launch copy links no proven Feature"}},
	} {
		_, probs := CheckDraft(tt.md, "o/r", tt.kind, proven, unproven, tt.b, UTM{})
		var got []string
		for _, p := range probs {
			got = append(got, p.Reason)
		}
		ok := len(got) == len(tt.want)
		for i := 0; ok && i < len(got); i++ {
			ok = strings.Contains(got[i], tt.want[i])
		}
		if !ok {
			t.Errorf("%s: problems %q, want %q", tt.name, got, tt.want)
		}
	}
}
