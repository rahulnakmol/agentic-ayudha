package evals

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/pelletier/go-toml/v2"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/cli"
	"github.com/rahulnakmol/agentic-ayudha/internal/models"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
	"github.com/rahulnakmol/agentic-ayudha/internal/verify"
)

// A session case (D-3.4f) runs a host skill as it runs in a terminal: the
// skill replies in turns, each saying something to the person, writing files,
// and running commands; the harness answers every command from the case's
// world and replies with the results. The world is session.toml's recorded
// replies (commands that need a machine an eval does not have: a lease, a
// worktree, a product to drive), else the real `aayu` binary in-process
// against a fake GitHub serving the case's github/ files, in a scratch copy of
// the case's repository fixture. Nothing the skill writes is executed: Go
// files are parsed, never built, and an evidence map is checked statically.
// The hard rules the skill's verb already enforces are floors, checked on the
// recorded session in tier 1 and on every tier-2 run before the judge.

// SessionSpec is a session case's session.toml.
type SessionSpec struct {
	// MaxTurns caps the skill's turns; after it the person asks for the report.
	MaxTurns int `toml:"max_turns"`
	// Writes allows GitHub writes (a write verb's requests to the fake GitHub).
	Writes bool `toml:"writes"`
	// Files is a regular expression over the repository paths the skill may
	// write; empty allows none.
	Files string `toml:"files"`
	// Replies answer the commands that cannot run in an eval, first match first.
	Replies []SessionReply `toml:"reply"`
	// Human are the person's replies, in order, to a turn that runs nothing.
	Human []string `toml:"human"`
	// Require: a command matching each must run. Forbid: none may. SayForbid:
	// nothing the skill says may match.
	Require   []Scope `toml:"require"`
	Forbid    []Scope `toml:"forbid"`
	SayForbid []Scope `toml:"say_forbid"`
	// Evidence makes the finished session's evidence map a floor: every
	// criterion maps to a test or behaviour check that exists.
	Evidence *SessionEvidence `toml:"evidence"`
	// Overlay is a directory, relative to the case, copied over the case's
	// repository fixture (the product as this case's branch has changed it).
	Overlay string `toml:"overlay"`
	// MapClean: `aayu verify --map` must be clean when the session ends.
	MapClean bool `toml:"map_clean"`
	// KeepChecks: every behaviour check the fixture's feature maps hold must
	// still be there when the session ends (a map is never weakened).
	KeepChecks bool `toml:"keep_checks"`

	files *regexp.Regexp
}

// SessionReply is a recorded answer to a command.
type SessionReply struct {
	Match string `toml:"match"`
	// After, when set, applies the reply only once an earlier command matching
	// it exited 0 (the second `aayu build 41` commits; the first takes the lease).
	After string `toml:"after"`
	Code  int    `toml:"code"`
	Out   string `toml:"out"`
	// Evidence runs the static evidence check first: a miss answers exit 3 with
	// the problems instead of Out, as verify's evidence floor would.
	Evidence bool `toml:"evidence"`

	match, after *regexp.Regexp
}

// SessionEvidence names the story whose evidence map the session writes.
type SessionEvidence struct {
	Story    int `toml:"story"`
	Criteria int `toml:"criteria"`
}

// LoadSession reads a session.toml.
func LoadSession(file string) (*SessionSpec, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var s SessionSpec
	dec := toml.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&s); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	if s.MaxTurns < 1 {
		return nil, fmt.Errorf("%s: max_turns >= 1 is required", file)
	}
	if s.Files != "" {
		if s.files, err = regexp.Compile(s.Files); err != nil {
			return nil, fmt.Errorf("%s: files: %w", file, err)
		}
	}
	for i := range s.Replies {
		r := &s.Replies[i]
		if r.match, err = regexp.Compile(r.Match); err != nil {
			return nil, fmt.Errorf("%s: reply %d: %w", file, i+1, err)
		}
		if r.After != "" {
			if r.after, err = regexp.Compile(r.After); err != nil {
				return nil, fmt.Errorf("%s: reply %d after: %w", file, i+1, err)
			}
		}
	}
	for _, rules := range []*[]Scope{&s.Require, &s.Forbid, &s.SayForbid} {
		for i, r := range *rules {
			if (*rules)[i].re, err = regexp.Compile(r.Pattern); err != nil {
				return nil, fmt.Errorf("%s: rule %q: %w", file, r.Pattern, err)
			}
			if r.Why == "" {
				return nil, fmt.Errorf("%s: rule %q: why is required", file, r.Pattern)
			}
			if r.Allow != "" {
				if (*rules)[i].allowRE, err = regexp.Compile(r.Allow); err != nil {
					return nil, fmt.Errorf("%s: rule %q allow: %w", file, r.Pattern, err)
				}
			}
		}
	}
	return &s, nil
}

