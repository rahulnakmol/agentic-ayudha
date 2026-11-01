package agentenv

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ProbePrompt is what every host's cloud agent is asked, so a live
// environment is proven the same way everywhere: run the check, change
// nothing, and reply with its last line.
const ProbePrompt = "aayu environment probe. From the repository root, run exactly `" + CheckCommand + "` and wait for it to finish; it can take several minutes. " +
	"Do not edit, commit, push, or open anything, and do not try to fix a failure. Reply with only the last line the command printed, verbatim. " +
	"If it failed, reply `aayu-env failed:` followed by the last error line."

// CloudResult is one live probe.
type CloudResult struct {
	Host   string `json:"host"`
	OK     bool   `json:"ok"`
	URL    string `json:"url,omitempty"`
	Reply  string `json:"reply,omitempty"`
	Detail string `json:"detail"`
}

// Exec runs a command and returns its stdout (the probes' and Detect's
// transport; tests inject fakes).
type Exec func(ctx context.Context, dir, name string, args ...string) (string, error)

var ampThreadURL = regexp.MustCompile(`https://ampcode\.com/threads/T-[0-9a-f-]{36}`)

// ProbeAmp starts an orb thread in the repository's Amp project with the
// probe (`amp -ox --stream-json`, which stays attached until the turn ends)
// and checks the reply's marker against want. The orb runs the project's base
// branch, so want is that branch's fingerprint. Once the reply is read the
// thread is archived (founder decision 2026-10-16): `amp -x` archives a new
// thread when it finishes, and ProbeAmp archives it again explicitly so a
// changed default cannot leave it open. The URL stays as evidence.
func ProbeAmp(ctx context.Context, run Exec, project, want string) CloudResult {
	r := CloudResult{Host: Amp}
	out, err := run(ctx, "", "amp", "-ox", ProbePrompt, "--project", project, "--stream-json", "--title", "aayu environment probe")
	sc := bufio.NewScanner(strings.NewReader(out))
	sc.Buffer(make([]byte, 4<<20), 4<<20)
	for sc.Scan() {
		var ev struct {
			Type      string `json:"type"`
			Result    string `json:"result"`
			SessionID string `json:"session_id"`
			IsError   bool   `json:"is_error"`
		}
		if json.Unmarshal(sc.Bytes(), &ev) != nil {
			continue
		}
		if ev.SessionID != "" && r.URL == "" {
			r.URL = "https://ampcode.com/threads/" + ev.SessionID
		}
		if ev.Type == "result" {
			r.Reply = strings.TrimSpace(ev.Result)
		}
	}
	if r.URL == "" {
		r.URL = ampThreadURL.FindString(out)
	}
	if err != nil && r.Reply == "" {
		r.Detail = "amp -ox failed: " + err.Error() + ": " + clip(out, 300)
	} else {
		r = judge(r, want, true)
	}
	if id, ok := strings.CutPrefix(r.URL, "https://ampcode.com/threads/"); ok {
		if aout, aerr := run(ctx, "", "amp", "threads", "archive", id); aerr != nil {
			r.Detail += "; the probe thread was not archived (`amp threads archive " + id + "`): " + clip(strings.TrimSpace(aout), 200)
		}
	}
	return r
}

var markerLine = regexp.MustCompile(`^` + regexp.QuoteMeta(Marker) + ` (sha256:[0-9a-f]{16})$`)

// judge checks a probe's reply against want. With exact, only the reply's
// last line counts, and it must be the marker line itself, so a reply that
// only quotes the marker never passes (Bug #1211); otherwise the marker may
// stand anywhere in the text, as in Codex's status output.
func judge(r CloudResult, want string, exact bool) CloudResult {
	got, ok := ScanMarker(r.Reply)
	if exact {
		lines := strings.Split(strings.TrimSpace(r.Reply), "\n")
		m := markerLine.FindStringSubmatch(strings.TrimSpace(lines[len(lines)-1]))
		got, ok = "", m != nil
		if ok {
			got = m[1]
		}
	}
	switch {
	case !ok:
		r.Detail = "the reply's last line is not the `" + Marker + "` line: " + clip(r.Reply, 300)
	case got != want:
		r.Detail = fmt.Sprintf("the cloud environment is %s, but this checkout's spec is %s: merge the environment to the default branch, then probe again", got, want)
	default:
		r.OK, r.Detail = true, "the live environment ran "+CheckCommand+" and replied with environment "+got
	}
	return r
}

var codexTask = regexp.MustCompile(`https://chatgpt\.com/codex/tasks/([A-Za-z0-9_-]+)|\b(task_[A-Za-z0-9_-]+)\b`)

