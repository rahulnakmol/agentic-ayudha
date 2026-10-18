package aayudha

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"

	"github.com/rahulnakmol/agentic-ayudha/internal/initrepo"
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

// TestEvalStepsAreSavedAndResumed: tier 2 saves each run's steps under evals/results/steps/<group>, uploads them
// even when the job fails, and a dispatch with resume_run downloads exactly that artifact of the earlier run into the
// same directory. The resume script is run with a stand-in gh that answers for run 4242 at commit abc123.
func TestEvalStepsAreSavedAndResumed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash steps")
	}
	b, err := os.ReadFile(".github/workflows/evals.yml")
	if err != nil {
		t.Fatal(err)
	}
	type step struct {
		Name string            `yaml:"name"`
		Uses string            `yaml:"uses"`
		If   string            `yaml:"if"`
		Run  string            `yaml:"run"`
		With map[string]string `yaml:"with"`
	}
	var wf struct {
		On struct {
			Dispatch struct {
				Inputs map[string]any `yaml:"inputs"`
			} `yaml:"workflow_dispatch"`
		} `yaml:"on"`
		Jobs map[string]struct {
			Steps []step `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(b, &wf); err != nil {
		t.Fatal(err)
	}
	if _, ok := wf.On.Dispatch.Inputs["resume_run"]; !ok {
		t.Error("evals.yml has no resume_run dispatch input")
	}
	var resume, upload string
	saves := false
	for _, s := range wf.Jobs["tier2"].Steps {
		switch {
		case strings.Contains(s.Run, "go run ./internal/evals/cmd/eval -tier 2"):
			saves = strings.Contains(s.Run, `-steps "evals/results/steps/$GROUP"`)
		case strings.Contains(s.Run, "eval-steps-"):
			resume = s.Run
		case strings.HasPrefix(s.Uses, "actions/upload-artifact@") && s.With["path"] == "evals/results/steps/${{ matrix.group }}":
			if s.If != "always()" {
				t.Errorf("the steps upload runs only %q; a failed job must still upload what it saved", s.If)
			}
			upload = s.With["name"]
		}
	}
	if !saves || upload != "eval-steps-${{ github.sha }}-${{ matrix.group }}" || resume == "" {
		t.Fatalf("tier 2 saves steps %v, uploads %q, resume step %v", saves, upload, resume != "")
	}
	bin, work := t.TempDir(), t.TempDir()
	gh := "#!/bin/sh\ncase \"$1 $2\" in\n\"run view\") echo abc123 ;;\n\"run download\") echo \"$@\" >> \"$GH_LOG\" ;;\nesac\n"
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(gh), 0o755); err != nil {
		t.Fatal(err)
	}
	logf := filepath.Join(work, "gh.log")
	run := func(id string) (string, error) {
		cmd := exec.Command("bash", "-c", resume)
		cmd.Dir = work
		cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "GH_LOG="+logf, "RESUME="+id, "GROUP=survey")
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	if out, err := run("4242"); err != nil {
		t.Fatalf("resume from run 4242: %v: %s", err, out)
	}
	got, _ := os.ReadFile(logf)
	if want := "run download 4242 --name eval-steps-abc123-survey --dir evals/results/steps/survey"; strings.TrimSpace(string(got)) != want {
		t.Errorf("downloaded %q, want %q (the name the upload gives run 4242's survey steps)", strings.TrimSpace(string(got)), want)
	}
	_ = os.Remove(logf)
	if out, err := run("4242 --repo other/repo"); err == nil {
		t.Errorf("a resume_run that is not a run ID was accepted: %s", out)
	}
	if _, err := os.Stat(logf); err == nil {
		t.Error("a resume_run that is not a run ID still downloaded")
	}
}