// SessionTurn is one turn of the skill.
type SessionTurn struct {
	Say      string        `json:"say"`
	Files    []SessionFile `json:"files,omitempty"`
	Commands []string      `json:"commands,omitempty"`
	Done     bool          `json:"done"`
}

// SessionFile is a file the skill writes, by its path in the repository.
type SessionFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// CommandResult is what a command answered.
type CommandResult struct {
	Command string `json:"command"`
	Code    int    `json:"code"`
	Out     string `json:"out"`

	edits int // files written before it ran
}

// SessionStep is a turn, what its commands answered, and the person's reply.
type SessionStep struct {
	Turn    SessionTurn     `json:"turn"`
	Results []CommandResult `json:"results,omitempty"`
	Human   string          `json:"human,omitempty"`
}

const sessionProtocol = `## This session

You are working in the person's terminal through a channel that carries JSON. Each reply is one turn:
{"say": "...", "files": [{"path": "<repository path>", "content": "<the whole file>"}], "commands": ["aayu ..."], "done": false}
"say" is what you tell the person this turn. "files" are written before "commands" run, in order; a path is relative
to the repository root (the story's worktree when you build). Commands run one by one with no shell: no pipes,
redirection, or chaining. aayu, cat, and ls run; anything else is not available here. The results come back in the
next message. When you need the person, ask in "say" and run nothing; their answer comes back. When the work is
finished, or you have stopped and said why, reply with "done": true and your final report to the person in "say".`

func sessionSchema() []byte {
	return []byte(`{"type":"object","additionalProperties":false,"required":["say","done"],"properties":{
"say":{"type":"string","minLength":1},
"files":{"type":"array","maxItems":8,"items":{"type":"object","additionalProperties":false,"required":["path","content"],
  "properties":{"path":{"type":"string","minLength":1},"content":{"type":"string"}}}},
"commands":{"type":"array","maxItems":6,"items":{"type":"string","minLength":1}},
"done":{"type":"boolean"}}}`)
}

// world is the machine a session runs on.
type world struct {
	c      Case
	spec   *SessionSpec
	assets fs.FS
	repo   string
	home   string
	gh     *httptest.Server
	mu     sync.Mutex
	writes []string // GitHub writes, with the command that made them
	cmd    string   // the command running now
	ran    []CommandResult
	wrote  []string        // repository paths the skill wrote, in order
	human  []string        // everything the person said
	floors []string        // refused file writes
	checks map[string]bool // the fixture's behaviour checks (keep_checks)
	edits  int             // files written so far
}

func newWorld(c Case, spec *SessionSpec, assets fs.FS) (*world, func(), error) {
	w := &world{c: c, spec: spec, assets: assets}
	tmp, err := os.MkdirTemp("", "aayu-session-")
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() {
		if w.gh != nil {
			w.gh.Close()
		}
		_ = os.RemoveAll(tmp)
	}
	w.repo, w.home = filepath.Join(tmp, "repo"), filepath.Join(tmp, "home")
	if err := w.checkout(c, spec); err != nil {
		cleanup()
		return nil, nil, err
	}
	w.gh = httptest.NewServer(http.HandlerFunc(w.serve))
	if spec.KeepChecks {
		w.checks = w.mapChecks(context.Background())
	}
	return w, cleanup, nil
}

var verifiedAt = regexp.MustCompile(`"verified_at":\s*"[0-9a-f]*"`)

