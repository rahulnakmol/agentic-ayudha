package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/agentenv"
)

// Bug #1211: the committed record of the amp environment is stale, and the
// Kanha's orb has no Docker daemon. Build verifies amp with the live probe in
// a new orb instead of a container, and a current record changes nothing.

const staleEnv = "sha256:0000000000000000"

const probeThread = "T-01a1c0de-0000-7000-8000-000000001211"

// ampEnv commits an amp agent environment to dev, generated from m's
// checkout, with a container and a cloud record of environment fp ("" for
// the spec's own, a current record), and returns the spec.
func (m *missionWorld) ampEnv(fp string) agentenv.Spec {
	m.t.Helper()
	d, err := agentenv.Discover(m.dir)
	if err != nil {
		m.t.Fatal(err)
	}
	s, err := agentenv.Settle(d, map[string]agentenv.Answer{"hosts": {Value: "amp", Source: "stated"}})
	if err != nil {
		m.t.Fatal(err)
	}
	if err := agentenv.Write(m.dir, s); err != nil {
		m.t.Fatal(err)
	}
	if fp == "" {
		fp = s.Fingerprint()
	}
	if err := (agentenv.Records{Container: map[string]agentenv.Record{agentenv.Amp: {Fingerprint: fp, Method: "container", Image: "debian:12", At: "2026-10-16T02:35:02Z"}},
		Cloud: map[string]agentenv.Record{agentenv.Amp: {Fingerprint: fp, Method: "cloud", Project: "o/r", At: "2026-10-16T03:20:30Z"}}}).Save(m.dir); err != nil {
		m.t.Fatal(err)
	}
	git(m.t, m.dir, "add", ".aayudha", ".agents")
	git(m.t, m.dir, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "--quiet", "-m", "build(env): the amp environment")
	git(m.t, m.dir, "push", "--quiet", "origin", "HEAD:dev")
	git(m.t, m.dir, "remote", "set-head", "origin", "dev")
	return s
}

// noDocker makes every docker command fail, as on an Amp orb, and returns
// the commands tried.
func (m *missionWorld) noDocker() *[]string {
	var calls []string
	m.opts.EnsureEnv = nil // the real agentenv.Ensure
	m.opts.EnvRunner = func(_ context.Context, _ io.Reader, _ io.Writer, name string, args ...string) error {
		calls = append(calls, name+" "+strings.Join(args, " "))
		return errors.New("Cannot connect to the Docker daemon at unix:///var/run/docker.sock")
	}
	return &calls
}

// probeReplies makes the amp CLI's live probe reply with reply, and returns
// the probes it started.
func (m *missionWorld) probeReplies(reply string) *[]string {
	var probes []string
	prev := m.opts.Exec
	m.opts.Exec = func(ctx context.Context, dir, name string, args ...string) (string, error) {
		switch {
		case name == "amp" && len(args) > 1 && args[0] == "-ox":
			probes = append(probes, args[1])
			return fmt.Sprintf("{\"type\":\"system\",\"session_id\":%q}\n{\"type\":\"result\",\"result\":%q}\n", probeThread, reply), nil
		case name == "amp" && len(args) > 1 && args[0] == "threads" && args[1] == "archive":
			return "", nil
		}
		return prev(ctx, dir, name, args...)
	}
	return &probes
}

// startNext lists mission #889 and checks that it marks story n start.
func (m *missionWorld) startNext(n int) {
	m.t.Helper()
	if code, out, errOut := m.next(889); code != 0 || !strings.Contains(workLines(out), fmt.Sprintf("start #%d\n", n)) {
		m.t.Fatalf("mission next: exit %d: %q%s", code, out, errOut)
	}
}

func (m *missionWorld) cloudRecord() agentenv.Record {
	m.t.Helper()
	v, err := agentenv.LoadVerified(m.dir)
	if err != nil {
		m.t.Fatal(err)
	}
	return v.Cloud[agentenv.Amp]
}

