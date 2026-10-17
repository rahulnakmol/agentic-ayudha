package survey

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Cases is a characterisation cases file (--golden, --parity): inputs that
// run the legacy build, whose outputs become the golden files a new build must
// reproduce. The person or the host's model writes it from the survey's case
// sets; the binary only runs it.
type Cases struct {
	Cases []Case `json:"cases"`
}

// Case is one run of the program under test.
type Case struct {
	ID      string   `json:"id"`                // a file-safe name: test001, md-emphasis
	Set     string   `json:"set"`               // the survey's case set: G1, G2, …
	Argv    []string `json:"argv"`              // {bin} is replaced by --bin
	Stdin   string   `json:"stdin,omitempty"`   // a file under the surveyed checkout fed to stdin
	Cwd     string   `json:"cwd,omitempty"`     // relative to the surveyed checkout; default its root
	Compare []string `json:"compare,omitempty"` // stdout, stderr, exit; default all three
	Timeout int      `json:"timeout_s,omitempty"`
	// Env is what the case needs beyond the hermetic default (PATH, a scratch
	// HOME, LC_ALL=C, TZ=UTC), as NAME=value: GEM_PATH, JAVA_HOME, a config
	// file's variable. Each one is a dependency the survey can cite.
	Env []string `json:"env,omitempty"`
	// Area groups a compatibility suite's tests for its score (--compat);
	// Unfixable says why a test can never pass on the new build, so it is
	// left out of the score.
	Area      string `json:"area,omitempty"`
	Unfixable string `json:"unfixable,omitempty"`
}

// Golden is the manifest of a capture: what ran, from which commit, and the
// SHA-256 of each case's outputs.
type Golden struct {
	Bin       string         `json:"bin"`
	BinSHA256 string         `json:"bin_sha256,omitempty"`
	Commit    string         `json:"commit,omitempty"` // the surveyed checkout's commit
	Captured  string         `json:"captured"`
	Cases     []GoldenResult `json:"cases"`
}

// GoldenResult is one case's recorded outputs.
type GoldenResult struct {
	Case
	Exit         int    `json:"exit"`
	StdoutSHA256 string `json:"stdout_sha256"`
	StderrSHA256 string `json:"stderr_sha256"`
	TimedOut     bool   `json:"timed_out,omitempty"`
}

// Diff is a case whose new output differs from its golden output.
type Diff struct {
	ID     string `json:"id"`
	Set    string `json:"set"`
	Stream string `json:"stream"` // stdout, stderr, exit, missing
	Detail string `json:"detail"`
}

