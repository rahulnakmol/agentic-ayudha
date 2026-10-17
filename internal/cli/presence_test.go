package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
)

// The presence check: a human-only command in an agent-marked shell (an orb's
// Terminal tab inherits AMP_THREAD_ID) runs only after the person whose
// account acts approves it through GitHub's device flow, with the repository
// access it needs. Every other outcome refuses and changes nothing.
func TestPresenceCheckDeviceFlow(t *testing.T) {
	const client = "Ov23liPresence"
	for _, tt := range []struct {
		name   string
		env    map[string]string // added to the marked shell's environment
		args   []string          // added to the command
		vars   map[string]string // repository variables besides the halt
		perm   string            // the founder's repository permission; "" is the fake's default (admin)
		device fake.DeviceFlow
		want   string // "" approves; else part of the refusal
	}{
		{name: "approve", device: fake.DeviceFlow{Approver: "founder", Pending: 2, Outcome: "approve"}},
		{name: "approve through the App's client ID", vars: map[string]string{"AAYUDHA_DEVICE_CLIENT_ID": "", "AAYUDHA_APP_CLIENT_ID": client},
			device: fake.DeviceFlow{Approver: "founder", Outcome: "approve"}},
		{name: "deny", device: fake.DeviceFlow{Approver: "founder", Outcome: "deny"}, want: "was not approved: the request was denied"},
		{name: "timeout", device: fake.DeviceFlow{Approver: "founder", ExpiresIn: 1}, want: "was not approved: the code expired"},
		{name: "bot approver", device: fake.DeviceFlow{Approver: "aayudha-r[bot]", ApproverType: "Bot", Outcome: "approve"}, want: "a bot identity"},
		{name: "configured bot login", env: map[string]string{"AAYU_IDENTITY__BOT_LOGINS": "founder"},
			device: fake.DeviceFlow{Approver: "founder", Outcome: "approve"}, want: "a bot identity"},
		{name: "under-permissioned approver", perm: "triage", device: fake.DeviceFlow{Approver: "founder", Outcome: "approve"}, want: "needs write access; @founder has triage"},
		{name: "wrong account", device: fake.DeviceFlow{Approver: "mallory", Outcome: "approve"}, want: "approved by @mallory, but aayu acts as @founder"},
		{name: "no client ID", vars: map[string]string{"AAYUDHA_DEVICE_CLIENT_ID": ""}, device: fake.DeviceFlow{Approver: "founder", Outcome: "approve"},
			want: "refused for an agent lane (AMP_THREAD_ID=T-0000)"},
		{name: "aayu's own agent lane", env: map[string]string{"AAYU_AGENT": "1"}, device: fake.DeviceFlow{Approver: "founder", Outcome: "approve"},
			want: "refused for an agent lane (AAYU_AGENT=1)"},
		{name: "in CI", args: []string{"--ci"}, device: fake.DeviceFlow{Approver: "founder", Outcome: "approve"},
			want: "refused for an agent lane (AMP_THREAD_ID=T-0000)"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := fake.New(t, "o/r", "User")
			g.Viewer = "founder"
			if tt.perm != "" {
				g.Permissions = map[string]string{"founder": tt.perm}
			}
			g.Variables = map[string]string{HaltVariable: "incident 9"}
			g.Variables["AAYUDHA_DEVICE_CLIENT_ID"] = client
			for k, v := range tt.vars {
				if v == "" {
					delete(g.Variables, k)
				} else {
					g.Variables[k] = v
				}
			}
			d := tt.device
			d.ClientID = client
			g.Device = &d
			dir := t.TempDir()
			env := map[string]string{"AAYU_GITHUB_TOKEN": "t", "XDG_STATE_HOME": filepath.Join(dir, "state")}
			for k, v := range hostShells["amp"] {
				env[k] = v
			}
			for k, v := range tt.env {
				env[k] = v
			}
			// Even at a terminal: the marked shell may be an agent typing into it.
			opts := Options{IsTerminal: stdinTTY, GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir, DevicePoll: 10 * time.Millisecond}
			code, _, errOut := run(t, env, opts, append([]string{"resume", "--repo", "o/r"}, tt.args...)...)
			g.Mu.Lock()
			_, halted := g.Variables[HaltVariable]
			started := g.Device.Started
			g.Mu.Unlock()
			log, _ := os.ReadFile(filepath.Join(dir, "state", "aayudha", "telemetry", "actions.jsonl"))
			if tt.want == "" {
				if code != 0 || halted || !strings.Contains(errOut, "resume on o/r needs a person's approval.\n  On any device, open https://github.com/login/device and enter WDJB-MJHT") ||
					!strings.Contains(errOut, "Approved by @founder (device-flow, admin access)") ||
					!bytes.Contains(log, []byte(`"kind":"presence.approve"`)) || !bytes.Contains(log, []byte(`"approver":"founder"`)) || bytes.Contains(log, []byte(fake.DeviceToken)) {
					t.Fatalf("exit %d, halted %v: %s\n%s", code, halted, errOut, log)
				}
				return
			}
			if code != ExitBlocked || !halted || !strings.Contains(errOut, tt.want) || bytes.Contains(log, []byte("presence.approve")) {
				t.Fatalf("exit %d, halted %v, want %q: %s\n%s", code, halted, tt.want, errOut, log)
			}
			if asked := started > 0; asked != (tt.name != "no client ID" && tt.name != "aayu's own agent lane" && tt.name != "in CI") {
				t.Errorf("device codes issued: %d", started)
			}
		})
	}
}

