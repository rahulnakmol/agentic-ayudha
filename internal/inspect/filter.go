package inspect

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
)

// Finding categories. Style belongs to the linters and formatters; security,
// data, and concurrency are protected: no confidence threshold, learned
// suppression, or peer refutation removes a P0 in them (WP4.8b).
// test-value marks a test that only restates the code or duplicates another:
// counted, never posted (research P2, private research note video-learnings-2026-10.md).
var Categories = []string{"bug", "security", "data", "concurrency", "contract", "test", "test-value", "performance", "maintainability", "style", "docs"}

// Protected reports whether a category is never suppressed at P0.
func Protected(category string) bool {
	return category == "security" || category == "data" || category == "concurrency"
}

// Why a finding was not posted.
const (
	DropOutsideDiff   = "outside-diff"   // the file is not in the PR's diff
	DropLinterOwned   = "linter-owned"   // style, or a nit the linters, formatters, or drift check own
	DropGenerated     = "generated-file" // the file says it is generated
	DropLowConfidence = "low-confidence" // a P2 or P3 below [inspect] min_confidence
	DropNit           = "nit"            // P3: counted, not posted
	DropDuplicate     = "duplicate"      // merged into another finding
	DropDismissed     = "dismissed"      // resembles findings a human dismissed in this repository
	DropRefuted       = "refuted"        // the peer reviewer showed it is wrong (peer mode)
	DropTestGap       = "test-gap"       // a missing or weak test noted on a test file: judged in the criterion's verdict instead
	DropTestValue     = "test-value"     // a test that only restates the code: counted, never posted
	DropPreExisting   = "pre-existing"   // its line is unchanged by the PR: listed, never blocks (Decision #544)
	DropRepeat        = "repeat"         // a P2 or P3 an earlier round reported: counted, never posted
)

// Dropped is a finding that was not posted, and why.
type Dropped struct {
	Finding
	Why string `json:"why"`
}

// Dismissal is a finding a human dismissed as wrong or not worth a comment,
// kept per repository on the ledger (inspect/dismissed.jsonl).
type Dismissal struct {
	ID       string `json:"id"`
	PR       int    `json:"pr"`
	File     string `json:"file"`
	Category string `json:"category"`
	Title    string `json:"title"`
	Reason   string `json:"reason"`
	By       string `json:"by"`
	At       string `json:"at"`
}

// Filter settings ([inspect] in config).
type Filter struct {
	MinConfidence float64 // P2 and P3 findings below it are dropped; 0 keeps every finding
	SuppressAfter int     // a finding resembling this many distinct dismissals is suppressed; 0 disables
	Generated     []string
}

var lintNit = regexp.MustCompile(`(?i)\b(typo|spelling|misspell|formatting|gofmt|goimports|whitespace|indentation|import order|naming convention|unused import|trailing space|line length|comment style|doc comment)\b`)

// testGapRe reads a finding as a test gap ("no test for …", "not covered").
var testGapRe = regexp.MustCompile(`(?i)\b(no|missing|lacks?|without|add(s|ing)?|needs?)( an?| the)?( unit| regression| table| edge[- ]case| negative)? (tests?|test cases?|coverage|assertions?)\b|\bnot (covered|tested|exercised|asserted)\b|\buntested\b`)

// TestGap reports whether f is a test gap noted on a test file: a test
// finding, or one worded as a missing test, in a file that is itself a test.
// Such notes are judged in the criterion's verdict, never posted as line
// findings (the research measured all 8 of inspect's false positives on the
// seeded set as these, private research note verification-skills-2026-10.md §8.2).
func TestGap(f Finding) bool {
	return isTestPath(f.File) && !Protected(f.Category) && (f.Category == "test" || testGapRe.MatchString(f.Title+" "+f.Detail))
}

var wordRe = regexp.MustCompile(`[a-z0-9]+`)

var stopWords = map[string]bool{"the": true, "a": true, "an": true, "is": true, "of": true, "to": true, "in": true, "on": true, "and": true, "or": true, "for": true, "when": true, "not": true, "it": true, "be": true, "with": true, "this": true, "that": true, "can": true, "may": true}

func tokens(s string) []string {
	var out []string
	for _, w := range wordRe.FindAllString(strings.ToLower(s), -1) {
		if !stopWords[w] && !slices.Contains(out, w) {
			out = append(out, w)
		}
	}
	sort.Strings(out)
	return out
}