var caseID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$`)

// maxStream caps what one case may write to stdout or stderr.
const maxStream = 8 << 20

// ParseCases reads and checks a cases file: unique file-safe IDs, a case set
// each, an argv that runs {bin}, paths inside the checkout, and known compare
// streams.
func ParseCases(b []byte) (Cases, []string, error) {
	var c Cases
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return c, nil, err
	}
	var probs []string
	add := func(f string, a ...any) { probs = append(probs, fmt.Sprintf(f, a...)) }
	if len(c.Cases) == 0 {
		add("no cases")
	}
	seen := map[string]bool{}
	for _, k := range c.Cases {
		if !caseID.MatchString(k.ID) || seen[k.ID] {
			add("case %q: IDs are file-safe ([A-Za-z0-9._-]) and unique", k.ID)
		}
		seen[k.ID] = true
		if !gID.MatchString(k.Set) && !sID.MatchString(k.Set) {
			add("case %s: set must name the survey's case set (G1, G2, …) or suite (S1, …)", k.ID)
		}
		if sID.MatchString(k.Set) && k.Area == "" {
			add("case %s: a suite's test names its area, the group it is scored in", k.ID)
		}
		if !strings.Contains(strings.Join(k.Argv, "\x00"), "{bin}") {
			add("case %s: argv must run {bin}", k.ID)
		}
		for _, p := range []string{k.Stdin, k.Cwd} {
			if p != "" && (filepath.IsAbs(p) || strings.HasPrefix(filepath.Clean(p), "..")) {
				add("case %s: %q must be a path inside the surveyed checkout", k.ID, p)
			}
		}
		for _, s := range k.Compare {
			if s != "stdout" && s != "stderr" && s != "exit" {
				add("case %s: compare %q is not stdout, stderr, or exit", k.ID, s)
			}
		}
		for _, kv := range k.Env {
			if name, _, ok := strings.Cut(kv, "="); !ok || name == "" {
				add("case %s: env %q is not NAME=value", k.ID, kv)
			}
		}
		if k.Timeout < 0 || k.Timeout > 600 {
			add("case %s: timeout_s is 1 to 600 (default 30)", k.ID)
		}
	}
	return c, probs, nil
}

// output is what one run of a case produced.
type output struct {
	stdout, stderr []byte
	exit           int
	timedOut       bool
}

// RunCase runs one case against bin in root, in a fixed environment: PATH
// only from the caller, with HOME a scratch directory, LC_ALL=C, and TZ=UTC,
// so no token or credential reaches the program and its output does not
// depend on who ran it.
func runCase(ctx context.Context, root, bin, home string, k Case, env ...string) (output, error) {
	argv := make([]string, len(k.Argv))
	for i, a := range k.Argv {
		argv[i] = strings.ReplaceAll(a, "{bin}", bin)
	}
	t := time.Duration(k.Timeout) * time.Second
	if t == 0 {
		t = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, t)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = filepath.Join(root, k.Cwd)
	cmd.Env = withEnv(withEnv([]string{"PATH=" + os.Getenv("PATH"), "HOME=" + home, "LC_ALL=C", "LANG=C", "TZ=UTC"}, k.Env), env)
	var out, errb capped
	cmd.Stdout, cmd.Stderr = &out, &errb
	if k.Stdin != "" {
		f, err := os.Open(filepath.Join(root, k.Stdin))
		if err != nil {
			return output{}, fmt.Errorf("case %s: %w", k.ID, err)
		}
		defer func() { _ = f.Close() }()
		cmd.Stdin = f
	}
	err := cmd.Run()
	o := output{stdout: out.Bytes(), stderr: errb.Bytes(), timedOut: ctx.Err() == context.DeadlineExceeded}
	var ee *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &ee):
		o.exit = ee.ExitCode()
	case o.timedOut:
		o.exit = -1
	default:
		return o, fmt.Errorf("case %s: %w", k.ID, err)
	}
	if out.over || errb.over {
		return o, fmt.Errorf("case %s: output over %d bytes; narrow the case", k.ID, maxStream)
	}
	return o, nil
}

// capped is a buffer that stops growing at maxStream.
type capped struct {
	bytes.Buffer
	over bool
}

func (c *capped) Write(p []byte) (int, error) {
	if c.Len()+len(p) > maxStream {
		c.over = true
		return len(p), nil
	}
	return c.Buffer.Write(p)
}

func sum(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

func fileSum(p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return sum(b)
}

// Capture runs every case against the legacy bin and writes dir/<id>.stdout,
// dir/<id>.stderr, and dir/manifest.json.
func Capture(ctx context.Context, root, bin, commit, dir string, c Cases) (*Golden, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	home, err := os.MkdirTemp("", "aayu-golden-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(home) }()
	g := &Golden{Bin: bin, BinSHA256: fileSum(bin), Commit: commit, Captured: time.Now().UTC().Format(time.RFC3339)}
	for _, k := range c.Cases {
		o, err := runCase(ctx, root, bin, home, k)
		if err != nil {
			return nil, err
		}
		for ext, b := range map[string][]byte{".stdout": o.stdout, ".stderr": o.stderr} {
			if err := os.WriteFile(filepath.Join(dir, k.ID+ext), b, 0o644); err != nil {
				return nil, err
			}
		}
		g.Cases = append(g.Cases, GoldenResult{Case: k, Exit: o.exit, StdoutSHA256: sum(o.stdout), StderrSHA256: sum(o.stderr), TimedOut: o.timedOut})
	}
	b, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return nil, err
	}
	return g, os.WriteFile(filepath.Join(dir, "manifest.json"), append(b, '\n'), 0o644)
}

// Replay runs every case against the new bin and compares what each case
// compares with the golden capture in dir. A case the capture lacks is a
// difference, so parity cannot pass on cases that were never captured.
func Replay(ctx context.Context, root, bin, dir string, c Cases) ([]Diff, *Golden, error) {
	b, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return nil, nil, fmt.Errorf("no golden capture: %w; run --golden with the legacy build first", err)
	}
	var g Golden
	if err := json.Unmarshal(b, &g); err != nil {
		return nil, nil, err
	}
	got := map[string]GoldenResult{}
	for _, r := range g.Cases {
		got[r.ID] = r
	}
	home, err := os.MkdirTemp("", "aayu-parity-")
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = os.RemoveAll(home) }()
	var diffs []Diff
	for _, k := range c.Cases {
		want, ok := got[k.ID]
		if !ok {
			diffs = append(diffs, Diff{ID: k.ID, Set: k.Set, Stream: "missing", Detail: "no golden output for this case"})
			continue
		}
		o, err := runCase(ctx, root, bin, home, k)
		if err != nil {
			return nil, nil, err
		}
		compare := k.Compare
		if len(compare) == 0 {
			compare = []string{"stdout", "stderr", "exit"}
		}
		for _, s := range compare {
			switch s {
			case "exit":
				if o.exit != want.Exit {
					diffs = append(diffs, Diff{ID: k.ID, Set: k.Set, Stream: "exit", Detail: fmt.Sprintf("exit %d, golden %d", o.exit, want.Exit)})
				}
			case "stdout", "stderr":
				have := o.stdout
				wantSum := want.StdoutSHA256
				if s == "stderr" {
					have, wantSum = o.stderr, want.StderrSHA256
				}
				if sum(have) != wantSum {
					gold, _ := os.ReadFile(filepath.Join(dir, k.ID+"."+s))
					diffs = append(diffs, Diff{ID: k.ID, Set: k.Set, Stream: s, Detail: firstDiff(gold, have)})
				}
			}
		}
	}
	return diffs, &g, nil
}

// firstDiff names the first line where two outputs part, clipped.
func firstDiff(want, have []byte) string { return lineDiff("golden", "got", want, have) }

// lineDiff names the first line where two labelled outputs part, clipped.
func lineDiff(aName, bName string, want, have []byte) string {
	wl, hl := strings.Split(string(want), "\n"), strings.Split(string(have), "\n")
	for i := 0; i < len(wl) || i < len(hl); i++ {
		var w, h string
		if i < len(wl) {
			w = wl[i]
		}
		if i < len(hl) {
			h = hl[i]
		}
		if w != h || i >= len(wl) || i >= len(hl) {
			return fmt.Sprintf("line %d: %s %q, %s %q", i+1, aName, clipRaw(w), bName, clipRaw(h))
		}
	}
	return "outputs differ"
}

// clipRaw keeps whitespace, which may be the difference.
func clipRaw(s string) string {
	if r := []rune(s); len(r) > 80 {
		return string(r[:80]) + "…"
	}
	return s
}

// withEnv replaces or adds KEY=value entries.
func withEnv(base, over []string) []string {
	out := append([]string(nil), base...)
	for _, kv := range over {
		k, _, _ := strings.Cut(kv, "=")
		replaced := false
		for i, b := range out {
			if strings.HasPrefix(b, k+"=") {
				out[i], replaced = kv, true
			}
		}
		if !replaced {
			out = append(out, kv)
		}
	}
	return out
}

// compareStreams lists the streams a case compares on which two runs differ;
// the names label the runs in each difference.
func compareStreams(k Case, aName, bName string, a, b output) []Diff {
	compare := k.Compare
	if len(compare) == 0 {
		compare = []string{"stdout", "stderr", "exit"}
	}
	var ds []Diff
	for _, s := range compare {
		switch s {
		case "exit":
			if a.exit != b.exit {
				ds = append(ds, Diff{ID: k.ID, Set: k.Set, Stream: "exit", Detail: fmt.Sprintf("%s exit %d, %s exit %d", aName, a.exit, bName, b.exit)})
			}
		case "stdout":
			if !bytes.Equal(a.stdout, b.stdout) {
				ds = append(ds, Diff{ID: k.ID, Set: k.Set, Stream: "stdout", Detail: lineDiff(aName, bName, a.stdout, b.stdout)})
			}
		case "stderr":
			if !bytes.Equal(a.stderr, b.stderr) {
				ds = append(ds, Diff{ID: k.ID, Set: k.Set, Stream: "stderr", Detail: lineDiff(aName, bName, a.stderr, b.stderr)})
			}
		}
	}
	return ds
}
