# Implementation plan: #917 GitHub Copilot as a first-class host

## Filed Stories and Bugs

| Number | Title | Type | Status | Effort | Risk | Depends on | Layers or paths | Evidence |
|---|---|---|---|---|---|---|---|---|
| — | No Story or Bug is filed under #917. | — | — | — | — | — | — | GitHub sub-issues API: `[]` (2026-11-07); no row in `_data/subs.tsv`; no PR refs 917 in `_data/merge-refs.tsv` or `_data/open-prs.tsv`. |

## Delivered

Nothing of #917 is delivered. These commits on `dev` are building blocks that #917 uses. They were built for other
work items:

| Commit | What it gives #917 |
|---|---|
| [b53f60e2](https://github.com/rahulnakmol/agentic-ayudha/commit/b53f60e2) (#491, before v0.3.0) | The Copilot host: `verbs.HostCopilot`, `internal/hosts/copilot.go`, install of skills, MCP, and hooks. |
| [b53f60e2](https://github.com/rahulnakmol/agentic-ayudha/commit/b53f60e2) (same commit) | Copilot session-log reading: `hosts.CopilotSessionLog`, `hosts.ReadCopilotSession`, `CopilotModels.Trailers`, `hosts.CopilotFamilies` (`internal/hosts/copilot.go:193-375`). The base for 917-B and the trailers of 917-D. |
| [80af4e1a](https://github.com/rahulnakmol/agentic-ayudha/commit/80af4e1a) (#1125, Story #1012) | A Copilot CLI session started and resumed by aayu (`delegate.KanhaStart`, `delegate.KanhaResume`, `delegate.NewSessionID`), for a mission's local Kanha. |
| [7239cb04](https://github.com/rahulnakmol/agentic-ayudha/commit/7239cb04) (#1136) | A two-minute limit on every hand-off and follow-up to a receiver. |
| [0cd53b4b](https://github.com/rahulnakmol/agentic-ayudha/commit/0cd53b4b) (#1279) | A stalled receiver ends build's status read at the time limit. |

## In review

No open PR refs #917 (`_data/open-prs.tsv`, 2026-11-07).

## Pending

All of Scope C1 to C4 is pending and not sliced. The slices below are **proposed, not filed**. The issue gives the
Feature `effort:L` and `risk:medium`. It gives no estimate per slice, so the slices carry none.

| Slice | Title (proposed, not filed) | Scope item | Depends on | Layers and paths |
|---|---|---|---|---|
| 917-A | Copilot lanes default to Auto tiers, and `models show` says why | C1 | — | L03 `internal/verbs/registry.go`; L05 `internal/models/resolve.go`; L02 `internal/cli/models.go`; `docs/guide/how-models.md`; generated docs |
| 917-B | aayu records the model of each Copilot review, and a Copilot Auto review counts with no family check | C2 | 917-A | L05 `internal/models/resolve.go` (`CheckFamilies`); L03 `internal/inspect/**` (inferred) |
| 917-C | A Copilot receiver's environment holds the Copilot token only | C3 (security part) | — | L06 `internal/hosts/copilot.go` (`CopilotToken`); L05 `internal/models/copilot.go` |
| 917-D | `aayu build --delegate copilot --local` runs `copilot -p`, resumes with `--resume`, and reads trailers from the session log | C3 | 917-A, 917-C | L03 `internal/delegate/state.go`, `internal/delegate/local.go`; L02 `internal/cli/delegate.go`; L03 `internal/verbs/registry.go` (the `--delegate` enum) |
| 917-E | `aayu install --host copilot` writes `copilot-setup-steps.yml`, and doctor refuses a plan that is not Business or Enterprise | C4 (setup part) | — | L06 `internal/hosts/install.go`, `hosts/**`; L03 `internal/doctor/checks.go`; L09 `.github/workflows/copilot-setup-steps.yml` |
| 917-F | `aayu build --delegate copilot` starts a cloud task on `dev`, follows up with a new task, and records model and usage | C4 | 917-D, 917-E | L03 `internal/delegate/receivers.go`, `internal/delegate/state.go`; L02 `internal/cli/delegate.go` |
| 917-G | The Copilot host guide covers the receivers, the tiers, and the plan rule | C1, C4 | 917-A, 917-F | `docs/guide/hosts/copilot.md`, `docs/guide/hosts/README.md` |

### Gaps the sources do not close

- The issue cites `internal/delegate/state.go:23` and `internal/hosts/copilot.go:389`. On 884d81b the first line is the
  `Copilot` constant's comment, and the second is `CopilotToken`. Both still match the problem they describe.
- The REST tasks API, its model field, and the plan check have no source in this repository. 917-E and 917-F each need
  a live probe first. Unverified.
- #917 depends on #871 ("Every agent write names its host and session"), which has no filed Stories in
  `_data/subs.tsv`. What #917 needs from #871 is not stated.
