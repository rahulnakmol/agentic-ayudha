# Implementation plan: Feature #895

## Sources

- Sub-issues, states, and labels: `_data/subs.tsv` (feature 895).
- Merges: `_data/merge-refs.tsv` has no squash merge that refs any of the 15 sub-issues.
- Open PRs: `_data/open-prs.tsv` has no open PR that refs any of the 15 sub-issues.
- Depends-on: the "Links" section of each issue body.

## All Stories and Bugs

| # | Title | Type | Status | Effort | Risk | Depends on | Layers or paths (planned) | Evidence |
|---|---|---|---|---|---|---|---|---|
| [#924](https://github.com/rahulnakmol/agentic-ayudha/issues/924) | A changed card writes nothing and says what changed, and every result shows on its card | Bug | planned | L | medium | — | L02: `inboxtui.go`, `inboxwork.go`, `inboxwhy.go`, `inbox.go` | not started |
| [#925](https://github.com/rahulnakmol/agentic-ayudha/issues/925) | The inbox never shows an empty list before its first read ends, and says when it read, when GitHub is slow, and when it is limited | Bug | planned | L | low | — | L02: `inboxtui.go`, `inbox.go` (cache); L07: `internal/config/defaults.toml` | not started |
| [#926](https://github.com/rahulnakmol/agentic-ayudha/issues/926) | A review that cannot finish waits in the inbox, and approve and ship is one choice | Story | planned | M | medium | #924 | L02: `inbox.go`, `inboxtui.go`; L03: `internal/inspect/inspect.go` | not started |
| [#927](https://github.com/rahulnakmol/agentic-ayudha/issues/927) | Ship a protected-path PR from its card with a one-line reason | Story | planned | L | high | #924 | L02: `inbox.go`, `inboxready.go`, `ship.go` (narrow set) | not started |
| [#928](https://github.com/rahulnakmol/agentic-ayudha/issues/928) | Ship several cards with one check and a hold to stop, and every inbox time is a config key | Story | planned | M | medium | #924, #925, #927 | L02: `inboxtui.go`, `inbox_test.go`; L07: `defaults.toml` | not started |
| [#929](https://github.com/rahulnakmol/agentic-ayudha/issues/929) | Links open or copy in any terminal: local, tmux, herdr, and SSH | Story | planned | M | low | — | L02: `inboxlogin.go`, `inbox.go` (`openURL`), `inboxtui.go`; docs: `docs/guide/` | not started |
| [#930](https://github.com/rahulnakmol/agentic-ayudha/issues/930) | A card says why you, shows the evidence, and puts the counsel above the choices | Story | planned | M | low | #924 | L02: `inboxtui.go` (`cardView`), `inbox.go` (`inboxCounsel`) | not started |
| [#931](https://github.com/rahulnakmol/agentic-ayudha/issues/931) | You learn that a card needs you without watching, and what changed since you looked | Story | planned | S | low | — | L02: `inboxtui.go`, `inbox.go`; L03: `internal/verbs/registry.go` (`--brief`); L07: `[ui] bell` | not started |
| [#932](https://github.com/rahulnakmol/agentic-ayudha/issues/932) | The header and the footer say where you are and what is armed | Story | planned | S | low | #925 | L02: `inboxtui.go` | not started |
| [#933](https://github.com/rahulnakmol/agentic-ayudha/issues/933) | Leave at any time, see what you did, and continue on another surface | Story | planned | M | low | #924 | L02: `inboxtui.go`, `inbox.go`; L03: `registry.go` (`--today`, positional) | not started |
| [#934](https://github.com/rahulnakmol/agentic-ayudha/issues/934) | Learn the keys when you need them, and see only the choices you may take | Story | planned | M | medium | #924 | L02: `inboxtui.go` (`helpView`), a keymap source (new), `inbox.go` | not started |
| [#935](https://github.com/rahulnakmol/agentic-ayudha/issues/935) | A code owner approves a protected-path PR from the inbox with a real GitHub review | Story | planned | M | high | — | L09: `.github/CODEOWNERS`; L07: `internal/config/`; L02: `inbox.go`; L06: `internal/github` (inferred) | not started |
| [#936](https://github.com/rahulnakmol/agentic-ayudha/issues/936) | Dismiss a false finding and waive a floor from the card | Story | planned | M | high | #924, #816 | L02: `inboxtui.go`, `inbox.go`, `inspect.go`; L03: `internal/inspect` | not started |
| [#980](https://github.com/rahulnakmol/agentic-ayudha/issues/980) | Command output wraps at a word, with its continuation indented | Story | planned | S | low | — | L03: `internal/ui/ui.go`; L02: `internal/cli/look.go` | not started |
| [#1122](https://github.com/rahulnakmol/agentic-ayudha/issues/1122) | aayu inbox fails with exit 5 when a marker's author gets a 404 from GitHub's permission read | Bug | planned | XS | low | — | L02: `inbox.go` (`canWrite`, `postedCounsel`), `inboxready.go` (`readyItem`) | not started |

The "Layers or paths" column is planned. It comes from the Feature's Expected paths (issue #895), the Story
Assumptions, and the code symbols named in [03-technical-design.md](03-technical-design.md). Paths marked "inferred"
are not named by a source.

## Delivered

No Story or Bug of #895 is merged into dev (`_data/merge-refs.tsv`). The inbox on dev has parts that the Stories
build on. None of them meets a full Completion Goal:

| Part on dev | Code | Came from | Stories it starts |
|---|---|---|---|
| Re-read before a write, refuse a changed card, say what changed | `reread`, `inboxMaterial`, `errInboxChanged` (`internal/cli/inbox.go`); `changedLine` (`inboxwhy.go`) | before v0.3.0; #850 fixed Bugs #846 and #847 for v0.3.0 (issue #895, Scope) | #924 |
| Write stages from HTTP calls | `inboxWork`, `inboxStage`, `inboxStepOf` (`internal/cli/inboxwork.go`) | before v0.3.0 | #924 C8 |
| Solo-mode bypass ship of a Story PR's Ready card | `shipReady` (`inbox.go:2062`), `shipBypassNote` (`ship.go:985`) | [c1dff14](https://github.com/rahulnakmol/agentic-ayudha/commit/c1dff14) (#848), before v0.3.0 | #927 (non-Story PRs are the new part) |
| `human-review` card when the gate records the human rung | `inbox.go:860` | Story #675, before v0.3.0 (code comment) | #926 |
| Housekeeping multi-select | `toggle`, `selectHouse` (`inboxtui.go:1568`) | [17142e2](https://github.com/rahulnakmol/agentic-ayudha/commit/17142e2) (#707), before v0.3.0 | #928 |
| Quit waits for writes under way | `inboxtui.go:1286` to `:1304` | before v0.3.0 | #933 C1 |
| Key help and finder | `helpView` (`inboxtui.go:2580`); `/` and ctrl+k | [2929997](https://github.com/rahulnakmol/agentic-ayudha/commit/2929997) (#587), before v0.3.0 | #934 |
| OSC 52 copy of the device code | `inboxlogin.go` | before v0.3.0 | #929 |
| Dismiss and waiver as typed commands | `aayu inspect --dismiss --reason`, `aayu decide --waiver` | before v0.3.0 | #936 |

"Before v0.3.0" means the code exists on dev and no commit in `_data/merge-refs.tsv` that touches
`internal/cli/inbox*.go` refs these Stories (`git log 53e328d..origin/dev -- 'internal/cli/inbox*'`).

Since v0.3.0, other Features changed the same inbox files: Feature #1170 (#1172, #1173, #1175, #1176, #1177, and #1178
as [bc78349](https://github.com/rahulnakmol/agentic-ayudha/commit/bc78349) on 2026-11-07; for example
[c858a80](https://github.com/rahulnakmol/agentic-ayudha/commit/c858a80),
[eb51d1b](https://github.com/rahulnakmol/agentic-ayudha/commit/eb51d1b),
[2715ef2](https://github.com/rahulnakmol/agentic-ayudha/commit/2715ef2),
[188bbb0](https://github.com/rahulnakmol/agentic-ayudha/commit/188bbb0),
[66397d3](https://github.com/rahulnakmol/agentic-ayudha/commit/66397d3)) and Feature #889 (#1013, #1015; for example
[3d42ff1](https://github.com/rahulnakmol/agentic-ayudha/commit/3d42ff1),
[2c6c6b0](https://github.com/rahulnakmol/agentic-ayudha/commit/2c6c6b0)).

## In review

No open PR refs a sub-issue of #895 (`_data/open-prs.tsv`). Four PRs from other work touch the same files. On
2026-11-07, one merged and three closed unmerged when the mission of #1170 ended (`_data/closed-prs.tsv`):

| PR | State on 2026-11-07 | Refs | Inbox files it touches |
|---|---|---|---|
| [#1265](https://github.com/rahulnakmol/agentic-ayudha/issues/1265) | closed unmerged on 2026-11-07 when the mission ended; branch `story/1253-aayu-inbox-asks-up-to-five-decisions-to` at 306d1a8 kept | Bug #1253 | `inbox.go`, `inboxapprove.go`, `inboxbaseline.go`, `inboxtui.go`, `inboxwhy.go`, `inboxwork.go` |
| [#1270](https://github.com/rahulnakmol/agentic-ayudha/issues/1270) | draft; closed unmerged on 2026-11-07 when the mission ended; branch `story/1254-two-person-windows-a-day-outside-them-no` at 7364899 kept | #1254 | `inbox.go`, `inboxday.go`, `inboxevening.go`, `ship.go`, `defaults.toml` |
| [#1238](https://github.com/rahulnakmol/agentic-ayudha/issues/1238) | merged on proof on 2026-11-07 as [bc78349](https://github.com/rahulnakmol/agentic-ayudha/commit/bc78349) | Story #1178 | `inbox.go`, `inboxevening.go`, new `inboxovernight.go` (`git show --stat bc78349`) |
| [#1282](https://github.com/rahulnakmol/agentic-ayudha/issues/1282) | draft; closed unmerged on 2026-11-07 when the mission ended; branch `feat/1281-auto-merge-on-proof` at 6f2aae7 kept | #1281, #1255 | `inboxaudit.go`, `defaults.toml` |

(File lists from `gh pr view <n> --json files`, filtered to inbox, config, and ship paths.)

## Pending

All 15 sub-issues are pending. Bug #1122 is XS and stands alone. The critical path in the Feature body is "#924 → #927 → #928; the wrap story runs beside it" (issue #895, Scope).

Estimate from the source: "14 stories (3 L, 8 M, 3 S) … about 10.6 h of mission time at this repository's median
open-to-merge hours with 5 agents (Decision #974), under `[mission] warn_hours = 24`" (issue #895, Scope). Bug #1122
(XS) was filed later and is not in that count.

### Work not sliced into Stories, or outside this Feature

| Gap | Why it matters | Source |
|---|---|---|
| Intent, spec, and UX design baselines for #895 | Stories are made Ready only after these are approved | issue #895, Scope; no `specs/895/` on the ledger |
| Hanu's review of the web and terminal prototype states (D4) | Entry condition for Ready | issue #895, Scope |
| Approve to build of #895 (#877) | Entry condition for Ready | issue #895, Scope |
| Bug [#816](https://github.com/rahulnakmol/agentic-ayudha/issues/816): a recorded dismissal does not change a cached verdict | #936 depends on it; it is open, `priority:P3`, with no milestone | #936 Links; `gh issue view 816` |
| Review guidance and Behaviours sections | Each Story waits for Features #887 and #891 to add the form fields, then is made Ready again | each Story's Assumptions |
| The web face of every touchpoint | Wave 3 (#976) | #924, #925, #931, #933 Assumptions |
| #873's review-budget Need kind | #926 is its card; #873 has no sub-issues (GitHub API `sub_issues_summary.total = 0`) | #926 Assumptions |

No source gives an estimate for these gaps.
