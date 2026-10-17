package agentenv

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// States of a host's environment (doctor's agent-env check).
const (
	Verified = "verified" // generated from the current spec and verified in a clean container
	Stale    = "stale"    // the code's toolchain or the files changed since generation or verification
	Missing  = "missing"  // not generated or never verified
)

// HostStatus is one host's environment.
type HostStatus struct {
	Host   string   `json:"host"`
	State  string   `json:"state"`
	Detail string   `json:"detail"`
	Live   bool     `json:"live"` // a live cloud probe passed for this environment
	Lines  []string `json:"lines,omitempty"`
}

// Status reports every host's environment. It reads files only (and runs no
// command), so doctor and Detect can call it freely.
func Status(root string) ([]HostStatus, error) {
	s, ok, err := LoadSpec(root)
	if err != nil {
		return nil, err
	}
	var out []HostStatus
	if !ok {
		for _, h := range Hosts {
			out = append(out, HostStatus{Host: h, State: Missing, Detail: "no " + SpecPath + ": `aayu install --env --apply --verify` discovers, generates, and verifies it"})
		}
		return out, nil
	}
	var stale []string
	if disc, err := Discover(root); err == nil {
		if now, err := Settle(disc, s.Answers); err == nil {
			stale = changes(s, now)
		}
	}
	files, _, err := Generate(root, s)
	if err != nil {
		return nil, err
	}
	v, err := LoadVerified(root)
	if err != nil {
		return nil, err
	}
	fp := s.Fingerprint()
	for _, h := range Hosts {
		st := HostStatus{Host: h}
		switch {
		case !s.HasHost(h):
			st.State, st.Detail = Missing, "not set up for "+h+" (answer hosts in "+SpecPath+")"
			out = append(out, st)
			continue
		case len(stale) > 0:
			st.State, st.Detail, st.Lines = Stale, "the toolchain changed since "+SpecPath+" was generated; `aayu install --env --apply --verify` refreshes it", stale
			out = append(out, st)
			continue
		}
		var drift []string
		for _, f := range files {
			if !forHost(f.Path, h) {
				continue
			}
			b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f.Path)))
			switch {
			case err != nil:
				drift = append(drift, f.Path+" is missing")
			case !bytes.Equal(bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")), f.Content):
				drift = append(drift, f.Path+" differs from what "+SpecPath+" generates")
			case f.Exec && !executable(filepath.Join(root, filepath.FromSlash(f.Path))):
				drift = append(drift, f.Path+" is not executable")
			}
		}
		rec, okc := v.Container[h]
		cl, okl := v.Cloud[h]
		st.Live = okl && cl.Fingerprint == fp
		switch {
		case len(drift) > 0:
			st.State, st.Detail, st.Lines = Stale, "generated files changed; `aayu install --env --apply` rewrites them", drift
		case !okc:
			st.State, st.Detail = Missing, "generated, never verified: `aayu install --env --host "+h+" --verify`"
		case rec.Fingerprint != fp:
			st.State, st.Detail = Stale, fmt.Sprintf("verified %s for environment %s; the spec is now %s", rec.At, rec.Fingerprint, fp)
		default:
			st.State = Verified
			st.Detail = fmt.Sprintf("verified %s in a clean %s container (%s)", rec.At, rec.Image, fp)
			if st.Live {
				st.Detail += fmt.Sprintf("; live in %s's cloud %s (%s)", h, cl.At, cl.Evidence)
			} else if okl {
				st.Detail += "; its last live probe was of an older environment"
			}
		}
		out = append(out, st)
	}
	return out, nil
}

func executable(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && (fi.Mode()&0o111 != 0 || os.PathSeparator == '\\')
}

// forHost reports whether a generated file belongs to host's environment.
func forHost(p, h string) bool {
	switch p {
	case CursorEnvPath:
		return h == Cursor
	case ClaudeSetPath:
		return h == Claude
	case ResumePath:
		return h == Amp
	}
	return true
}

