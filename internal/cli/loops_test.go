package cli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	aayudha "github.com/rahulnakmol/agentic-ayudha"
	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/loops"
)

// WP7.1 accept: the Actions fallback's digest job (guard, then status
// --digest) on schedule pages exactly one digest into the file sink; the other
// UTC cron of the day is not due; setting halt makes the next scheduled run
// exit 4 with no writes; the budget stops it with exit 6.
func TestLoopDigestOnSchedule(t *testing.T) {
	g := statusTree(t)
	dir, _ := ledgerCheckout(t) // status reads proof coverage from the ledger in a checkout
	_ = os.MkdirAll(filepath.Join(dir, "loops"), 0o755)
	_ = os.MkdirAll(filepath.Join(dir, ".aayudha"), 0o755)
	digest, err := os.ReadFile(filepath.Join("..", "..", "loops", "digest.toml"))
	if err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(dir, "loops", "digest.toml"), digest, 0o644)
	opened, err := os.ReadFile(filepath.Join("..", "..", "loops", "triage-opened.toml"))
	if err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(dir, "loops", "triage-opened.toml"), opened, 0o644)
	cfg, err := os.ReadFile(filepath.Join("..", "..", ".aayudha", "config.toml")) // aayudha's own: [notify] channel = "file"
	if err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(dir, ".aayudha", "config.toml"), cfg, 0o644)
	event := filepath.Join(t.TempDir(), "event.json")
	output := filepath.Join(t.TempDir(), "output")
	now := time.Date(2026, 10, 12, 8, 5, 0, 0, time.UTC) // Monday 09:05 London (BST)
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir, Now: func() time.Time { return now }}
	env := func(halt string) map[string]string {
		return map[string]string{"GITHUB_TOKEN": "workflow-token", "GITHUB_ACTIONS": "true", "GITHUB_EVENT_NAME": "schedule",
			"GITHUB_EVENT_PATH": event, "GITHUB_OUTPUT": output, "AAYUDHA_HALT": halt, "AAYU_AGENT": "1"}
	}
	guard := func(fired, halt string, extra ...string) (int, string) {
		t.Helper()
		_ = os.WriteFile(event, []byte(`{"schedule":"`+fired+`"}`), 0o644)
		_ = os.Remove(output)
		code, _, errOut := run(t, env(halt), opts, append([]string{"hook", "loop:digest", "--ci", "--repo", "o/r"}, extra...)...)
		out, _ := os.ReadFile(output)
		return code, string(out) + errOut
	}
	// An event loop is due whenever its tool's trigger starts it, at any time, but halt and the breaker still stop it.
	eventGuard := func(halt string) (int, string) {
		t.Helper()
		code, out, errOut := run(t, map[string]string{"GITHUB_TOKEN": "workflow-token", "AAYUDHA_HALT": halt, "AAYU_AGENT": "1"}, opts, "hook", "loop:triage-opened", "--repo", "o/r")
		return code, out + errOut
	}
	if code, out := eventGuard(""); code != 0 || !strings.Contains(out, "event: When an issue opens with the label intake") {
		t.Errorf("event loop at 09:05 on a Monday: exit %d, %q", code, out)
	}
	outbox := filepath.Join(dir, filepath.FromSlash(loops.OutboxPath))
	pages := func() int {
		b, _ := os.ReadFile(outbox)
		return strings.Count(string(b), "\n")
	}

	if code, out := guard("5 9 * * 1-5", ""); code != 0 || out != "due=false\n" {
		t.Errorf("the GMT cron in summer: exit %d, %q", code, out)
	}
	code, out := guard("5 8 * * 1-5", "")
	if code != 0 || out != "due=true\n" {
		t.Fatalf("the BST cron: exit %d, %q", code, out)
	}
	if code, _, errOut := run(t, env(""), opts, "status", "--digest", "--ci", "--repo", "o/r"); code != 0 || pages() != 1 {
		t.Fatalf("status --digest: exit %d, %d pages: %s", code, pages(), errOut)
	}
	if b, _ := os.ReadFile(outbox); !strings.Contains(string(b), `"key":"status-digest:o/r"`) || !strings.Contains(string(b), "aayudha status digest for o/r") {
		t.Errorf("outbox: %s", b)
	}

	// Halted: the next run exits 4 at the guard, and the verb itself refuses too; nothing is written.
	g.Reset()
	if code, _ := guard("5 8 * * 1-5", "founder at 2026-10-12: test"); code != ExitBlocked {
		t.Errorf("halted guard: exit %d, want %d", code, ExitBlocked)
	}
	if code, _, _ := run(t, env("founder at 2026-10-12: test"), opts, "status", "--digest", "--ci", "--repo", "o/r"); code != ExitBlocked {
		t.Errorf("halted status --digest: exit %d, want %d", code, ExitBlocked)
	}
	if code, _ := eventGuard("founder at 2026-10-12: test"); code != ExitBlocked {
		t.Errorf("halted event loop: exit %d, want %d", code, ExitBlocked)
	}
	if w := g.Writes(); len(w) != 0 || pages() != 1 {
		t.Errorf("halted run wrote %v; %d pages", w, pages())
	}

	// [budget] ci_minutes_month: two runs of 5.5 minutes are billed as 12; the boundary stops the loop.
	g.Mu.Lock()
	g.Static = map[string]any{"/repos/o/r/actions/workflows/aayudha-loops.yml/runs": map[string]any{"total_count": 2, "workflow_runs": []any{
		map[string]any{"id": 1, "status": "completed", "run_started_at": "2026-10-05T08:05:00Z", "updated_at": "2026-10-05T08:10:30Z"},
		map[string]any{"id": 2, "status": "completed", "run_started_at": "2026-10-06T08:05:00Z", "updated_at": "2026-10-06T08:10:30Z"},
	}}}
	g.Mu.Unlock()
	if code, out := guard("5 8 * * 1-5", "", "--set", "budget.ci_minutes_month=13"); code != 0 || out != "due=true\n" {
		t.Errorf("12 minutes used of 13: exit %d %q", code, out)
	}
	if code, _ := guard("5 8 * * 1-5", "", "--set", "budget.ci_minutes_month=12"); code != ExitBudget {
		t.Errorf("12 minutes used of 12: exit %d, want %d", code, ExitBudget)
	}
	// The [budget] breaker's pause file stops every host's run.
	_ = os.MkdirAll(filepath.Join(dir, ".aayudha", "state"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, ".aayudha", "state", "paused"), []byte("per_day_usd"), 0o644)
	if code, _ := guard("5 8 * * 1-5", ""); code != ExitBudget {
		t.Errorf("paused: exit %d, want %d", code, ExitBudget)
	}
	if code, _ := eventGuard(""); code != ExitBudget {
		t.Errorf("paused event loop: exit %d, want %d", code, ExitBudget)
	}
}

