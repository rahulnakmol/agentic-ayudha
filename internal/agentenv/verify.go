package agentenv

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Profile is the closest clean equivalent of a host's machine: its base image
// and the user its setup runs as, plus only what every host has before setup
// (git and CA certificates, for the clone; sudo for a non-root user).
type Profile struct {
	Image  string
	User   string // root or a sudo user
	Source string // where the host documents it
	// BashEntrypoint: the image's entrypoint prepares the machine, then runs
	// bash with the container's arguments (codex-universal's configures the
	// language runtimes, then `exec bash --login "$@"`), so it is kept.
	BashEntrypoint bool
}

// Profiles per host. Images are the hosts' documented bases; Codex's is its
// published default image.
var Profiles = map[string]Profile{
	Amp:    {Image: "debian:12", User: "user", Source: "Amp orbs are Debian 12 sandboxes; setup runs as user with sudo (ampcode.com/docs/orbs/customizing)"},
	Cursor: {Image: "ubuntu:24.04", User: "ubuntu", Source: "Cursor cloud agents run on isolated Ubuntu machines as a sudo user (cursor.com/docs/cloud-agent/setup)"},
	Codex:  {Image: "ghcr.io/openai/codex-universal:latest", User: "root", Source: "Codex cloud tasks run in the universal image (developers.openai.com/codex/cloud/environments, github.com/openai/codex-universal)", BashEntrypoint: true},
	Claude: {Image: "ubuntu:24.04", User: "root", Source: "Claude Code cloud setup runs as root on Ubuntu 24.04 (code.claude.com/docs/en/cloud-environments)"},
}

// Runner runs a command with stdin, streaming its combined output to out.
type Runner func(ctx context.Context, stdin io.Reader, out io.Writer, name string, args ...string) error

// ExecRunner runs commands for real.
func ExecRunner(ctx context.Context, stdin io.Reader, out io.Writer, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, out, out
	return cmd.Run()
}

// Docker returns the docker command that works here: docker, or sudo -n docker.
func Docker(ctx context.Context, run Runner) ([]string, error) {
	for _, c := range [][]string{{"docker"}, {"sudo", "-n", "docker"}} {
		if run(ctx, nil, io.Discard, c[0], append(c[1:], "info", "--format", "{{.ServerVersion}}")...) == nil {
			return c, nil
		}
	}
	return nil, errors.New("no Docker daemon answers (docker info); start Docker, or verify on a machine that has it")
}

// Result is one container verification.
type Result struct {
	Host     string         `json:"host"`
	Image    string         `json:"image"`
	OK       bool           `json:"ok"`
	Attempts int            `json:"attempts"`
	Fixes    []string       `json:"fixes,omitempty"`
	Phases   map[string]int `json:"phases,omitempty"` // seconds: setup-cold, setup-warm, check
	Log      string         `json:"log"`
	Tail     []string       `json:"tail,omitempty"`
	Marker   string         `json:"marker,omitempty"`
}

// VerifyOptions configure Verify.
type VerifyOptions struct {
	Root     string
	Host     string
	Run      Runner
	LogDir   string // where full logs go (local, not committed)
	Attempts int    // fix-and-retry budget; 0 means 3
	Now      func() time.Time
	Image    string // override the profile's image
}

var phaseLine = regexp.MustCompile(`(?m)^aayu-phase ([a-z-]+) ([0-9]+)$`)

