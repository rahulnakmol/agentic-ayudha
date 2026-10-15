# Delegation fixtures

Recorded shapes of each receiver's answers, for the adapters' tests (WP5.5b). No Cursor, Codex, or Amp account was
connected when they were written (2026-10-15), so the values are synthetic. The shapes follow each receiver's
official documentation on that date (sources in `docs/research/agent-delegation-and-review.md` §5):

- `cursor/`: Cursor cloud agents API v1, `POST /v1/agents`, `GET /v1/agents/{id}/runs/{runId}`, `POST /v1/agents/{id}/runs`.
- `codex/`: the output of `codex cloud exec`, `codex cloud status`, and `codex cloud diff` (codex-cli 0.162.1, marked
  experimental; the text format is not documented, so the adapter matches task URLs, task IDs, and status words).
- `amp/`: the output of `amp -ox --project`, which prints the new thread's URL.

When a receiver is connected, `AAYU_LIVE=1` runs the live tests in `internal/delegate/live_test.go` against it.