// Criteria 2 and 4: the amp record is stale, no Docker daemon answers, and
// build runs in an Amp thread whose amp CLI is signed in. Build probes amp
// live, records [cloud.amp] at the new environment, and hands #20 off. The
// next story finds the record current: no container and no probe.
func TestAmpStaleEnvProbedLive(t *testing.T) {
	m := newMissionWorld(t)
	m.ampMission(889, 20, 21)
	s := m.ampEnv(staleEnv)
	docker, probes := m.noDocker(), m.probeReplies(agentenv.Marker+" "+s.Fingerprint())
	m.startNext(20)
	res := m.kanhaBuild(20, "amp")
	rec := m.cloudRecord()
	if res.Delegation.Status != dlgHandedOff || res.Delegation.Task != kanhaAmpThread || len(*probes) != 1 || (*probes)[0] != agentenv.ProbePrompt ||
		rec.Fingerprint != s.Fingerprint() || rec.Method != "cloud" || rec.Evidence != "https://ampcode.com/threads/"+probeThread || rec.Project != "o/r" {
		t.Fatalf("build 20 with a stale record and no Docker: %+v, probes %q, [cloud.amp] %+v", res.Delegation, *probes, rec)
	}
	if slices.ContainsFunc(*docker, func(c string) bool { return !strings.Contains(c, " info ") }) {
		t.Fatalf("a container ran with no Docker daemon: %q", *docker)
	}
	if !slices.ContainsFunc(res.Notes, func(n string) bool {
		return strings.HasPrefix(n, "amp's cloud environment: verified ") && strings.Contains(n, "live in amp's cloud")
	}) {
		t.Fatalf("build's notes do not say how amp was verified: %q", res.Notes)
	}
	tried := len(*docker) // mission next marked #21 start too
	if res := m.kanhaBuild(21, "amp"); res.Delegation.Status != dlgHandedOff || len(*probes) != 1 || len(*docker) != tried {
		t.Fatalf("build 21 with the record current: %+v, probes %d, docker %q", res.Delegation, len(*probes), (*docker)[tried:])
	}
}

// Criterion 4: the committed amp record is current. Build hands #20 off with
// no container and no probe, as before.
func TestAmpCurrentEnvHandsOff(t *testing.T) {
	m := newMissionWorld(t)
	m.ampMission(889, 20)
	m.ampEnv("")
	docker, probes := m.noDocker(), m.probeReplies("unexpected")
	m.startNext(20)
	if res := m.kanhaBuild(20, "amp"); res.Delegation.Status != dlgHandedOff || len(*probes) != 0 || len(*docker) != 0 {
		t.Fatalf("build 20 with a current record: %+v, probes %q, docker %q", res.Delegation, *probes, *docker)
	}
}

// Criterion 3: the amp record is stale and the live probe fails. Build stops
// with the probe's last error line and the command to run; #20 is
// status:ready again with its claim and lease released, and nothing is
// recorded or handed off.
func TestAmpStaleEnvProbeFails(t *testing.T) {
	m := newMissionWorld(t)
	m.ampMission(889, 20)
	m.ampEnv(staleEnv)
	m.noDocker()
	probes := m.probeReplies("aayu-env failed: go: command not found")
	// The amp CLI selects another Amp project: its orb proves nothing here, so no probe starts.
	other, exec := true, m.opts.Exec
	m.opts.Exec = func(ctx context.Context, dir, name string, args ...string) (string, error) {
		if name == "amp" && strings.Join(args, " ") == "projects status" && other {
			return "Project: other/r\n", nil
		}
		return exec(ctx, dir, name, args...)
	}
	m.startNext(20)
	code, out, errOut := run(t, m.agent, m.opts, "build", "20", "--delegate", "amp", "--repo", "o/r", "--asked", "aayu mission next marked it start")
	if code == 0 || len(*probes) != 0 || !strings.Contains(errOut, "amp projects status selects the Amp project other/r, not this repository (o/r)") || m.cloudRecord().Project != "o/r" {
		t.Fatalf("build 20 with another Amp project selected: exit %d, probes %q, [cloud.amp] %+v: %s%s", code, *probes, m.cloudRecord(), out, errOut)
	}
	other = false
	m.startNext(20)
	calls := len(m.ampCalls())
	code, out, errOut = run(t, m.agent, m.opts, "build", "20", "--delegate", "amp", "--repo", "o/r", "--asked", "aayu mission next marked it start")
	if code == 0 || len(*probes) != 1 || !strings.Contains(errOut, "the live amp probe did not pass (https://ampcode.com/threads/"+probeThread+")") ||
		!strings.Contains(errOut, "aayu-env failed: go: command not found") || !strings.Contains(errOut, "`aayu install --env --host amp --apply --verify`") ||
		!strings.Contains(errOut, "#20 is status:ready again, and its claim and lease are released") {
		t.Fatalf("build 20 with a failing probe: exit %d: %s%s", code, out, errOut)
	}
	if m.claims(889)[20] != 0 || !slices.Contains(m.labels(20), "status:ready") || slices.Contains(m.labels(20), LeaseLabel) {
		t.Fatalf("#20 is still claimed: claims %v, labels %v", m.claims(889), m.labels(20))
	}
	if got := m.ampCalls()[calls:]; slices.ContainsFunc(got, func(c string) bool { return strings.HasPrefix(c, "amp -ox") }) || m.cloudRecord().Fingerprint != staleEnv {
		t.Fatalf("a failed probe handed off %q or recorded %+v", got, m.cloudRecord())
	}
}

