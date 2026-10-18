package aayudha

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"

	"github.com/rahulnakmol/agentic-ayudha/internal/initrepo"
	"github.com/rahulnakmol/agentic-ayudha/internal/verify"
)

// TestEveryWorkflowJobStartsWithHaltCheck holds the WP5.7 rule that `aayu halt` stops every workflow (Bug #173):
// each job with steps, in this repository's workflows and in the templates `init --gates` installs, starts with the
// halt check. The step runs only while vars.AAYUDHA_HALT is set, and its script must fail, so GitHub runs no
// later step of the job.
func TestEveryWorkflowJobStartsWithHaltCheck(t *testing.T) {
	var files []string
	for _, pattern := range []string{".github/workflows/*.yml", ".github/workflows/*.yaml", "internal/initrepo/*.yml", "internal/initrepo/*.yaml"} {
		m, _ := filepath.Glob(pattern)
		files = append(files, m...)
	}
	if len(files) < 10 {
		t.Fatalf("found only %d workflow files", len(files))
	}
	type step struct {
		Name string `yaml:"name"`
		If   string `yaml:"if"`
		Run  string `yaml:"run"`
	}
	var scripts []string
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var wf struct {
			Jobs map[string]struct {
				Steps []step `yaml:"steps"`
			} `yaml:"jobs"`
		}
		if err := yaml.Unmarshal([]byte(strings.ReplaceAll(string(b), "{{VERSION}}", "dev")), &wf); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		for name, job := range wf.Jobs {
			if len(job.Steps) == 0 {
				continue // a job that calls a reusable workflow has no steps of its own
			}
			s := job.Steps[0]
			if !strings.HasPrefix(s.Name, "Halt check") || s.If != "vars.AAYUDHA_HALT != ''" || !strings.Contains(s.Run, "exit ") {
				t.Errorf("%s: job %s does not start with the halt check step (aayu halt, WP5.7); its first step is %q", f, name, s.Name)
				continue
			}
			scripts = append(scripts, s.Run)
		}
	}
	if runtime.GOOS == "windows" {
		return
	}
	// The halt check's script, run as GitHub runs it while halted, fails with the halt message.
	for _, run := range scripts {
		cmd := exec.Command("bash", "-c", run)
		cmd.Env = append(os.Environ(), "AAYUDHA_HALT=halted by test at 2026-10-15T00:00:00Z")
		out, err := cmd.CombinedOutput()
		if err == nil || !strings.Contains(string(out), "Halted by aayu halt") {
			t.Errorf("halt check %q: want a failing exit and the halt message, got %v: %s", run, err, out)
		}
	}
}

// TestDevInstallRunsNoGitMaintenance: installing aayudha at dev fetches the module with git (GOPROXY=direct). Each
// git fetch in Go's module cache starts a detached `git maintenance run --auto`, whose repack rewrites .git/shallow
// while Go's next fetch reads it, so the install failed at random with "fatal: shallow file has changed since we
// read it". Every such step, here and in the templates `init --gates` installs, turns auto-maintenance off for the
// git that go install runs: the script is run with a stand-in go that prints the git config it would see.
func TestDevInstallRunsNoGitMaintenance(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash steps")
	}
	var files []string
	for _, pattern := range []string{".github/workflows/*.yml", "internal/initrepo/*.yml"} {
		m, _ := filepath.Glob(pattern)
		files = append(files, m...)
	}
	bin := t.TempDir()
	fake := "#!/bin/sh\ngit config --get maintenance.auto || echo unset\n"
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var wf struct {
			Jobs map[string]struct {
				Steps []struct {
					Run string `yaml:"run"`
				} `yaml:"steps"`
			} `yaml:"jobs"`
		}
		if err := yaml.Unmarshal([]byte(strings.ReplaceAll(string(b), "{{VERSION}}", "dev")), &wf); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		for name, job := range wf.Jobs {
			for _, s := range job.Steps {
				if !strings.Contains(s.Run, "GOPROXY=direct") || !strings.Contains(s.Run, "go install") {
					continue
				}
				found++
				cmd := exec.Command("bash", "-c", s.Run)
				cmd.Dir = t.TempDir()
				cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "AAYU_VERSION=dev")
				out, err := cmd.CombinedOutput()
				if err != nil || strings.TrimSpace(string(out)) != "false" {
					t.Errorf("%s: job %s installs dev with git auto-maintenance on (maintenance.auto = %q, %v)", f, name, strings.TrimSpace(string(out)), err)
				}
			}
		}
	}
	if found < 4 {
		t.Fatalf("found only %d dev install steps", found)
	}
}

