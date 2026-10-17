// Package agentenv sets up the environments that agent hosts (Amp orbs, Cursor
// cloud agents, Codex cloud, Claude Code on the web) run a repository in, from
// one spec discovered from the code (founder request 2026-10-16, Appendix B
// S-3.2c). Discover reads the code; the interview settles what the code cannot
// say; Resolve pins exact versions with checksums; Generate writes each host's
// native configuration, all of which call one shared setup script; Verify runs
// that script and the repository's checks in a clean container per host, and
// cloud probes prove a live environment; Status reports verified, stale, or
// missing per host; Detect reports which agent CLIs and cloud connections this
// machine has, offline.
package agentenv

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/pelletier/go-toml/v2"
)

// Paths of the files aayu owns. The setup script is the one source every host
// runs; check proves an environment; start brings services up per session.
const (
	SpecPath      = ".aayudha/env.toml"
	VerifiedPath  = ".aayudha/env-verified.toml"
	SetupPath     = ".agents/setup"
	SetupCommand  = "bash .agents/setup"
	CheckPath     = ".agents/check"
	CheckCommand  = "bash .agents/check"
	StartPath     = ".agents/start"
	StartCommand  = "bash .agents/start"
	ResumePath    = ".agents/resume"
	CursorEnvPath = ".cursor/environment.json"
	ClaudeSetPath = ".claude/settings.json"
)

// Hosts, in the order aayu reports them.
const (
	Amp    = "amp"
	Cursor = "cursor"
	Codex  = "codex"
	Claude = "claude"
)

// Hosts lists every host aayu generates an environment for.
var Hosts = []string{Amp, Cursor, Codex, Claude}

// Tool is one toolchain: what the code requires (Want, with its Evidence) and
// the exact version Resolve pinned, with a SHA-256 per architecture when the
// script downloads it.
type Tool struct {
	Name     string            `toml:"name"`
	Want     string            `toml:"want"`              // the requirement as the code states it
	Version  string            `toml:"version,omitempty"` // the pinned version
	Accept   []string          `toml:"accept,omitempty"`  // "lo hi" version ranges an installed copy may satisfy (hi empty: open)
	SHA256   map[string]string `toml:"sha256,omitempty"`  // amd64, arm64
	Via      string            `toml:"via,omitempty"`     // corepack, toolchain, tarball, rustup
	Evidence string            `toml:"evidence"`
}

// Step is a command setup runs, with why.
type Step struct {
	Run      string `toml:"run"`
	Evidence string `toml:"evidence"`
}

// Service is a backing service the tests need, installed natively and started
// per session (setup must not leave processes running on any host).
type Service struct {
	Name     string `toml:"name"` // postgres, redis, mysql
	User     string `toml:"user,omitempty"`
	Password string `toml:"password,omitempty"` // only a placeholder from .env.example, never a real secret
	Database string `toml:"database,omitempty"`
	Evidence string `toml:"evidence"`
}

// EnvVar is a variable the code reads, by name only.
type EnvVar struct {
	Name     string `toml:"name"`
	Secret   bool   `toml:"secret"`
	Evidence string `toml:"evidence"`
}

// GateJob is one CI job a pull request must pass and how the local gate
// (`.agents/check --full`) mirrors it (Story #399): the commands that run its
// checks here, and, for what cannot run here, why it runs only in CI. Discover
// finds the jobs in the workflows; the commands are the repository's own, kept
// across rediscovery (KeepPins). A job with neither is a gap: the gate fails on
// it, naming it.
type GateJob struct {
	Job      string   `toml:"job"`              // the job's check name
	Workflow string   `toml:"workflow"`         // the workflow file that defines it
	Run      []string `toml:"run,omitempty"`    // bash commands, in order; the job fails at the first that fails
	Remote   string   `toml:"remote,omitempty"` // what of it runs only in CI, and why
}

// Answer is one interview answer: the value and whether the human said it or
// accepted the default.
type Answer struct {
	Value  string `toml:"value"`
	Source string `toml:"source"` // confirmed, stated, default
}

// Spec is the discovered environment (.aayudha/env.toml).
type Spec struct {
	Version  int               `toml:"version"`
	Hosts    []string          `toml:"hosts"`
	Tools    []Tool            `toml:"tool"`
	Packages []string          `toml:"packages"` // apt packages
	Deps     []Step            `toml:"deps"`
	Browsers []Step            `toml:"browsers"` // browser installs (Playwright)
	Services []Service         `toml:"service"`
	Env      []EnvVar          `toml:"env"`
	EnvFile  string            `toml:"env_file,omitempty"` // .env.example copied to this name when absent
	Checks   []Step            `toml:"checks"`
	Steps    []Step            `toml:"steps"` // custom setup steps the human added
	Gate     []GateJob         `toml:"gate"`  // the local gate's mirror of the required CI jobs; not part of the environment
	Notes    []string          `toml:"notes"`
	Answers  map[string]Answer `toml:"answers"`
}

// Tool returns the named tool, or nil.
func (s *Spec) Tool(name string) *Tool {
	for i := range s.Tools {
		if s.Tools[i].Name == name {
			return &s.Tools[i]
		}
	}
	return nil
}