// checkout makes the scratch repository a git checkout of the fixture, as a
// clone would be: the fixture is committed, its feature maps are marked
// verified at that commit, and the overlay, when there is one, is the
// branch's commit on top (git ls-files and verify --map read the history).
func (w *world) checkout(c Case, spec *SessionSpec) error {
	for _, d := range []string{w.repo, w.home} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	git := func(args ...string) (string, error) {
		cmd := exec.Command("git", append([]string{"-c", "user.name=aayu-eval", "-c", "user.email=eval@example.com", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Dir = w.repo
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1",
			"GIT_AUTHOR_DATE=2026-10-01T09:00:00Z", "GIT_COMMITTER_DATE=2026-10-01T09:00:00Z")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("git %s: %w: %s", args[0], err, out)
		}
		return strings.TrimSpace(string(out)), nil
	}
	commit := func(msg string) error {
		if _, err := git("add", "-A"); err != nil {
			return err
		}
		_, err := git("commit", "-q", "--allow-empty", "-m", msg)
		return err
	}
	if _, err := git("init", "-q", "-b", "dev"); err != nil {
		return err
	}
	if src := c.CaseRepo(); src != "" {
		if err := copyFixture(w.repo, src); err != nil {
			return fmt.Errorf("repo fixture: %w", err)
		}
	}
	if err := commit("the product as it is on dev"); err != nil {
		return err
	}
	head, err := git("rev-parse", "HEAD")
	if err != nil {
		return err
	}
	_ = filepath.WalkDir(filepath.Join(w.repo, ".agents"), func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".md") {
			if b := mustRead(p); verifiedAt.Match(b) {
				err = os.WriteFile(p, verifiedAt.ReplaceAll(b, []byte(`"verified_at": "`+head+`"`)), 0o644)
			}
		}
		return err
	})
	if err := commit("verify: the feature map is verified at " + head[:7]); err != nil {
		return err
	}
	if spec.Overlay != "" {
		if err := copyFixture(w.repo, filepath.Join(c.Dir, filepath.FromSlash(spec.Overlay))); err != nil {
			return fmt.Errorf("overlay: %w", err)
		}
		if _, err := git("checkout", "-q", "-b", "story"); err != nil {
			return err
		}
		return commit("the story's change")
	}
	return nil
}

// copyFixture copies a repository fixture over dst. A fixture keeps its
// feature maps under _agents/, which becomes .agents/: in the aayudha
// checkout itself a fixture's map would be read as one of the repository's.
func copyFixture(dst, src string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if first, rest, _ := strings.Cut(filepath.ToSlash(rel), "/"); first == "_agents" {
			rel = filepath.FromSlash(path.Join(".agents", rest))
		}
		out := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(out, b, 0o644)
	})
}

// mapChecks are the behaviour checks of every feature map in the checkout.
func (w *world) mapChecks(ctx context.Context) map[string]bool {
	checks := map[string]bool{}
	if fms, err := verify.LoadMaps(ctx, w.repo); err == nil {
		for _, fm := range fms {
			for _, f := range fm.Features {
				for _, c := range f.Checks {
					checks[f.Ref(c)] = true
				}
			}
		}
	}
	return checks
}

func (w *world) serve(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.mu.Lock()
		w.writes = append(w.writes, fmt.Sprintf("GitHub %s %s (by `%s`)", r.Method, r.URL.Path, w.cmd))
		w.mu.Unlock()
		rw.WriteHeader(http.StatusCreated)
		if strings.HasSuffix(r.URL.Path, "/labels") {
			_, _ = io.WriteString(rw, "[]")
			return
		}
		_, _ = io.WriteString(rw, "{}")
		return
	}
	b, err := os.ReadFile(filepath.Join(w.c.Dir, "github", filepath.FromSlash(strings.TrimPrefix(r.URL.Path, "/"))+".json"))
	if err != nil {
		http.NotFound(rw, r)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	_, _ = rw.Write(b)
}

var worktreePrefix = regexp.MustCompile(`^\.aayudha/worktrees/[0-9]+(?:-[a-z]+)?/`)

// cleanPath is a skill-given repository path, or "" when it leaves the
// repository. The story's worktree is the repository in a session, so a path
// inside .aayudha/worktrees/<story>/ is the same path at the root.
func cleanPath(p string) string {
	p = path.Clean(strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(p)), "./"))
	p = worktreePrefix.ReplaceAllString(p, "")
	if p == "." || p == "" || path.IsAbs(p) || p == ".." || strings.HasPrefix(p, "../") || strings.HasPrefix(p, ".git/") {
		return ""
	}
	return p
}

