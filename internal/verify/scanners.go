package verify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
)

// Severity counts of a scanner's findings.
type Severity struct {
	Critical, High, Medium, Low int
	Uncalled                    int       // osv-scanner call analysis: vulnerable code the module never calls (not counted)
	Top                         []string  // the first critical/high findings, for the pack detail
	Found                       []Finding // every critical/high finding, for the security workflow's Bugs
}

// Finding is one finding at a level that fails its floor: what the security
// workflow (`verify --security`) files a Bug for.
type Finding struct {
	Floor string `json:"floor"`
	ID    string `json:"id"`    // rule or advisory ID
	Path  string `json:"path"`  // the file or lockfile
	Where string `json:"where"` // file:line, or package@version
	Level string `json:"level"`
}

// Fingerprint identifies a finding across runs: its floor, ID, and path,
// never a line number, so a moved line stays the same finding.
func (f Finding) Fingerprint() string { return f.Floor + " " + f.ID + " " + f.Path }

// found records a finding; critical and high ones are kept.
func (s *Severity) found(level, what string, f Finding) {
	s.add(level, what)
	if level == "critical" || level == "high" {
		f.Level = level
		s.Found = append(s.Found, f)
	}
}

func (s *Severity) add(level, what string) {
	switch level {
	case "critical":
		s.Critical++
	case "high":
		s.High++
	case "medium":
		s.Medium++
	default:
		s.Low++
	}
	if (level == "critical" || level == "high") && len(s.Top) < 5 {
		s.Top = append(s.Top, what)
	}
}

func (s Severity) detail() string {
	d := fmt.Sprintf("%d critical, %d high, %d medium, %d low", s.Critical, s.High, s.Medium, s.Low)
	if s.Uncalled > 0 {
		d += fmt.Sprintf(" (%d more not called, by osv-scanner's call analysis)", s.Uncalled)
	}
	if len(s.Top) > 0 {
		d += ": " + strings.Join(s.Top, "; ")
	}
	return d
}

// severityCheck fails when critical or high findings exceed the floor.
func severityCheck(s Severity, maxCritical, maxHigh int) proof.Check {
	c := proof.Check{Value: float64(s.Critical + s.High), Threshold: float64(maxCritical + maxHigh), Detail: s.detail(), Result: proof.Pass}
	if s.Critical > maxCritical || s.High > maxHigh {
		c.Result = proof.Fail
	}
	return c
}

// over returns the findings of each severity whose maximum was exceeded: a
// severity still within its allowance files no Bug, even when the floor fails.
func (s Severity) over(maxCritical, maxHigh int) []Finding {
	var out []Finding
	for _, f := range s.Found {
		if (f.Level == "critical" && s.Critical > maxCritical) || (f.Level == "high" && s.High > maxHigh) {
			out = append(out, f)
		}
	}
	return out
}

// need checks that a tool is on the floor's PATH (the child's, not the parent's).
func (r *Run) need(tool string) error {
	if _, err := lookPath(tool, r.Sandbox.Env(Cmd{}, NetAllowed)); err != nil {
		return fmt.Errorf("%s is not installed: the floor cannot run, so it does not pass (install the pinned version; `aayu doctor` lists optional tools)", tool)
	}
	return nil
}

// semgrepPacks are the registry rule packs per language present in the checkout.
var semgrepPacks = []struct {
	pack string
	exts []string
}{
	{"p/golang", []string{".go"}},
	{"p/javascript", []string{".js", ".jsx", ".mjs", ".cjs"}},
	{"p/typescript", []string{".ts", ".tsx"}},
	{"p/python", []string{".py"}},
	{"p/java", []string{".java"}},
	{"p/kotlin", []string{".kt"}},
	{"p/ruby", []string{".rb"}},
	{"p/php", []string{".php"}},
	{"p/csharp", []string{".cs"}},
	{"p/swift", []string{".swift"}},
	{"p/rust", []string{".rs"}},
}

// SemgrepConfigs returns the rule packs for the languages of files tracked in root.
func SemgrepConfigs(ctx context.Context, root string) []string {
	files, _ := gitOut(ctx, root, "ls-files")
	var packs []string
	list := string(files)
	for _, p := range semgrepPacks {
		for _, e := range p.exts {
			if strings.Contains(list, e+"\n") {
				packs = append(packs, p.pack)
				break
			}
		}
	}
	if strings.Contains(list, ".github/workflows/") {
		packs = append(packs, "p/github-actions")
	}
	return packs
}

