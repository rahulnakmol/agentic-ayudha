# Human-only commands from host agent shells (Bug #170)

Recorded on 2026-10-15 with the binary built from `fix/170-agent-detection`, each human-only command with
`--dry-run` against this repository (every refusal comes before any GitHub call):

- `amp-shell.txt`: from the shell of the Amp thread that fixed the Bug, which sets `AGENT=amp` and `AMP_THREAD_ID`
  and not `AAYU_AGENT`.
- `claude-code-bash.txt`: from Claude Code 2.1.300's own Bash tool, the tool a skill's shell call goes through.
  Claude Code (`claude -p`) talked to a scripted local stand-in for the Anthropic Messages API (no real model and no
  key) that asked the Bash tool to run one command and then ended the turn; Claude Code ran it with `CLAUDECODE=1`.
  The Amp variables were removed first, so `CLAUDECODE` is the only marker.
- `claude-code.txt`: from a Claude Code 2.1.300 `SessionStart` hook (`claude -p` with an invalid API key, so no
  model ran). Claude Code spawns hook commands as it spawns its Bash tool's commands, with `CLAUDECODE=1`; the hook
  removed the Amp variables first, so `CLAUDECODE` is the only marker.
- `no-marker.txt`: with every marker removed, once with no terminal (refused) and once in a pseudo-terminal from
  `script(1)` (the human check passes and `ship` refuses on its own merge-gate check).

Cursor could not run here: its agent needs a Cursor account and Cursor's own backend, which no local stand-in
replaces. Its marker, `CURSOR_AGENT=1`, comes from Cursor's agent
execution layer as reported on Cursor's forum (Appendix B D-1.4c). `TestHostShellRecords` checks each refusal
against the current code's markers.