// changes lists how the code's requirements now differ from the spec's.
func changes(old, now Spec) []string {
	if old.Requirements() == now.Requirements() {
		return nil
	}
	var out []string
	for _, t := range now.Tools {
		if o := old.Tool(t.Name); o == nil {
			out = append(out, "new toolchain "+t.Name+" ("+t.Evidence+")")
		} else if o.Want != t.Want {
			out = append(out, fmt.Sprintf("%s: the code now asks for %s, the spec has %s (%s)", t.Name, t.Want, o.Want, t.Evidence))
		}
	}
	for _, t := range old.Tools {
		if now.Tool(t.Name) == nil && t.Name != "corepack" {
			out = append(out, t.Name+" is no longer used")
		}
	}
	diff := func(what string, a, b []string) {
		if strings.Join(sorted(a), "\n") != strings.Join(sorted(b), "\n") {
			out = append(out, fmt.Sprintf("%s: %s, was %s", what, orStr(strings.Join(b, ", "), "none"), orStr(strings.Join(a, ", "), "none")))
		}
	}
	runs := func(xs []Step) []string {
		var o []string
		for _, x := range xs {
			o = append(o, x.Run)
		}
		return o
	}
	diff("system packages", old.Packages, now.Packages)
	diff("dependency installs", runs(old.Deps), runs(now.Deps))
	diff("checks", runs(old.Checks), runs(now.Checks))
	diff("browsers", runs(old.Browsers), runs(now.Browsers))
	var os_, ns []string
	for _, s := range old.Services {
		os_ = append(os_, s.Name)
	}
	for _, s := range now.Services {
		ns = append(ns, s.Name)
	}
	diff("services", os_, ns)
	if len(out) == 0 {
		out = append(out, "the discovered requirements differ from the spec")
	}
	return out
}

// ErrNotLanded means the environment is verified here but its files are not
// on the branch the cloud host builds from yet.
var ErrNotLanded = errors.New("the agent environment is not on the default branch yet")

// EnsureOptions configure Ensure. The zero value works: origin, the branch
// origin/HEAD names, real downloads and Docker, logs under .aayudha/state/env.
type EnsureOptions struct {
	Base   string // the default branch hosts build from (dev here)
	Remote string // origin
	Fetch  Fetch  // resolves pins when the spec must be regenerated
	Run    Runner // container runner
	LogDir string
}

func (o *EnsureOptions) defaults(root string) {
	if o.Remote == "" {
		o.Remote = "origin"
	}
	if o.Base == "" {
		o.Base = "main"
		if out, err := exec.Command("git", "-C", root, "symbolic-ref", "--short", "refs/remotes/"+o.Remote+"/HEAD").Output(); err == nil {
			o.Base = strings.TrimPrefix(strings.TrimSpace(string(out)), o.Remote+"/")
		}
	}
	if o.Fetch == nil {
		o.Fetch = HTTPFetch(nil)
	}
	if o.Run == nil {
		o.Run = ExecRunner
	}
	if o.LogDir == "" {
		o.LogDir = filepath.Join(root, ".aayudha", "state", "env")
	}
}

// Ensure makes host's environment verified before a cloud hand-off: when it
// is missing or stale it regenerates the files from the code (recorded
// answers; new questions take their defaults), verifies them in a clean
// container, and records the result. A verified environment whose files are
// not on Remote/Base yet returns an error wrapping ErrNotLanded whose text is
// the exact command that lands them.
func Ensure(ctx context.Context, root, host string, o EnsureOptions) (HostStatus, error) {
	o.defaults(root)
	sts, err := Status(root)
	if err != nil {
		return HostStatus{}, err
	}
	st := find(sts, host)
	if st.State != Verified {
		disc, err := Discover(root)
		if err != nil {
			return st, err
		}
		old, _, _ := LoadSpec(root)
		s, err := Settle(disc, old.Answers)
		if err != nil {
			return st, err
		}
		if !s.HasHost(host) {
			s.Hosts = append(s.Hosts, host)
		}
		KeepPins(&s, old)
		if err := Resolve(ctx, &s, o.Fetch); err != nil {
			return st, err
		}
		s, res, err := Verify(ctx, s, VerifyOptions{Root: root, Host: host, Run: o.Run, LogDir: o.LogDir})
		if err != nil {
			return st, err
		}
		if err := Write(root, s); err != nil {
			return st, err
		}
		if !res.OK {
			return st, fmt.Errorf("the %s environment failed in a clean %s container after %d attempt(s); log %s:\n%s", host, res.Image, res.Attempts, res.Log, strings.Join(res.Tail, "\n"))
		}
		if err := RecordContainer(root, host, s, res); err != nil {
			return st, err
		}
		if sts, err = Status(root); err != nil {
			return st, err
		}
		st = find(sts, host)
	}
	if missing := notLanded(root, o.Remote+"/"+o.Base, host); len(missing) > 0 {
		return st, fmt.Errorf("%w: git add %s && git commit -s -m \"build(env): set up the %s agent environment\", then open a pull request to %s and merge it (%s)",
			ErrNotLanded, strings.Join(missing, " "), host, o.Base, strings.Join(missing, ", ")+" differ from "+o.Remote+"/"+o.Base)
	}
	return st, nil
}