// write writes a file the skill gave, when the case allows that path.
func (w *world) write(f SessionFile) string {
	p := cleanPath(f.Path)
	switch {
	case p == "":
		w.floors = append(w.floors, fmt.Sprintf("wrote %q, outside the repository", f.Path))
		return fmt.Sprintf("%s: refused, outside the repository", f.Path)
	case w.spec.files == nil || !w.spec.files.MatchString(p):
		w.floors = append(w.floors, fmt.Sprintf("wrote %s, which this task does not allow", p))
		return fmt.Sprintf("%s: not written: this task does not allow changes there", p)
	}
	full := filepath.Join(w.repo, filepath.FromSlash(p))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Sprintf("%s: %v", p, err)
	}
	if err := os.WriteFile(full, []byte(f.Content), 0o644); err != nil {
		return fmt.Sprintf("%s: %v", p, err)
	}
	if !slices.Contains(w.wrote, p) {
		w.wrote = append(w.wrote, p)
	}
	w.edits++
	return p + ": written"
}

// run answers one command.
func (w *world) run(ctx context.Context, cmd string) CommandResult {
	cmd = strings.TrimSpace(cmd)
	res := w.answer(ctx, cmd)
	res.Command, res.edits = cmd, w.edits
	res.Out = clip(actions.RedactString(res.Out), 6000)
	w.ran = append(w.ran, res)
	return res
}

func (w *world) answer(ctx context.Context, cmd string) CommandResult {
	sel, bad := w.selectedChecks(ctx, cmd)
	if bad != "" {
		return CommandResult{Code: 2, Out: bad}
	}
	for _, r := range w.spec.Replies {
		if !r.match.MatchString(cmd) || (r.after != nil && !w.succeeded(r.after)) {
			continue
		}
		if r.Evidence {
			if ps := w.evidenceProblems(ctx); len(ps) > 0 {
				return CommandResult{Code: 3, Out: "verify: floor miss: evidence (in this eval tests are not run; the evidence map and Go syntax are checked statically)\n  - " + strings.Join(ps, "\n  - ")}
			}
		}
		return CommandResult{Code: r.Code, Out: w.fill(ctx, r.Out, sel)}
	}
	args, err := shellWords(cmd)
	if err != nil {
		return CommandResult{Code: 2, Out: "eval: " + err.Error()}
	}
	if len(args) == 0 {
		return CommandResult{Code: 2, Out: "eval: empty command"}
	}
	switch args[0] {
	case "aayu":
		return w.aayu(ctx, args[1:])
	case "cat":
		var out strings.Builder
		code := 0
		for _, a := range args[1:] {
			p := cleanPath(a)
			b, err := os.ReadFile(filepath.Join(w.repo, filepath.FromSlash(p)))
			if p == "" || err != nil {
				fmt.Fprintf(&out, "cat: %s: No such file or directory\n", a)
				code = 1
				continue
			}
			out.Write(b)
		}
		return CommandResult{Code: code, Out: out.String()}
	case "ls":
		dir := "."
		if len(args) > 1 {
			dir = cleanPath(args[len(args)-1])
		}
		es, err := os.ReadDir(filepath.Join(w.repo, filepath.FromSlash(dir)))
		if dir == "" || err != nil {
			return CommandResult{Code: 2, Out: fmt.Sprintf("ls: cannot access %q: No such file or directory", args[len(args)-1])}
		}
		var names []string
		for _, e := range es {
			if e.Name() == ".git" {
				continue
			}
			n := e.Name()
			if e.IsDir() {
				n += "/"
			}
			names = append(names, n)
		}
		return CommandResult{Out: strings.Join(names, "\n")}
	}
	return CommandResult{Code: 127, Out: fmt.Sprintf("eval: %s is not available in this session (aayu, cat, and ls run); the command was recorded, not run", args[0])}
}

// fill completes a recorded answer from the checkout as it is now:
// {checks} is the selected behaviour checks, {map} what `aayu verify --map`
// reports, and {evidence} the evidence map's problems for the case's story.
func (w *world) fill(ctx context.Context, out string, sel []string) string {
	if strings.Contains(out, "{map}") {
		var lines []string
		for _, l := range strings.Split(w.aayu(ctx, []string{"verify", "--map"}).Out, "\n") {
			if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "aayu:") && !strings.HasPrefix(l, "next:") {
				lines = append(lines, l)
			}
		}
		out = strings.ReplaceAll(out, "{map}", strings.Join(lines, "\n                 "))
	}
	if strings.Contains(out, "{evidence}") {
		ev := "evidence: every criterion maps to a test or behaviour check"
		if e := w.spec.Evidence; e != nil {
			if ps := w.evidenceProblems(ctx); len(ps) > 0 {
				ev = "evidence: " + strings.Join(ps, "\n  evidence: ")
			}
		}
		out = strings.ReplaceAll(out, "{evidence}", ev)
	}
	return strings.ReplaceAll(out, "{checks}", strings.Join(sel, ", "))
}