// TestGitleaksScansOnlyTheCommitUnderTest: a secrets scan of a full-history checkout runs `gitleaks git`, which
// with no --log-opts scans `git log --all`: every fetched branch, so a planted test token on someone else's
// unmerged branch failed the security job of every PR. Each scan in the workflows and in the templates
// `init --gates` installs is run, in a repository whose HEAD has a clean history and whose other branch has the
// leak, with a stand-in gitleaks that lists the commits real gitleaks would read (its --log-opts, else --all).
func TestGitleaksScansOnlyTheCommitUnderTest(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash steps")
	}
	repo := t.TempDir()
	gitIn := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	gitIn("init", "-q", "-b", "dev")
	gitIn("commit", "-q", "--allow-empty", "-m", "base")
	base := gitIn("rev-parse", "HEAD")
	gitIn("checkout", "-q", "-b", "prep")
	gitIn("commit", "-q", "--allow-empty", "-m", "a token on another branch")
	other := gitIn("rev-parse", "HEAD")
	gitIn("checkout", "-q", "dev")
	bin := t.TempDir()
	fake := "#!/bin/sh\nopts=--all\nfor a in \"$@\"; do case $a in --log-opts=*) opts=${a#--log-opts=};; esac; done\nexec git log --format=%H $opts\n"
	if err := os.WriteFile(filepath.Join(bin, "gitleaks"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, pattern := range []string{".github/workflows/*.yml", "internal/initrepo/*.yml"} {
		m, _ := filepath.Glob(pattern)
		files = append(files, m...)
	}
	gitleaks := regexp.MustCompile(`(?m)^\s*\S*gitleaks"? git .*$`)
	found := 0
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range gitleaks.FindAllString(string(b), -1) {
			found++
			cmd := exec.Command("bash", "-c", line)
			cmd.Dir = repo
			cmd.Env = append(os.Environ(), "RUNNER_TEMP="+bin, "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			out, err := cmd.CombinedOutput()
			if err != nil || !strings.Contains(string(out), base) || strings.Contains(string(out), other) {
				t.Errorf("%s: %q scans %q (%v); want HEAD's history only", f, strings.TrimSpace(line), out, err)
			}
		}
	}
	if found == 0 {
		t.Fatal("found no gitleaks git scan in the workflows")
	}
}

// TestSnapshotNamesItselfFromTheNewestRelease: goreleaser names a snapshot `incpatch .Version` from its current
// tag. v* tags sit on main's Release merge commits, which dev and rolling never contain, and a rolling commit
// carries a rolling-<date> tag, which goreleaser takes first and cannot parse as a version (release-snapshot failed
// on every rolling push). Each workflow running `release --snapshot` must first set GORELEASER_CURRENT_TAG to the
// newest v* tag by version, here v0.10.0: not an ancestor of HEAD, after v0.9.0 only in version order, above
// its prereleases v0.10.0-rc.1 and v0.10.0-rc.2; and v11.0.0, v10junk, and v99.0.0 do not count, since release.yml
// refuses a lightweight, malformed, or off-main tag.
func TestSnapshotNamesItselfFromTheNewestRelease(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash steps")
	}
	repo := t.TempDir()
	gitIn := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	gitIn("init", "-q", "-b", "rolling")
	gitIn("commit", "-q", "--allow-empty", "-m", "base")
	gitIn("checkout", "-q", "-b", "main")
	gitIn("commit", "-q", "--allow-empty", "-m", "Release: v0.9.0")
	gitIn("tag", "-a", "-m", "v0.9.0", "v0.9.0")
	gitIn("tag", "-a", "-m", "v0.10.0-rc.1", "v0.10.0-rc.1")
	gitIn("commit", "-q", "--allow-empty", "-m", "Release: v0.10.0")
	gitIn("tag", "-a", "-m", "v0.10.0", "v0.10.0")
	gitIn("tag", "-a", "-m", "v0.10.0-rc.2", "v0.10.0-rc.2") // tagged late; a prerelease still ranks below its release
	// Tags release.yml refuses never name a snapshot: lightweight, malformed, or off main.
	gitIn("tag", "v11.0.0")
	gitIn("tag", "-a", "-m", "v10junk", "v10junk")
	gitIn("update-ref", "refs/remotes/origin/main", "main") // the checkout's fetch-depth 0 fetches origin/main
	gitIn("checkout", "-q", "rolling")
	gitIn("commit", "-q", "--allow-empty", "-m", "Rolling snapshot")
	gitIn("tag", "rolling-2026-10-16.2")
	gitIn("tag", "-a", "-m", "v99.0.0", "v99.0.0")
	step := regexp.MustCompile(`(?m)^\s*run: (.*GORELEASER_CURRENT_TAG=.*)$`)
	files, _ := filepath.Glob(".github/workflows/*.yml")
	found := 0
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if snap := strings.Index(string(b), "release --snapshot"); snap >= 0 {
			if m := step.FindStringSubmatchIndex(string(b)); m == nil || m[0] > snap {
				t.Errorf("%s runs `goreleaser release --snapshot` without first setting GORELEASER_CURRENT_TAG", f)
			}
		}
		for _, m := range step.FindAllStringSubmatch(string(b), -1) {
			found++
			env := filepath.Join(t.TempDir(), "env")
			cmd := exec.Command("bash", "-c", m[1])
			cmd.Dir = repo
			cmd.Env = append(os.Environ(), "GITHUB_ENV="+env)
			out, err := cmd.CombinedOutput()
			got, _ := os.ReadFile(env)
			if err != nil || strings.TrimSpace(string(got)) != "GORELEASER_CURRENT_TAG=v0.10.0" {
				t.Errorf("%s: %q wrote %q (%v, %s); want GORELEASER_CURRENT_TAG=v0.10.0", f, m[1], got, err, out)
			}
		}
	}
	if found == 0 {
		t.Fatal("found no GORELEASER_CURRENT_TAG step in the workflows")
	}
}