// HasHost reports whether the spec sets up host.
func (s *Spec) HasHost(h string) bool {
	for _, x := range s.Hosts {
		if x == h {
			return true
		}
	}
	return false
}

// Requirements is what the code asks for, without resolved pins: a spec is
// stale when the code's requirements change ("the toolchain changed").
func (s *Spec) Requirements() string {
	type tool struct{ Name, Want string }
	r := struct {
		Tools                     []tool
		Packages, Browsers, Hosts []string
		Deps, Checks, Steps       []string
		Services                  []string
		EnvFile                   string
	}{Packages: sorted(s.Packages), Hosts: s.Hosts, EnvFile: s.EnvFile}
	for _, b := range s.Browsers {
		r.Browsers = append(r.Browsers, b.Run)
	}
	for _, t := range s.Tools {
		if t.Name != "corepack" { // Resolve adds it; the code does not ask for it
			r.Tools = append(r.Tools, tool{t.Name, t.Want})
		}
	}
	for _, d := range s.Deps {
		r.Deps = append(r.Deps, d.Run)
	}
	for _, c := range s.Checks {
		r.Checks = append(r.Checks, c.Run)
	}
	for _, c := range s.Steps {
		r.Steps = append(r.Steps, c.Run)
	}
	for _, sv := range s.Services {
		r.Services = append(r.Services, sv.Name+"/"+sv.User+"/"+sv.Database)
	}
	b, _ := json.Marshal(r)
	return hash(b)
}

// Fingerprint identifies the exact environment the generated files install:
// the requirements plus every pin. The check script prints it, so a probe's
// reply names the environment it ran.
func (s *Spec) Fingerprint() string {
	type tool struct {
		Name, Version string
		SHA           []string
	}
	var ts []tool
	for _, t := range s.Tools {
		var sums []string
		for k, v := range t.SHA256 {
			sums = append(sums, k+"="+v)
		}
		sort.Strings(sums)
		ts = append(ts, tool{t.Name, t.Version, sums})
	}
	b, _ := json.Marshal(struct {
		Req   string
		Tools []tool
	}{s.Requirements(), ts})
	return hash(b)
}

func hash(b []byte) string {
	h := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(h[:])[:16]
}

func sorted(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

const specHeader = `# The agent environment of this repository, discovered from the code by ` + "`aayu install --env`" + `.
# Facts carry their evidence; [answers] records what the interview settled. Every host's
# environment (.agents/setup and the files that call it) is generated from this file:
# change it, then run ` + "`aayu install --env --apply`" + `.
`

// LoadSpec reads .aayudha/env.toml; ok is false when it does not exist.
func LoadSpec(root string) (s Spec, ok bool, err error) {
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(SpecPath)))
	if errors.Is(err, fs.ErrNotExist) {
		return Spec{}, false, nil
	}
	if err != nil {
		return Spec{}, false, err
	}
	if err := toml.Unmarshal(b, &s); err != nil {
		return Spec{}, false, fmt.Errorf("%s: %w", SpecPath, err)
	}
	return s, true, nil
}

// Marshal renders the spec as committed TOML.
func (s Spec) Marshal() []byte {
	var buf bytes.Buffer
	buf.WriteString(specHeader)
	enc := toml.NewEncoder(&buf)
	enc.SetIndentTables(false)
	_ = enc.Encode(s)
	return buf.Bytes()
}

// Record is one host's verification: what was verified, how, and the evidence.
type Record struct {
	Fingerprint string `toml:"fingerprint"`
	Method      string `toml:"method"` // container, cloud, reply
	Image       string `toml:"image,omitempty"`
	At          string `toml:"at"`
	Seconds     int    `toml:"seconds,omitempty"`
	Evidence    string `toml:"evidence,omitempty"` // a log path or a cloud task URL
	Project     string `toml:"project,omitempty"`  // Amp: the project the orb ran in
}

// Records is .aayudha/env-verified.toml: per host, the container
// verification and the live cloud verification, each the latest that passed.
type Records struct {
	Container map[string]Record `toml:"container"`
	Cloud     map[string]Record `toml:"cloud"`
}

// LoadVerified reads the verification records (empty when absent).
func LoadVerified(root string) (Records, error) {
	v := Records{Container: map[string]Record{}, Cloud: map[string]Record{}}
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(VerifiedPath)))
	if errors.Is(err, fs.ErrNotExist) {
		return v, nil
	}
	if err != nil {
		return v, err
	}
	if err := toml.Unmarshal(b, &v); err != nil {
		return v, fmt.Errorf("%s: %w", VerifiedPath, err)
	}
	if v.Container == nil {
		v.Container = map[string]Record{}
	}
	if v.Cloud == nil {
		v.Cloud = map[string]Record{}
	}
	return v, nil
}

// Save writes the verification records.
func (v Records) Save(root string) error {
	var buf bytes.Buffer
	buf.WriteString("# Agent environments aayu verified (`aayu install --env --verify`, `--cloud`). Generated; do not edit.\n")
	enc := toml.NewEncoder(&buf)
	enc.SetIndentTables(false)
	if err := enc.Encode(v); err != nil {
		return err
	}
	p := filepath.Join(root, filepath.FromSlash(VerifiedPath))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, buf.Bytes(), 0o644)
}