// ProbeCodex submits the probe as a Codex cloud task in the environment
// (`codex cloud exec`, experimental) and polls `codex cloud status` until the
// task ends, then looks for the marker in what the CLI prints. When the CLI
// does not print the reply, the human reads it in the task and records it.
func ProbeCodex(ctx context.Context, run Exec, envID, branch, want string, poll time.Duration) CloudResult {
	r := CloudResult{Host: Codex}
	out, err := run(ctx, "", "codex", "cloud", "exec", "--env", envID, "--branch", branch, ProbePrompt)
	if err != nil {
		r.Detail = "codex cloud exec failed: " + err.Error() + ": " + clip(out, 300)
		return r
	}
	m := codexTask.FindStringSubmatch(out)
	if m == nil {
		r.Detail = "no task ID in codex cloud exec's output: " + clip(out, 200)
		return r
	}
	id := m[1] + m[2]
	r.URL = "https://chatgpt.com/codex/tasks/" + id
	for {
		st, err := run(ctx, "", "codex", "cloud", "status", id)
		if err != nil {
			r.Detail = "codex cloud status failed: " + err.Error()
			return r
		}
		if regexp.MustCompile(`(?i)\b(completed|complete|ready|succeeded|done|failed|error|cancell?ed)\b`).MatchString(st) {
			r.Reply = st
			break
		}
		select {
		case <-ctx.Done():
			r.Detail = "gave up waiting for " + r.URL
			return r
		case <-time.After(poll):
		}
	}
	if _, ok := ScanMarker(r.Reply); !ok {
		r.Detail = "the task ended, but the CLI does not show its reply; open " + r.URL + " and run `aayu install --env --host codex --record '<the reply line>'`"
		return r
	}
	return judge(r, want, false)
}

// Record checks a probe reply a human copied from a host's UI and returns the
// record for it.
func RecordReply(host, reply, url, want string, now time.Time) (Record, error) {
	r := judge(CloudResult{Host: host, Reply: reply}, want, true)
	if !r.OK {
		return Record{}, fmt.Errorf("%s", r.Detail)
	}
	return Record{Fingerprint: want, Method: "reply", At: now.UTC().Format(time.RFC3339), Evidence: orStr(url, "a reply copied from the "+host+" UI")}, nil
}

