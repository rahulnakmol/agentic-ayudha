// Package tell is the deterministic half of `aayu tell` (WP7.5). The host's
// model writes the copy; this package gives it the brand's tone, checks the
// draft (every link to a work item must be a Feature with authoritative packs,
// and no word on the brand's avoid list), tags every outbound link with UTM
// parameters, and writes release notes from shipped stories.
package tell

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v4"
)

// Kinds are what tell writes.
var Kinds = []string{"docs", "guide", "changelog", "release", "launch", "brand"}

// BrandPath is where `grill brand` (WP5.3) writes the brand, relative to the
// repository root.
const BrandPath = "design/brand.yaml"

// Brand is the part of design/brand.yaml that tell uses: the voice and tone
// the host writes in, and what the copy must avoid. Single (or hyphenated)
// words on the avoid list are refused literally; longer entries ("exclamation
// marks") guide the host only.
type Brand struct {
	Voice  string   `json:"voice"`
	Tone   []string `json:"tone"`
	Avoid  []string `json:"avoid"`
	Source string   `json:"source"` // BrandPath, or "default" when it is absent
}

// Default is the tone used when design/brand.yaml is absent, and for each key
// it does not set.
var Default = Brand{
	Voice: "Plain and specific: say what a Feature does for the reader, in short sentences and the active voice, and link its proof.",
	Tone:  []string{"plain", "specific", "calm"},
	Avoid: []string{"revolutionary", "game-changing", "seamless", "seamlessly", "world-class", "best-in-class", "cutting-edge",
		"blazing", "effortless", "effortlessly", "magical", "guaranteed"},
	Source: "default",
}

// LoadBrand reads design/brand.yaml under root, as `grill brand` (WP5.3)
// writes it: tell uses its name and `voice: {tone, avoid}` and ignores the
// palette and pairs. A missing file gives Default; a missing tone or avoid
// list, its default.
func LoadBrand(root string) (Brand, error) {
	b := Default
	if root == "" {
		return b, nil
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(BrandPath)))
	if errors.Is(err, fs.ErrNotExist) {
		return b, nil
	} else if err != nil {
		return b, err
	}
	var raw struct {
		Name  string `yaml:"name"`
		Voice struct {
			Tone  []string `yaml:"tone"`
			Avoid []string `yaml:"avoid"`
		} `yaml:"voice"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return b, fmt.Errorf("%s: %w", BrandPath, err)
	}
	b.Source = BrandPath
	if name := strings.TrimSpace(raw.Name); name != "" {
		b.Voice = "Write as " + name + ". " + b.Voice
	}
	if len(raw.Voice.Tone) > 0 {
		b.Tone = raw.Voice.Tone
	}
	if raw.Voice.Avoid != nil {
		b.Avoid = raw.Voice.Avoid
	}
	return b, nil
}

// Feature is a Feature whose shipped stories all have authoritative packs.
type Feature struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	URL    string `json:"url"`
}

// UTM is the campaign tagging applied to every outbound link.
type UTM struct {
	Source   string `json:"utm_source"`
	Medium   string `json:"utm_medium"`
	Campaign string `json:"utm_campaign"`
}

// Problem is one reason a draft is refused.
type Problem struct {
	Line   int    `json:"line"`
	Ref    int    `json:"ref,omitempty"`
	Reason string `json:"reason"`
}

func (p Problem) String() string { return fmt.Sprintf("line %d: %s", p.Line, p.Reason) }

// Draft is a checked draft.
type Draft struct {
	Copy   string `json:"-"`      // the draft with UTM parameters on every outbound link
	Cited  []int  `json:"cited"`  // proven Features the draft links
	Tagged int    `json:"tagged"` // links given UTM parameters
}

var (
	fence   = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})(.*)$")
	linkURL = regexp.MustCompile("https?://[^\\s<>()\\[\\]\"'`]+")
	word    = regexp.MustCompile(`[\p{L}\p{N}][\p{L}\p{N}'-]*`)
)

var (
	pathRef = regexp.MustCompile(`(?i)(?:^|[^\w-])(?:issues|pulls?)/([0-9]+)\b`)
	foreign = regexp.MustCompile(`(?i)github\.com/([\w.-]+/[\w.-]+)/$`)
)

// refs finds the work items of repo a line links: #N and owner/name#N, and
// any issues/N or pull/N path (absolute, root-relative, or relative, as GitHub
// resolves them) unless it names another repository on github.com.
func refs(line, repo string) []int {
	var out []int
	short := regexp.MustCompile(`(?:^|[^\w&/#])(?:(?i:` + regexp.QuoteMeta(repo) + `))?#([0-9]+)\b`)
	for _, m := range short.FindAllStringSubmatch(line, -1) {
		n, _ := strconv.Atoi(m[1])
		out = append(out, n)
	}
	for _, m := range pathRef.FindAllStringSubmatchIndex(line, -1) {
		if f := foreign.FindStringSubmatch(line[:m[0]+1]); f != nil && !strings.EqualFold(f[1], repo) {
			continue
		}
		n, _ := strconv.Atoi(line[m[2]:m[3]])
		out = append(out, n)
	}
	return out
}