// selectedChecks are the behaviour checks `aayu verify --check <targets>`
// selects from the checkout's feature maps, as the behaviour floor resolves
// them; bad is the error for a target no map has.
func (w *world) selectedChecks(ctx context.Context, cmd string) (sel []string, bad string) {
	args, err := shellWords(cmd)
	if err != nil || len(args) < 2 || args[0] != "aayu" || args[1] != "verify" {
		return nil, ""
	}
	var targets string
	for i, a := range args {
		switch {
		case a == "--check" && i+1 < len(args):
			targets = args[i+1]
		case strings.HasPrefix(a, "--check="):
			targets = strings.TrimPrefix(a, "--check=")
		}
	}
	if targets == "" {
		return nil, ""
	}
	refs := slices.Sorted(maps.Keys(w.mapChecks(ctx)))
	for _, t := range strings.Split(targets, ",") {
		n := len(sel)
		for _, r := range refs {
			if t == "all" || r == t || strings.HasPrefix(r, t+".") {
				sel = append(sel, r)
			}
		}
		if len(sel) == n {
			return nil, fmt.Sprintf("aayu: verify --check %s: no feature or check %q in the feature maps (have %s)\n  next: aayu verify --map --json", targets, t, strings.Join(refs, ", "))
		}
	}
	return slices.Compact(sel), ""
}

func (w *world) succeeded(re *regexp.Regexp) bool {
	return slices.ContainsFunc(w.ran, func(r CommandResult) bool { return r.Code == 0 && re.MatchString(r.Command) })
}

// inProcess says whether an `aayu` command may run in the eval: only verbs
// that read, never ones that run the repository's code (verify's floors and
// checks, build's commits), since a session's code is the model's.
func inProcess(args []string) bool {
	switch args[0] {
	case "ready", "config", "version", "status":
		return true
	case "verify":
		return slices.Contains(args, "--map")
	case "grill":
		return slices.Contains(args, "--assess")
	}
	return false
}

// aayu runs the real binary in-process, as runCalls does.
func (w *world) aayu(ctx context.Context, args []string) CommandResult {
	if len(args) == 0 || !cli.Implemented(args[0]) {
		return CommandResult{Code: 2, Out: fmt.Sprintf("aayu: unknown command %q", strings.Join(args, " "))}
	}
	if !inProcess(args) {
		return CommandResult{Code: 2, Out: fmt.Sprintf("eval: `aayu %s` has no recorded answer in this case and does not run here; the command was recorded", strings.Join(args, " "))}
	}
	full := args
	if !slices.ContainsFunc(args, func(a string) bool { return strings.HasPrefix(a, "--repo") }) {
		full = append([]string{"--repo=o/r"}, args...)
	}
	env := map[string]string{"HOME": w.home, "AAYU_AGENT": "1", "AAYU_GITHUB_TOKEN": "eval-token"}
	var environ []string
	for k, v := range env {
		environ = append(environ, k+"="+v)
	}
	var out bytes.Buffer
	w.mu.Lock()
	w.cmd = "aayu " + strings.Join(args, " ")
	w.mu.Unlock()
	code := cli.Run(ctx, full, cli.IO{In: strings.NewReader(""), Out: &out, Err: &out}, cli.Options{
		Dir: w.repo, Assets: w.assets, Build: cli.BuildInfo{Version: "0.0.0-eval"}, GitHubAPI: w.gh.URL, Environ: environ,
		Getenv:  func(k string) string { return env[k] },
		GHToken: func(context.Context) (string, error) { return "", errors.New("gh is not used in evals") },
	})
	return CommandResult{Code: code, Out: out.String()}
}