// SAST is the semgrep floor (WP4.4): zero critical and high findings.
type SAST struct{}

// Name implements Floor.
func (SAST) Name() string { return "sast" }

// Applies implements Floor.
func (SAST) Applies(*Run) (bool, string) { return true, "" }

// Run implements Floor.
func (SAST) Run(ctx context.Context, r *Run) (Result, error) {
	res := Result{Check: proof.Check{Tool: "semgrep"}}
	if err := r.need("semgrep"); err != nil {
		return res, err
	}
	res.Check.Version = version(ctx, r, "semgrep", "--version")
	packs := SemgrepConfigs(ctx, r.Root)
	if len(packs) == 0 {
		res.Check.Result, res.Check.Detail = proof.Pass, "no language semgrep has rules for"
		return res, nil
	}
	out := r.tempFile("semgrep.json")
	args := []string{"scan", "--json", "--output", out, "--metrics=off", "--disable-version-check", "--quiet"}
	for _, p := range packs {
		args = append(args, "--config", p)
	}
	// The registry rule packs are downloaded, so this floor needs the network.
	o, err := r.Sandbox.Run(ctx, Cmd{Dir: r.Root, Name: "semgrep", Args: args, Network: true})
	if err != nil {
		return res, err
	}
	b := readFile(out)
	var doc struct {
		Results []struct {
			CheckID string `json:"check_id"`
			Path    string `json:"path"`
			Start   struct {
				Line int `json:"line"`
			} `json:"start"`
			Extra struct {
				Severity string `json:"severity"`
				Metadata struct {
					Impact string `json:"impact"`
				} `json:"metadata"`
			} `json:"extra"`
		} `json:"results"`
		Errors []semgrepError `json:"errors"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return res, fmt.Errorf("semgrep exited %d without a JSON report: %s", o.Code, tail(o.Stderr))
	}
	if gap := semgrepGaps(o.Code, doc.Errors); gap != "" {
		return res, fmt.Errorf("semgrep left part of the tree unanalysed (%s), so the floor cannot pass: fix the file or list it in .semgrepignore", gap)
	}
	var s Severity
	for _, f := range doc.Results {
		s.found(semgrepLevel(f.Extra.Severity, f.Extra.Metadata.Impact), fmt.Sprintf("%s %s:%d", f.CheckID, f.Path, f.Start.Line), Finding{Floor: "sast", ID: f.CheckID, Path: f.Path, Where: fmt.Sprintf("%s:%d", f.Path, f.Start.Line)})
	}
	c := severityCheck(s, r.Floors.SAST.MaxCritical, r.Floors.SAST.MaxHigh)
	c.Tool, c.Version, c.Network = res.Check.Tool, res.Check.Version, o.Network
	c.Detail = strings.Join(packs, " ") + ": " + c.Detail
	res.Check, res.Findings = c, s.over(r.Floors.SAST.MaxCritical, r.Floors.SAST.MaxHigh)
	res.Artifacts = append(res.Artifacts, Evidence{Path: "reports/semgrep.json", Kind: "report", Data: b})
	return res, nil
}

// semgrepLevel maps a semgrep severity to the floor's levels: CRITICAL is
// critical; ERROR and HIGH are high (ERROR is what semgrep --error blocks on);
// WARNING and MEDIUM are medium; the rest low.
// semgrepError is one entry of a semgrep report's errors.
type semgrepError struct {
	Level string          `json:"level"`
	Type  json.RawMessage `json:"type"` // a name, or [name, details] (PartialParsing)
	Path  string          `json:"path"`
}

// semgrepUnanalysed are the error types that leave a file, or a rule's run on
// it, unfinished: a finding there would go unreported. A partly parsed file
// (PartialParsing, common for scripts embedded in YAML) or a rule that failed
// to match one construct (Internal matching error) is still analysed.
var semgrepUnanalysed = []string{"Lexical error", "Syntax error", "Other syntax error", "AST builder error", "Timeout",
	"Out of memory", "Stack overflow", "Timeout during interfile analysis", "OOM during interfile analysis", "Too many matches", "Fatal error"}

// semgrepGaps says what a scan left unanalysed: an exit of 2 or more, an error
// at level error, or an error of a semgrep Unanalysed type; "" when none.
func semgrepGaps(code int, errs []semgrepError) string {
	var gaps []string
	if code >= 2 {
		gaps = append(gaps, fmt.Sprintf("exit %d", code))
	}
	for _, e := range errs {
		var name string
		if json.Unmarshal(e.Type, &name) != nil {
			var parts []json.RawMessage
			if json.Unmarshal(e.Type, &parts) == nil && len(parts) > 0 {
				_ = json.Unmarshal(parts[0], &name)
			}
		}
		if e.Level == "error" || slices.Contains(semgrepUnanalysed, name) {
			gaps = append(gaps, strings.TrimSpace(name+" "+e.Path))
		}
	}
	if len(gaps) > 3 {
		gaps = append(gaps[:3], fmt.Sprintf("%d more", len(gaps)-3))
	}
	return strings.Join(gaps, "; ")
}

func semgrepLevel(sev, impact string) string {
	switch strings.ToUpper(sev) {
	case "CRITICAL":
		return "critical"
	case "ERROR", "HIGH":
		if strings.EqualFold(impact, "CRITICAL") {
			return "critical"
		}
		return "high"
	case "WARNING", "MEDIUM":
		return "medium"
	}
	return "low"
}

// Secrets is the gitleaks floor (WP4.4): no secrets in the PR's commits.
type Secrets struct{}

// Name implements Floor.
func (Secrets) Name() string { return "secrets" }

// Applies implements Floor.
func (Secrets) Applies(*Run) (bool, string) { return true, "" }

// Run scans the commits between base and head (gitleaks git --log-opts).
func (Secrets) Run(ctx context.Context, r *Run) (Result, error) {
	res := Result{Check: proof.Check{Tool: "gitleaks", Threshold: float64(r.Floors.Secrets.MaxFindings)}}
	if err := r.need("gitleaks"); err != nil {
		return res, err
	}
	res.Check.Version = version(ctx, r, "gitleaks", "version")
	out := r.tempFile("gitleaks.json")
	args := []string{"git", "--no-banner", "--redact", "--exit-code", "0", "--report-format", "json", "--report-path", out}
	// Without a base, the whole history of the commit under test: never
	// `git log --all`, which would also scan every other fetched branch.
	logOpts := r.Head
	if r.Base != "" {
		logOpts = r.Base + ".." + r.Head
	}
	if logOpts != "" {
		args = append(args, "--log-opts", logOpts)
	}
	args = append(args, ".")
	o, err := r.Sandbox.Run(ctx, Cmd{Dir: r.Root, Name: "gitleaks", Args: args})
	if err != nil {
		return res, err
	}
	b := readFile(out)
	var findings []struct {
		RuleID string `json:"RuleID"`
		File   string `json:"File"`
		Line   int    `json:"StartLine"`
		Commit string `json:"Commit"`
	}
	if err := json.Unmarshal(b, &findings); err != nil {
		return res, fmt.Errorf("gitleaks exited %d without a JSON report: %s", o.Code, tail(o.Stderr))
	}
	// --exit-code 0: any other exit is gitleaks failing, whatever report it left.
	if o.Code != 0 {
		return res, fmt.Errorf("gitleaks exited %d, so its report is not a complete scan: %s", o.Code, tail(o.Stderr))
	}
	res.Check.Network = o.Network
	res.Check.Value = float64(len(findings))
	res.Check.Result = proof.Pass
	var top []string
	for _, f := range findings {
		res.Findings = append(res.Findings, Finding{Floor: "secrets", ID: f.RuleID, Path: f.File, Where: fmt.Sprintf("%s:%d (%s)", f.File, f.Line, short(f.Commit)), Level: "high"})
		if len(top) < 5 {
			top = append(top, fmt.Sprintf("%s %s:%d (%s)", f.RuleID, f.File, f.Line, short(f.Commit)))
		}
	}
	res.Check.Detail = plural(len(findings), "finding")
	if len(top) > 0 {
		res.Check.Detail += ": " + strings.Join(top, "; ")
	}
	if len(findings) > r.Floors.Secrets.MaxFindings {
		res.Check.Result = proof.Fail
	}
	res.Artifacts = append(res.Artifacts, Evidence{Path: "reports/gitleaks.json", Kind: "report", Data: b}) // --redact: no secret values
	return res, nil
}

// Deps is the osv-scanner floor (WP4.4): zero critical and high
// vulnerabilities in dependencies.
type Deps struct{}

// Name implements Floor.
func (Deps) Name() string { return "deps" }

// Applies implements Floor.
func (Deps) Applies(*Run) (bool, string) { return true, "" }

// Run implements Floor.
func (Deps) Run(ctx context.Context, r *Run) (Result, error) {
	res := Result{Check: proof.Check{Tool: "osv-scanner"}}
	if err := r.need("osv-scanner"); err != nil {
		return res, err
	}
	res.Check.Version = strings.TrimPrefix(version(ctx, r, "osv-scanner", "--version"), "osv-scanner version: ")
	out := r.tempFile("osv.json")
	// The lockfiles git tracks are named one by one: a recursive scan follows
	// .gitignore rules, which can hide the whole checkout (a build worktree
	// inside an ignored directory) and turn the floor into a silent pass.
	lockfiles := TrackedLockfiles(ctx, r.Root)
	if len(lockfiles) == 0 {
		res.Check.Result, res.Check.Detail, res.Check.Network = proof.Pass, "no dependency lockfiles tracked in git", NetAllowed
		return res, nil
	}
	args := []string{"scan", "source", "--format", "json", "--output", out}
	for _, l := range lockfiles {
		args = append(args, "--lockfile", l)
	}
	// The OSV database is online, so this floor needs the network.
	o, err := r.Sandbox.Run(ctx, Cmd{Dir: r.Root, Name: "osv-scanner", Args: args, Network: true})
	if err != nil {
		return res, err
	}
	b := readFile(out)
	s, err := ParseOSV(b)
	if err != nil {
		return res, fmt.Errorf("osv-scanner exited %d without a JSON report for %s: %s", o.Code, strings.Join(lockfiles, ", "), tail(o.Stderr))
	}
	// osv-scanner's return codes: 0 none found, 1 vulnerabilities found, 128 no packages in the lockfiles; any other
	// (127 is its general error) means the scan did not finish, whatever report it left.
	if o.Code != 0 && o.Code != 1 && o.Code != 128 {
		return res, fmt.Errorf("osv-scanner exited %d for %s, so its report is not a complete scan: %s", o.Code, strings.Join(lockfiles, ", "), tail(o.Stderr))
	}
	c := severityCheck(s, r.Floors.Deps.MaxCritical, r.Floors.Deps.MaxHigh)
	c.Tool, c.Version, c.Network = res.Check.Tool, res.Check.Version, o.Network
	c.Detail = strings.Join(lockfiles, " ") + ": " + c.Detail
	res.Check, res.Findings = c, s.over(r.Floors.Deps.MaxCritical, r.Floors.Deps.MaxHigh)
	res.Artifacts = append(res.Artifacts, Evidence{Path: "reports/osv-scanner.json", Kind: "report", Data: b})
	return res, nil
}

// lockfileNames are the dependency manifests and lockfiles osv-scanner reads.
var lockfileNames = []string{"go.mod", "package-lock.json", "pnpm-lock.yaml", "yarn.lock", "bun.lock", "requirements.txt", "poetry.lock",
	"Pipfile.lock", "uv.lock", "Cargo.lock", "Gemfile.lock", "composer.lock", "pom.xml", "gradle.lockfile", "packages.lock.json",
	"pubspec.lock", "mix.lock", "renv.lock", "Package.resolved"}

// TrackedLockfiles lists the lockfiles git tracks in root, outside vendored
// and test-data trees.
func TrackedLockfiles(ctx context.Context, root string) []string {
	files, _ := gitOut(ctx, root, "ls-files")
	var out []string
	for _, f := range strings.Split(strings.TrimSpace(string(files)), "\n") {
		if f == "" || strings.Contains("/"+f, "/testdata/") || strings.Contains("/"+f, "/vendor/") || strings.Contains("/"+f, "/node_modules/") {
			continue
		}
		if slices.Contains(lockfileNames, path.Base(f)) {
			out = append(out, f)
		}
	}
	return out
}

// ParseOSV counts osv-scanner findings by severity: one per vulnerability
// group, from its CVSS max_severity score (≥ 9 critical, ≥ 7 high, ≥ 4
// medium), else the advisory's database severity label. Groups the call
// analysis reports as not called are counted apart and do not fail.
func ParseOSV(b []byte) (Severity, error) {
	var doc struct {
		Results []struct {
			Source struct {
				Path string `json:"path"`
			} `json:"source"`
			Packages []struct {
				Package struct {
					Name      string `json:"name"`
					Version   string `json:"version"`
					Ecosystem string `json:"ecosystem"`
				} `json:"package"`
				Groups []struct {
					IDs         []string `json:"ids"`
					MaxSeverity string   `json:"max_severity"`
					Analysis    map[string]struct {
						Called      bool `json:"called"`
						Unimportant bool `json:"unimportant"`
					} `json:"experimental_analysis"`
				} `json:"groups"`
				Vulnerabilities []struct {
					ID               string `json:"id"`
					DatabaseSpecific struct {
						Severity string `json:"severity"`
					} `json:"database_specific"`
				} `json:"vulnerabilities"`
			} `json:"packages"`
		} `json:"results"`
	}
	var s Severity
	if err := json.Unmarshal(b, &doc); err != nil || b == nil {
		return s, errors.Join(errors.New("osv-scanner report"), err)
	}
	for _, res := range doc.Results {
		for _, p := range res.Packages {
			for _, g := range p.Groups {
				// Call analysis (Go): a group whose every advisory is reported not
				// called, or unimportant, does not fail the floor, as it does not
				// fail osv-scanner itself.
				quiet := len(g.Analysis) > 0
				for _, id := range g.IDs {
					a, ok := g.Analysis[id]
					quiet = quiet && ok && (!a.Called || a.Unimportant)
				}
				if quiet {
					s.Uncalled++
					continue
				}
				level := ""
				if score, err := strconv.ParseFloat(g.MaxSeverity, 64); err == nil {
					switch {
					case score >= 9:
						level = "critical"
					case score >= 7:
						level = "high"
					case score >= 4:
						level = "medium"
					default:
						level = "low"
					}
				} else {
					rank := map[string]int{"": 0, "low": 1, "medium": 2, "high": 3, "critical": 4}
					for _, v := range p.Vulnerabilities {
						if !slices.Contains(g.IDs, v.ID) {
							continue
						}
						l := map[string]string{"CRITICAL": "critical", "HIGH": "high", "MODERATE": "medium", "MEDIUM": "medium", "LOW": "low"}[strings.ToUpper(v.DatabaseSpecific.Severity)]
						if rank[l] > rank[level] {
							level = l
						}
					}
					if level == "" {
						level = "high" // unscored: treated as high until someone looks (rule 7 errs closed)
					}
				}
				// The ID is the group's least ID, so a finding keeps its fingerprint while
				// its version changes; the path is the lockfile.
				ids := slices.Sorted(slices.Values(g.IDs))
				id := strings.Join(ids, ",")
				if len(ids) > 0 {
					id = ids[0] + " " + p.Package.Name
				}
				what := fmt.Sprintf("%s %s@%s (%s)", strings.Join(g.IDs, ","), p.Package.Name, p.Package.Version, p.Package.Ecosystem)
				s.found(level, what, Finding{Floor: "deps", ID: id, Path: res.Source.Path, Where: what})
			}
		}
	}
	return s, nil
}

func short(s string) string {
	if len(s) > 7 {
		return s[:7]
	}
	return s
}

// Standard is every floor, in the order verify runs them.
func Standard() []Floor {
	return []Floor{Coverage{}, SAST{}, Secrets{}, Deps{}, A11y{}, Perf{}, ScreenReader{}, DAST{}, API{}, Mobile{}, MapFloor{}, Behaviour{}, Parity{}}
}

// SecurityFloors are the floors `verify --security` runs on a pushed commit
// and nightly (aayudha-security.yml, Decision #480).
func SecurityFloors() []Floor { return []Floor{SAST{}, Secrets{}, Deps{}} }

// ByName selects one floor.
func ByName(floors []Floor, name string) ([]Floor, error) {
	for _, f := range floors {
		if f.Name() == name {
			return []Floor{f}, nil
		}
	}
	var names []string
	for _, f := range floors {
		names = append(names, f.Name())
	}
	return nil, fmt.Errorf("no floor %q (floors: %s)", name, strings.Join(names, ", "))
}