// Founder decisions of 2026-10-15 and 2026-10-16 (Appendix B S-4.8c, S-2.9b, D-2.9b): product CI never calls a model,
// and this repository's model checks never gate. Every workflow `init --gates` installs holds no model key or
// AI-gateway environment; the four workflows that call the gateway for aayudha's own model code run on no pull
// request event (so none can be a required check of a PR), and ci.yml's gates (PR, rolling, stable) never name them.
func TestModelChecksNeverGateAndNeverShip(t *testing.T) {
	keys := regexp.MustCompile(`ai-gateway|AAYU_SECRET_|AAYU_MODELS__|AAYU_AI_GATEWAY`)
	for name, b := range initrepo.GateWorkflows("dev") {
		if m := keys.Find(b); m != nil {
			t.Errorf("init --gates installs %s, which names %q: product CI never calls a model", name, m)
		}
	}
	ci, err := os.ReadFile(".github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"inspect-live", "aayudha-model-lanes", "models-live", "ux-review-live"} {
		b, err := os.ReadFile(".github/workflows/" + f + ".yml")
		if err != nil {
			t.Fatal(err)
		}
		var wf struct {
			On   map[string]any `yaml:"on"`
			Jobs map[string]struct {
				Name string `yaml:"name"`
			} `yaml:"jobs"`
		}
		if err := yaml.Unmarshal(b, &wf); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		for ev := range wf.On {
			if strings.HasPrefix(ev, "pull_request") {
				t.Errorf("%s runs on %s, so it could become a required check", f, ev)
			}
		}
		names := []string{f + ".yml"} // and each job's check name (a job ID such as "live" is an ordinary word)
		for _, j := range wf.Jobs {
			names = append(names, j.Name)
		}
		for _, n := range names {
			if n != "" && strings.Contains(string(ci), n) {
				t.Errorf("ci.yml names %q of %s: a model check must not be part of a gate", n, f)
			}
		}
	}
}

