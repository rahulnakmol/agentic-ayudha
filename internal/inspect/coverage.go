package inspect

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
)

// File coverage states in the sealed manifest.
const (
	Reviewed   = "reviewed"
	SkippedCov = "skipped"
	Unreviewed = "unreviewed"
)

// Coverage is one changed file in the sealed coverage manifest: reviewed,
// skipped with a reason, or left unreviewed.
type Coverage struct {
	Path   string `json:"path"`
	Status string `json:"status"`
	Why    string `json:"why,omitempty"`
}

// Manifest is inspect's sealed coverage manifest (an idea from Open Code
// Review, written for aayudha; nothing is copied): every changed file of the
// diff, in diff order, and whether the review covered it. SHA256 seals the
// list, so the state and the comment name exactly what was reviewed. A file
// left unreviewed fails the gate (Enforce).
type Manifest struct {
	Files  []Coverage `json:"files"`
	State  string     `json:"state"` // complete, or partial when a file is unreviewed
	SHA256 string     `json:"sha256"`
}

// Seal builds the manifest from the diff: deleted and generated files are
// skipped with their reason; a file the reviewer covered is reviewed; a test
// file it did not cover is skipped (test gaps are judged in the criteria's
// verdicts); any other file is unreviewed.
func Seal(files []DiffFile, areas []Area, fl Filter, covered func(path string) bool) Manifest {
	generated := map[string]bool{}
	for _, a := range areas {
		for _, g := range a.Generated {
			generated[g] = true
		}
	}
	m := Manifest{Files: []Coverage{}, State: "complete"}
	for _, f := range files {
		c := Coverage{Path: f.Path}
		switch {
		case f.Deleted:
			c.Status, c.Why = SkippedCov, "deleted"
		case generated[f.Path] || fl.isGenerated(f.Path):
			c.Status, c.Why = SkippedCov, "generated: the drift check owns it"
		case covered(f.Path):
			c.Status = Reviewed
		case isTestPath(f.Path):
			c.Status, c.Why = SkippedCov, "test file: test gaps are judged in the criteria's verdicts"
		default:
			c.Status, c.Why = Unreviewed, "the review did not cover it"
			m.State = "partial"
		}
		m.Files = append(m.Files, c)
	}
	b, _ := json.Marshal(m.Files)
	s := sha256.Sum256(b)
	m.SHA256 = hex.EncodeToString(s[:])
	return m
}

// Unreviewed lists the manifest's unreviewed files.
func (m *Manifest) Unreviewed() []string {
	if m == nil {
		return nil
	}
	var out []string
	for _, c := range m.Files {
		if c.Status == Unreviewed {
			out = append(out, c.Path)
		}
	}
	return out
}

// inPrompt reports which files' whole diff sections fit in the first limit
// bytes of diff: the files a model reviewer was shown (Prompt caps the diff).
func inPrompt(diff string, limit int) func(string) bool {
	shown := map[string]bool{}
	starts := []int{}
	for i := 0; i < len(diff); {
		j := strings.Index(diff[i:], "diff --git ")
		if j < 0 {
			break
		}
		if i+j == 0 || diff[i+j-1] == '\n' {
			starts = append(starts, i+j)
		}
		i += j + 1
	}
	for k, s := range starts {
		end := len(diff)
		if k+1 < len(starts) {
			end = starts[k+1]
		}
		if end > limit {
			break
		}
		for _, f := range ParseDiff(diff[s:end]) {
			shown[f.Path] = true
		}
	}
	return func(p string) bool { return shown[p] }
}

// Weakening is a check the PR made weaker: fewer assertions, a skipped test,
// removed CI steps, a failure that no longer fails, or a looser threshold. A
// PR may edit its own checks; inspect flags each weakening and the proof
// comment shows it. It is a flag, not a verdict.
type Weakening struct {
	File string `json:"file"`
	What string `json:"what"`
}