func find(sts []HostStatus, h string) HostStatus {
	for _, s := range sts {
		if s.Host == h {
			return s
		}
	}
	return HostStatus{Host: h, State: Missing}
}

// KeepPins carries resolved pins over from the previous spec when the code's
// requirement for that tool did not change, so a regeneration is stable.
func KeepPins(s *Spec, old Spec) {
	for i := range s.Tools {
		t := &s.Tools[i]
		if o := old.Tool(t.Name); o != nil && o.Want == t.Want && (t.Version == "" || t.Version == o.Version) {
			t.Version, t.SHA256, t.Accept, t.Via = o.Version, o.SHA256, o.Accept, orStr(o.Via, t.Via)
		}
	}
	if c := old.Tool("corepack"); c != nil && s.Tool("corepack") == nil && (s.Tool("pnpm") != nil || s.Tool("yarn") != nil) {
		s.Tools = append(s.Tools, *c)
	}
	keepGate(s, old)
}

// notLanded lists the host's files that ref does not have as they are here.
func notLanded(root, ref, host string) []string {
	s, ok, err := LoadSpec(root)
	if err != nil || !ok {
		return nil
	}
	files, _, err := Generate(root, s)
	if err != nil {
		return nil
	}
	var out []string
	for _, f := range append(files, File{Path: VerifiedPath}) {
		if !forHost(f.Path, host) {
			continue
		}
		local, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(f.Path)))
		remote, err := exec.Command("git", "-C", root, "show", ref+":"+f.Path).Output()
		if err != nil || !bytes.Equal(remote, local) {
			out = append(out, f.Path)
		}
	}
	return out
}

// Write writes the generated files.
func Write(root string, s Spec) error {
	files, _, err := Generate(root, s)
	if err != nil {
		return err
	}
	for _, f := range files {
		p := filepath.Join(root, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if f.Exec {
			mode = 0o755
		}
		if err := os.WriteFile(p, f.Content, mode); err != nil {
			return err
		}
		if err := os.Chmod(p, mode); err != nil {
			return err
		}
	}
	return nil
}

func nowRFC3339() string { return time.Now().UTC().Format(time.RFC3339) }

// RecordContainer records a passing container verification.
func RecordContainer(root, host string, s Spec, res Result) error {
	v, err := LoadVerified(root)
	if err != nil {
		return err
	}
	secs := 0
	for _, n := range res.Phases {
		secs += n
	}
	v.Container[host] = Record{Fingerprint: s.Fingerprint(), Method: "container", Image: res.Image, At: nowRFC3339(), Seconds: secs,
		Evidence: fmt.Sprintf("setup cold %ds, warm %ds; check %ds; %d attempt(s)", res.Phases["setup-cold"], res.Phases["setup-warm"], res.Phases["check"], res.Attempts)}
	return v.Save(root)
}

// RecordCloud records a passing live probe.
func RecordCloud(root, host string, rec Record) error {
	v, err := LoadVerified(root)
	if err != nil {
		return err
	}
	v.Cloud[host] = rec
	return v.Save(root)
}