// Story #487 criterion 6 (Decision #480): every run step of the security workflow `init --gates` installs runs with
// stand-in tools. The downloads must match their pinned SHA-256 (wrong bytes fail the step), semgrep is pinned to its
// version, and the floors step calls `aayudha verify --security` on the pushed range, then writes the job summary.
func TestSecurityWorkflowSteps(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash steps")
	}
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("no jq")
	}
	// The floors verify --security runs (Decision #485 adds dast): a dropped scanner fails here.
	var floors []string
	for _, f := range verify.SecurityFloors() {
		floors = append(floors, f.Name())
	}
	if got := strings.Join(floors, ","); got != "sast,secrets,deps,dast" {
		t.Fatalf("verify --security floors %q", got)
	}
	raw := initrepo.GateWorkflows("dev")[initrepo.SecurityWorkflow]
	var wf struct {
		On struct {
			Push struct {
				Branches []string `yaml:"branches"`
			} `yaml:"push"`
		} `yaml:"on"`
		Jobs map[string]struct {
			Env   map[string]string `yaml:"env"`
			Steps []struct {
				Name string            `yaml:"name"`
				If   string            `yaml:"if"`
				Uses string            `yaml:"uses"`
				Run  string            `yaml:"run"`
				Env  map[string]string `yaml:"env"`
				Dir  string            `yaml:"working-directory"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(raw, &wf); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(wf.On.Push.Branches, ","); got != "dev,rolling,main" {
		t.Fatalf("push branches %q", got)
	}
	job := wf.Jobs["security"]
	// The real downloads' bytes are replaced by fixtures: a gitleaks tarball and an osv-scanner binary.
	fixtures := t.TempDir()
	if err := os.WriteFile(filepath.Join(fixtures, "gitleaks"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	tgz := "gitleaks_" + job.Env["GITLEAKS_VERSION"] + "_linux_x64.tar.gz"
	if out, err := exec.Command("tar", "-czf", filepath.Join(fixtures, tgz), "-C", fixtures, "gitleaks").CombinedOutput(); err != nil {
		t.Fatalf("tar: %v %s", err, out)
	}
	_ = os.WriteFile(filepath.Join(fixtures, "osv-scanner_linux_amd64"), []byte("#!/bin/sh\n"), 0o755)
	sum := func(name string) string {
		b, _ := os.ReadFile(filepath.Join(fixtures, name))
		return fmt.Sprintf("%x", sha256.Sum256(b))
	}
	stand := map[string]string{
		// curl -fsSLo <out> <url>: copies the fixture named by the URL's last segment, and logs the URL.
		"curl":    "#!/bin/sh\nout=$2; url=$3\necho \"$url\" >> \"$LOG/curl\"\ncp \"$FIXTURES/${url##*/}\" \"$out\"\n",
		"pipx":    "#!/bin/sh\necho \"$@\" >> \"$LOG/pipx\"\n",
		"sudo":    "#!/bin/sh\nexit 0\n",
		"go":      "#!/bin/sh\necho \"$@\" >> \"$LOG/go\"\n",
		"aayudha": "#!/bin/sh\necho \"$@\" > \"$LOG/aayudha\"\necho '{\"branch\":\"dev\",\"head\":\"0123456789\",\"checks\":[{\"floor\":\"sast\",\"result\":\"fail\",\"detail\":\"1 high\"},{\"floor\":\"dast\",\"result\":\"skipped\",\"detail\":\"skipped: dast: no runnable target\"}],\"bugs\":[{\"number\":12,\"action\":\"filed\",\"fingerprint\":\"sast r main.go\"}]}'\nexit 3\n",
	}
	run := func(t *testing.T, before string, pins map[string]string) (map[string]string, error) {
		t.Helper()
		bin, logs, tmp := t.TempDir(), t.TempDir(), t.TempDir()
		for name, script := range stand {
			if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		work := t.TempDir()
		head := filepath.Join(work, "head")
		_ = os.MkdirAll(head, 0o755)
		if out, err := exec.Command("git", "-C", head, "init", "-q").CombinedOutput(); err != nil {
			t.Fatalf("git init: %v %s", err, out)
		}
		summary, ghPath := filepath.Join(tmp, "summary.md"), filepath.Join(tmp, "path")
		env := append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "LOG="+logs, "FIXTURES="+fixtures,
			"RUNNER_TEMP="+tmp, "GITHUB_STEP_SUMMARY="+summary, "GITHUB_PATH="+ghPath, "AAYU_VERSION=dev", "REPO=o/r", "BEFORE="+before)
		for k, v := range job.Env {
			if strings.Contains(v, "${{") { // an expression the runner evaluates: BEFORE, set above
				continue
			}
			if p, ok := pins[k]; ok {
				v = p
			}
			env = append(env, k+"="+v)
		}
		var stepErr error
		for _, s := range job.Steps {
			if s.Run == "" || s.If != "" { // actions, and the halt check (it runs only while halted)
				continue
			}
			cmd := exec.Command("bash", "--noprofile", "--norc", "-eo", "pipefail", "-c", s.Run)
			cmd.Dir = filepath.Join(work, s.Dir)
			cmd.Env = append(slices.Clone(env), "BRANCH=dev", "GITHUB_TOKEN=t")
			if out, err := cmd.CombinedOutput(); err != nil {
				stepErr = fmt.Errorf("step %q: %w\n%s", s.Name, err, out)
				break
			}
		}
		got := map[string]string{}
		for _, f := range []string{"curl", "pipx", "go", "aayudha"} {
			b, _ := os.ReadFile(filepath.Join(logs, f))
			got[f] = string(b)
		}
		b, _ := os.ReadFile(summary)
		got["summary"] = string(b)
		return got, stepErr
	}
	pins := map[string]string{"GITLEAKS_SHA256": sum(tgz), "OSV_SCANNER_SHA256": sum("osv-scanner_linux_amd64")}

	// A push: every step runs; verify --security scans the pushed range and its floor miss fails the job.
	before := strings.Repeat("a", 40)
	got, err := run(t, before, pins)
	if err == nil || !strings.Contains(err.Error(), `step "Security floors on the commit"`) || !strings.Contains(err.Error(), "exit status 3") {
		t.Fatalf("the floors step's floor miss should fail the job, and only that step: %v", err)
	}
	if !strings.Contains(got["curl"], "/gitleaks/releases/download/v"+job.Env["GITLEAKS_VERSION"]+"/") || !strings.Contains(got["curl"], "/osv-scanner/releases/download/v"+job.Env["OSV_SCANNER_VERSION"]+"/") {
		t.Errorf("downloads: %q", got["curl"])
	}
	if got["pipx"] != "install semgrep=="+job.Env["SEMGREP_VERSION"]+"\n" || !strings.Contains(got["go"], "install github.com/rahulnakmol/agentic-ayudha/cmd/aayudha@dev") {
		t.Errorf("installs: pipx %q, go %q", got["pipx"], got["go"])
	}
	if want := "verify --security --ci --repo o/r --json --since " + before + "\n"; got["aayudha"] != want {
		t.Errorf("aayudha %q, want %q", got["aayudha"], want)
	}
	if !strings.Contains(got["summary"], "| sast | fail | 1 high |") || !strings.Contains(got["summary"], "| dast | skipped | skipped: dast: no runnable target |") || !strings.Contains(got["summary"], "- Bug #12 filed: sast r main.go") {
		t.Errorf("job summary: %q", got["summary"])
	}
	// The nightly run (no before): the whole history, so no --since.
	if got, _ := run(t, "", pins); got["aayudha"] != "verify --security --ci --repo o/r --json\n" {
		t.Errorf("nightly aayudha %q", got["aayudha"])
	}
	// The pinned SHA-256s of the real releases do not match the stand-in bytes: the tools step must refuse them.
	if _, err := run(t, before, nil); err == nil || !strings.Contains(err.Error(), `step "Floor tools (pinned, SHA-256 checked)"`) {
		t.Fatalf("downloads that do not match their SHA-256 were accepted: %v", err)
	}
	if _, err := run(t, before, map[string]string{"GITLEAKS_SHA256": sum(tgz)}); err == nil || !strings.Contains(err.Error(), "osv-scanner") {
		t.Fatalf("an osv-scanner download that does not match its SHA-256 was accepted: %v", err)
	}
}