// An approval at write access does not stand in for an admin action (a
// bypass), and one approval covers the command's later steps at its level.
func TestPresenceCheckAccess(t *testing.T) {
	g := fake.New(t, "o/r", "User")
	g.Viewer = "founder"
	g.Permissions = map[string]string{"founder": "maintain"}
	g.Variables = map[string]string{"AAYUDHA_DEVICE_CLIENT_ID": "c"}
	g.Device = &fake.DeviceFlow{ClientID: "c", Approver: "founder", Outcome: "approve"}
	dir := t.TempDir()
	var errb bytes.Buffer
	e, err := newEnv(IO{In: strings.NewReader(""), Out: &bytes.Buffer{}, Err: &errb}, Options{
		Getenv: func(k string) string {
			return map[string]string{"AAYU_GITHUB_TOKEN": "t", "CLAUDECODE": "1", "XDG_STATE_HOME": dir, "HOME": dir}[k]
		},
		GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir, DevicePoll: time.Millisecond,
		GHToken: func(context.Context) (string, error) { return "", os.ErrNotExist },
	})
	if err != nil {
		t.Fatal(err)
	}
	e.Repo = "o/r"
	ctx := context.Background()
	if err := e.requireHuman(ctx, "plan --solo-bypass", AccessAdmin); err == nil || !strings.Contains(err.Error(), "needs admin access; @founder has write") {
		t.Fatalf("admin action approved at maintain: %v\n%s", err, errb.String())
	}
	if err := e.requireHuman(ctx, "decide --accept", AccessWrite); err != nil {
		t.Fatalf("write action: %v\n%s", err, errb.String())
	}
	if err := e.requireHuman(ctx, "inspect --dismiss", AccessWrite); err != nil || g.Device.Started != 2 {
		t.Fatalf("second write step: %v, device codes %d", err, g.Device.Started)
	}
	if err := e.requireHuman(ctx, "ship --solo-bypass", AccessAdmin); err == nil || g.Device.Started != 3 {
		t.Fatalf("an admin step after a write approval asked again and was refused: %v, device codes %d", err, g.Device.Started)
	}
}

// A fresh runner has no token and maybe no checkout: an action that needs no
// repository access is still approved, with the client ID from config, and
// the approval is refused when it cannot be recorded.
func TestPresenceCheckBootstrapAndRecord(t *testing.T) {
	g := fake.New(t, "o/r", "User")
	g.Device = &fake.DeviceFlow{ClientID: "c", Approver: "founder", Outcome: "approve"}
	newMarked := func(state string) (*Env, *bytes.Buffer) {
		var errb bytes.Buffer
		vars := map[string]string{"AMP_THREAD_ID": "T-0000", "XDG_STATE_HOME": state, "HOME": t.TempDir()}
		e, err := newEnv(IO{In: strings.NewReader(""), Out: &bytes.Buffer{}, Err: &errb}, Options{
			Getenv: func(k string) string { return vars[k] }, Environ: []string{"AAYU_IDENTITY__DEVICE_CLIENT_ID=c"},
			GitHubAPI: g.URL(), GitHubGap: -1, Dir: t.TempDir(), DevicePoll: time.Millisecond,
			GHToken: func(context.Context) (string, error) { return "", os.ErrNotExist },
		})
		if err != nil {
			t.Fatal(err)
		}
		return e, &errb
	}
	ctx := context.Background()
	e, errb := newMarked(t.TempDir())
	if e.confirm("Also edit your home directory? [y/N] ") {
		t.Fatal("confirmed off a terminal with no approval")
	}
	if err := e.requireHuman(ctx, "auth set-secret github-token", AccessAny); err != nil || !strings.Contains(errb.String(), "auth set-secret github-token needs a person's approval.") ||
		!strings.Contains(errb.String(), "Approved by @founder (device-flow, no repository access)") {
		t.Fatalf("bootstrap approval: %v\n%s", err, errb)
	}
	if !e.confirm("Also edit your home directory? [y/N] ") { // nobody can answer here: the approval of the command is its yes
		t.Fatal("an approved command's own confirmation was refused off a terminal")
	}
	if err := e.requireHuman(ctx, "resume", AccessWrite); err == nil {
		t.Fatal("a write action was approved with no repository to check the approver against")
	}
	blocked := filepath.Join(t.TempDir(), "file")
	_ = os.WriteFile(blocked, nil, 0o644)
	e, errb = newMarked(filepath.Join(blocked, "state")) // the actions log cannot be created under a file
	if err := e.requireHuman(ctx, "self-update", AccessAny); err == nil || !strings.Contains(err.Error(), "recording the approval in the actions log") ||
		strings.Contains(errb.String(), "Approved by") || e.approval != nil {
		t.Fatalf("an unrecorded approval: %v\n%s", err, errb)
	}
}