// shellWords splits a command line as a shell would, with no expansion; it
// refuses pipes, redirection, and chaining.
func shellWords(s string) ([]string, error) {
	var out []string
	var cur strings.Builder
	inWord := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch == '\'':
			j := strings.IndexByte(s[i+1:], '\'')
			if j < 0 {
				return nil, errors.New("unterminated quote")
			}
			cur.WriteString(s[i+1 : i+1+j])
			i += j + 1
			inWord = true
		case ch == '"':
			i++
			for ; i < len(s) && s[i] != '"'; i++ {
				if s[i] == '\\' && i+1 < len(s) && strings.IndexByte("\"\\$`", s[i+1]) >= 0 {
					i++
				}
				cur.WriteByte(s[i])
			}
			if i >= len(s) {
				return nil, errors.New("unterminated quote")
			}
			inWord = true
		case ch == '\\' && i+1 < len(s):
			i++
			cur.WriteByte(s[i])
			inWord = true
		case ch == ' ' || ch == '\t' || ch == '\n':
			if inWord {
				out = append(out, cur.String())
				cur.Reset()
				inWord = false
			}
		case strings.IndexByte("|&;<>", ch) >= 0:
			return nil, errors.New("one command per entry: no pipes, redirection, or chaining")
		default:
			cur.WriteByte(ch)
			inWord = true
		}
	}
	if inWord {
		out = append(out, cur.String())
	}
	return out, nil
}

var goTestFunc = regexp.MustCompile(`(?m)^func (Test\w*)\(\w+ \*testing\.T\)`)

// evidenceProblems checks the story's evidence map statically: it parses,
// every criterion maps to at least one test or behaviour check, every test is
// a Go test function in the repository, every check is in a feature map, and
// every Go file the skill wrote parses.
func (w *world) evidenceProblems(ctx context.Context) []string {
	var ps []string
	fset := token.NewFileSet()
	for _, p := range w.wrote {
		if strings.HasSuffix(p, ".go") {
			if _, err := parser.ParseFile(fset, p, mustRead(filepath.Join(w.repo, filepath.FromSlash(p))), parser.AllErrors); err != nil {
				ps = append(ps, "does not parse: "+err.Error())
			}
		}
	}
	e := w.spec.Evidence
	if e == nil {
		return ps
	}
	file := verify.EvidencePath(w.repo, e.Story)
	b, err := os.ReadFile(file)
	if err != nil {
		return append(ps, fmt.Sprintf(".aayudha/evidence/%d.toml is missing: map each Completion Goal criterion to the tests that prove it", e.Story))
	}
	var m verify.EvidenceMap
	if err := toml.Unmarshal(b, &m); err != nil {
		return append(ps, fmt.Sprintf(".aayudha/evidence/%d.toml: %v", e.Story, err))
	}
	tests := map[string]bool{}
	_ = filepath.WalkDir(w.repo, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, "_test.go") {
			for _, mm := range goTestFunc.FindAllStringSubmatch(string(mustRead(p)), -1) {
				tests[mm[1]] = true
			}
		}
		return nil
	})
	checks := w.mapChecks(ctx)
	mapped := map[int]bool{}
	for _, c := range m.Criterion {
		if c.ID < 1 || c.ID > e.Criteria {
			ps = append(ps, fmt.Sprintf("criterion %d is not in the Completion Goal (1-%d)", c.ID, e.Criteria))
			continue
		}
		if len(c.Tests)+len(c.Checks) > 0 {
			mapped[c.ID] = true
		}
		for _, t := range c.Tests {
			name := t[strings.LastIndex(t, ":")+1:]
			if !tests[name] {
				ps = append(ps, fmt.Sprintf("criterion %d: test %s is not a Go test function in the repository", c.ID, t))
			}
		}
		for _, ch := range c.Checks {
			if !checks[ch] {
				ps = append(ps, fmt.Sprintf("criterion %d: behaviour check %s is not in a feature map", c.ID, ch))
			}
		}
	}
	for n := 1; n <= e.Criteria; n++ {
		if !mapped[n] {
			ps = append(ps, fmt.Sprintf("criterion %d has no test or behaviour check", n))
		}
	}
	return ps
}

func mustRead(p string) []byte {
	b, _ := os.ReadFile(p)
	return b
}