// Bug #698: in a shell with an agent marker (an orb's Terminal tab inherits
// AMP_THREAD_ID), --apply asks the person through the device flow, naming the
// loops it starts as them, instead of refusing; a denied code starts nothing,
// and with no terminal it is refused with no code.
func TestInstallLoopsApplyPresence(t *testing.T) {
	g := fake.New(t, "o/r", "User")
	g.Viewer = "founder"
	g.Variables = map[string]string{"AAYUDHA_DEVICE_CLIENT_ID": "c"}
	dir := checkout(t)
	_ = os.MkdirAll(filepath.Join(dir, "loops", "amp"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "loops", "triage-daily.toml"), []byte("description = \"Triage.\"\ncommand = [\"triage\"]\ncron = \"0 9 * * *\"\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "loops", "amp", "triage-daily.json"), []byte("{}\n"), 0o644)
	threads := 0
	opts := Options{Dir: dir, GitHubAPI: g.URL(), GitHubGap: -1, DevicePoll: time.Millisecond, IsTerminal: stdinTTY,
		LookPath: func(b string) (string, error) { return "/usr/bin/" + b, nil },
		Exec: func(_ context.Context, _, name string, args ...string) (string, error) {
			if slices.Contains(args, "-ox") {
				threads++
			}
			return "schedule s-1 on T-01a14215-c759-72bf-9a65-d2317a72a878", nil
		}}
	env := map[string]string{"AAYU_GITHUB_TOKEN": "t", "AMP_THREAD_ID": "T-0000", "XDG_STATE_HOME": t.TempDir(), "CURSOR_TOKEN": "k"}
	apply := func(outcome string) (int, string) {
		t.Helper()
		g.Mu.Lock()
		g.Device = &fake.DeviceFlow{ClientID: "c", Approver: "founder", Outcome: outcome}
		g.Mu.Unlock()
		code, _, errOut := run(t, env, opts, "install", "--loops", "--repo", "o/r", "--host", "amp", "--apply")
		return code, errOut
	}
	code, errOut := apply("deny")
	if code != ExitBlocked || threads != 0 || !strings.Contains(errOut, "was not approved: the request was denied") {
		t.Fatalf("denied: exit %d, %d threads: %s", code, threads, errOut)
	}
	code, errOut = apply("approve")
	if code != 0 || threads != 1 || !strings.Contains(errOut, "install --loops --apply on o/r needs a person's approval.") ||
		!strings.Contains(errOut, "Approving: Starts one headless Amp thread per loop in orbs of the o/r project, to set up triage-daily. They act as you") ||
		!strings.Contains(errOut, "Approved by @founder") {
		t.Fatalf("approved: exit %d, %d threads: %s", code, threads, errOut)
	}
	// --run-now and Cursor's --apply name what they start as the person; Claude Code's --apply starts nothing, so it asks nothing.
	for _, tt := range []struct{ args, want string }{
		{"--host amp --run-now triage-daily", "Approving: Runs the loop triage-daily once now with amp's CLI in this checkout. It acts as you."},
		{"--host cursor --apply", "Approving: Runs terraform apply of loops/cursor/aayudha-loops.tf with your CURSOR_TOKEN, which creates a Cursor automation per loop (triage-daily) that runs as you."},
		{"--host claude --apply", ""},
	} {
		g.Mu.Lock()
		g.Device = &fake.DeviceFlow{ClientID: "c", Approver: "founder", Outcome: "approve"}
		g.Mu.Unlock()
		code, _, errOut := run(t, env, opts, append([]string{"install", "--loops", "--repo", "o/r"}, strings.Fields(tt.args)...)...)
		if asked := strings.Contains(errOut, "needs a person's approval"); code != 0 || asked != (tt.want != "") || !strings.Contains(errOut, tt.want) {
			t.Errorf("%s: exit %d: %s", tt.args, code, errOut)
		}
	}
	opts.IsTerminal = func(any) bool { return false }
	code, errOut = apply("approve")
	if code != ExitBlocked || threads != 1 || g.Device.Started != 0 || !strings.Contains(errOut, "a human runs them (agent lane: AMP_THREAD_ID=T-0000)") {
		t.Errorf("no terminal: exit %d, %d threads, %d codes: %s", code, threads, g.Device.Started, errOut)
	}
}

// WP7.1b, Bug #709: amp -ox returns when the thread starts, before its agent
// replies, so Amp's --apply waits for an event loop's thread to reply with its
// subscription ID, found on any line of its last message, and records it. The
// thread is asked to update and enable an equivalent subscription that Amp
// returns. With no reply in time nothing is recorded, and the step that
// records it by hand is printed.
func TestInstallLoopsEventSubscription(t *testing.T) {
	dir := checkout(t)
	_ = os.MkdirAll(filepath.Join(dir, "loops", "amp"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "loops", "triage-opened.toml"), []byte("description = \"Triage.\"\ncommand = [\"triage\"]\non = \"issues.opened\"\nfilter = { label = \"intake\" }\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "loops", "amp", "triage-opened.json"), []byte("{}\n"), 0o644)
	const thread = "T-01a152c0-d5d9-76b2-a75c-9f165feca52c"
	start := time.Date(2026, 10, 19, 6, 2, 0, 0, time.UTC)
	now, prompt, replyAt, polls, started := start, "", 0, 0, "https://ampcode.com/threads/"+thread
	opts := Options{Dir: dir, Now: func() time.Time { return now },
		Sleep:    func(_ context.Context, d time.Duration) error { now = now.Add(d); return nil },
		LookPath: func(b string) (string, error) { return "/usr/bin/" + b, nil },
		Exec: func(_ context.Context, _, name string, args ...string) (string, error) {
			if slices.Contains(args, "-ox") {
				prompt = args[len(args)-1]
				return started, nil
			}
			if strings.Join(args, " ") != "threads markdown "+thread {
				return "", errors.New("unexpected: " + strings.Join(args, " "))
			}
			// The thread's markdown: the prompt, which names `subscription <its ID>`, then the agent's work, then its reply.
			polls++
			// While it works, it names the subscription in a sentence: that is not its reply.
			md := "# aayudha loop: triage-opened\n\n## User\n\n" + prompt + "\n\n## Assistant\n\nAmp returned an existing subscription 5d10ad31-52c8-42ef-9860-3d874655784f; updating and enabling it.\n\n**Tool Use:** `code_exec`\n"
			if replyAt > 0 && polls >= replyAt {
				md += "\n## Assistant\n\nsubscription 5d10ad31-52c8-42ef-9860-3d874655784f\n\nAmp returned an existing subscription, which I updated and enabled.\n"
			}
			return md, nil
		}}
	report := func() loopsReport {
		t.Helper()
		now, polls = start, 0
		code, out, errOut := runIn(t, map[string]string{"AAYU_GITHUB_TOKEN": "t"}, opts, "", "install", "--loops", "--repo", "o/r", "--json", "--host", "amp", "--apply")
		var r loopsReport
		if err := json.Unmarshal([]byte(out), &r); err != nil || code != 0 {
			t.Fatalf("exit %d: %v: %s", code, err, errOut)
		}
		return r
	}
	r := report()
	if r.States[0].Status != loops.Missing || now.Sub(start) != 5*time.Minute || !strings.Contains(strings.Join(r.Steps, "\n"), "--record triage-opened=subscription <ID>") {
		t.Errorf("no reply: waited %s: %+v, steps %q", now.Sub(start), r.States, r.Steps)
	}
	if !strings.Contains(prompt, "create_github_subscription") || !strings.Contains(prompt, "update_github_subscription with its subscription_id, every field of this input, that project_id, and enabled: true") ||
		!strings.Contains(prompt, "reply with exactly one line, `subscription <its ID>`") || !strings.Contains(prompt, "If a call fails, reply with its error instead") {
		t.Errorf("prompt: %s", prompt)
	}
	// amp printed no thread ID: there is nothing to wait on.
	started = "amp: could not start the thread"
	if r = report(); r.States[0].Status != loops.Missing || polls != 0 || now != start {
		t.Errorf("no thread: %d looks, waited %s: %+v", polls, now.Sub(start), r.States)
	}
	started = "https://ampcode.com/threads/" + thread
	replyAt = 3
	if r = report(); r.States[0].Status != loops.Active || !strings.Contains(r.States[0].Detail, "subscription 5d10ad31-52c8-42ef-9860-3d874655784f") || polls != 3 {
		t.Errorf("reply on the third look (%d looks): %+v", polls, r.States)
	}
	// A CLI that waits for the reply prints it: recorded with no look.
	started = "Done.\nsubscription 395a6071-c14a-4fae-87bf-2d18da7f5bad"
	if r = report(); !strings.Contains(r.States[0].Detail, "subscription 395a6071-c14a-4fae-87bf-2d18da7f5bad") || polls != 0 {
		t.Errorf("reply printed by amp (%d looks): %+v", polls, r.States)
	}
}

// S-7.1e: install --loops sets every loop up in the one tool the user picks
// from those detected; Amp's --apply starts one headless Amp thread per loop
// and records it; recording a loop in a second tool warns that it runs twice,
// and an empty ID forgets it; --run-now tests one loop with the tool's CLI.
func TestInstallLoops(t *testing.T) {
	dir := checkout(t)
	_ = os.MkdirAll(filepath.Join(dir, "loops"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "loops", "digest.toml"), []byte("description = \"Digest.\"\ncommand = [\"status\", \"--digest\"]\ncron = \"5 9 * * 1-5\"\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "loops", "triage-daily.toml"), []byte("description = \"Triage.\"\ncommand = [\"triage\"]\ncron = \"0 9 * * *\"\nhosts = [\"amp\", \"codex\"]\n"), 0o644)
	// The rendered Amp inputs, as init --gates writes them.
	files, err := func() (map[string][]byte, error) {
		ls, err := loops.Load(dir)
		if err != nil {
			return nil, err
		}
		return loops.Files(ls, "o/r", "dev", "UTC", "dev", Implemented)
	}()
	if err != nil {
		t.Fatal(err)
	}
	for p, b := range files {
		_ = os.MkdirAll(filepath.Join(dir, filepath.Dir(p)), 0o755)
		_ = os.WriteFile(filepath.Join(dir, p), b, 0o644)
	}
	var calls []string
	opts := Options{Dir: dir,
		LookPath: func(b string) (string, error) {
			if b == "amp" || b == "codex" {
				return "/usr/bin/" + b, nil
			}
			return "", os.ErrNotExist
		},
		Exec: func(_ context.Context, _, name string, args ...string) (string, error) {
			calls = append(calls, strings.Join(append([]string{name}, args...), " "))
			if strings.HasSuffix(name, "amp") && slices.Contains(args, "-ox") {
				return "Done.\nschedule s-" + strings.TrimPrefix(args[slices.Index(args, "--title")+1], "aayudha loop: ") + " on T-01a14215-c759-72bf-9a65-d2317a72a878", nil
			}
			if strings.HasSuffix(name, "amp") && args[0] == "-x" {
				return "loop triage: ran", nil
			}
			return "", errors.New("not found")
		}}
	env := map[string]string{"AAYU_GITHUB_TOKEN": "t"}
	report := func(stdin string, args ...string) (int, loopsReport, string) {
		t.Helper()
		code, out, errOut := runIn(t, env, opts, stdin, append([]string{"install", "--loops", "--repo", "o/r", "--json"}, args...)...)
		var r loopsReport
		_ = json.Unmarshal([]byte(out), &r)
		return code, r, errOut
	}
	states := func(r loopsReport) string {
		var s []string
		for _, st := range r.States {
			s = append(s, st.Loop+"="+st.Owner+":"+st.Status)
		}
		return strings.Join(s, " ")
	}
	// Two tools detected and no terminal to ask: name one.
	if code, _, errOut := report(""); code != ExitUsage || !strings.Contains(errOut, "detected: amp, codex") {
		t.Fatalf("two tools, no terminal: exit %d: %s", code, errOut)
	}
	// At a terminal it offers only the detected tools.
	term := opts
	term.IsTerminal = func(any) bool { return true }
	if code, _, errOut := runIn(t, env, term, "cursor\n", "install", "--loops", "--repo", "o/r"); code != ExitUsage || !strings.Contains(errOut, "Detected: amp, codex. [amp/codex]") {
		t.Errorf("an undetected pick: exit %d: %s", code, errOut)
	}
	if code, _, errOut := runIn(t, env, term, "codex\nn\n", "install", "--loops", "--repo", "o/r"); code != 0 || !strings.Contains(errOut, "codex/scheduled-tasks.md (2 loops)") || !strings.Contains(errOut, "Test the triage-daily loop now in codex") {
		t.Errorf("codex picked: exit %d: %s", code, errOut)
	}
	// Amp with --apply: one headless thread per loop sets its schedule, and each loop is recorded.
	code, r, errOut := report("", "--host", "amp", "--apply")
	if code != 0 || states(r) != "digest=amp:active triage-daily=amp:active" {
		t.Fatalf("amp --apply: exit %d %s: %s", code, states(r), errOut)
	}
	if n := len(slices.DeleteFunc(slices.Clone(calls), func(c string) bool {
		return !strings.HasPrefix(c, "/usr/bin/amp -ox --project o/r --no-archive-after-execute")
	})); n != 2 {
		t.Errorf("amp threads started: %d, calls %v", n, calls)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "loops", "active.json")); !strings.Contains(string(b), `"id": "schedule s-digest on T-01a14215`) {
		t.Errorf("active.json:\n%s", b)
	}
	// Recording triage-daily in Codex as well: it would run twice, so both say so.
	if code, out, _ := run(t, env, opts, "install", "--loops", "--repo", "o/r", "--host", "codex", "--record", "triage-daily=task-1"); code != 0 || !strings.Contains(out, "triage-daily is also recorded in amp: it runs twice") {
		t.Errorf("double record: exit %d: %s", code, out)
	}
	code, r, _ = report("", "--host", "codex")
	if code != 0 || states(r) != "digest=amp:active triage-daily=amp,codex:double" || len(r.Warnings) != 1 || !strings.Contains(r.Warnings[0], "records digest, triage-daily as set up in amp") {
		t.Errorf("codex with loops in amp: exit %d %s %v", code, states(r), r.Warnings)
	}
	if code, _, _ := run(t, env, opts, "install", "--loops", "--repo", "o/r", "--host", "codex", "--record", "triage-daily="); code != 0 {
		t.Errorf("forget: exit %d", code)
	}
	_ = os.WriteFile(filepath.Join(dir, "loops", "digest.toml"), []byte("description = \"Digest.\"\ncommand = [\"status\", \"--digest\"]\ncron = \"5 8 * * 1-5\"\n"), 0o644)
	if _, r, _ := report("", "--host", "amp"); states(r) != "digest=amp:stale triage-daily=amp:active" || r.RunNow != "aayu install --loops --host amp --run-now triage-daily" {
		t.Errorf("after forgetting codex and moving the digest: %s, run now %q", states(r), r.RunNow)
	}
	// Run now: the tool's own CLI runs the loop once, past "not due".
	calls = nil
	code, out, errOut := run(t, env, opts, "install", "--loops", "--repo", "o/r", "--host", "amp", "--run-now", "triage-daily")
	if calls = slices.DeleteFunc(calls, func(c string) bool { return !strings.Contains(c, " -x ") }); code != 0 || out != "loop triage: ran\n" ||
		len(calls) != 1 || !strings.HasPrefix(calls[0], "/usr/bin/amp -x This is a Run now test") {
		t.Errorf("run now: exit %d %q %v: %s", code, out, calls, errOut)
	}
	// D-7.1f: a loop that needs no agent runs in the Actions fallback. Still recorded in Amp, it runs twice until
	// Amp forgets it; then only Actions runs it, and the report says so.
	_ = os.WriteFile(filepath.Join(dir, "loops", "digest.toml"), []byte("description = \"Digest.\"\ncommand = [\"status\", \"--digest\"]\ncron = \"5 8 * * 1-5\"\nagent = false\n"), 0o644)
	if _, r, _ := report("", "--host", "amp"); states(r) != "digest=actions,amp:double triage-daily=amp:active" {
		t.Errorf("digest now agent = false, still in amp: %s", states(r))
	}
	if code, _, errOut := run(t, env, opts, "install", "--loops", "--repo", "o/r", "--host", "amp", "--record", "digest="); code != 0 {
		t.Errorf("forget a loop amp no longer runs: exit %d: %s", code, errOut)
	}
	if _, r, _ := report("", "--host", "amp"); states(r) != "digest=actions:active triage-daily=amp:active" || !slices.Equal(r.Actions, []string{"digest"}) {
		t.Errorf("digest in Actions only: %s, actions %v", states(r), r.Actions)
	}
}

// D-7.1g: in a fresh repository, install --loops adds the default loops and every tool's render of them, so the
// Claude Code and Codex steps name files that exist; a product's copy carries no aayudha-only setup.
func TestInstallLoopsAddsDefaults(t *testing.T) {
	for _, host := range []string{"claude", "codex"} {
		dir := checkout(t)
		if host == "codex" { // D-7.1h: a pnpm product's sweep installs its dependencies and the browser with pnpm
			_ = os.WriteFile(filepath.Join(dir, "pnpm-lock.yaml"), []byte("lockfileVersion: '9.0'\n"), 0o644)
		}
		opts := Options{Dir: dir, Assets: aayudha.Assets, LookPath: func(string) (string, error) { return "", os.ErrNotExist }}
		code, out, errOut := run(t, map[string]string{"AAYU_GITHUB_TOKEN": "t"}, opts, "install", "--loops", "--repo", "o/r", "--host", host, "--json")
		var r loopsReport
		_ = json.Unmarshal([]byte(out), &r)
		if _, err := os.Stat(filepath.Join(dir, "loops", "triage-daily.toml")); code != 0 || err != nil || len(r.Stale) != 0 || len(r.States) == 0 {
			t.Fatalf("%s in a fresh repository: exit %d, stale %v, %d loops: %s", host, code, r.Stale, len(r.States), errOut)
		}
		prompts := map[string]string{"claude": "loops/claude/routines.md", "codex": "loops/codex/scheduled-tasks.md"}[host]
		if b, err := os.ReadFile(filepath.Join(dir, prompts)); err != nil || !strings.Contains(string(b), "aayudha triage-daily (o/r)") {
			t.Errorf("%s: %s: %v", host, prompts, err)
		}
		if b, _ := os.ReadFile(filepath.Join(dir, ".github", "workflows", loops.Workflow)); !strings.Contains(string(b), "loop:verify-sweep") || strings.Contains(string(b), "aayudha/floors") {
			t.Errorf("%s: the Actions fallback runs the sweep without aayudha's own setup:\n%s", host, b)
		}
		if b, _ := os.ReadFile(filepath.Join(dir, ".github", "workflows", loops.Workflow)); (host == "codex") != strings.Contains(string(b), "pnpm exec playwright install --with-deps chromium") {
			t.Errorf("%s: the sweep's setup does not follow the lockfile:\n%s", host, b)
		}
	}
}