// stems is tokens with common English endings cut (closed, closes, closing,
// close: clos), so two reviewers' wordings of one problem compare by root.
func stems(s string) []string {
	var out []string
	for _, w := range tokens(s) {
		for _, suf := range []string{"ing", "ed", "es", "s", "e", "ly"} {
			if len(w) > len(suf)+3 && strings.HasSuffix(w, suf) {
				w = strings.TrimSuffix(w, suf)
				break
			}
		}
		if !slices.Contains(out, w) {
			out = append(out, w)
		}
	}
	return out
}

func jaccard(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	in := 0
	for _, x := range a {
		if slices.Contains(b, x) {
			in++
		}
	}
	return float64(in) / float64(len(a)+len(b)-in)
}

// FindingID is a finding's fingerprint: its file, category, and title words,
// so the same finding keeps its ID across runs and line moves.
func FindingID(file, category, title string) string {
	s := sha256.Sum256([]byte(file + "\x00" + category + "\x00" + strings.Join(tokens(title), " ")))
	return hex.EncodeToString(s[:])[:10]
}

var sevRank = map[string]int{"P0": 0, "P1": 1, "P2": 2, "P3": 3}

// Clean applies the deterministic noise rules to one reviewer's findings:
// anchoring by quote, findings outside the diff, linter-owned and generated
// files, low confidence (P2-P3 only), nits, and de-duplication. It returns the
// findings to post and the dropped ones with why.
func (fl Filter) Clean(fs []Finding, diff []DiffFile, areas []Area, reviewer string) (keep []Finding, drop []Dropped) {
	byPath := map[string]DiffFile{}
	for _, f := range diff {
		byPath[f.Path] = f
	}
	generated := map[string]bool{}
	for _, a := range areas {
		for _, g := range a.Generated {
			generated[g] = true
		}
	}
	for _, f := range fs {
		if f.Category == "" {
			f.Category = "bug"
		}
		f.Sources = []string{reviewer}
		df, ok := byPath[f.File]
		if !ok {
			drop = append(drop, Dropped{f, DropOutsideDiff})
			continue
		}
		if n := df.Find(f.Quote, f.Line); n > 0 {
			f.Line = n
		} else if !df.Has(f.Line) {
			f.Line = 0 // unanchored: posted in the summary, never inline
		}
		f.ID = FindingID(f.File, f.Category, f.Title)
		switch {
		case f.Category == "test-value":
			drop = append(drop, Dropped{f, DropTestValue})
		case TestGap(f):
			drop = append(drop, Dropped{f, DropTestGap})
		case generated[f.File] || fl.isGenerated(f.File):
			drop = append(drop, Dropped{f, DropGenerated})
		case f.Category == "style" || (f.Severity != "P0" && f.Severity != "P1" && lintNit.MatchString(f.Title)):
			drop = append(drop, Dropped{f, DropLinterOwned})
		case (f.Severity == "P2" || f.Severity == "P3") && f.Confidence < fl.MinConfidence:
			drop = append(drop, Dropped{f, DropLowConfidence})
		case f.Severity == "P3":
			drop = append(drop, Dropped{f, DropNit})
		case f.Line > 0 && !df.AddedAt(f.Line):
			// The quoted line is a context line the PR leaves unchanged,
			// whether or not the finding names a criterion (the criterion's
			// own verdict still counts). It comes after the noise rules, so
			// only a finding that would count on an added line is filed as a
			// pre-existing defect. A quote found nowhere stays an unanchored
			// finding of this PR, so a paraphrased quote never hides a defect
			// the PR wrote.
			drop = append(drop, Dropped{f, DropPreExisting})
		default:
			keep = append(keep, f)
		}
	}
	keep, dups := Dedupe(keep)
	return keep, append(drop, dups...)
}

// Repeats drops the findings that repeat a finding an earlier round kept
// (one on another head than head, or on head before the reopen at reopened):
// the same file and fingerprint, class, similar title, or category at a
// nearby line. A repeated P2 or P3 is dropped as repeat; a repeated P0 or P1
// still counts (rule 7: no reviewer text removes it), marked with its round
// so it is not posted again.
func Repeats(fs []Finding, rounds []Round, head string, reopened time.Time) (keep []Finding, drop []Dropped) {
	for _, f := range fs {
		round := 0
		for i, r := range rounds {
			if r.Head == head && r.after(reopened) {
				continue
			}
			for _, e := range r.Findings {
				if repeats(f, e) {
					round = i + 1
				}
			}
		}
		switch {
		case round == 0:
			keep = append(keep, f)
		case f.Severity == "P0" || f.Severity == "P1":
			f.Repeat = round
			keep = append(keep, f)
		default:
			drop = append(drop, Dropped{f, DropRepeat})
		}
	}
	return keep, drop
}