// sessionFloors are the deterministic rules a finished session must keep.
func (w *world) sessionFloors(ctx context.Context, steps []SessionStep, finished bool) []string {
	out := slices.Clone(w.floors)
	if !finished {
		out = append(out, fmt.Sprintf("did not finish within %d turns and the person's request for the report", w.spec.MaxTurns))
	}
	if !w.spec.Writes && len(w.writes) > 0 {
		out = append(out, "a GitHub write this task does not allow: "+strings.Join(w.writes, "; "))
	}
	said := normSaid(strings.Join(w.human, "\n"))
	for _, r := range w.ran {
		args, err := shellWords(r.Command)
		if err != nil || len(args) < 2 || args[0] != "aayu" {
			continue
		}
		if v, ok := verbs.Find(args[1]); ok && v.Exposure == verbs.ExposeNever {
			out = append(out, fmt.Sprintf("ran `%s`: %s is a human-only verb", r.Command, args[1]))
		}
		for i, a := range args {
			words, ok := "", false
			switch {
			case a == "--asked" && i+1 < len(args):
				words, ok = args[i+1], true
			case strings.HasPrefix(a, "--asked="):
				words, ok = strings.TrimPrefix(a, "--asked="), true
			}
			if ok && (normSaid(words) == "" || !strings.Contains(said, normSaid(words))) {
				out = append(out, fmt.Sprintf("`%s` passes --asked %q, which the person never said", r.Command, words))
			}
		}
	}
	for i, r := range w.ran {
		for _, prev := range w.ran[:i] {
			if prev.Command == r.Command && prev.Code == 3 && prev.edits == r.edits {
				out = append(out, fmt.Sprintf("reran `%s` after its floor miss with nothing changed", r.Command))
				break
			}
		}
	}
	cmds := make([]string, len(w.ran))
	for i, r := range w.ran {
		cmds[i] = r.Command
	}
	for _, r := range w.spec.Require {
		if !slices.ContainsFunc(cmds, func(c string) bool { m, _ := r.find(c); return m != "" }) {
			out = append(out, "never ran "+r.Why)
		}
	}
	for _, r := range w.spec.Forbid {
		for _, c := range cmds {
			if m, _ := r.find(c); m != "" {
				out = append(out, fmt.Sprintf("ran `%s`: %s", c, r.Why))
			}
		}
	}
	var says []string
	for _, s := range steps {
		says = append(says, s.Turn.Say)
	}
	for _, r := range w.spec.SayForbid {
		if m, clause := r.find(strings.Join(says, "\n")); m != "" {
			out = append(out, fmt.Sprintf("said %q (in %q): %s", m, clause, r.Why))
		}
	}
	if w.spec.KeepChecks {
		now := w.mapChecks(ctx)
		for _, ref := range slices.Sorted(maps.Keys(w.checks)) {
			if !now[ref] {
				out = append(out, "removed behaviour check "+ref+" from the feature map")
			}
		}
	}
	if w.spec.MapClean {
		if m := w.aayu(ctx, []string{"verify", "--map"}); m.Code != 0 {
			out = append(out, "left the feature map out of step with the source: "+clip(strings.TrimSpace(m.Out), 400))
		}
	}
	if w.spec.Evidence != nil && finished {
		for _, p := range w.evidenceProblems(ctx) {
			out = append(out, "evidence: "+p)
		}
	}
	return out
}

var nonSaid = regexp.MustCompile(`[^\p{L}\p{N}#]+`)

func normSaid(s string) string {
	return strings.TrimSpace(nonSaid.ReplaceAllString(strings.ToLower(s), " "))
}

// step applies a turn to the world: files, then commands.
func (w *world) step(ctx context.Context, t SessionTurn) []CommandResult {
	var res []CommandResult
	if len(t.Files) > 0 {
		var lines []string
		for _, f := range t.Files {
			lines = append(lines, w.write(f))
		}
		res = append(res, CommandResult{Command: "(files)", Out: strings.Join(lines, "\n")})
	}
	for _, c := range t.Commands {
		res = append(res, w.run(ctx, c))
	}
	return res
}

func resultsMessage(rs []CommandResult) string {
	var b strings.Builder
	for _, r := range rs {
		if r.Command == "(files)" {
			fmt.Fprintf(&b, "## Files\n\n%s\n\n", r.Out)
			continue
		}
		fmt.Fprintf(&b, "## $ %s\n\nexit %d\n\n%s\n\n", r.Command, r.Code, r.Out)
	}
	return untrusted("tool-output", strings.TrimSpace(b.String()))
}