func clip(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

// Allowlists: the domains each host lets a setup or session reach without
// configuration (the parts that matter to the script), per the hosts' docs.
var defaultAllowed = map[string][]string{
	// Claude Code's Trusted level (code.claude.com/docs/en/cloud-environments#default-allowed-domains).
	Claude: {"proxy.golang.org", "sum.golang.org", "golang.org", "registry.npmjs.org", "registry.yarnpkg.com", "nodejs.org", "github.com",
		"objects.githubusercontent.com", "release-assets.githubusercontent.com", "codeload.github.com", "pypi.org", "files.pythonhosted.org",
		"index.crates.io", "static.crates.io", "crates.io", "static.rust-lang.org", "archive.ubuntu.com", "security.ubuntu.com"},
	// Codex's Package managers preset (developers.openai.com/codex/environments/cloud-environments).
	Codex: {"proxy.golang.org", "sum.golang.org", "dl.google.com", "registry.npmjs.org", "registry.yarnpkg.com", "github.com", "codeload.github.com",
		"release-assets.githubusercontent.com", "pypi.org", "files.pythonhosted.org", "index.crates.io", "static.crates.io", "crates.io",
		"static.rust-lang.org", "archive.ubuntu.com", "security.ubuntu.com", "deb.debian.org", "security.debian.org"},
}

var stepURL = regexp.MustCompile(`https?://([A-Za-z0-9.-]+)`)

// Domains is every host the setup script may reach, given the spec.
func Domains(s Spec) []string {
	set := map[string]bool{}
	add := func(ds ...string) {
		for _, d := range ds {
			set[d] = true
		}
	}
	add("archive.ubuntu.com", "security.ubuntu.com", "deb.debian.org", "security.debian.org", "github.com")
	for _, t := range s.Tools {
		switch t.Name {
		case "go":
			add("proxy.golang.org", "sum.golang.org", "dl.google.com")
		case "node":
			add("nodejs.org")
		case "pnpm", "corepack":
			add("registry.npmjs.org")
		case "yarn":
			add("registry.npmjs.org", "repo.yarnpkg.com")
		case "uv", "python":
			add("release-assets.githubusercontent.com", "objects.githubusercontent.com", "pypi.org", "files.pythonhosted.org")
		case "rust":
			add("static.rust-lang.org", "index.crates.io", "static.crates.io")
		case "aayu":
			if t.Via == "release" {
				add("release-assets.githubusercontent.com", "objects.githubusercontent.com")
			}
		}
	}
	if len(s.Browsers) > 0 {
		add("cdn.playwright.dev", "playwright.download.prss.microsoft.com")
	}
	for _, st := range s.Steps {
		for _, m := range stepURL.FindAllStringSubmatch(st.Run, -1) {
			add(m[1])
		}
	}
	var out []string
	for d := range set {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

// ExtraDomains is what a host's default allowlist lacks, so its UI must add.
func ExtraDomains(host string, s Spec) []string {
	allowed := map[string]bool{}
	for _, d := range defaultAllowed[host] {
		allowed[d] = true
	}
	var out []string
	for _, d := range Domains(s) {
		if !allowed[d] {
			out = append(out, d)
		}
	}
	return out
}

// Manual is what a human does for a host whose environment lives in its own
// UI (Codex, Claude Code on the web) or needs a UI step (Cursor, Amp): the
// exact values and steps, then how aayu verifies the result.
func Manual(host string, s Spec, repo string) []string {
	var secrets, vars []string
	for _, e := range s.Env {
		if e.Secret {
			secrets = append(secrets, e.Name)
		} else {
			vars = append(vars, e.Name)
		}
	}
	needed := s.Answers["secrets"].Value
	if strings.EqualFold(needed, "none") {
		needed = ""
	}
	services := len(s.Services) > 0
	var l []string
	switch host {
	case Codex:
		extra := ExtraDomains(Codex, s)
		l = append(l,
			"Codex cloud environments are created in its UI only (no file or API defines one).",
			"1. Open https://chatgpt.com/codex/settings/environments and select Create environment; pick "+orStr(repo, "this repository")+".",
			"2. Install script (or, in Codex Cloud (Legacy), Setup script): "+SetupCommand)
		if services {
			l = append(l, "3. Start skill: Run `"+StartCommand+"` from the repository root and wait for `aayu-env: services ready`. (Legacy: Maintenance script `"+SetupCommand+" && "+StartCommand+"`.)")
		} else {
			l = append(l, "3. Start skill: none needed. (Legacy: Maintenance script `"+SetupCommand+"`, which re-syncs dependencies on a cached container.)")
		}
		net := "4. Internet access: on, Allow domains: Package managers"
		if len(extra) > 0 {
			net += "; Additional allowed domains: " + strings.Join(extra, ", ")
		} else {
			net += "; no additional domains"
		}
		l = append(l, net)
		if len(vars) > 0 {
			l = append(l, "5. Environment variables: "+strings.Join(vars, ", ")+" with the example values (names from "+strings.Split(s.EnvFile, " ")[0]+").")
		}
		if needed != "" {
			l = append(l, "6. Network secrets (or Personal vault): "+needed+"; never an environment variable.")
		}
		l = append(l, "7. Publish, copy the environment ID, and set it: add `codex_env = \"<id>\"` under [delegate] in .aayudha/config.toml.",
			"8. Verify: `aayu install --env --host codex --cloud` (needs `codex login` with ChatGPT), or start a task with the probe below and run `aayu install --env --host codex --record '<its reply>'`.")
	case Claude:
		extra := ExtraDomains(Claude, s)
		l = append(l,
			"Claude Code on the web: the repository's .claude/settings.json SessionStart hook runs "+SetupPath+" in every cloud session (CLAUDE_CODE_REMOTE=true), so no setup script is needed in the UI.",
			"1. Sign in to the CLI with your claude.ai account (`claude auth login`; an API key does not reach cloud sessions) and run `/web-setup`, or connect GitHub at https://claude.ai/code.")
		if len(extra) > 0 {
			l = append(l, "2. At https://claude.ai/code open the environment selector (the cloud icon), edit the environment, and set Network access to Custom, check Also include default list of common package managers, and allow: "+strings.Join(extra, ", ")+".")
		} else {
			l = append(l, "2. Network access: Trusted (the default) covers every host the setup reaches.")
		}
		if len(vars) > 0 {
			l = append(l, "3. Environment variables: "+strings.Join(vars, ", ")+" with the example values.")
		}
		if needed != "" {
			l = append(l, "4. Secrets the checks need ("+needed+"): network secrets on Pro and Max; on Team and Enterprise an environment variable, readable by everyone using the environment.")
		}
		l = append(l, "5. Verify: `claude --cloud \"<the probe below>\"`, then `aayu install --env --host claude --record '<its reply>'`.")
	case Cursor:
		l = append(l,
			"Cursor cloud agents read "+CursorEnvPath+" from the default branch: merge it there.",
			"1. Connect GitHub at https://cursor.com/dashboard/integrations and open https://cursor.com/dashboard/cloud-agents (environments) for this repository.",
			"2. Network access: the default allows every host the setup reaches; with an allowlist, add: "+strings.Join(Domains(s), ", ")+".")
		if len(secrets)+len(vars) > 0 && needed != "" {
			l = append(l, "3. Secrets tab: "+needed+".")
		}
		l = append(l, "4. Verify: start a cloud agent with the probe below, then `aayu install --env --host cursor --record '<its reply>'`.")
	case Amp:
		l = append(l,
			"Amp orbs run "+SetupPath+" from the project's base branch before snapshotting: merge it there.",
			"1. The repository needs an Amp project (`amp projects create`, or https://ampcode.com/settings).")
		if needed != "" {
			l = append(l, "2. Secrets the checks need ("+needed+"): `amp secrets` or the project's Secrets & Env Vars; setup gets no personal credentials.")
		}
		l = append(l, "3. Verify: `aayu install --env --host amp --cloud` (runs the probe in a new orb with `amp -ox`).")
	}
	n := 0
	for i, x := range l {
		if m := stepNum.FindStringIndex(x); m != nil {
			n++
			l[i] = fmt.Sprintf("%d. %s", n, x[m[1]:])
		}
	}
	return append(l, "Probe: "+ProbePrompt)
}

var stepNum = regexp.MustCompile(`^[0-9]+\. `)