// Verify runs the generated setup twice (cold, then warm, which proves it is
// idempotent) and then the check in a fresh login shell, in a clean container
// of the host's profile. On a failure it applies the fixes it knows (a missing
// command's package, for example), regenerates, and retries, the way an orb
// setup is fixed until it passes. It returns the spec with any fixes, which
// the caller writes.
func Verify(ctx context.Context, s Spec, o VerifyOptions) (Spec, Result, error) {
	prof, ok := Profiles[o.Host]
	if !ok {
		return s, Result{}, fmt.Errorf("no container profile for host %q", o.Host)
	}
	if o.Image != "" {
		prof.Image = o.Image
	}
	if o.Attempts == 0 {
		o.Attempts = 3
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	docker, err := Docker(ctx, o.Run)
	if err != nil {
		return s, Result{}, err
	}
	tag := "aayu-env-" + o.Host + ":" + strings.TrimPrefix(Requirements0(prof), "sha256:")
	var build bytes.Buffer
	if err := o.Run(ctx, strings.NewReader(profileDockerfile(prof)), &build, docker[0], append(docker[1:], "build", "-q", "-t", tag, "-")...); err != nil {
		return s, Result{}, fmt.Errorf("build the %s profile image from %s: %w: %s", o.Host, prof.Image, err, lastLines(build.String(), 10))
	}
	res := Result{Host: o.Host, Image: prof.Image}
	if err := os.MkdirAll(o.LogDir, 0o755); err != nil {
		return s, res, err
	}
	for attempt := 1; ; attempt++ {
		res.Attempts = attempt
		files, _, err := Generate(o.Root, s)
		if err != nil {
			return s, res, err
		}
		tarball, err := sourceTar(o.Root, files)
		if err != nil {
			return s, res, err
		}
		res.Log = filepath.Join(o.LogDir, fmt.Sprintf("%s-%s-%d.log", o.Host, o.Now().UTC().Format("20060102T150405Z"), attempt))
		lf, err := os.Create(res.Log)
		if err != nil {
			return s, res, err
		}
		var out bytes.Buffer
		fmt.Fprintf(lf, "# aayu install --env --verify: host %s, image %s (%s), attempt %d, environment %s\n", o.Host, prof.Image, prof.Source, attempt, s.Fingerprint())
		args := append(docker[1:], "run", "--rm", "-i", tag)
		if !prof.BashEntrypoint {
			args = append(args, "bash")
		}
		runErr := o.Run(ctx, bytes.NewReader(tarball), io.MultiWriter(lf, &out), docker[0], append(args, "-c", inContainer)...)
		_ = lf.Close()
		log := out.String()
		res.Phases = map[string]int{}
		for _, m := range phaseLine.FindAllStringSubmatch(log, -1) {
			res.Phases[m[1]], _ = strconv.Atoi(m[2])
		}
		res.Tail = strings.Split(lastLines(log, 40), "\n")
		want := Marker + " " + s.Fingerprint()
		if runErr == nil && strings.Contains(log, want) {
			res.OK, res.Marker = true, want
			return s, res, nil
		}
		if attempt >= o.Attempts {
			return s, res, nil
		}
		fixed, what := Fix(s, log)
		if what == "" {
			return s, res, nil
		}
		s = fixed
		res.Fixes = append(res.Fixes, what)
	}
}

// Requirements0 keys the profile image on its Dockerfile, so a changed profile rebuilds.
func Requirements0(p Profile) string { return hash([]byte(profileDockerfile(p))) }

func profileDockerfile(p Profile) string {
	var b strings.Builder
	fmt.Fprintf(&b, "FROM %s\n", p.Image)
	b.WriteString("USER root\n")
	pkgs := "git ca-certificates"
	if p.User != "root" {
		pkgs += " sudo"
	}
	fmt.Fprintf(&b, "RUN if ! command -v git >/dev/null || { [ %q != root ] && ! command -v sudo >/dev/null; }; then apt-get update -qq && DEBIAN_FRONTEND=noninteractive apt-get install -y -qq --no-install-recommends %s && rm -rf /var/lib/apt/lists/*; fi\n", p.User, pkgs)
	if p.User != "root" {
		fmt.Fprintf(&b, "RUN id -u %[1]s >/dev/null 2>&1 || useradd -m -s /bin/bash %[1]s; echo '%[1]s ALL=(ALL) NOPASSWD:ALL' > /etc/sudoers.d/aayu-env\n", p.User)
		fmt.Fprintf(&b, "USER %s\n", p.User)
	}
	b.WriteString("RUN git config --global user.name 'aayu env' && git config --global user.email aayu-env@localhost && git config --global init.defaultBranch main\n")
	b.WriteString("WORKDIR /tmp\n")
	return b.String()
}

// inContainer unpacks the source like a clone, runs setup cold and warm, and
// runs the check in a fresh login shell that has the machine's own
// environment (the image's, as the host gives an agent) and nothing setup
// exported, as an agent's shell starts after setup.
const inContainer = `export -p > /tmp/aayu-env0
set -euo pipefail
mkdir -p "$HOME/workspace/repo" && cd "$HOME/workspace/repo"
tar -x
git init -q && git add -A && git commit -qm "aayu env verification snapshot"
phase() { local name=$1 t0; shift; t0=$(date +%s); "$@"; echo "aayu-phase $name $(( $(date +%s) - t0 ))"; }
phase setup-cold bash .agents/setup
phase setup-warm bash .agents/setup
phase check env -i bash -c '. /tmp/aayu-env0 && export USER="$(id -un)" TERM=dumb && cd "$HOME/workspace/repo" && exec bash -lc "bash .agents/check"'
`

// sourceTar is the repository as a clone of it would look with the generated
// files committed: tracked and untracked-but-not-ignored files, with the
// generated ones replaced by their new content.
func sourceTar(root string, gen []File) ([]byte, error) {
	cmd := exec.Command("git", "-C", root, "ls-files", "-z", "-c", "-o", "--exclude-standard")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}
	over := map[string]File{}
	for _, f := range gen {
		over[f.Path] = f
	}
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	seen := map[string]bool{}
	add := func(name string, mode int64, body []byte) error {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: int64(len(body)), ModTime: time.Unix(0, 0)}); err != nil {
			return err
		}
		_, err := tw.Write(body)
		return err
	}
	for _, p := range strings.Split(string(out), "\x00") {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		if f, ok := over[p]; ok {
			mode := int64(0o644)
			if f.Exec {
				mode = 0o755
			}
			if err := add(p, mode, f.Content); err != nil {
				return nil, err
			}
			continue
		}
		fp := filepath.Join(root, filepath.FromSlash(p))
		fi, err := os.Lstat(fp)
		if err != nil || !fi.Mode().IsRegular() {
			continue // deleted in the worktree, a symlink, or a submodule
		}
		body, err := os.ReadFile(fp)
		if err != nil {
			return nil, err
		}
		if err := add(p, int64(fi.Mode().Perm()), body); err != nil {
			return nil, err
		}
	}
	for _, f := range gen {
		if !seen[f.Path] {
			mode := int64(0o644)
			if f.Exec {
				mode = 0o755
			}
			if err := add(f.Path, mode, f.Content); err != nil {
				return nil, err
			}
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// commandPackages maps a command a log says is missing to the Debian and
// Ubuntu package that provides it.
var commandPackages = map[string]string{
	"make": "make", "gcc": "build-essential", "cc": "build-essential", "g++": "build-essential", "pkg-config": "pkg-config",
	"unzip": "unzip", "zip": "zip", "jq": "jq", "python3": "python3", "psql": "postgresql-client", "file": "file",
	"bzip2": "bzip2", "patch": "patch", "gpg": "gnupg", "ssh": "openssh-client", "rsync": "rsync", "wget": "wget",
	"sqlite3": "sqlite3", "age": "age", "git-lfs": "git-lfs", "lsof": "lsof", "zstd": "zstd", "ps": "procps",
	"less": "less", "xvfb-run": "xvfb", "tesseract": "tesseract-ocr", "ffmpeg": "ffmpeg", "convert": "imagemagick",
	"magick": "imagemagick", "shellcheck": "shellcheck", "tzdata": "tzdata", "locale-gen": "locales", "netstat": "net-tools",
}

var (
	notFound    = regexp.MustCompile(`(?m)(?:^|[:\s])([A-Za-z0-9][A-Za-z0-9+._-]*): (?:command )?not found`)
	execMissing = regexp.MustCompile(`exec: "([A-Za-z0-9][A-Za-z0-9+._-]*)": executable file not found`)
	dubious     = regexp.MustCompile(`detected dubious ownership in repository`)
)

// Fix returns the spec with the first known fix for a failure log, and what
// it did; what is empty when no rule applies (the caller reports the log).
func Fix(s Spec, log string) (Spec, string) {
	for _, re := range []*regexp.Regexp{execMissing, notFound} {
		for _, m := range re.FindAllStringSubmatch(log, -1) {
			if p, ok := commandPackages[m[1]]; ok && !contains(s.Packages, p) {
				s.Packages = append(append([]string(nil), s.Packages...), p)
				s.Notes = append(append([]string(nil), s.Notes...), fmt.Sprintf("fix: %s was missing in a clean container; apt package %s added", m[1], p))
				s.Answers = withAnswer(s.Answers, "fix-packages", p)
				return s, "added apt package " + p + " (" + m[1] + " was not found)"
			}
		}
	}
	if dubious.MatchString(log) {
		step := SafeDirectoryStep
		for _, x := range s.Steps {
			if x.Run == step.Run {
				return s, ""
			}
		}
		s.Steps = append(append([]Step(nil), s.Steps...), step)
		s.Answers = withAnswer(s.Answers, "fix-steps", "safe-directory")
		return s, "marked the checkout a safe git directory"
	}
	return s, ""
}

// withAnswer records a fix as an answer (source "fix"), so rediscovery
// re-applies it and the spec does not look stale.
func withAnswer(a map[string]Answer, id, v string) map[string]Answer {
	out := map[string]Answer{}
	for k, x := range a {
		out[k] = x
	}
	cur := strings.Fields(out[id].Value)
	if !contains(cur, v) {
		cur = append(cur, v)
	}
	out[id] = Answer{Value: strings.Join(cur, " "), Source: "fix"}
	return out
}

// SafeDirectoryStep is the fix for git's dubious-ownership refusal.
var SafeDirectoryStep = Step{Run: `git config --global --add safe.directory "$PWD"`, Evidence: "fix: git refused the checkout as dubious ownership in a clean container"}

// ScanMarker finds a check marker in text (a probe's reply).
func ScanMarker(text string) (fingerprint string, ok bool) {
	sc := bufio.NewScanner(strings.NewReader(text))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	re := regexp.MustCompile(regexp.QuoteMeta(Marker) + ` (sha256:[0-9a-f]{16})`)
	for sc.Scan() {
		if m := re.FindStringSubmatch(sc.Text()); m != nil {
			fingerprint, ok = m[1], true
		}
	}
	return fingerprint, ok
}