var (
	assertRe   = regexp.MustCompile(`\bt\.(Error|Errorf|Fatal|Fatalf|Fail|FailNow)\(|\b(assert|require)\.\w+\(|\bexpect\(|\bassert(Equal|True|False|Raises|In|Is\w*)?\(|\bassert\s|\bshould\.`)
	skipRe     = regexp.MustCompile(`\bt\.Skip(f|Now)?\(|\.skip\(|\bx(it|describe|test)\(|@pytest\.mark\.skip|@unittest\.skip|\.only\(`)
	stepRe     = regexp.MustCompile(`^\s*-\s+(name|run|uses):`)
	numRe      = regexp.MustCompile(`^\s*"?([A-Za-z0-9_.-]+)"?\s*[:=]\s*"?(-?\d+(?:\.\d+)?)%?"?\s*,?\s*(#.*)?$`)
	tighterLow = regexp.MustCompile(`(?i)(min|threshold|coverage|floor|lower|least)`)
	tighterHi  = regexp.MustCompile(`(?i)(max|budget|limit|ceiling|timeout|tolerance|allow|upper|most)`)
)

func isCIFile(p string) bool {
	return (strings.HasPrefix(p, ".github/workflows/") && (strings.HasSuffix(p, ".yml") || strings.HasSuffix(p, ".yaml"))) ||
		p == ".gitlab-ci.yml" || strings.HasPrefix(p, ".circleci/") || path.Base(p) == "Makefile"
}

func isConfigFile(p string) bool {
	switch path.Ext(p) {
	case ".toml", ".yml", ".yaml", ".json", ".ini", ".cfg":
		return true
	}
	return path.Base(p) == ".nycrc" || path.Base(p) == "setup.cfg"
}

// Weakened finds the checks the diff weakens, deterministically from its
// added and removed lines: fewer assertions or a new skip in a test file,
// fewer steps, continue-on-error, a disabled step, or an ignored failure in
// CI, and a numeric threshold moved in its loose direction in a config file
// (a min/threshold/coverage key lowered, a max/budget/limit key raised).
func Weakened(files []DiffFile) []Weakening {
	var out []Weakening
	for _, f := range files {
		var added, removed []string
		for _, h := range f.Hunks {
			removed = append(removed, h.Removed...)
			for _, l := range h.Lines {
				if l.Added {
					added = append(added, l.Text)
				}
			}
		}
		if f.Deleted && (isTestPath(f.Path) || isCIFile(f.Path)) {
			out = append(out, Weakening{f.Path, "deleted"})
			continue
		}
		count := func(re *regexp.Regexp, ls []string) int {
			n := 0
			for _, l := range ls {
				n += len(re.FindAllString(l, -1))
			}
			return n
		}
		switch {
		case isTestPath(f.Path):
			if a, r := count(assertRe, added), count(assertRe, removed); r > a {
				out = append(out, Weakening{f.Path, fmt.Sprintf("fewer assertions (%d removed, %d added)", r, a)})
			}
			if count(skipRe, added) > count(skipRe, removed) {
				out = append(out, Weakening{f.Path, "a test is skipped or narrowed (skip or only added)"})
			}
		case isCIFile(f.Path):
			if a, r := count(stepRe, added), count(stepRe, removed); r > a {
				out = append(out, Weakening{f.Path, fmt.Sprintf("fewer steps (%d removed, %d added)", r, a)})
			}
			for _, l := range added {
				t := strings.TrimSpace(l)
				switch {
				case strings.HasPrefix(t, "continue-on-error:") && strings.Contains(t, "true"):
					out = append(out, Weakening{f.Path, "continue-on-error added: a failing step no longer fails the job"})
				case t == "if: false" || t == "if: ${{ false }}":
					out = append(out, Weakening{f.Path, "a step or job is disabled (if: false)"})
				case strings.Contains(t, "|| true") || strings.Contains(t, "|| exit 0"):
					out = append(out, Weakening{f.Path, "a command's failure is ignored (|| true)"})
				}
			}
		}
		if isConfigFile(f.Path) && !isCIFile(f.Path) {
			old := map[string]float64{}
			for _, l := range removed {
				if m := numRe.FindStringSubmatch(l); m != nil {
					old[m[1]], _ = strconv.ParseFloat(m[2], 64)
				}
			}
			for _, l := range added {
				m := numRe.FindStringSubmatch(l)
				if m == nil {
					continue
				}
				was, ok := old[m[1]]
				now, _ := strconv.ParseFloat(m[2], 64)
				if !ok || was == now {
					continue
				}
				if (tighterHi.MatchString(m[1]) && now > was) || (!tighterHi.MatchString(m[1]) && tighterLow.MatchString(m[1]) && now < was) {
					out = append(out, Weakening{f.Path, fmt.Sprintf("threshold `%s` loosened from %g to %g", m[1], was, now)})
				}
			}
		}
	}
	return out
}