// Criteria 2 and 3 in a Kanha thread whose amp CLI is not signed in (Bug
// #1208): build starts no CLI. It writes the probe for the host and releases
// #20's start. The Kanha runs the probe in a thread of its own, records the
// reply, and the next start hands #20 off to the host.
func TestAmpStaleEnvHostProbe(t *testing.T) {
	m := newMissionWorld(t)
	m.hostMission(20)
	s := m.ampEnv(staleEnv)
	m.noDocker()
	probes := m.probeReplies("unexpected")
	// A probe that cannot be written for the host fails the start too, and releases it.
	dir := filepath.Join(m.dir, ".aayudha", "state", "delegate", "20")
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	m.startNext(20)
	code, out, errOut := run(t, m.agent, m.opts, "build", "20", "--delegate", "amp", "--repo", "o/r", "--asked", "aayu mission next marked it start")
	if code == 0 || !strings.Contains(errOut, "the probe for the host was not written") || !strings.Contains(errOut, "#20 is status:ready again, and its claim and lease are released") {
		t.Fatalf("build 20 with no place for the probe: exit %d: %s%s", code, out, errOut)
	}
	if err := os.Remove(dir); err != nil {
		t.Fatal(err)
	}
	m.startNext(20)
	code, out, errOut = run(t, m.agent, m.opts, "build", "20", "--delegate", "amp", "--repo", "o/r", "--asked", "aayu mission next marked it start")
	hand := regexp.MustCompile(`create one Amp thread titled "aayu environment probe" with the text of (\S+) as its prompt, and wait for its reply, at most 15 minutes\. ` +
		"Then run `aayu install --env --host amp --record '<its reply>'`, and start #20 again").FindStringSubmatch(errOut)
	if code == 0 || hand == nil || len(*probes) != 0 || !strings.Contains(errOut, "#20 is status:ready again, and its claim and lease are released") {
		t.Fatalf("build 20 with no amp sign-in: exit %d: %s%s", code, out, errOut)
	}
	if b, err := os.ReadFile(hand[1]); err != nil || string(b) != agentenv.ProbePrompt || m.claims(889)[20] != 0 {
		t.Fatalf("the probe for the host %s: %q %v; claims %v", hand[1], b, err, m.claims(889))
	}
	if code, out, errOut := run(t, m.agent, m.opts, "install", "--env", "--host", "amp", "--record", agentenv.Marker+" "+s.Fingerprint()); code != 0 {
		t.Fatalf("install --env --record: exit %d: %s%s", code, out, errOut)
	}
	m.startNext(20)
	code, out, errOut = run(t, m.agent, m.opts, "build", "20", "--delegate", "amp", "--repo", "o/r", "--asked", "aayu mission next marked it start")
	if code != 0 || !strings.Contains(out, "host hand-off: create one Amp thread titled \"aayu #20: Item 20\"") || m.state(20).Status != dlgHost || len(*probes) != 0 {
		t.Fatalf("build 20 after the host's probe: exit %d: %s%s", code, out, errOut)
	}
}

// Criterion 5: a person starts a mission on amp while the amp record is
// stale. Start says so and names the command before the device code; with a
// current record it says nothing.
func TestMissionStartWarnsStaleEnv(t *testing.T) {
	m := newMissionWorld(t)
	m.ampEnv(staleEnv)
	m.featureWith(889, 20)
	if code, out, errOut := m.approve(889); code != 0 {
		t.Fatalf("approve: exit %d: %s%s", code, out, errOut)
	}
	m.setTool("amp")
	code, out, errOut := m.mission(m.person, "rahulnakmol", "start", 889)
	warn := strings.Index(errOut, "warning: the amp environment record is stale: verified 2026-10-16T02:35:02Z for environment "+staleEnv)
	cmd := strings.Index(errOut, "Verify it with `aayu install --env --host amp --apply --verify`")
	device := strings.Index(errOut, "On any device, open")
	if code != 0 || warn < 0 || cmd < warn || device < cmd {
		t.Fatalf("mission start 889 with a stale record: exit %d: %s%s", code, out, errOut)
	}
	m.ampEnv("")
	m.featureWith(874, 950)
	if code, out, errOut := m.approve(874); code != 0 {
		t.Fatalf("approve 874: exit %d: %s%s", code, out, errOut)
	}
	if code, out, errOut := m.mission(m.person, "rahulnakmol", "start", 874); code != 0 || strings.Contains(errOut, "environment record is stale") {
		t.Fatalf("mission start 874 with a current record: exit %d: %s%s", code, out, errOut)
	}
}
