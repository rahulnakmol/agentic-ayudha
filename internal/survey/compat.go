package survey

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Shadow runs every case against the legacy and the new program on the same
// inputs and reports each compared stream that differs: shadow running before
// cut-over, with no golden files.
func Shadow(ctx context.Context, root, legacy, next string, c Cases) ([]Diff, error) {
	home, err := os.MkdirTemp("", "aayu-shadow-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(home) }()
	var diffs []Diff
	for _, k := range c.Cases {
		a, err := runCase(ctx, root, legacy, home, k)
		if err != nil {
			return nil, err
		}
		b, err := runCase(ctx, root, next, home, k)
		if err != nil {
			return nil, err
		}
		diffs = append(diffs, compareStreams(k, "legacy", "new", a, b)...)
	}
	return diffs, nil
}

// AreaScore is one area of a compatibility suite: how many of its tests the
// legacy build passes, and how many of those the new build passes.
type AreaScore struct {
	Area        string   `json:"area"`
	Tests       int      `json:"tests"`
	Unfixable   int      `json:"unfixable"`   // left out of the score, each with its reason in the cases file
	LegacyPass  int      `json:"legacy_pass"` // passed (exit 0) when the legacy build was captured
	Pass        int      `json:"pass"`        // of those, passed by the new build
	Skipped     int      `json:"skipped"`     // exit 77, the automake convention for a skipped test
	Regressions []string `json:"regressions,omitempty"`
	Fixed       []string `json:"fixed,omitempty"` // failed on the legacy build, pass on the new one
}

// CompatReport is one run of a legacy test suite against a new build, also
// appended to compat-history.jsonl so the score is tracked over time.
type CompatReport struct {
	Bin         string      `json:"bin"`
	BinSHA256   string      `json:"bin_sha256"`
	Commit      string      `json:"commit,omitempty"` // the legacy capture's commit
	Ran         string      `json:"ran"`
	Areas       []AreaScore `json:"areas"`
	LegacyPass  int         `json:"legacy_pass"`
	Pass        int         `json:"pass"`
	Regressions int         `json:"regressions"`
}

// Compat runs a legacy test suite (each case one test, passing on exit 0)
// against the new build and scores it per area against the legacy build's
// results captured by --golden in dir. A test the legacy build passes and the
// new build fails is a regression; a test that fails on both is not counted.
func Compat(ctx context.Context, root, bin, dir string, c Cases) (*CompatReport, error) {
	b, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return nil, fmt.Errorf("no legacy results: %w; run the suite on the legacy build first with --golden", err)
	}
	var g Golden
	if err := json.Unmarshal(b, &g); err != nil {
		return nil, err
	}
	legacy := map[string]GoldenResult{}
	for _, r := range g.Cases {
		legacy[r.ID] = r
	}
	home, err := os.MkdirTemp("", "aayu-compat-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(home) }()
	r := &CompatReport{Bin: bin, BinSHA256: fileSum(bin), Commit: g.Commit, Ran: time.Now().UTC().Format(time.RFC3339)}
	areas := map[string]*AreaScore{}
	for _, k := range c.Cases {
		a := areas[k.Area]
		if a == nil {
			a = &AreaScore{Area: k.Area}
			areas[k.Area] = a
		}
		a.Tests++
		if k.Unfixable != "" {
			a.Unfixable++
			continue
		}
		want, ok := legacy[k.ID]
		if !ok {
			return nil, fmt.Errorf("test %s has no legacy result; capture the suite again with --golden", k.ID)
		}
		o, err := runCase(ctx, root, bin, home, k)
		if err != nil {
			return nil, err
		}
		pass := o.exit == 0 && !o.timedOut
		if o.exit == 77 {
			a.Skipped++
		}
		switch {
		case want.Exit == 0 && !want.TimedOut:
			a.LegacyPass++
			if pass {
				a.Pass++
			} else {
				a.Regressions = append(a.Regressions, k.ID)
			}
		case pass:
			a.Fixed = append(a.Fixed, k.ID)
		}
	}
	for _, a := range areas {
		r.Areas = append(r.Areas, *a)
		r.LegacyPass += a.LegacyPass
		r.Pass += a.Pass
		r.Regressions += len(a.Regressions)
	}
	sort.Slice(r.Areas, func(i, j int) bool { return r.Areas[i].Area < r.Areas[j].Area })
	line, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, "compat-history.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	_, err = f.Write(append(line, '\n'))
	return r, err
}

// Probes is the runtime evidence gathered by running the legacy program:
// what the code alone cannot show. Each fact cites what was run and what it
// showed; a draft cites a probe fact by its ID like an index fact.
type Probes struct {
	Bin       string   `json:"bin"`
	BinSHA256 string   `json:"bin_sha256"`
	Cases     string   `json:"cases"`
	Ran       []string `json:"ran"` // what was run, one line each
	Skipped   []string `json:"skipped,omitempty"`
	Facts     []Fact   `json:"facts"`
}

// ProbeOptions are the optional probes.
type ProbeOptions struct {
	CheckBin string // a sanitiser or checker build of the legacy program
	Coverage string // an lcov tracefile from running the cases under coverage
}

// checkerReport finds a sanitiser's or a checker's report on stderr.
var checkerReport = regexp.MustCompile(`(?m)^.*(?:ERROR: AddressSanitizer|ERROR: LeakSanitizer|WARNING: ThreadSanitizer|runtime error:|ERROR SUMMARY: [1-9]|Fortran runtime (?:error|warning)|Invalid read of size|Invalid write of size|Conditional jump or move depends on uninitialised|definitely lost: [1-9]|: warning: .*deprecated|DEPRECATION WARNING|Exception in thread).*$`)

// envProbes are the variants each case runs under, one change at a time.
var envProbes = []struct {
	name string
	env  []string
}{
	{"TZ", []string{"TZ=Asia/Kathmandu"}},
	{"locale", []string{"LC_ALL=de_DE.UTF-8", "LANG=de_DE.UTF-8"}},
	{"HOME", nil}, // a second scratch HOME, holding one dotfile
}

// Probe runs the legacy program's cases to find what reading the code does
// not: output that changes between two identical runs, output that depends on
// the time zone, the locale, or HOME, files read and written (under strace,
// when installed), checker reports from a sanitiser build, and functions the
// cases never reach (from an lcov tracefile).
func Probe(ctx context.Context, ix *Index, root, bin, casesFile string, c Cases, opt ProbeOptions) (*Probes, error) {
	p := &Probes{Bin: bin, BinSHA256: fileSum(bin), Cases: casesFile, Facts: []Fact{}}
	root, err := filepath.Abs(root) // strace and lcov report absolute paths; the checkout may be given as "."
	if err != nil {
		return nil, err
	}
	home, err := os.MkdirTemp("", "aayu-probe-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(home) }()
	home2, err := os.MkdirTemp("", "aayu-probe-home-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(home2) }()
	if err := os.WriteFile(filepath.Join(home2, ".aayu-probe"), []byte("probe\n"), 0o644); err != nil {
		return nil, err
	}
	locales := installedLocales()
	add := func(kind, key, detail string, n int) {
		p.Facts = append(p.Facts, Fact{ID: "probe:" + kind + ":" + key, Kind: kind, At: casesFile, Detail: clip(detail, 200), Count: n})
	}
	p.Ran = append(p.Ran, fmt.Sprintf("every case twice with HOME a scratch directory, LC_ALL=C, TZ=UTC (%s)", filepath.Base(bin)))
	for _, v := range envProbes {
		switch {
		case v.name == "locale" && !locales["de_de.utf8"]:
			p.Skipped = append(p.Skipped, "locale: de_DE.UTF-8 is not installed here, so a locale dependency would not show (install it and run --probe again)")
			continue
		case v.name == "HOME":
			p.Ran = append(p.Ran, "every case with another HOME holding a dotfile")
		default:
			p.Ran = append(p.Ran, "every case with "+strings.Join(v.env, " "))
		}
	}
	strace, _ := exec.LookPath("strace")
	if strace != "" {
		p.Ran = append(p.Ran, "every case under strace -f -y -e trace=open,openat,creat")
	} else {
		p.Skipped = append(p.Skipped, "files read and written: strace is not installed")
	}
	reads, writes := map[string][]string{}, map[string][]string{}
	for _, k := range c.Cases {
		first, err := runCase(ctx, root, bin, home, k)
		if err != nil {
			return nil, err
		}
		second, err := runCase(ctx, root, bin, home, k)
		if err != nil {
			return nil, err
		}
		if ds := compareStreams(k, "first run", "second run", first, second); len(ds) > 0 {
			add("nondeterministic", k.ID, fmt.Sprintf("two identical runs differ on %s: %s", ds[0].Stream, ds[0].Detail), len(ds))
			continue // its output changes anyway, so the environment probes would show nothing about the environment
		}
		for _, v := range envProbes {
			h, env := home, v.env
			if v.name == "HOME" {
				h = home2
			} else if v.name == "locale" && !locales["de_de.utf8"] {
				continue
			}
			o, err := runCase(ctx, root, bin, h, k, env...)
			if err != nil {
				return nil, err
			}
			if ds := compareStreams(k, "as captured", "changed", first, o); len(ds) > 0 {
				with := strings.Join(env, " ")
				if with == "" {
					with = "another HOME"
				}
				add("env-"+strings.ToLower(v.name), k.ID, fmt.Sprintf("with %s, %s changes: %s", with, ds[0].Stream, ds[0].Detail), len(ds))
			}
		}
		if strace != "" {
			r, w, err := traceFiles(ctx, root, strace, bin, home, k)
			if err != nil {
				return nil, err
			}
			inputs := map[string]bool{filepath.ToSlash(filepath.Clean(k.Stdin)): k.Stdin != ""}
			for _, a := range k.Argv {
				inputs[filepath.ToSlash(filepath.Join(k.Cwd, a))] = true
			}
			for _, f := range r {
				if ix.isCode(f) || inputs[f] {
					continue // an interpreter loading the program's own source, or the input the case names, is not a dependency
				}
				reads[f] = append(reads[f], k.ID)
			}
			for _, f := range w {
				writes[f] = append(writes[f], k.ID)
			}
		}
		if opt.CheckBin != "" {
			o, err := runCase(ctx, root, opt.CheckBin, home, k)
			if err != nil {
				return nil, err
			}
			if m := checkerReport.Find(o.stderr); m != nil {
				add("checker", k.ID, fmt.Sprintf("%s reports: %s", filepath.Base(opt.CheckBin), strings.TrimSpace(string(m))), len(checkerReport.FindAll(o.stderr, -1)))
			}
			// The same code built another way should print the same: a
			// difference is undefined behaviour or a flag that changes results.
			out := k
			out.Compare = []string{}
			for _, st := range k.Compare {
				if st != "stderr" {
					out.Compare = append(out.Compare, st)
				}
			}
			if len(k.Compare) == 0 {
				out.Compare = []string{"stdout", "exit"} // stderr carries the checker's own report
			}
			if ds := compareStreams(out, "as captured", "checker build", first, o); len(out.Compare) > 0 && len(ds) > 0 {
				add("check-differs", k.ID, fmt.Sprintf("the checker build %s changes %s: %s", filepath.Base(opt.CheckBin), ds[0].Stream, ds[0].Detail), len(ds))
			}
		}
	}
	if opt.CheckBin != "" {
		p.Ran = append(p.Ran, "every case on the checker build "+filepath.Base(opt.CheckBin))
	}
	for _, kind := range []string{"reads", "writes"} {
		m := reads
		if kind == "writes" {
			m = writes
		}
		// Outside the checkout, many files under one directory are one
		// dependency (a class path, a gem directory): one fact for it.
		under := map[string][]string{}
		for f := range m {
			if filepath.IsAbs(f) {
				under[filepath.Dir(f)] = append(under[filepath.Dir(f)], f)
			}
		}
		grouped := map[string][]string{} // fact key → case IDs
		for f, ids := range m {
			if d := filepath.Dir(f); len(under[d]) > 5 {
				f = d + "/"
			}
			grouped[f] = append(grouped[f], ids...)
		}
		for _, f := range sortedKeys(grouped) {
			ids := slices.Compact(slices.Sorted(slices.Values(grouped[f])))
			what := f
			if strings.HasSuffix(f, "/") {
				what = fmt.Sprintf("%d files under %s", len(under[strings.TrimSuffix(f, "/")]), f)
			}
			add(kind, f, fmt.Sprintf("%s %s in %d case(s): %s", kind, what, len(ids), strings.Join(ids[:min(len(ids), 5)], ", ")), len(ids))
		}
	}
	if opt.Coverage != "" {
		n, err := unreached(ix, root, opt.Coverage, add)
		if err != nil {
			return nil, err
		}
		p.Ran = append(p.Ran, fmt.Sprintf("coverage from %s: %d function(s) the cases never reach", filepath.Base(opt.Coverage), n))
	}
	sort.SliceStable(p.Facts, func(i, j int) bool { return p.Facts[i].ID < p.Facts[j].ID })
	return p, nil
}

func installedLocales() map[string]bool {
	out, err := exec.Command("locale", "-a").Output()
	m := map[string]bool{}
	if err != nil {
		return m
	}
	for _, l := range strings.Fields(string(out)) {
		m[strings.ToLower(strings.ReplaceAll(l, "-", ""))] = true
	}
	return m
}

// straceOpen is an open, openat, or creat line of strace's output: the path
// and the flags.
var straceOpen = regexp.MustCompile(`(?:open|openat|creat)\((?:AT_FDCWD|\d+)?(?:<[^>]*>)?,?\s*"([^"]+)"(?:,\s*([A-Z_|]+))?[^=]*=\s*(-?\d+)(?:<([^>]*)>)?`)

// traceFiles runs one case under strace and lists the files it opened for
// reading and for writing, outside the system's own directories: the
// configuration, data, and dotfiles the program depends on.
func traceFiles(ctx context.Context, root, strace, bin, home string, k Case) (reads, writes []string, err error) {
	log, err := os.CreateTemp("", "aayu-strace-")
	if err != nil {
		return nil, nil, err
	}
	_ = log.Close()
	defer func() { _ = os.Remove(log.Name()) }()
	traced := k
	traced.Argv = append([]string{strace, "-f", "-qq", "-y", "-e", "trace=open,openat,creat", "-o", log.Name()}, k.Argv...)
	if _, err := runCase(ctx, root, bin, home, traced); err != nil {
		return nil, nil, err
	}
	f, err := os.Open(log.Name())
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = f.Close() }()
	seen := map[string]bool{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		m := straceOpen.FindStringSubmatch(sc.Text())
		if m == nil {
			continue
		}
		if n, _ := strconv.Atoi(m[3]); n < 0 {
			continue // a failed open: the program looked for it and went on
		}
		path := m[1]
		if m[4] != "" {
			path = m[4] // -y resolved it: right after a chdir, where the case's directory would be wrong
		} else if !filepath.IsAbs(path) {
			path = filepath.Join(root, k.Cwd, path)
		}
		if fi, err := os.Stat(path); err == nil && fi.IsDir() {
			continue // a directory opened to list it or to remember the working directory
		}
		name := systemPath(path, root, home)
		if name == "" || path == bin || seen[name+m[2]] {
			continue
		}
		seen[name+m[2]] = true
		if strings.Contains(m[2], "O_WRONLY") || strings.Contains(m[2], "O_RDWR") || strings.Contains(m[2], "O_CREAT") {
			writes = append(writes, name)
		} else {
			reads = append(reads, name)
		}
	}
	return reads, writes, sc.Err()
}

// isCode reports whether p is one of the checkout's source, test, or
// vendored files.
func (ix *Index) isCode(p string) bool {
	if ix == nil {
		return false
	}
	k, ok := ix.byPath[p]
	return ok && slices.Contains([]string{"source", "test", "vendored"}, ix.Files[k].Role)
}

// systemPath names an opened path for the report: inside the checkout
// relative to it, in the scratch HOME as ~/…, and "" for the system's own
// files (libraries, locales, /proc, /dev) and the program's own build.
func systemPath(p, root, home string) string {
	p = filepath.Clean(p)
	if rel, err := filepath.Rel(root, p); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	if rel, err := filepath.Rel(home, p); err == nil && !strings.HasPrefix(rel, "..") {
		return "~/" + filepath.ToSlash(rel)
	}
	for _, pre := range []string{"/usr/", "/lib", "/bin/", "/sbin/", "/proc/", "/sys/", "/dev/", "/etc/ld.so", "/etc/localtime", "/etc/locale", "/etc/passwd", "/etc/group", "/etc/nsswitch.conf", "/run/", "/tmp/aayu-", "/tmp/hsperfdata_", "/opt/", "/snap/", "/nix/"} {
		if strings.HasPrefix(p, pre) {
			return ""
		}
	}
	return p
}

// unreached reads an lcov tracefile and records each function of the index
// whose every instrumented line has a zero count: code the cases never reach.
func unreached(ix *Index, root, tracefile string, add func(kind, key, detail string, n int)) (int, error) {
	f, err := os.Open(tracefile)
	if err != nil {
		return 0, err
	}
	defer func() { _ = f.Close() }()
	hits := map[string]map[int]int{} // file → line → count
	cur := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "SF:"):
			p := filepath.Clean(strings.TrimPrefix(line, "SF:"))
			if rel, err := filepath.Rel(root, p); err == nil && !strings.HasPrefix(rel, "..") {
				p = rel
			}
			cur = filepath.ToSlash(p)
			if hits[cur] == nil {
				hits[cur] = map[int]int{}
			}
		case strings.HasPrefix(line, "DA:") && cur != "":
			a, b, _ := strings.Cut(strings.TrimPrefix(line, "DA:"), ",")
			n, _ := strconv.Atoi(a)
			c, _ := strconv.Atoi(strings.Split(b, ",")[0])
			hits[cur][n] += c
		}
	}
	if err := sc.Err(); err != nil {
		return 0, err
	}
	count := 0
	for _, s := range ix.Symbols {
		h, ok := hits[s.file]
		if !ok || !isDefKind(s.Kind) {
			continue
		}
		lines, reached := 0, false
		for ln := s.line; ln <= s.End; ln++ {
			if c, ok := h[ln]; ok {
				lines++
				reached = reached || c > 0
			}
		}
		if lines > 0 && !reached {
			count++
			add("unreached", strings.TrimPrefix(s.ID, "sym:"), fmt.Sprintf("%s %s (%s): %d instrumented lines, none run by the cases", s.Kind, s.Name, s.At, lines), lines)
		}
	}
	return count, nil
}

// LoadProbes adds the probe facts written under out/golden/*/probes.json to
// the index's citable IDs, so a draft may cite runtime evidence.
func (ix *Index) LoadProbes(out string) error {
	files, _ := filepath.Glob(filepath.Join(out, "golden", "*", "probes.json"))
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		var p Probes
		if err := json.Unmarshal(b, &p); err != nil {
			return fmt.Errorf("%s: %w", file, err)
		}
		for _, f := range p.Facts {
			ix.ids[f.ID] = true
		}
	}
	return nil
}
