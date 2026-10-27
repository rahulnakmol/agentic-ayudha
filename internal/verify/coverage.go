package verify

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
)

// Lines is line coverage per repository-relative file: line → hit (true) or
// executable but not hit (false). Lines absent from the map are not executable.
type Lines map[string]map[int]bool

func (l Lines) set(file string, line int, hit bool) {
	m := l[file]
	if m == nil {
		m = map[int]bool{}
		l[file] = m
	}
	m[line] = m[line] || hit
}

// ParseCoverage reads a Go coverprofile, an lcov tracefile, or a Cobertura
// XML report. module is the Go module path (go.mod) that coverprofile file
// names start with; root is the checkout, for making absolute paths relative.
func ParseCoverage(report []byte, module, root string) (Lines, string, error) {
	t := bytes.TrimSpace(report)
	switch {
	case bytes.HasPrefix(t, []byte("mode:")):
		l, err := parseGoProfile(t, module)
		return l, "go coverprofile", err
	case bytes.Contains(t[:min(len(t), 4096)], []byte("<coverage")):
		l, err := parseCobertura(t, root)
		return l, "cobertura", err
	case bytes.Contains(t, []byte("SF:")):
		l, err := parseLcov(t, root)
		return l, "lcov", err
	}
	return nil, "", errors.New("coverage report is not a Go coverprofile, lcov tracefile, or Cobertura XML")
}

var goBlock = regexp.MustCompile(`^(.+):([0-9]+)\.[0-9]+,([0-9]+)\.[0-9]+ [0-9]+ ([0-9]+)$`)

func parseGoProfile(b []byte, module string) (Lines, error) {
	l := Lines{}
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "mode:") || line == "" {
			continue
		}
		m := goBlock.FindStringSubmatch(line)
		if m == nil {
			return nil, fmt.Errorf("coverprofile: bad line %q", line)
		}
		file := m[1]
		if module != "" {
			file = strings.TrimPrefix(strings.TrimPrefix(file, module), "/")
		}
		start, _ := strconv.Atoi(m[2])
		end, _ := strconv.Atoi(m[3])
		count, _ := strconv.Atoi(m[4])
		for n := start; n <= end; n++ {
			l.set(file, n, count > 0)
		}
	}
	return l, sc.Err()
}

func rel(root, p string) string {
	p = filepath.ToSlash(p)
	r := filepath.ToSlash(root)
	if r != "" && strings.HasPrefix(p, r+"/") {
		return strings.TrimPrefix(p, r+"/")
	}
	return strings.TrimPrefix(path.Clean(p), "./")
}

func parseLcov(b []byte, root string) (Lines, error) {
	l := Lines{}
	file := ""
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case strings.HasPrefix(line, "SF:"):
			file = rel(root, strings.TrimPrefix(line, "SF:"))
		case strings.HasPrefix(line, "DA:") && file != "":
			parts := strings.Split(strings.TrimPrefix(line, "DA:"), ",")
			if len(parts) < 2 {
				return nil, fmt.Errorf("lcov: bad line %q", line)
			}
			n, err1 := strconv.Atoi(parts[0])
			hits, err2 := strconv.ParseFloat(parts[1], 64)
			if err1 != nil || err2 != nil {
				return nil, fmt.Errorf("lcov: bad line %q", line)
			}
			l.set(file, n, hits > 0)
		case line == "end_of_record":
			file = ""
		}
	}
	return l, sc.Err()
}

func parseCobertura(b []byte, root string) (Lines, error) {
	var doc struct {
		Sources []string `xml:"sources>source"`
		Classes []struct {
			Filename string `xml:"filename,attr"`
			Lines    []struct {
				Number int     `xml:"number,attr"`
				Hits   float64 `xml:"hits,attr"`
			} `xml:"lines>line"`
		} `xml:"packages>package>classes>class"`
	}
	if err := xml.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("cobertura: %w", err)
	}
	l := Lines{}
	for _, c := range doc.Classes {
		f := c.Filename
		for _, s := range doc.Sources {
			if s != "" && !filepath.IsAbs(f) {
				f = filepath.Join(s, f)
				break
			}
		}
		f = rel(root, f)
		for _, ln := range c.Lines {
			l.set(f, ln.Number, ln.Hits > 0)
		}
	}
	return l, nil
}

// DiffCoverage is coverage of the changed executable lines.
type DiffCoverage struct {
	Covered    int              `json:"covered"`
	Executable int              `json:"executable"`
	Percent    float64          `json:"percent"`
	Threshold  float64          `json:"threshold"`
	Pass       bool             `json:"pass"`
	Uncovered  map[string][]int `json:"uncovered"`
	Format     string           `json:"format"`
}

// Diff computes coverage of the changed lines against a threshold in percent.
// Changed lines the report does not list are not executable (comments,
// declarations, tests). The comparison is exact: 899 of 1000 lines (89.9 %)
// fails 90.0, 900 of 1000 passes.
func Diff(cov Lines, changed map[string][]int, threshold float64) DiffCoverage {
	d := DiffCoverage{Threshold: threshold, Uncovered: map[string][]int{}}
	for file, lines := range changed {
		m := cov[file]
		for _, n := range lines {
			hit, exec := m[n]
			if !exec {
				continue
			}
			d.Executable++
			if hit {
				d.Covered++
			} else {
				d.Uncovered[file] = append(d.Uncovered[file], n)
			}
		}
	}
	for f := range d.Uncovered {
		slices.Sort(d.Uncovered[f])
	}
	if d.Executable == 0 {
		d.Percent, d.Pass = 100, true
		return d
	}
	d.Percent = float64(d.Covered) * 100 / float64(d.Executable)
	// covered/executable ≥ threshold/100, compared in integers of 1/1000 % so 89.9 never rounds up to 90.
	d.Pass = int64(d.Covered)*100000 >= int64(threshold*1000+0.5)*int64(d.Executable)
	return d
}