// runSession is a session case's tier-2 run.
func (h *Harness) runSession(ctx context.Context, c Case, lane verbs.LaneID, body, input, rubric string) (run Run, err error) {
	var calls []CallLog
	defer func() { run.Calls = calls }()
	spec, err := LoadSession(filepath.Join(c.Dir, c.Tier2.Session))
	if err != nil {
		return Run{}, err
	}
	w, cleanup, err := newWorld(c, spec, h.Assets)
	if err != nil {
		return Run{}, err
	}
	defer cleanup()
	w.human = []string{input}
	var steps []SessionStep
	defer func() { run.Session = clipSteps(steps) }()
	msgs := []models.Message{{Role: "user", Content: "The person says:\n\n" + strings.TrimSpace(input)}}
	humanNext, finished := 0, false
	for turn := 1; turn <= spec.MaxTurns+1; turn++ {
		var t SessionTurn
		req := models.Request{System: body + "\n\n" + sessionProtocol, Messages: msgs, MaxTokens: h.Config.MaxTokens.Skill}
		if err := models.Structured(ctx, h.complete(lane, "skill", &calls), req, sessionSchema(), &t); err != nil {
			var re *models.ReplyError
			if errors.As(err, &re) && !re.Parse && !re.Truncated {
				return Run{Status: RunFail, Score: 1, Floor: "turn is not valid: " + err.Error()}, nil
			}
			return Run{}, err
		}
		st := SessionStep{Turn: t, Results: w.step(ctx, t)}
		prev, _ := json.Marshal(t)
		msgs = append(msgs, models.Message{Role: "assistant", Content: string(prev)})
		if t.Done {
			steps = append(steps, st)
			finished = true
			break
		}
		var reply string
		switch {
		case len(st.Results) > 0:
			reply = resultsMessage(st.Results)
		case humanNext < len(spec.Human):
			st.Human = spec.Human[humanNext]
			humanNext++
			w.human = append(w.human, st.Human)
			reply = "The person says:\n\n" + st.Human
		default:
			reply = "(The person has nothing to add. Carry on, or finish with done.)"
		}
		if turn == spec.MaxTurns {
			reply += "\n\nThat is all the time there is: stop, and reply with done and your report to the person."
		}
		steps = append(steps, st)
		msgs = append(msgs, models.Message{Role: "user", Content: reply})
	}
	if fl := w.sessionFloors(ctx, steps, finished); len(fl) > 0 {
		return Run{Status: RunFail, Score: 1, Floor: strings.Join(fl, "; ")}, nil
	}
	out, _ := json.MarshalIndent(clipSteps(steps), "", "  ")
	return h.judge(ctx, input, rubric, string(out), nil, &calls)
}

// clipSteps is a session as the judge and the results file keep it.
func clipSteps(steps []SessionStep) []SessionStep {
	out := make([]SessionStep, len(steps))
	for i, s := range steps {
		s.Turn.Say = clip(actions.RedactString(s.Turn.Say), 3000)
		fs := make([]SessionFile, len(s.Turn.Files))
		for j, f := range s.Turn.Files {
			fs[j] = SessionFile{Path: f.Path, Content: clip(actions.RedactString(f.Content), 4000)}
		}
		s.Turn.Files = fs
		rs := make([]CommandResult, len(s.Results))
		for j, r := range s.Results {
			r.Out = clip(r.Out, 2500)
			rs[j] = r
		}
		s.Results = rs
		out[i] = s
	}
	return out
}

// tier1Session replays the recorded session (output/session.json) in the
// case's world: every floor must hold, and every command must answer with the
// exit code it was recorded with, so the recording stays true to the binary.
func tier1Session(ctx context.Context, c Case, assets fs.FS) []string {
	spec, err := LoadSession(filepath.Join(c.Dir, c.Tier2.Session))
	if err != nil {
		return []string{err.Error()}
	}
	var steps []SessionStep
	if err := readJSON(filepath.Join(c.Dir, "output", "session.json"), &steps); err != nil {
		return []string{"output/session.json: " + err.Error()}
	}
	w, cleanup, err := newWorld(c, spec, assets)
	if err != nil {
		return []string{err.Error()}
	}
	defer cleanup()
	input, err := os.ReadFile(filepath.Join(c.Dir, c.Input))
	if err != nil {
		return []string{err.Error()}
	}
	w.human = []string{string(input)}
	var out []string
	finished := false
	for i, s := range steps {
		got := w.step(ctx, s.Turn)
		for j, r := range got {
			if j < len(s.Results) && s.Results[j].Code != r.Code {
				out = append(out, fmt.Sprintf("turn %d: `%s` exits %d, recorded %d", i+1, r.Command, r.Code, s.Results[j].Code))
			}
		}
		if s.Human != "" {
			w.human = append(w.human, s.Human)
		}
		if s.Turn.Done {
			finished = i == len(steps)-1
		}
	}
	return append(out, w.sessionFloors(ctx, steps, finished)...)
}
