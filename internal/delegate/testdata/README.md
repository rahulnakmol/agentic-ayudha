# Delegation fixtures

Recorded shapes of each receiver's answers, for the adapters' tests (WP5.5b, WP5.5c). No Cursor, Codex, or Amp account was
connected when they were written (2026-10-15), so the values are synthetic. The shapes follow each receiver's
official documentation on that date (sources in `docs/research/agent-delegation-and-review.md` §5):

- `cursor/`: Cursor cloud agents API v1, `POST /v1/agents`, `GET /v1/agents/{id}/runs/{runId}`, `POST /v1/agents/{id}/runs`.
- `codex/`: the output of `codex cloud exec`, `codex cloud status`, and `codex cloud diff` (codex-cli 0.162.1, marked
  experimental; the text format is not documented, so the adapter matches task URLs, task IDs, and status words).
- `codex/exec-local.txt`: what `codex exec` prints in a local hand-off. The header and the echoed prompt are as
  codex-cli 0.163.0 printed them in an orb (2026-10-16, no sign-in); the answer after `codex` is synthetic.
- `amp/`: the output of `amp -ox --project`, which prints the new thread's URL.
- `claude/`: the output of `claude --cloud "<brief>"` (a session ID and its `claude.ai/code` link; the new-session output
  is not documented, so the adapter matches either) and of `claude -p <msg> --cloud <session> --output-format json`,
  whose `{ok, session_id, url}` shape is documented (code.claude.com/docs/en/claude-code-on-the-web, 2026-10-16).
  Synthetic: no claude.ai sign-in was available.

When a receiver is connected, `AAYU_LIVE=1` runs the live tests in `internal/delegate/live_test.go` against it.