// Coverage is the diff-coverage floor (WP4.3).
type Coverage struct{}

// Name implements Floor.
func (Coverage) Name() string { return "coverage" }

// Applies implements Floor: coverage always runs.
func (Coverage) Applies(*Run) (bool, string) { return true, "" }

// Run runs the test command with coverage, then measures the changed lines.
func (Coverage) Run(ctx context.Context, r *Run) (Result, error) {
	cfg := r.Floors.Coverage
	res := Result{Check: proof.Check{Threshold: cfg.MinDiffPercent, Tool: "aayudha diff coverage", Version: "1"}}
	cmd, report := cfg.Command, cfg.Report
	module := goModule(r.Root)
	if len(cmd) == 0 {
		if module == "" {
			return res, errors.New("no coverage command: set [verify.floors.coverage] command and report (lcov, Cobertura, or Go coverprofile)")
		}
		report = r.tempFile("cover.out")
		// On a 2-core host internal/cli runs close to go test's 10-minute
		// default timeout, whose panic fails the package with no test named.
		cmd = []string{"go", "test", "-json", "-timeout", "30m", "-covermode=set", "-coverpkg=./...", "-coverprofile=" + report, "./..."}
	} else if report != "" && !filepath.IsAbs(report) {
		report = filepath.Join(r.Root, report)
	}
	out, err := r.Sandbox.Run(ctx, Cmd{Dir: r.Root, Name: cmd[0], Args: cmd[1:]})
	if err != nil {
		return res, err
	}
	res.Check.Network = out.Network
	if cmd[0] == "go" && slices.Contains(cmd, "-json") {
		r.TestEvents = out.Stdout
		res.Check.Tool = "aayudha diff coverage (go test -coverprofile)"
		res.Check.Version = version(ctx, r, "go", "env", "GOVERSION")
	}
	b, err := os.ReadFile(report)
	if err != nil {
		return res, fmt.Errorf("coverage report %s: %w (test command exited %d: %s)", report, err, out.Code, tail(out.Stderr))
	}
	cov, format, err := ParseCoverage(b, module, r.Root)
	if err != nil {
		return res, err
	}
	d := Diff(cov, r.Lines, cfg.MinDiffPercent)
	d.Format = format
	res.Check.Value = float64(int(d.Percent*10)) / 10 // shown to one decimal, never rounded up
	res.Check.Result = proof.Pass
	res.Check.Detail = fmt.Sprintf("%d of %d changed executable lines covered (%s)", d.Covered, d.Executable, format)
	if !d.Pass {
		res.Check.Result = proof.Fail
	}
	if out.Code != 0 {
		res.Check.Result = proof.Fail
		res.Check.Detail += fmt.Sprintf("; the test command failed (exit %d): %s", out.Code, tail(failedTests(out.Stdout, out.Stderr)))
	}
	j, _ := json.MarshalIndent(d, "", "  ")
	res.Artifacts = append(res.Artifacts, Evidence{Path: "coverage/diff.json", Kind: "metric", Data: append(j, '\n')})
	return res, nil
}

func goModule(root string) string {
	b, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		if m, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.Trim(strings.TrimSpace(m), `"`)
		}
	}
	return ""
}

func tail(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 600 {
		s = "…" + s[len(s)-600:]
	}
	return s
}

// failedTests names what failed in a go test -json stream: the failing
// tests, then each package that failed with no failing test (a timeout, a
// build error, a panic outside a test) and why; else stderr.
func failedTests(stdout, stderr []byte) []byte {
	evs := testEvents(stdout)
	var names, msg []string
	testFailed := map[string]bool{}
	for _, ev := range evs {
		if ev.Action == "fail" && ev.Test != "" {
			names = append(names, ev.Package+" "+ev.Test)
			testFailed[ev.Package] = true
		}
	}
	if len(names) > 0 {
		msg = append(msg, "failing tests: "+strings.Join(names, ", "))
	}
	for _, ev := range evs {
		if ev.Action == "fail" && ev.Test == "" && !testFailed[ev.Package] {
			msg = append(msg, "package "+ev.Package+" failed outside a test: "+packageFailure(evs, ev))
		}
	}
	if len(msg) == 0 {
		return stderr
	}
	return []byte(strings.Join(msg, "; "))
}

// packageFailure is why package fail failed: its last build output lines;
// else its first panic with the lines after it (a timeout lists the running
// tests there, though go test -json files the panic under one of them),
// then the package's last line.
func packageFailure(evs []TestEvent, fail TestEvent) string {
	var build, pkg, panicked []string
	for _, ev := range evs {
		line := strings.TrimSpace(ev.Output)
		switch {
		case line == "":
		case ev.Action == "build-output" && fail.FailedBuild != "" && ev.ImportPath == fail.FailedBuild:
			build = append(build, line)
		case ev.Action != "output" || ev.Package != fail.Package:
		case len(panicked) > 0 && len(panicked) < 4:
			panicked = append(panicked, line)
		case len(panicked) == 0 && strings.HasPrefix(line, "panic: "):
			panicked = append(panicked, line)
		case ev.Test == "":
			pkg = append(pkg, line)
		}
	}
	if len(build) > 0 {
		return strings.Join(build[max(0, len(build)-5):], " / ")
	}
	if len(pkg) > 0 {
		panicked = append(panicked, pkg[len(pkg)-1])
	}
	return strings.Join(panicked, " / ")
}
