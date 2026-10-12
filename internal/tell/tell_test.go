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
// its info string opens nothing.
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