// Check checks a Markdown draft for repo and returns it with UTM tags. Every
// link to an issue or pull request of repo is a claim and must name a Feature
// in proven (the claims rule); no word on the brand's avoid list may appear.
// Fenced code blocks are not checked or tagged: a fence closes only on a run
// of its own character at least as long, as CommonMark renders it.
func Check(md, repo string, proven map[int]Feature, b Brand, u UTM) (Draft, []Problem) {
	var d Draft
	var probs []Problem
	avoid := map[string]bool{}
	for _, w := range b.Avoid {
		avoid[strings.ToLower(strings.TrimSpace(w))] = true
	}
	cited := map[int]bool{}
	lines := strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n")
	open := "" // the run that opened the current fenced code block (CommonMark)
	for i, line := range lines {
		if m := fence.FindStringSubmatch(line); m != nil {
			switch {
			case open == "" && (m[1][0] != '`' || !strings.Contains(m[2], "`")):
				open = m[1]
				continue
			case open != "" && m[1][0] == open[0] && len(m[1]) >= len(open) && strings.TrimSpace(m[2]) == "":
				open = ""
				continue
			}
		}
		if open != "" {
			continue
		}
		for _, n := range refs(line, repo) {
			if _, ok := proven[n]; ok {
				cited[n] = true
				continue
			}
			probs = append(probs, Problem{Line: i + 1, Ref: n, Reason: fmt.Sprintf("#%d is not a Feature with authoritative packs; copy links only proven Features", n)})
		}
		for _, w := range word.FindAllString(linkURL.ReplaceAllString(line, " "), -1) {
			if avoid[strings.ToLower(w)] {
				probs = append(probs, Problem{Line: i + 1, Reason: fmt.Sprintf("%q is on the brand's avoid list (%s)", w, b.Source)})
			}
		}
		lines[i] = linkURL.ReplaceAllStringFunc(line, func(raw string) string {
			trail := ""
			for strings.ContainsAny(raw[len(raw)-1:], ".,;:!?") {
				trail = raw[len(raw)-1:] + trail
				raw = raw[:len(raw)-1]
			}
			tagged, ok := Tag(raw, u)
			if ok {
				d.Tagged++
			}
			return tagged + trail
		})
	}
	for n := range cited {
		d.Cited = append(d.Cited, n)
	}
	sort.Ints(d.Cited)
	d.Copy = strings.Join(lines, "\n")
	return d, probs
}

// Tag adds the UTM parameters to an absolute http(s) URL that has none; a URL
// that already carries a utm_ parameter is left as the author set it.
func Tag(raw string, u UTM) (string, bool) {
	p, err := url.Parse(raw)
	if err != nil || p.Host == "" || (p.Scheme != "http" && p.Scheme != "https") {
		return raw, false
	}
	q := p.Query()
	for k := range q {
		if strings.HasPrefix(strings.ToLower(k), "utm_") {
			return raw, false
		}
	}
	add := url.Values{}
	add.Set("utm_source", u.Source)
	add.Set("utm_medium", u.Medium)
	add.Set("utm_campaign", u.Campaign)
	if p.RawQuery == "" {
		p.RawQuery = add.Encode()
	} else {
		p.RawQuery += "&" + add.Encode()
	}
	return p.String(), true
}

// Shipped is a story or bug that shipped, with the Feature it rolls up to (0
// when none).
type Shipped struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	Feature int    `json:"feature,omitempty"`
}

// ReleaseNotes writes the notes of a release: the shipped stories grouped
// under the proven Feature each belongs to, linking only the Feature. Stories
// of Features without authoritative packs are counted, never described.
func ReleaseNotes(title string, proven map[int]Feature, stories []Shipped) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## %s\n", title)
	by := map[int][]Shipped{}
	unproven := 0
	for _, s := range stories {
		if _, ok := proven[s.Feature]; !ok {
			unproven++
			continue
		}
		by[s.Feature] = append(by[s.Feature], s)
	}
	feats := make([]int, 0, len(by))
	for f := range by {
		feats = append(feats, f)
	}
	slices.Sort(feats)
	for _, f := range feats {
		fmt.Fprintf(&b, "\n### %s ([#%d](%s))\n\n", proven[f].Title, f, proven[f].URL)
		ss := by[f]
		sort.Slice(ss, func(i, j int) bool { return ss[i].Number < ss[j].Number })
		for _, s := range ss {
			fmt.Fprintf(&b, "- %s\n", s.Title)
		}
	}
	if len(feats) == 0 {
		b.WriteString("\nNo Feature in this release has authoritative packs yet, so these notes claim nothing.\n")
	}
	if unproven > 0 {
		fmt.Fprintf(&b, "\n%d more change(s) shipped under Features without authoritative packs; they are not described here.\n", unproven)
	}
	return b.String()
}
