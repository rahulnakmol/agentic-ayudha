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

	"github.com/rahulnakmol/agentic-ayudha/internal/design"
)

// Kinds are what tell writes.
var Kinds = []string{"docs", "guide", "changelog", "release", "launch", "brand"}

// Brand is the part of design/brand.yaml that tell uses: the voice and tone
// the host writes in, and what the copy must avoid. Single (or hyphenated)
// words on the avoid list are refused literally; longer entries ("exclamation
// marks") guide the host only.
type Brand struct {
	Name   string   `json:"name,omitempty"` // design/brand.yaml's name; the docs site's title when set
	Voice  string   `json:"voice"`
	Tone   []string `json:"tone"`
	Avoid  []string `json:"avoid"`
	Source string   `json:"source"` // design/brand.yaml, or "default" when it is absent
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

// LoadBrand reads design/brand.yaml under root with grill brand's parser
// (WP5.3): tell uses its name and voice and leaves the palette and pairs to
// grill. A missing file gives Default; a missing tone or avoid list, its
// default.
func LoadBrand(root string) (Brand, error) {
	b := Default
	if root == "" {
		return b, nil
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(design.BrandPath)))
	if errors.Is(err, fs.ErrNotExist) {
		return b, nil
	} else if err != nil {
		return b, err
	}
	br, err := design.ParseBrand(data)
	if err != nil {
		return b, fmt.Errorf("%s: %w", design.BrandPath, err)
	}
	b.Source = design.BrandPath
	if name := strings.TrimSpace(br.Name); name != "" {
		b.Name = name
		b.Voice = "Write as " + name + ". " + b.Voice
	}
	if len(br.Voice.Tone) > 0 {
		b.Tone = br.Voice.Tone
	}
	if br.Voice.Avoid != nil {
		b.Avoid = br.Voice.Avoid
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
	Page   string `json:"page,omitempty"` // the docs site page's source (tell docs --site)
	Line   int    `json:"line"`
	Ref    int    `json:"ref,omitempty"`
	Reason string `json:"reason"`
}

func (p Problem) String() string {
	if p.Page != "" {
		return fmt.Sprintf("%s line %d: %s", p.Page, p.Line, p.Reason)
	}
	return fmt.Sprintf("line %d: %s", p.Line, p.Reason)
}

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
// Code is not checked or tagged, because GitHub renders no link in it: a
// fenced block closes only on a run of its own character at least as long,
// and an inline code span on the next backtick run of the same length, as
// CommonMark renders them.
func Check(md, repo string, proven map[int]Feature, b Brand, u UTM) (Draft, []Problem) {
	var d Draft
	var probs []Problem
	avoid := map[string]bool{}
	for _, w := range b.Avoid {
		avoid[strings.ToLower(strings.TrimSpace(w))] = true
	}
	cited := map[int]bool{}
	lines := strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n")
	for i, p := range proseLines(md) {
		line := lines[i]
		if p == "" && line != "" { // inside a fenced code block, or its fence
			continue
		}
		spans := codeSpans(line)
		prose := []byte(p)
		for _, n := range refs(string(prose), repo) {
			if _, ok := proven[n]; ok {
				cited[n] = true
				continue
			}
			probs = append(probs, Problem{Line: i + 1, Ref: n, Reason: fmt.Sprintf("#%d is not a Feature with authoritative packs; copy links only proven Features", n)})
		}
		for _, w := range word.FindAllString(linkURL.ReplaceAllString(string(prose), " "), -1) {
			if avoid[strings.ToLower(w)] {
				probs = append(probs, Problem{Line: i + 1, Reason: fmt.Sprintf("%q is on the brand's avoid list (%s)", w, b.Source)})
			}
		}
		var out strings.Builder
		at := 0
		for _, s := range append(spans, [2]int{len(line), len(line)}) {
			out.WriteString(linkURL.ReplaceAllStringFunc(line[at:s[0]], func(raw string) string {
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
			}))
			out.WriteString(line[s[0]:s[1]])
			at = s[1]
		}
		lines[i] = out.String()
	}
	for n := range cited {
		d.Cited = append(d.Cited, n)
	}
	sort.Ints(d.Cited)
	d.Copy = strings.Join(lines, "\n")
	return d, probs
}

// Superlatives are refused in a host's draft whatever the brand says: a
// superlative claims a comparison no pack proves (the Marketer & Documenter's
// voice: "no superlatives").
var Superlatives = []string{"best", "fastest", "easiest", "simplest", "quickest", "smartest", "greatest", "biggest", "finest",
	"safest", "strongest", "smoothest", "cleanest", "ultimate", "unmatched", "unparalleled", "unrivalled", "unrivaled",
	"unbeatable", "perfect", "perfectly", "flawless", "flawlessly"}

// mostRE is a "most" or "least" superlative of a quality only a comparison
// could prove.
var mostRE = regexp.MustCompile(`(?i)\b(?:most|least)\s+(?:powerful|advanced|intuitive|innovative|reliable|secure|complete|comprehensive|efficient|popular|trusted|loved|accurate|flexible|robust|elegant|beautiful|productive)\b`)

// claimKinds are the kinds whose copy announces Features, so it links at
// least one proven Feature when there is one.
var claimKinds = []string{"release", "changelog", "launch"}

// CheckDraft is Check plus the rules for copy a host wrote (`tell --from`):
// no superlative; no exclamation mark when the brand avoids them; no sentence
// that names a Feature without authoritative packs (a sentence names one when
// it has every content word of its title), linked or not; a proven Feature the
// draft names is linked at least once; and release, changelog, and launch
// copy links a proven Feature when there is one. The repository's own pages
// (tell docs --site) and release's generated notes take Check alone.
func CheckDraft(md, repo, kind string, proven map[int]Feature, unproven []Feature, b Brand, u UTM) (Draft, []Problem) {
	d, probs := Check(md, repo, proven, b, u)
	noBang := slices.ContainsFunc(b.Avoid, func(a string) bool { return strings.Contains(strings.ToLower(a), "exclamation") })
	sup := map[string]bool{}
	for _, w := range Superlatives {
		sup[w] = true
	}
	named := map[int]int{} // proven Feature -> first line naming it
	raw := strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n")
	quoted := false // the previous line is inside a top-level blockquote, a lazy continuation line included
	for i, line := range proseLines(md) {
		opens := quoteRE.MatchString(raw[i])
		if opens && !quoted && alertRE.MatchString(raw[i]) { // only alone on the blockquote's first line, code spans included
			line = ""
		}
		quoted = opens || (quoted && strings.TrimSpace(raw[i]) != "")
		text := linkURL.ReplaceAllString(line, " ")
		for _, w := range word.FindAllString(text, -1) {
			if sup[strings.ToLower(w)] {
				probs = append(probs, Problem{Line: i + 1, Reason: fmt.Sprintf("%q is a superlative: it claims a comparison no pack proves; say what the Feature does instead", w)})
			}
		}
		if m := mostRE.FindString(text); m != "" {
			probs = append(probs, Problem{Line: i + 1, Reason: fmt.Sprintf("%q is a superlative: it claims a comparison no pack proves; say what the Feature does instead", m)})
		}
		if noBang && strings.Contains(strings.ReplaceAll(text, "![", "["), "!") {
			probs = append(probs, Problem{Line: i + 1, Reason: fmt.Sprintf("an exclamation mark: the brand avoids them (%s)", b.Source)})
		}
		for _, s := range sentenceRE.Split(text, -1) {
			for _, f := range unproven {
				if names(s, f.Title) {
					probs = append(probs, Problem{Line: i + 1, Ref: f.Number, Reason: fmt.Sprintf("names #%d %q, which has no authoritative packs: leave it out of the copy, without a hint that it is coming", f.Number, f.Title)})
				}
			}
			for n, f := range proven {
				if _, seen := named[n]; !seen && names(s, f.Title) {
					named[n] = i + 1
				}
			}
		}
	}
	for n, line := range named {
		if !slices.Contains(d.Cited, n) {
			probs = append(probs, Problem{Line: line, Ref: n, Reason: fmt.Sprintf("names #%d %q without linking it: link the Feature (#%d) so the reader can see its proof", n, proven[n].Title, n)})
		}
	}
	if slices.Contains(claimKinds, kind) && len(proven) > 0 && len(d.Cited) == 0 {
		probs = append(probs, Problem{Line: 1, Reason: fmt.Sprintf("%s copy links no proven Feature: claim at least one and link it as #N", kind)})
	}
	sort.SliceStable(probs, func(i, j int) bool { return probs[i].Line < probs[j].Line })
	return d, probs
}

var sentenceRE = regexp.MustCompile(`[.!?;:]\s+`)

// alertRE is a GitHub alert marker (`> [!NOTE]`), alone on its line. GitHub
// renders it as syntax only on the first line of a top-level blockquote (one
// may follow a paragraph directly), so CheckDraft skips it only there; a
// nested, indented, or later marker renders as text and its "!" is refused.
// An indented one may sit in a list item, where GitHub renders no alert, so
// only a marker at the start of its line counts.
var alertRE = regexp.MustCompile(`^>\s*\[!(?i:note|tip|important|warning|caution)\]\s*$`)

// quoteRE is a line of a top-level blockquote.
var quoteRE = regexp.MustCompile(`^ {0,3}>`)

// stop words carry no Feature name.
var stop = map[string]bool{"the": true, "and": true, "for": true, "with": true, "from": true, "your": true, "our": true,
	"its": true, "their": true, "into": true, "new": true, "all": true, "any": true, "via": true}

// names reports whether a sentence names a Feature: the title has at least
// two content words (three letters or more, not a stop word, plurals folded)
// and the sentence has them all, in any order, within a run of three words
// more than the title has ("Teams will share a workspace" names Team
// workspaces; "Note: the file holds the claims" does not name Claim notes).
func names(sentence, title string) bool {
	norm := func(w string) string {
		w = strings.ToLower(w)
		if len(w) > 3 && strings.HasSuffix(w, "s") && !strings.HasSuffix(w, "ss") {
			w = w[:len(w)-1]
		}
		return w
	}
	want := map[string]bool{}
	for _, w := range word.FindAllString(title, -1) {
		if len(w) >= 3 && !stop[strings.ToLower(w)] {
			want[norm(w)] = true
		}
	}
	if len(want) < 2 {
		return false
	}
	ws := word.FindAllString(sentence, -1)
	for i := range ws {
		if !want[norm(ws[i])] {
			continue
		}
		have := map[string]bool{}
		for j := i; j < len(ws) && j < i+len(want)+3; j++ {
			if w := norm(ws[j]); want[w] {
				have[w] = true
			}
		}
		if len(have) == len(want) {
			return true
		}
	}
	return false
}

// proseLines returns the draft's lines with fenced code blocks and inline
// code spans blanked, as Check reads them.
func proseLines(md string) []string {
	lines := strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n")
	out := make([]string, len(lines))
	open := ""
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
		prose := []byte(line)
		for _, s := range codeSpans(line) {
			for j := s[0]; j < s[1]; j++ {
				prose[j] = ' '
			}
		}
		out[i] = string(prose)
	}
	return out
}

// codeSpans returns the byte ranges of a line's inline code spans: a run of n
// backticks opens a span that the next run of exactly n closes; a run that
// nothing closes is literal text (CommonMark).
func codeSpans(line string) [][2]int {
	run := func(i int) int {
		j := i
		for j < len(line) && line[j] == '`' {
			j++
		}
		return j - i
	}
	var spans [][2]int
	for i := 0; i < len(line); {
		if line[i] != '`' {
			i++
			continue
		}
		n, closed := run(i), false
		for j := i + n; j < len(line); {
			if line[j] != '`' {
				j++
				continue
			}
			m := run(j)
			if m == n {
				spans = append(spans, [2]int{i, j + m})
				i, closed = j+m, true
				break
			}
			j += m
		}
		if !closed {
			i += n
		}
	}
	return spans
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
