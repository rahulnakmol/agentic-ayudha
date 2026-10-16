package loops

import (
	"fmt"
	"strings"
)

// The hosts below have no file or API that creates a recurring run, so a loop
// renders as the exact values and steps to enter (Appendix B S-7.1b).
const (
	claudeDocs = "https://code.claude.com/docs/en/routines"
	codexDocs  = "https://developers.openai.com/codex/automations"
)

const generatedMD = "<!-- Code generated from loops/*.toml by aayudha (WP7.1); DO NOT EDIT. -->\n\n"

// ClaudeRoutines renders the loops as Claude Code routines: one section per
// loop with the values for the claude.ai/code/routines form or `/schedule` in
// the CLI. A routine runs in Anthropic's cloud on a fresh clone, under the
// user's claude.ai login.
func ClaudeRoutines(ls []Loop, repo, branch, tz string) []byte {
	var b strings.Builder
	b.WriteString(generatedMD)
	fmt.Fprintf(&b, "# Claude Code routines for %s\n\n", repo)
	fmt.Fprintf(&b, "Each loop is a routine ([docs](%s)). Create it at `claude.ai/code/routines` (**New routine**), or run\n", claudeDocs)
	b.WriteString("`/schedule` in Claude Code and give it the values below; a GitHub trigger from the CLI needs Claude Code v2.1.225 or\n")
	b.WriteString("later and the Claude GitHub App on the repository. Routines need a claude.ai subscription login (not an API key), run\n")
	b.WriteString("as you, and count against your plan's daily routine runs. The routine's cloud environment must have the aayu\n")
	b.WriteString("binary: `aayu install --env --host claude` prints its setup script.\n\n")
	var gaps []string
	for _, l := range ls {
		if !l.Runs(HostClaude) {
			continue
		}
		if ok, why := l.Native(HostClaude); !ok {
			gaps = append(gaps, fmt.Sprintf("- `%s` (%s): %s.", l.Name, l.Label(tz), why))
			continue
		}
		fmt.Fprintf(&b, "## aayudha %s\n\n", l.Name)
		fmt.Fprintf(&b, "- **Name:** `aayudha %s (%s)`\n", l.Name, repo)
		fmt.Fprintf(&b, "- **Repository:** `%s`, branch `%s`\n", repo, branch)
		if ev, isEvent := l.Event(); isEvent {
			kind := map[string]string{"pull_request": "Pull request", "release": "Release"}[ev.Name]
			fmt.Fprintf(&b, "- **Trigger:** GitHub event, **%s**, action `%s`\n", kind, ev.Action)
			if l.Filter.Label != "" {
				fmt.Fprintf(&b, "- **Filter:** Labels is one of `%s`\n", l.Filter.Label)
			}
		} else {
			fmt.Fprintf(&b, "- **Trigger:** Schedule, %s\n", l.Label(tz))
			fmt.Fprintf(&b, "- **Cron:** `%s` in %s; when no preset says it, pick the closest one, then set this cron with `/schedule update`\n", l.Cron, tz)
		}
		b.WriteString("- **Prompt:**\n\n")
		writeFence(&b, l.AgentPrompt(repo, branch))
	}
	writeGaps(&b, gaps)
	return []byte(strings.TrimRight(b.String(), "\n") + "\n")
}

// CodexTasks renders the scheduled loops as Codex scheduled tasks in the
// ChatGPT desktop app: one section per loop with its RFC 5545 rule. A project
// task runs on this machine while the app is open.
func CodexTasks(ls []Loop, repo, branch, tz string) []byte {
	var b strings.Builder
	b.WriteString(generatedMD)
	fmt.Fprintf(&b, "# Codex scheduled tasks for %s\n\n", repo)
	fmt.Fprintf(&b, "Each scheduled loop is a Codex scheduled task ([docs](%s)). In the ChatGPT desktop app, open **Scheduled**, then\n", codexDocs)
	fmt.Fprintf(&b, "**New task**, choose this repository's local project (`%s`), run it on a **worktree**, set a custom schedule with the\n", repo)
	b.WriteString("rule below, and paste the prompt. The task runs as you, while the machine is on and the app is open; it uses your\n")
	b.WriteString("default sandbox settings, and the aayu binary must be on its PATH.\n\n")
	var gaps []string
	for _, l := range ls {
		if !l.Runs(HostCodex) {
			continue
		}
		if ok, why := l.Native(HostCodex); !ok {
			gaps = append(gaps, fmt.Sprintf("- `%s` (%s): %s.", l.Name, l.Label(tz), why))
			continue
		}
		fmt.Fprintf(&b, "## aayudha %s\n\n", l.Name)
		fmt.Fprintf(&b, "- **Name:** `aayudha %s (%s)`\n", l.Name, repo)
		fmt.Fprintf(&b, "- **Project:** `%s` on `%s`, run on a worktree\n", repo, branch)
		fmt.Fprintf(&b, "- **Schedule:** %s\n", l.Label(tz))
		_, rule, _ := strings.Cut(l.RRule(tz), "\n")
		fmt.Fprintf(&b, "- **Rule** (the app's times are your computer's time zone; this rule is in %s):\n\n", tz)
		writeFence(&b, rule+"\n")
		b.WriteString("- **Prompt:**\n\n")
		writeFence(&b, l.AgentPrompt(repo, branch))
	}
	writeGaps(&b, gaps)
	return []byte(strings.TrimRight(b.String(), "\n") + "\n")
}

func writeFence(b *strings.Builder, body string) {
	b.WriteString("  ```text\n")
	for _, line := range strings.Split(strings.TrimRight(body, "\n"), "\n") {
		if line == "" {
			b.WriteString("\n")
			continue
		}
		b.WriteString("  " + line + "\n")
	}
	b.WriteString("  ```\n\n")
}

func writeGaps(b *strings.Builder, gaps []string) {
	if len(gaps) == 0 {
		return
	}
	b.WriteString("## Not set up on this host\n\nThese loops start on an event this host has no trigger for; a host that has one runs them.\n\n")
	b.WriteString(strings.Join(gaps, "\n") + "\n")
}
