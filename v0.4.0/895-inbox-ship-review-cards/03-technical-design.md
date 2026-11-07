# Technical design: Feature #895

## Source note

There is no technical design baseline (`origin/ledger` has no `specs/895/`). The planned design below comes from the
Feature's "Expected paths" and the Stories' Assumptions. The "Today on dev" parts cite code at origin/dev
[bc78349](https://github.com/rahulnakmol/agentic-ayudha/commit/bc78349) (884d81b plus #1238, which moved lines in
`inbox.go`; line numbers checked 2026-11-07). No commit since v0.3.0
([53e328d](https://github.com/rahulnakmol/agentic-ayudha/commit/53e328d)) refs a sub-issue of #895
(`_data/merge-refs.tsv`), so all inbox code that wave 1 builds on was there at v0.3.0 or came from other Features.

## Architecture today

`aayu inbox` is one Go process (`internal/cli/inbox.go`, `runInbox`). The verb is registered as `plumb("inbox", …)`
with one flag, `--list` (`internal/verbs/registry.go:1361`).

- Read side: `inboxSource` (`internal/cli/inbox.go`, `newInboxSource`, `gather`, `refresh`, `items`) reads issues,
  PRs, comments, and ledger records from GitHub and builds `InboxItem` values. Each item has a `Kind`, `Why`,
  `Options` (`InboxAction`), `Counsel`, `Evidence`, and `Head` (`InboxItem` struct, `internal/cli/inbox.go:258`).
- Kinds: `inboxKinds` maps each kind to its icon, word, seat, line, and explainer; `inboxOrder` fixes the order
  (`internal/cli/inbox.go:69`, `:94`). `inboxOrder` puts the morning's decisions first, from Story #1175
  ([2715ef2](https://github.com/rahulnakmol/agentic-ayudha/commit/2715ef2)).
- Terminal face: a Bubble Tea model, `inboxModel` (`internal/cli/inboxtui.go`), with the key switch near
  `inboxtui.go:1367`, `View` (`:1760`), `cardView` (`:2205`), and `helpView` (`:2580`). Results and refusals go to one
  status field, `m.status` (for example `inboxtui.go:1069`, `:1207`).
- Writes: `inboxAct` and `shipReady` (`internal/cli/inbox.go:1978`, `:2062`) call the owning verb's handler (ship,
  decide, inspect). `inboxWork` and `inboxWatch` record the stages of a write from its HTTP calls
  (`internal/cli/inboxwork.go`).
- Stale check: `reread` reads an item again before a write; `inboxMaterial` and `errInboxChanged` refuse when what
  the choice acts on changed (`internal/cli/inbox.go:1179`, `:1195`, `:1219`). `changedLine` and `changedWords` put it
  in words (`internal/cli/inboxwhy.go:370`, `:378`).
- Sign-in: `presenceCheck` (`internal/cli/presence.go:136`) runs GitHub's device flow; the inbox shows the code in
  its own frame and copies it with OSC 52 (`internal/cli/inboxlogin.go`, header comment and `inboxSignin`).
- Cache: one JSON file per repository at `$XDG_STATE_HOME/aayudha/inbox/<owner>_<repo>.json`, else
  `~/.local/state/…` (`inboxCachePath`). `saveInboxCache` writes `<path>.tmp` and renames it; `inboxCacheMu` is an
  in-process mutex (`internal/cli/inbox.go:2106` to `:2181`).

## Layers touched

From `LAYERS.md` and the Feature's Expected paths (issue #895):

| Layer | Owner seat | Paths in this Feature |
|---|---|---|
| L02 Interaction / API boundary | Code Architect | `internal/cli/inbox*.go`, `internal/cli/presence.go`, `internal/cli/inspect.go`, `internal/cli/ship.go`, `internal/cli/look.go` |
| L03 Application / workflow | Engineering Manager | `internal/ui/**` (#980), `internal/inspect/inspect.go` (#926, #936), `internal/verbs/registry.go` (new flags) |
| L07 Operational data / state | Production Engineer | `internal/config/defaults.toml` and the config structs (#925, #928, #931, #935) |
| L09 Platform / operations | DevOps & DevEx Tinkerer | `.github/CODEOWNERS` (#935) |
| Docs | — | `docs/guide/how-trust.md`, a tmux and herdr section in `docs/guide/` (#929 C8) |

`[autonomy] never = ["L04", "L07", "L09"]` (`internal/config/defaults.toml`). A plan that touches a never layer
needs a plan-time review (`aayu plan --approve`, from `aayu plan --help`). So #925, #928, #931, and #935 need it
(inferred from their config and CODEOWNERS paths).

## Planned design, by Story and Bug (planned, not built)

| Issue | Design from the issue body | Code it starts from on dev |
|---|---|---|
| #924 | Move every outcome from `m.status` to a block on the card; wait for a running refresh before a write; keep the cursor card in place across a reorder; read a cut-off issue again. "Ship's own checks stay: the base tip and policy are judged at merge time (`internal/cli/ship.go`)." | `reread`, `inboxMaterial`, `errInboxChanged`, `changedLine`, `inboxWork`, `explainFailure` (`inboxwhy.go:182`) |
| #925 | A loading state until the first read ends; a read state in the header; banners for slow and rate-limited GitHub; a cache safe for two processes; read times from `[inbox]`. "The JSON cache stays (design study Q4 A); no SQLite." | `inboxRefreshEvery = 45 * time.Second` and `inboxReadTimeout = 3 * time.Minute` are constants (`inboxtui.go:48`, `:54`); `skeleton` (`inboxtui.go:2026`) |
| #926 | A `human-review` item also for a head where rung B failed and the reviewer app is silent for 30 minutes, and for a PR at its review budget; one "Approve and ship" choice. It "is the card for #873's Need kind review-budget". | `InboxHumanReview` shows only when the gate records the human rung (`inbox.go:860`); approve is choice 2 and runs `aayu inspect <n> --approve` (`inbox.go:864`) |
| #927 | A bypass card for a non-Story protected-path PR; one-line reason; the comment, then a REST squash merge at the head's SHA; ledger record; branch deleted. "Ruleset `protected-paths-codeowner-review` lists the admin role as a bypass actor." | `shipReady` already ships a Story PR's Ready card under a solo-mode bypass, with `shipBypassNote` and `--solo-bypass` (`inbox.go:2062`; `ship.go:985`; [c1dff14](https://github.com/rahulnakmol/agentic-ayudha/commit/c1dff14), #848, before v0.3.0) |
| #928 | Select Ship cards, a hold read from `[inbox] batch_hold_seconds`, one presence check, then ship in turn; a test that fails on a duration literal in an inbox file. | `toggle` and `selectable` select housekeeping only (`inboxtui.go:1568`); `inboxParallel = 2` writes run at once (`inboxtui.go:392`) |
| #929 | OSC 8 links with an `id=`; OSC 52 copy on c; SSH detection (`SSH_CONNECTION`, `SSH_TTY`); `$BROWSER`; tmux and herdr pass-through; docs. | OSC 52 copy exists only for the device code (`inboxlogin.go`, `tmuxClip`); `openURL` runs `open`, `rundll32`, or `xdg-open` and ignores `$BROWSER` and SSH (`inbox.go:2223`) |
| #930 | Card order: why you, evidence row, counsel panel, brief, choices; a fact needs a source and time; a recommendation only from its source; Atri's counsel inline where none is given. | `InboxItem.Why`, `Counsel`, `CounselFrom`, `Evidence`; `inboxCounsel` (`inbox.go:1030`); `cardView` |
| #931 | Terminal title and herdr pane state when unfocused; `[ui] bell`; a "Since …" line; `aayu inbox --brief`. | None found: no title or herdr code in `inbox*.go` (rg for `herdr`, `HERDR`, title escape) |
| #932 | Header and micro footer. "The read state … is the reads Bug's." | The header is drawn in `View` (`inboxtui.go:1760`) |
| #933 | Quit waits for writes; a session summary; `aayu inbox --today` from the ledger; `aayu inbox <n>` deep link. | q with writes under way asks and waits (`inboxtui.go:1286` to `:1304`); no `--today` or positional argument (`aayu inbox --help`) |
| #934 | A grouped overlay; one keymap source that generates every face's keys and the docs, with a test; hide choices the reader may not take; show another session's write. | `helpView` lists keys by hand (`inboxtui.go:2580`); `/` and ctrl+k open a finder (`inboxtui.go:1402`) |
| #935 | `[codeowners]` in config writes the aayudha block of `.github/CODEOWNERS`, with a drift check; an owner card that posts a real APPROVED review. "While solo mode holds, the bypass card shows instead; never both for one PR." | None found: no `[codeowners]` key in `internal/config/defaults.toml` |
| #936 | Findings at the head on the Ship card; x dismisses with one line; a WAIVER card for a floor-waiver Decision. "Bug #816 comes first: a recorded dismissal must change the cached verdict." | `aayu inspect --dismiss <id> --reason` and `aayu decide --waiver` exist as commands (`aayu inspect --help`, `aayu decide --help`); [#816](https://github.com/rahulnakmol/agentic-ayudha/issues/816) is open |
| #980 | Wrap in `ui.Printer`: break at the last space before min(width, 100), hanging indent 6 after "aayu: ", else own indent + 2; no wrap without a terminal. "The inbox panes keep their own wrap." | `ui.Printer` has `Width` (`internal/ui/ui.go:196`, set in `internal/cli/look.go:59`); `Println` does not wrap (`ui.go:258`) |
| #1122 | Reuse `scopeTrusted`'s 404 rule in `readyItem` and `postedCounsel`. | `canWrite` returns the permission error (`inbox.go:622`); `readyItem` (`inboxready.go:221`) and `postedCounsel` (`inbox.go:977`) pass it on; `scopeTrusted` maps `github.NotFound` to "not trusted" (`scope.go:1247`) |

## Data and ledger records

| Record | Use | Source |
|---|---|---|
| `inspect/approved.jsonl` on the ledger | Each head approval from Approve and ship | #926 C5 |
| A ship record with the bypass comment URL | Each bypass merge | #927 C4 |
| The actions log | Review spend for the budget card | #926 C6 |
| The inbox cache (JSON, per user, outside the repository) | Done marks shared across sessions | `inboxCache` (`inbox.go:2093`); #925 C10, #933 C4 |
| `.github/CODEOWNERS` aayudha block | Written from `[codeowners]` | #935 C1 |

## Config keys

| Key | Default | Status | Source |
|---|---|---|---|
| `[inbox] refresh_seconds` | 45 | planned | #925 C11 |
| `[inbox] read_timeout_seconds` | 180 | planned | #925 C11 |
| `[inbox] read_retry_seconds` | 30 | planned | #925 C11 |
| `[inbox] batch_hold_seconds` | 5 | planned | #928 C6 |
| `[ui] bell` | false (inferred from "no bell unless `[ui] bell = true`") | planned | #931 C1 |
| `[codeowners]` | per-path owners | planned | #935 C1 |
| `[budget] per_pr_inspect_max` | 15.0 in this repository | exists | `.aayudha/config.toml` |
| `[autonomy] approver`, `always_person` | `rahulnakmol`, the narrow set | exists | `.aayudha/config.toml` |

There is no `[inbox]` section in `internal/config/defaults.toml` today (checked). A new key in a rendered section
needs a row in `internal/config/gen` (AGENTS.md, "Generated files").

## Contracts

- `aayu inbox --list --json` prints `InboxItem` objects (`kind`, `number`, `title`, `url`, `seat`, `why`, `options`,
  `head`, and more; struct tags in `internal/cli/inbox.go:258`). #926 C2 adds why-fields for kind `human-review`; #927
  adds a bypass kind (its name is not given in the body).
- New CLI surface: `aayu inbox --brief` (#931 C4), `aayu inbox --today` (#933 C3), `aayu inbox <n>` (#933 C4). Each
  needs a registry entry in `internal/verbs/registry.go`, which regenerates `docs/README.md`, `skills/*/SKILL.md`
  frontmatter, and `internal/mcp/schemas/*.json` (AGENTS.md, "Generated files").

## Key decisions

| Decision | State | Effect here |
|---|---|---|
| [#903](https://github.com/rahulnakmol/agentic-ayudha/issues/903) Approve to build agrees the whole Feature once | closed, `status:done` | A story added or changed after Approve to build needs the founder's acknowledgment (each Story's Assumptions) |
| [#905](https://github.com/rahulnakmol/agentic-ayudha/issues/905) Review speed | closed, `status:cancelled`; superseded by [#910](https://github.com/rahulnakmol/agentic-ayudha/issues/910) (comment on #905) | #926 cites it through #897; the text should be checked against #910 before Ready |
| [#974](https://github.com/rahulnakmol/agentic-ayudha/issues/974) Mission rules and the 48-hour limit | closed, `status:done` | The estimate basis in issue #895 |
| [#593](https://github.com/rahulnakmol/agentic-ayudha/issues/593) Merge on proof; the narrow set | closed, `status:done` | #927 and #935 change who may bypass or approve, so a person ships them |
| D4 (Hanu's prototype review) | not linked; no issue number in the body | Entry condition for Ready |
