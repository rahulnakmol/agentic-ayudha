# Implementation plan: Feature #977

## All Stories and Bugs

Type, status, effort and risk are from `_data/subs.tsv`. Depends-on is from each issue's Links section. Paths are the
Feature's expected paths ([#977](https://github.com/rahulnakmol/agentic-ayudha/issues/977)) split by Story; the split
is inferred. No PR into dev since v0.3.0 refers to these issues (`_data/merge-refs.tsv`), and no open PR does
(`_data/open-prs.tsv`).

| # | Title | Type | Status | Effort | Risk | Depends on | Layers / paths (planned) | Evidence |
|---|---|---|---|---|---|---|---|---|
| [#969](https://github.com/rahulnakmol/agentic-ayudha/issues/969) | Deck: clear an area one card at a time, with swipes that never ship | Story | planned | M | high | #937, #958, #928 | L02 `internal/cli/inboxweb/` (deck), `internal/cli/inboxtui.go` (`D`); L07 `internal/config/` (`[inbox] undo_seconds`) | not started |
| [#970](https://github.com/rahulnakmol/agentic-ayudha/issues/970) | Pair a phone on your tailnet with a one-time QR code | Story | planned | M | high | #957 | L02 `internal/cli/inboxweb/` (pairing), `internal/cli/inboxtui.go` (`Q`); L07 `internal/config/` (`[inbox.web] pair_code_minutes`); L09 `go.mod`; `THIRD_PARTY_NOTICES.md` | not started |
| [#971](https://github.com/rahulnakmol/agentic-ayudha/issues/971) | The Behaviours view: every behaviour you approved, its source, and its last result | Story | planned | M | medium | #891, #958 | L02 `internal/cli/inboxweb/` (view) | not started |
| [#972](https://github.com/rahulnakmol/agentic-ayudha/issues/972) | Ask why on every shipped item | Story | planned | S | medium | #890, #958 | L02 `internal/cli/inboxweb/`, `internal/cli/inbox*.go` (Done card action) | not started |
| [#973](https://github.com/rahulnakmol/agentic-ayudha/issues/973) | Ask Atri anywhere: counsel on any card or issue, and Atri never acts | Story | planned | S | low | #930, #964, #958 | L02 `internal/cli/inboxweb/`, `internal/cli/inbox.go` (`CounselFrom` facts), `internal/cli/inboxtui.go` (`?c`) | not started |

No Bug is filed under #977 (`_data/subs.tsv`).

## Delivered

Nothing. None of the five Stories has code on dev. The parts they reuse exist: card counsel (`InboxItem.Counsel`,
`CounselFrom` in `internal/cli/inbox.go`) and the `counsel` verb (`internal/verbs/registry.go`).

## In review

Nothing (`_data/open-prs.tsv`).

## Pending

All 5 Stories. They wait on:

1. Wave 3, Feature [#976](https://github.com/rahulnakmol/agentic-ayudha/issues/976): #957, #958 and #964, none built.
2. Wave 2, Feature [#975](https://github.com/rahulnakmol/agentic-ayudha/issues/975): #937 (0 of its 20 sub-issues
   merged). Wave 1, Feature [#895](https://github.com/rahulnakmol/agentic-ayudha/issues/895): #928 and #930 (0 of its
   15 sub-issues merged) (`_data/subs.tsv`).
3. [#891](https://github.com/rahulnakmol/agentic-ayudha/issues/891) (behaviours, v0.4.0) and
   [#890](https://github.com/rahulnakmol/agentic-ayudha/issues/890) (defense and `aayu explain`, v0.4.1).
4. The baselines, Hanu's review (D4), and Approve to build ([#977](https://github.com/rahulnakmol/agentic-ayudha/issues/977) Scope).

## Gaps not sliced into Stories

- Milestone mismatch: #977 (v0.4.0) and #972 depend on #890, which is in v0.4.1 (GitHub, read 2026-11-07). Either
  #972 moves to v0.4.1, or #890 moves into v0.4.0. A person decides.
- How the server checks that a tailnet request comes from the founder (#970 c3) is not designed in any source I
  found; unverified.
- The age at which a behaviour result becomes "stale" (#971 c3 uses 9 days) has no config key or rule in the issue.
- The UX design source (interaction specs v4, prototypes) is not in the repository. A UX baseline must pin it.
- No source gives a time estimate beyond "about 0.8 h of mission time"
  ([#977](https://github.com/rahulnakmol/agentic-ayudha/issues/977) Scope).
