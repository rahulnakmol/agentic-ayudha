package loops

import (
	"fmt"
	"strings"
	"time"
)

// These tools create a recurring run from a chat or slash command, not from a
// file aayu can apply, so a loop renders as the exact prompt to paste
// (Appendix B S-7.1e).
const (
	claudeDocs = "https://code.claude.com/docs/en/routines"
	codexDocs  = "https://learn.chatgpt.com/docs/automations"
	cursorDocs = "https://cursor.com/docs/cloud-agent/automations"
)

const generatedMD = "<!-- Code generated from loops/*.toml by aayudha (WP7.1); DO NOT EDIT. -->\n\n"

// RunNowPrompt is the loop's prompt for a one-off test run outside its
// schedule: the guard still stops a halted or over-budget repository, but
// `not due` does not stop the test.
func (l Loop) RunNowPrompt(repo, branch string) string {
	return "This is a Run now test of the loop, outside its schedule: if step 1 prints `not due`, carry on with step 2 anyway. Stop on exit 4 or 6 as written.\n\n" +
		l.AgentPrompt(repo, branch)
}

// ClaudeRoutines renders the loops as Claude Code routines: one `/schedule`
// prompt per loop, to paste into Claude Code in a checkout of the repository.
// A routine runs in Anthropic's cloud on a fresh clone, under the user's
// claude.ai login.
func ClaudeRoutines(ls []Loop, repo, branch, tz string) []byte {
	var b strings.Builder
	b.WriteString(generatedMD)
	fmt.Fprintf(&b, "# Claude Code routines for %s\n\n", repo)
	fmt.Fprintf(&b, "Each loop is a routine ([docs](%s)). In Claude Code v2.1.225 or later, signed in with a claude.ai\n", claudeDocs)
	b.WriteString("subscription (not an API key) and started in a checkout of this repository, paste each loop's `/schedule` prompt\n")
	b.WriteString("below. Claude asks what it still needs, then saves the routine to your account; it runs as you, in Anthropic's\n")
	b.WriteString("cloud. If Claude picks a preset instead of the cron, run `/schedule update` and give it the cron. The shortest\n")
	b.WriteString("interval a routine allows is one hour. The routine's cloud environment needs the aayu binary:\n")
	b.WriteString("`aayu install --env --host claude` prints its setup script.\n\n")
	b.WriteString("To test a routine, run `/schedule run` and name it, or click **Run now** on its page. To test the prompt here first,\n")
	b.WriteString("run `aayu install --loops --host claude --run-now <loop>`.\n\n")
	for _, l := range ls {
		if !l.Runs(HostClaude) {
			continue
		}
		fmt.Fprintf(&b, "## aayudha %s\n\n%s.\n\n", l.Name, l.Label(tz))
		writeFence(&b, fmt.Sprintf("/schedule Create a recurring routine named \"aayudha %s (%s)\" for the repository %s, branch %s, on the cron schedule `%s` in the time zone %s (%s). Each run uses this prompt, exactly:\n\n%s",
			l.Name, repo, repo, branch, l.Cron, tz, l.Label(tz), l.AgentPrompt(repo, branch)))
	}
	return []byte(strings.TrimRight(b.String(), "\n") + "\n")
}

// CodexTasks renders the loops as Codex scheduled tasks: one prompt per loop
// to paste into a Codex or ChatGPT chat in the desktop app, which creates a
// standalone task on the local project. The task runs as the user, while the
// machine is on and the app is open.
func CodexTasks(ls []Loop, repo, branch, tz string) []byte {
	var b strings.Builder
	b.WriteString(generatedMD)
	fmt.Fprintf(&b, "# Codex scheduled tasks for %s\n\n", repo)
	fmt.Fprintf(&b, "Each loop is a Codex scheduled task ([docs](%s)). In the ChatGPT desktop app, open a Codex or ChatGPT chat\n", codexDocs)
	fmt.Fprintf(&b, "in this repository's local project (`%s`) and paste each loop's prompt below; ChatGPT creates the task, which you\n", repo)
	b.WriteString("then find under **Scheduled**. A task runs as you, with your default sandbox settings, while the machine is on and\n")
	b.WriteString("the app is open, and the aayu binary must be on its PATH.\n\n")
	b.WriteString("Test each prompt before it runs on its own, as the Codex documentation advises: run\n")
	b.WriteString("`aayu install --loops --host codex --run-now <loop>`, which runs it once with `codex exec`.\n\n")
	for _, l := range ls {
		if !l.Runs(HostCodex) {
			continue
		}
		_, rule, _ := strings.Cut(l.RRule(tz), "\n")
		fmt.Fprintf(&b, "## aayudha %s\n\n%s.\n\n", l.Name, l.Label(tz))
		writeFence(&b, fmt.Sprintf("Create a standalone scheduled task named \"aayudha %s (%s)\" for this local project, running on a new worktree from %s, with the custom schedule %s in the time zone %s (%s). Each run starts a new chat with this prompt, exactly:\n\n%s",
			l.Name, repo, branch, rule, tz, l.Label(tz), l.AgentPrompt(repo, branch)))
	}
	return []byte(strings.TrimRight(b.String(), "\n") + "\n")
}

// CursorAutomate renders the loops as prompts for Cursor's /automate skill,
// for a user without Terraform or a Cursor API key: one per loop, to paste in
// a local Cursor agent session. Cursor's cron runs in UTC.
func CursorAutomate(ls []Loop, repo, branch, tz string, loc *time.Location) ([]byte, error) {
	var b strings.Builder
	b.WriteString(generatedMD)
	fmt.Fprintf(&b, "# Cursor automations for %s, with /automate\n\n", repo)
	fmt.Fprintf(&b, "`aayu install --loops --host cursor --apply` creates these automations with Terraform when `terraform` and a\n")
	fmt.Fprintf(&b, "Cursor API key (`CURSOR_TOKEN`) are there. Without them, paste each loop's prompt below into a local Cursor agent\n")
	fmt.Fprintf(&b, "session in this repository: the `/automate` skill sets the automation up ([docs](%s)). Each runs as you\n", cursorDocs)
	b.WriteString("(**Run as: Me**). To test a prompt first, run `aayu install --loops --host cursor --run-now <loop>`.\n\n")
	for _, l := range ls {
		if !l.Runs(HostCursor) {
			continue
		}
		crons, err := l.UTCCrons(loc)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&b, "## aayudha %s\n\n%s.\n\n", l.Name, l.Label(tz))
		writeFence(&b, fmt.Sprintf("/automate Create an automation named \"aayudha %s (%s)\" on the repository github.com/%s, branch %s, that runs as me, with a scheduled trigger for each of these UTC cron expressions: %s (%s; the prompt's first step skips the one that is not due). Its prompt, exactly:\n\n%s",
			l.Name, repo, repo, branch, "`"+strings.Join(crons, "`, `")+"`", l.Label(tz), l.AgentPrompt(repo, branch)))
	}
	return []byte(strings.TrimRight(b.String(), "\n") + "\n"), nil
}

func writeFence(b *strings.Builder, body string) {
	b.WriteString("```text\n" + strings.TrimRight(body, "\n") + "\n```\n\n")
}