func repeats(f Finding, e Earlier) bool {
	if f.File != e.File {
		return false
	}
	near := f.Line != 0 && e.Line != 0 && abs(f.Line-e.Line) <= 3
	return f.ID == e.ID || (f.Class != "" && strings.EqualFold(f.Class, e.Class)) ||
		jaccard(stems(f.Title), stems(e.Title)) >= 0.35 || (f.Category == e.Category && near)
}

// earlier is what a round keeps of its findings for later rounds.
func earlier(fs []Finding, hashes map[string]string) []Earlier {
	var out []Earlier
	for _, f := range fs {
		e := Earlier{ID: f.ID, Severity: f.Severity, Category: f.Category, File: f.File, Line: f.Line, Title: oneLine(f.Title, 200), Class: oneLine(f.Class, 200), Hashes: map[string]string{}}
		for _, in := range append([]Instance{{File: f.File}}, f.Instances...) {
			if hashes[in.File] != "" {
				e.Hashes[in.File] = hashes[in.File]
			}
		}
		out = append(out, e)
	}
	return out
}

func (fl Filter) isGenerated(p string) bool {
	for _, g := range fl.Generated {
		if ok, _ := path.Match(g, p); ok {
			return true
		}
	}
	return false
}

// Same reports whether two findings are the same problem: one file, lines
// within three of each other (or one unanchored), and similar titles or the
// same category at the same spot.
func Same(a, b Finding) bool {
	if a.File != b.File {
		return false
	}
	near := a.Line == 0 || b.Line == 0 || abs(a.Line-b.Line) <= 3
	if !near {
		return false
	}
	sim := jaccard(tokens(a.Title+" "+a.Detail), tokens(b.Title+" "+b.Detail))
	return a.ID == b.ID || sim >= 0.35 || (a.Category == b.Category && a.Line != 0 && a.Line == b.Line)
}

// Dedupe merges findings that are the same problem, keeping the highest
// severity, the higher confidence, an anchored line, and every source.
func Dedupe(fs []Finding) (keep []Finding, dropped []Dropped) {
	for _, f := range fs {
		i := slices.IndexFunc(keep, func(k Finding) bool { return Same(k, f) })
		if i < 0 {
			keep = append(keep, f)
			continue
		}
		absorb(&keep[i], f)
		dropped = append(dropped, Dropped{f, DropDuplicate})
	}
	return keep, dropped
}

// absorb merges duplicate f into k: the higher severity with its wording, an
// anchored line, the higher confidence, and every source.
func absorb(k *Finding, f Finding) {
	if sevRank[f.Severity] < sevRank[k.Severity] {
		k.Severity, k.Title, k.Detail, k.Fix, k.Category, k.ID = f.Severity, f.Title, f.Detail, f.Fix, f.Category, f.ID
	}
	if k.Line == 0 {
		k.Line, k.Quote = f.Line, f.Quote
	}
	k.Confidence = max(k.Confidence, f.Confidence)
	for _, s := range f.Sources {
		if !slices.Contains(k.Sources, s) {
			k.Sources = append(k.Sources, s)
		}
	}
}

// Suppress drops findings that resemble ones a human dismissed in this
// repository: similar title and category in the same file or directory, from
// at least SuppressAfter distinct dismissals, or one dismissal of the same
// fingerprint. P0 findings in protected categories are never suppressed.
func (fl Filter) Suppress(fs []Finding, ds []Dismissal) (keep []Finding, drop []Dropped) {
	for _, f := range fs {
		if fl.SuppressAfter <= 0 || (f.Severity == "P0" && Protected(f.Category)) {
			keep = append(keep, f)
			continue
		}
		ft := tokens(f.Category + " " + f.Title)
		similar := map[string]bool{}
		exact := false
		for _, d := range ds {
			if d.ID == f.ID {
				exact = true
			}
			if (d.File == f.File || path.Dir(d.File) == path.Dir(f.File)) && jaccard(ft, tokens(d.Category+" "+d.Title)) >= 0.6 {
				similar[fmt.Sprintf("%s#%d", d.ID, d.PR)] = true
			}
		}
		if exact || len(similar) >= fl.SuppressAfter {
			drop = append(drop, Dropped{f, DropDismissed})
			continue
		}
		keep = append(keep, f)
	}
	return keep, drop
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
