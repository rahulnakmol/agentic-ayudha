# Aayudha build plan

Repo: `rahulnakmol/agentic-ayudha` · binary `aayudha` (alias `aayu`) · BSD-3-Clause · public
Plan version: **0.3** (post three-judge audit + second-pass consistency review) · 2026-10-11 · owner founder A
v0.1 was audited by three independent judges (architecture/feasibility, agentic SDLC/human-in-the-loop, adversarial security/ops/cost); v0.2 accepted all findings (Appendix A). v0.2 was then re-read by a fourth reviewer for internal consistency and residual gaps; v0.3 applies those (Appendix B).

---

## Changes to previously settled decisions (veto points)

The audit forced these changes. Each is applied below; say so if you want any reverted.

| # | Settled | Now | Why |
|---|---|---|---|
| V1 | Agents use the user's `gh` token or PAT for everything | Humans use their own token; **agent lanes and CI use a bot identity** (GitHub App installed by `init`, or a dedicated machine-user PAT as fallback) | A human pass, a decision acceptance, and `tell publish` are unenforceable if agent and human write with the same identity |
| V2 | `.claude/skills` is a symlink to `.agents/skills` | **No committed symlinks.** `install` writes real copies and records them in `.aayudha/lock.toml`; `doctor --fix` re-syncs. `CLAUDE.md` is a stub that includes `@AGENTS.md` | Windows collaborators with `core.symlinks=false` get text files; trees differ per OS |
| V3 | Proof media in Git LFS on the `ledger` branch | LFS stays the default **for private repos**. On public repos `init` refuses media in LFS and requires `[proof] media = "<private bucket>"`; git holds `pack.json` + hashes. Proof captures use seeded/synthetic data only, with a redaction pass | Recordings and transcripts can hold PII; LFS objects cannot be purged from GitHub |
| V4 | Autonomy streak stored on `ledger` | Streak is **computed by a protected workflow from merged-PR history**; no agent-writable counter | The agent that the streak constrains could write it |
| V5 | Figma remote MCP primary, REST fallback | **REST + PAT primary**, remote MCP opt-in | MCP client-side OAuth is experimental in the Go SDK (verified by judge 1) |
| V6 | `[lfs] server` may be S3/R2 | 0.1 supports GitHub LFS only. 0.2 adds `aayu lfs-agent`, a git-lfs custom transfer agent inside the binary that talks to S3/R2 | S3/R2 do not speak the LFS batch API |
| V7 | 30 verbs (miscounted; the settled list is 31) | **32 verbs**: `evolve` added (quarterly skill evolution with eval-first gating). Plumbing subcommands (`config`, `auth`, `actions`, `models`, `mcp`, `self-update`, `upgrade`, `version`, `halt`, `resume`, `gc`, `uninit`, `hook`, `seat`, `lfs-agent`) are not verbs | Self-evolution had no verb; kill switch, GC, upgrade, and seat onboarding had no home |
| V8 | Decisions close in < 24 h | 24 h: escalate to founder B. 48 h: recommended option applied **provisionally** only if the Decision author marked it reversible; labelled `decision:provisional`, surfaced in `status` | SLA lapse had no consequence |

---

## 0. Ground rules

1. Promotion ladder dev → rolling → main from the first commit. From PR11 onward (once `init` and `ready` exist) every remaining WP is tracked as a Story in `agentic-ayudha` with a Completion Goal; PRs 1–10 are tracked in this file. The plan runs on its own process.
1a. Forward references: a WP may name an artifact built in a later milestone (notify, bot identity, leases, seats). Until that milestone the feature is "n/a" and the WP's acceptance check tests only what exists; `doctor` reports such features as "not yet available", never as failures.
2. One static Go binary, `CGO_ENABLED=0`. `git` and `git-lfs` binaries are **required** for ledger operations (go-git has no LFS or linked-worktree support); everything else optional and reported by `doctor`.
3. Embedded assets come from a root-level `embed.go` package beside `skills/`, `work-items/`, `hosts/`, `seats/`; CI diff-checks that embeds match sources.
4. **Verb registry is the single source**: `internal/verbs/registry.go` declares name, seat, lane, layers, typed inputs, outputs, gates, surfaces (skill / CLI / MCP / plumbing), model-calling mode. Cobra commands, MCP schemas, SKILL.md frontmatter, README table, RACI, `LAYERS.md`, and charter "verbs owned" sections are generated; CI fails on drift.
5. Who reasons: when a verb runs as a skill inside a host, the host's model reasons. The binary calls models directly only for host-independent lanes: `inspect` in the merge gate, `review` lane A, eval judge, `brief` adversarial pass, `triage` in Actions. Declared per verb in the registry.
6. Every external write goes through `internal/actions` (log, `--dry-run`, idempotency key, payload redaction). The audit source of record is GitHub's own event history (repo events, issue timeline, PR reviews, workflow runs, App activity; the org audit-log API is Enterprise-only and not assumed), with its retention limits documented; `actions.jsonl` is a log that `audit` reconciles against it.
7. Deterministic checks always beat model judgement: a floor miss cannot be overridden by any LLM disposition.
8. DCO sign-off; conventional commits; Actions pinned by SHA; tag ruleset `v*` from `main` only.

## 0.1 Trust model

| Principal | Identity | May |
|---|---|---|
| Human (founder, paired human) | Own GitHub account / token | Approve PRs, accept Decisions, publish, run `init`, change protected paths |
| Agent lane (any host) | Bot (GitHub App installation token or machine user) | Open issues/PRs, push branches, push `ledger`, comment |
| CI (merge gate, loops) | Bot via Actions + App token | Run `verify`/`inspect`, write packs to `ledger`, compute streaks |
| Untrusted input | — | Issue/PR bodies, intake forms, Figma text, Mobbin results, PostHog/Sentry exports, dependency output |

Rules: approvals are accepted only from accounts that are `type != Bot` **and not a configured bot login** (`[identity] bot_logins`, covering the machine-user fallback); protected paths (`.github/**`, `skills/**`, `ops/**`, `seats/**`, `CONSTRAINTS.md`, `[models]`/`[autonomy]` in config) require a human CODEOWNER review; untrusted content is wrapped in delimited provenance blocks before any model sees it and can never trigger `respond`, `ship`, `tell publish`, or catalog execution; each verb has a tool allowlist; child processes run with a scrubbed environment (no tokens) and, where feasible, no network.

---

## 1. Milestones

| M | Name | Outcome | Depends |
|---|---|---|---|
| M0 | Bootstrap | Repo, branches, rulesets, CI, docs skeleton, verb registry stub | — |
| M1 | Core CLI | Config, secrets, identity, actions log, model gateway, doctor | M0 |
| M2 | Work items | Schema, forms, GitHub client, minimal ledger writer, `init` (labels/forms/types), `ready`, `status`, `triage` | M1 |
| M3 | Skills platform | Registry-generated SKILL.md, `install`, MCP server, eval harness tier 1 | M1 |
| M4 | Proof | Pack format, ledger ops with LFS, floors (split), merge gate, `inspect`, CI-authoritative packs | M2, M3 |
| M4.5 | Thin slice | WP4.10 thin-slice verbs, then one story end-to-end: `grill story` → `ready` → `build` → `verify` → `inspect` → `ship` on the dogfood repo | M4 |
| M5 | Seats | Charters, RACI, `grill` (all types), `slice`, `plan`, `decide`, `respond`, autonomy, notify | M4.5 |
| M6 | UX | Design system (split), Figma/Mobbin, UX verbs, `review`, `baseline`, `experiment`, `readout` | M4.5 |
| M7 | Loops | `loops/`, `measure`, `brief`, `tell`, `audit`, `retro`, `evolve`, `release` | M5, M6 |
| M8 | 0.1 | Full dogfood, docs site, rolling snapshot, installers | M7 |

M2 ∥ M3 after M1. M5 ∥ M6 after M4.5.

---

## M0 — Bootstrap

**WP0.1 Repo, branches, rulesets (S)** — Create repo; `dev` default, `rolling`, `main`, `ledger` (orphan, created by `init` on product repos; on aayudha itself for its own proof). Rulesets: PR + status checks on `dev`; linear history on `rolling`/`main`; `ledger` pushable only by the bot identity and CI (n/a until WP1.4: until then `ledger` is pushable by the founder's identity and the ruleset is tightened by `init --identity`); tag ruleset `v*` from `main`; CODEOWNERS for protected paths requiring human review. **Solo-mode policy**: with one human in the org, CODEOWNER review of that human's own PR is impossible, so protected-path PRs authored by the founder merge with the founder's approving review on a *different* commit than the last push (GitHub "dismiss stale approvals" on), and every such self-approval is listed by `audit` as a solo-mode bypass; the rule switches to true second-human review the moment a second human seat is added (WP5.1). Files: LICENSE, README (verbs table generated), CODEOWNERS, SECURITY.md (incl. tool-incident plan: malicious skill or release → `aayu halt` + yanked release + advisory), CONTRIBUTING (DCO), THIRD_PARTY_NOTICES.md, .editorconfig, .gitattributes.
Accept: `gh api` shows rulesets and four branches; a PR touching `skills/` without an approving review is blocked (tested on a scratch PR); the solo-mode bypass of that PR appears in a hand-run reconciliation (automated by `audit` in WP7.6).

**WP0.2 Go module and layout (XS)** — Module `github.com/rahulnakmol/agentic-ayudha`, Go 1.25. `cmd/aayudha`, `embed.go`, `internal/{verbs,cli,config,secrets,identity,actions,models,github,ledger,workitems,skills,hosts,mcp,proof,verify,loops,notify,telemetry}`, `skills/`, `work-items/`, `hosts/`, `seats/`, `ops/`, `loops/`, `packages/design-system/`, `evals/`, `docs/`, `examples/`.
Accept: `go build ./... && go vet ./...` on stubs.

**WP0.3 Verb registry stub (S)** — `internal/verbs/registry.go` with all 32 verbs and plumbing commands, fields per rule 4, plus `go generate` targets: README table, `LAYERS.md`, RACI, SKILL.md frontmatter, MCP schemas. Verbs: `grill slice plan decide triage build verify inspect ship release proof ready research wireframe mockup prototype review polish baseline experiment readout respond status audit retro measure evolve brief tell init install doctor`.
Accept: generator runs; CI drift check fails when a generated file is edited by hand.

**WP0.4 CI (M)** — `ci.yml`: golangci-lint, test, build matrix linux/darwin/windows × amd64/arm64, DCO, conventional commits, embed/generate drift, skill lint, path filters, license scan (go-licenses + npm), gitleaks, semgrep, osv-scanner, binary size budget (fail > 60 MB). `release.yml`: goreleaser on `v*`, checksums, SBOM (syft), cosign keyless signing, SLSA provenance (`attest-build-provenance`), brew tap from a separate job with environment-scoped secret, Scoop manifest, winget manifest PR. `install.sh` verifies cosign signature against the pinned workflow identity, not only checksum. All actions SHA-pinned.
Accept: green on `dev`; goreleaser `--snapshot` job produces signed artifacts; `install.sh` refuses a tampered archive (test with a flipped byte).

**WP0.5 Governance docs v0 (S)** — `ETHOS.md`, `GLOSSARY.md`, `CONSTRAINTS.md` (tables **generated** from embedded default config: floors, autonomy, budgets, retention), `LAYERS.md` (generated: layer → owner seat → `path → layer` globs; seat column reads "n/a until WP5.1" from the registry stub), `AGENTS.md` for aayudha, `docs/threat-model.md` (= §0.1 expanded).
Accept: markdown lint + link check (lychee) pass in CI; generated tables are byte-identical to a second `go generate` run.

---

## M1 — Core CLI

**WP1.1 CLI frame (S)** — cobra root; help prints `aayu`; global flags `--dry-run --json --repo --host --yes` (human-only, refused when `AAYU_AGENT=1`) `--verbose`; stable exit-code taxonomy (0 ok, 2 usage, 3 floor miss, 4 blocked/needs human, 5 external error, 6 budget); all output through an `io.Writer` abstraction (never raw stdout, for MCP mode). `aayu version`. `aayu self-update`: detects brew/Scoop/winget installs and prints their upgrade command instead; verifies cosign signature + provenance; refuses downgrades; refuses to run when `AAYU_AGENT=1`.
Accept: exit-code table test; self-update rejects an unsigned release in a fixture.

**WP1.2 Config (M)** — koanf layering `~/.config/aayudha/config.toml` → `.aayudha/config.toml` → `.aayudha/config.local.toml` → `AAYU_*` → flags (`config.json` accepted; TOML wins). Schema **generated from structs** (`invopop/jsonschema`). Sections: `[seats.<name>] mode human page`, `[models.<lane>.<host>]`, `[lfs]`, `[proof] media retention`, `[beneficiaries]`, `[autonomy] streak never approver budget`, `[decisions] authority sla provisional_after`, `[notify] channel target`, `[budget] per_story_usd per_day_usd per_pr_inspect_max ci_minutes_month`, `[mcp] expose_writes`, `[integrations.<name>] secret scope`, `[verify] dast platforms upstream_checks`, `[time] tz business_hours` (SLA clocks in `[decisions]` and paging windows in `[notify]` count business hours in this zone; default `Europe/London`, Mon–Fri 09:00–18:00, DST-aware via the IANA zone; Sev0/Sev1 incidents ignore business hours), `[fleet] repos` (user-level only: list of product repos for `status --all` / `doctor --all`). `config show --origin`, `validate`, `init`. Schema version + migration hook (`config.version`).
Accept: precedence table test with distinct values per layer; unknown key rejected; migration from v0 fixture; a 24 h SLA started Friday 17:00 London time expires the following Monday 16:00 London time, and one started the Friday before the October clock change still expires at 16:00 local on Monday (table test).

**WP1.3 Secrets (M)** — Backend order: OS keychain (`go-keyring`) → `AAYU_SECRET_<NAME>` env → age-encrypted file under XDG (0600). `doctor` reports backend. Secret names referenced from config; values never in files. `[integrations.<name>] scope = "read"|"write"`; `doctor` verifies read-only scope where the provider API allows, refuses write-capable keys for read verbs.
Accept: automated canary test: after every secret-handling command, scan home, repo, temp for the canary; headless-Linux CI job exercises the env and file backends.

**WP1.4 Identity (M)** — `internal/identity`: human token resolution `AAYU_GITHUB_TOKEN` → `GITHUB_TOKEN` (only with `--ci`) → `gh auth token` → keychain → prompt. Bot identity: GitHub App (created by `init --identity`, private key in secrets store; installation token minted per run) or machine-user PAT fallback (`[identity] bot_logins` lists machine users so approvals from them are rejected). **The App private key never lives on a laptop**: agent lanes on a developer machine obtain a short-lived installation token (≤ 1 h) from a **token broker** — a `workflow_dispatch` job in the product repo (`aayudha-token.yml`, installed by `init --gates`) that mints the token with the App key held as a repo secret and returns it encrypted to the caller's ephemeral age key; the human's own token authorises the dispatch, so the broker call is itself attributed and logged. Agent lanes in CI use the App token directly. Verbs declare `identity: human|bot` in the registry; `ship`, `decide accept`, `tell publish`, `respond` (human actions) verify approver `type != Bot` and not in `bot_logins`. **Day-one path**: a repo without a bot identity runs every verb under the human identity, `ready` snapshots are posted under that identity and tagged `identity:human-fallback`, and `doctor` reports "bot identity: not configured" as a warning, not a failure; `audit` lists human-fallback writes. `doctor` warns on over-scoped human tokens (`workflow`, `admin:*`, `delete_repo`); `init` is the only admin-scope verb and requires a TTY confirmation.
Accept: a bot-authored PR review is rejected as a human pass in a fixture; a `bot_logins` review likewise; scope warnings golden-tested; broker round-trip test against a scratch repo behind `AAYU_LIVE=1` returns a token that expires ≤ 1 h and the App private key is never present in the client's environment or filesystem (canary scan).

**WP1.5 Actions log (S)** — `actions.Do(ctx, Action{Kind, Target, Seat, Host, Identity, IdempotencyKey, Payload})`; JSONL hash-chained; payload redaction list; `tokens_in/out`, `cost_usd`, `wall_time`, `attempts` fields; local file `telemetry/actions.jsonl`; segments appended to `ledger/actions/<date>/<host>-<run>.jsonl` at each checkpoint and on `ship` (n/a until WP2.4; until then the segment writer is a local-directory sink). Policy hooks: an action may carry a `capability` (e.g. `labels-only`) and `actions.Do` rejects any GitHub write outside that capability regardless of what the token could do. `aayu actions tail|grep|verify-chain`.
Accept: dry-run `init` records planned actions and performs none (fake client); chain verification detects a tampered line; a `labels-only` action attempting an issue-body edit is refused before any HTTP call (fake client records zero requests).

**WP1.6 Model gateway (M)** — `internal/models`: provider clients Anthropic, OpenAI, Google, xAI; lane resolution per host from `[models]`; `reviewer.family != builder.family` enforced per story; cost/token accounting into actions; `[budget]` circuit breaker (pauses agent lanes, pages via `[notify]`, exit 6); fake provider for tests; `aayu models show`; structured-output helper with JSON Schema validation. **Also home of `internal/notify`**: the `[notify]` sender (Slack webhook, email via SMTP, GitHub mention comment) with a fake sink for tests, business-hours gating from `[time]`, and a per-day dedupe key so one breach pages once. Later WPs (2.8, 5.5, 5.6, 5.7) only call it.
Accept: lane resolution table test per host; budget breaker trips at boundary and the fake sink receives exactly one page; builder=reviewer family rejected; the Cursor host with a GPT lane configured resolves to the Grok fallback and logs the substitution.

**WP1.7 Doctor (L)** — Checks: binary freshness; git ≥ 2.40 and git-lfs **required**; `gh` optional; token source and scopes; bot identity present; config valid; host detection; `.agents/skills` and `.claude/skills` copies in sync with `lock.toml`; MCP registrations; LFS quota (warn 80 %); issue-type support; integrations reachable and scoped; optional tools (node, playwright, axe, semgrep, gitleaks, osv-scanner, zap, k6, lighthouse, guidepup, impeccable); worktree leaks; stale leases; **existing product CI** (detects `.github/workflows/*`, branch names other than dev/rolling/main, rulesets already present, check names that would collide) and reports what `init` will add alongside rather than replace. Checks for features from later milestones (skills copies, MCP, leases, bot identity) report `not yet available` while their WP is unshipped, per rule 1a. `--fix` re-syncs copies and adapters after showing a diff. `--json`; nonzero on floor-blocking failures. `doctor --all` iterates `[fleet] repos` from the user-level config and prints one row per repo.
Accept: golden tests per check in pass/fail; `--fix` is a no-op on a healthy repo; a repo with a pre-existing `ci.yml` and `master` branch yields a coexistence report, exit 0.

**WP1.8 Telemetry (S)** — OTel traces of verb runs to file by default; OTLP opt-in. No remote telemetry without opt-in.
Accept: trace file produced by `doctor`; opt-in covered by test.

**WP1.9 `upgrade` (S)** — `aayu upgrade` (distinct from `self-update`, which replaces the binary): migrates `.aayudha/config.toml`, `lock.toml`, `.aayudha/state.json`, and installed skill copies from their recorded schema versions to the running binary's versions, showing a diff first. **Grandfathering**: Ready snapshots and proof packs already on `ledger` are never rewritten; the parser and `proof verify` keep readers for every schema version shipped since 0.1, and `status` shows the schema version per story. A binary refuses to run write verbs on a repo whose schema is newer than itself (exit 2 with the upgrade hint).
Accept: fixture repo at schema v1 upgrades to v2 with a byte-exact expected diff; a v1 pack still passes `proof verify` on a v2 binary; a v3 repo makes a v2 binary refuse `build`.

---

## M2 — Work items

**WP2.1 Schema (M)** — Ten types; Create/Ready/Complete sections; required fields per stage; allowed parents and links; Delivery Status `backlog discovery planned ready in-progress blocked validation merged released measured done cancelled`; Effort XS–L (XL in enum, blocks Ready); Completion Goal for Story/Bug as **numbered observable criteria** each with `{evidence_kind: screenshot|recording|sr-transcript|api-trace|metric, state, expected}` plus measure, threshold, scenario, environment. Decision carries `reversible: true|false` (required at Decision-ready; V8 provisional application is only possible when true) and a `supersedes` link. Field home decided once: Priority/Effort/Risk/Health/Horizon as labels on personal repos, GitHub issue fields where available; Projects v2 fields are views only, never written as truth. Schema versioned.
Accept: fixtures per type/stage validate; XL fixture fails Ready; a story with an unmapped criterion fails Ready; a Decision without `reversible` fails Decision-ready.

**WP2.2 Forms and labels (S)** — Forms generated from schema; label set; org issue types with label fallback.
Accept: forms pass GitHub form schema; `init --dry-run` lists labels/types.

**WP2.3 GitHub client (L)** — REST (`go-github`) for issues, sub-issues, issue types, fields; GraphQL (`githubv4`) only for Projects v2; ETag cache; primary + secondary rate-limit handling (serialized bulk writes with delay); pagination; call budget per verb reported in `--json`. GHES: explicitly unsupported in 0.1.
Accept: recorded-fixture contract tests; live smoke behind `AAYU_LIVE=1` on a scratch repo the test creates and deletes.

**WP2.4 Minimal ledger writer (S)** — `internal/ledger`: temp worktree, commit, push to `ledger` with fetch-rebase-retry (bounded backoff), per-repo file lock, content-addressed paths, no LFS yet. Used by `ready` snapshots and action segments.
Accept: 5 concurrent writers all land; no lost commits.

**WP2.5 `init` core (M)** — Labels, issue types, forms, `.aayudha/config.toml`, `AGENTS.md`, `ledger` branch, `.aayudha/state.json` with created object IDs; `--resume`; `aayu uninit`. Matching by stored IDs, markers only as hints. Admin-scope confirmation on TTY. **V3 enforcement**: `init` reads repo visibility; on a private repo it writes `.lfsconfig`/`.gitattributes` on `ledger` for media extensions; on a public repo it writes no LFS config, sets `[proof] media = ""` with a `# required: private bucket` comment, and `proof push` refuses media until it is set. A visibility change later is caught by `doctor` (private → public with LFS media present is a floor-blocking failure). **Coexistence**: `init` reads the `doctor` coexistence report (WP1.7); it never edits existing workflows, prefixes every check it registers with `aayudha/`, adds its rulesets alongside existing ones, and, when the default branch is not `dev`, writes `[branches] dev rolling main` mappings into config instead of renaming anything. Order of operations on a fresh repo: identity (`--identity`, or human fallback per WP1.4) → labels/types/forms → ledger → config.
Accept: run twice → zero changes; `--dry-run` no writes; partial run resumed from state; public-repo fixture ends with no `.lfsconfig` on `ledger` and `proof push --media` exits 4; a repo with existing `ci.yml` keeps it byte-identical after `init`.

**WP2.6 `init` extras (M)** — `--projects` (Portfolio/Delivery/Ops views), `--gates` (installs `aayudha-token.yml` broker, `aayudha-ready-guard.yml`, and — n/a until M4/M5/M7 — `aayudha-merge-gate.yml`, `aayudha-streak.yml`, loop workflows; each workflow file ships only once its WP exists, so `--gates` installs the subset the binary knows; workflow secrets: App key + model keys via repo/environment secrets, documented), `--identity` (GitHub App), `--integrations` (bucket, PostHog, Sentry, GrowthBook) each printing cost/scope summary and requiring `y/N`; `--yes` refused for agents.
Accept: each flag idempotent; orphaned external resources listed by `uninit --external`; `--gates` on the WP2.6 binary installs exactly two workflows (golden).

**WP2.7 Parser and `ready` (M)** — Body → typed struct by heading; stage validation; parent type; Effort ≠ XL; Completion Goal criteria complete; Technical Owner resolved (Code Architect twin in solo). `ready <issue>` sets `status:ready`, posts replace-in-place Ready snapshot (body hash + Completion Goal hash), writes `specs/<id>/ready.md` to `ledger` under the bot identity, or under the human identity tagged `identity:human-fallback` when no bot exists (WP1.4 day-one path); Technical Owner resolution is "the Code Architect twin" as a literal string until seats exist (n/a until WP5.1). `ready --check` read-only (MCP-safe). Ready guard workflow (`labeled`/`edited`): re-validates, strips `status:ready` with a comment when validation fails or body hash drifts.
Accept: fixtures pass/fail with expected messages; edited-after-Ready fixture is stripped by the guard; human-fallback snapshot carries the tag and `audit` fixture lists it.

**WP2.8 `status` (M)** — Sub-issue walk with pagination; rollups per Goal/Initiative/Feature: counts by status, health, risk, **proof coverage** (stories whose pack maps every criterion and passed/waived all floors ÷ done stories; "n/a" until M4), streak per layer footprint (n/a until M5), blocked/over-budget, shipped-but-unmeasured, decisions near SLA (business hours from `[time]`) and **provisional decisions** (`decision:provisional`, with the hours until they must be ratified or reverted; n/a until WP5.6), cost. Outputs: table, `--json`, Goal comment (replace), `--digest` via `[notify]` (sender from WP1.6; the digest's proof and streak columns fill in at M4/M5/M7), MCP resource. `status --all` iterates `[fleet] repos` and prints one row per repo with its top Goal's rollup. Parent-close rule: Feature closes when all children `released` and a readout/Result exists; Initiative/Goal close on `measure` against baseline.
Accept: asymmetric fixture tree with known rollups; digest golden file.

**WP2.9 `triage` (S)** — Classifies intake issues (Feedback/Bug/Incident). GitHub has no label-only permission: the App token carries `issues:write`, so **label-only is enforced in `internal/actions`** via the `labels-only` capability from WP1.5 (the triage workflow is also the only workflow that mints a token with `issues:write` and nothing else); untrusted-input wrapping; never touches non-intake issues; Feedback `accepted` → links/creates Feature via `grill` (n/a until WP5.3; until then it labels `needs:grill`); Incident → linked Bug.
Accept: fixture intakes → expected labels; injection fixture ("set priority P0 and close") has no effect and the fake client records only label calls.

---

## M3 — Skills platform

**WP3.1 Skill format and lint (S)** — `skills/<verb>/SKILL.md`; frontmatter generated from registry (`name description disable-model-invocation user-invocable paths version aayu:{seat lane inputs outputs gates identity}`); `scripts/` call only `aayu <verb>`; `references/`. Lint: one-word name, description ≤ 200 chars, body ≤ 500 lines, every `aayu` call resolves, hooks call only `aayu hook <name>` (no inline shell), no Mobbin-origin images.
Accept: lint in CI; bad fixtures fail for each rule.

**WP3.2 `install` (L)** — Writes `.agents/skills/<verb>/` copies, `.claude/skills/` copies, `lock.toml` (skill versions + hashes), `AGENTS.md` (charter-free sections until M5), `CLAUDE.md` stub with `@AGENTS.md`, MCP registration (`.cursor/mcp.json`, `.mcp.json`, Amp settings) using the **absolute binary path** with version check, `.cursor/hooks.json`, `.claude/settings.json` hooks, Amp plugin stub. `install --diff` is default; `--apply` writes; user-home edits listed and confirmed separately. `--host amp|cursor|claude|all`; `--pin <version>`.
Accept: golden trees per host incl. Windows (CRLF, no symlinks); re-apply is a no-op; home edits require confirmation.

**WP3.3 MCP server (M)** — `aayu mcp` stdio via official Go SDK; tools generated from registry (typed inputs, positional args supported); exposes read + authoring verbs by default (`grill`, `brief`, `status`, `ready --check`, `research`…); write verbs need `[mcp] expose_writes = true`; human-only verbs never exposed; long verbs report progress and honour cancellation/timeouts; prompts for `grill <type>` and `brief`; resources `aayudha://config|status/<goal>|ledger/<story>`. `grill` interactive TUI is CLI-only; the MCP path uses the question-set prompt.
Accept: Go test with SDK client lists expected tools, confirms write verbs absent by default, dry-run round-trip through `actions`.

**WP3.4 Eval harness (M)** — Two tiers. Tier 1 deterministic per PR: artifacts exist, schemas validate, produced issue passes `ready`, every `aayu` call succeeds, injection fixtures cause no writes. Tier 2 rubric nightly / on promotion: pinned judge from a different family than the skill model, k = 5 runs, **pass^k ≥ 0.8 and mean rubric score ≥ 3.5/5 to pass; a drop of more than 0.3 mean or any pass^k fall below baseline fails the promotion**, baseline = `evals/results/<main-sha>.json`, `[budget]` cap, skipped on fork PRs. First evals: `grill story`, `slice`, `ready` (the `slice`/`grill` cases are added when WP5.3/5.4 land; at WP3.4 the harness runs on `ready` and a stub skill).
Accept: tier 1 runs on CI and a fixture with a deleted artifact fails it; tier 2 produces a results file on a scheduled run and a fixture results file 0.4 below baseline is reported as failed.

---

## M4 — Proof

**WP4.1 Pack format (S)** — `pack.json` v1: story, **pr_head_sha**, checkpoint, seat, host, model, identity, `completion_goal_hash`, `criteria[] {id, evidence: [artifact refs]}`, artifacts[] {path (always `/`), kind, sha256, size, lfs|bucket}, checks[] {floor, result, value, threshold, tool, version, waiver?}, cost fields, links. Schema versioned.
Accept: schema validates samples; hash mismatch, missing criterion mapping, and `sha ≠ head` all detected by `proof verify`.

**WP4.2 Ledger with LFS and media store (M)** — Extends WP2.4: LFS tracking by extension (private repos); `[proof] media` bucket upload with lifecycle rules (public repos, or by choice); retention TTL from config; `aayu proof push|get|verify|list|purge`; one replace-in-place comment on PR and issue. Agent-pushed packs are **advisory**; CI-written packs (WP4.8) are authoritative.
Accept: live scratch test; public-repo fixture refuses LFS media; purge removes bucket objects and rewrites `pack.json` with tombstones.

**WP4.3 Floors framework + coverage (M)** — `internal/verify` runner: floor interface, scrubbed child env, no-network default, results into pack; diff coverage from lcov, cobertura, and Go coverprofile; threshold from config (default 90 %).
Accept: 89.9 % fails, 90.0 % passes; child process cannot read `AAYU_*` or `GITHUB_TOKEN` (test).

**WP4.4 Floors: SAST, secrets, deps (S)** — semgrep rulesets per language, gitleaks, osv-scanner; zero critical/high.
Accept: planted findings fail; clean fixture passes.

**WP4.5 Floors: web a11y + perf (M)** — Playwright drives every state listed in the Completion Goal; axe (zero serious/critical, WCAG 2.2 AA); keyboard-only pass; reduced-motion; 200 % text scaling; 44×44 targets; Lighthouse CI budgets from config; **seed data**: `verify` runs the app against `[verify] seed` (a fixture command or SQL that the product repo provides; `init` scaffolds `.aayudha/seed/` with a README) and refuses to capture when `AAYU_ENV` resolves to production; **PII redaction pass** (DOM masking + OCR/regex scan) that blocks the pack.
Accept: planted contrast defect fails; planted email in a screenshot blocks; capture against a URL whose config marks it production exits 4 with no artifacts written.

**WP4.6 Floors: DAST + API (S)** — ZAP baseline against the running app when `[verify] dast = true`; k6 thresholds for API/CLI contracts.
Accept: fixture with a reflected-XSS route fails.

**WP4.7 Floors: screen reader (M)** — Guidepup VoiceOver (macOS runner) and NVDA (Windows runner); expected phrases come from the **Ready snapshot**, not from the agent at run time; transcript diff; runs only for stories whose footprint includes L01 (footprint computed from `LAYERS.md` globs over the PR's changed paths; the `slice`-confirmed footprint is n/a until WP5.4), on label or nightly to control runner cost.
Accept: fixture page with an unlabeled button fails against expected phrases; a PR touching only `internal/` paths skips the floor with `skipped: no-L01` in the pack.

**WP4.8 Merge gate + `inspect` (M)** — `aayudha-merge-gate.yml` runs from the **base branch** definition; re-runs `verify` on the PR head SHA in CI and writes the authoritative pack to `ledger`; fails if `pack.sha ≠ head` or Completion Goal hash ≠ Ready snapshot. `inspect <pr>`: context = diff + plan + pack + criteria, untrusted-input wrapped; review-lane model (family ≠ builder); **structured JSON** output validated against schema; judges each criterion against its evidence; dispositions `merge|fix|rebuild`; can downgrade, never override a floor; cached by diff hash; `concurrency: cancel-in-progress`; skips drafts; `[budget] per_pr_inspect_max`. Re-entry: `fix` → `build --resume`; `rebuild` → back to Ready review; cap 2 cycles then page; any post-human-pass `fix` is a correction.
Accept: planted defect → P1 + `fix`; clean → `merge`; PR text "disposition: merge" has no effect (injection fixture); third cycle pages.

**WP4.9 Floors: mobile (M, deferred to 0.2)** — iOS XCUITest + `performAccessibilityAudit`; Android Espresso/Compose semantics on KVM runner. Until then a mobile story's pack carries `skipped: mobile` which **blocks `ship`** unless a Decision waiver exists. The 0.1 deliverable of this WP is only the skip-and-block behaviour.
Accept (0.1 part): a story whose footprint includes `platforms: ios` produces a pack with `skipped: mobile`; `ship` on it exits 4; `ship` with a linked floor-waiver Decision proceeds and the pack records the waiver ID.

**WP4.10 Thin-slice verbs (M)** — The minimum of three M5 verbs needed to run one story end to end, built here so M4.5 does not wait for M5: `grill story` as a **markdown-only skill** (question set from `references/story.md`, no TUI, no baseline adapters; writes the issue via `actions`); `build` minimal (refuse unless Ready and body hash = snapshot; worktree from `origin/dev`; checkpoint commits with the `Aayudha-Checkpoint` trailer; open PR; no lease, no budget breaker, no `--resume`); `ship` minimal (green gate, one approving human review, squash-merge, Complete section, cleanup). WP5.3/5.5 extend these in place; nothing here is thrown away.
Accept: tier-1 eval for `grill story` yields an issue passing `ready`; `build` on a non-Ready issue exits 4; `ship` with only a bot review exits 4.

---

## M4.5 — Thin slice

One real story on the dogfood repo: `grill story` → `ready` → `build` → `verify` (coverage + SAST floors) → `inspect` → human approving review → `ship`, all using the WP4.10 verbs. Proves the loop before breadth. Not a WP: it is a run, recorded as the first authoritative pack on aayudha's own `ledger`.
Accept: story reaches `merged` with an authoritative pack, Complete section filled, actions segment on `ledger`; `status` on aayudha shows proof coverage 1/1.

---

## M5 — Seats and authoring

**WP5.1 Seats, charters, RACI (M)** — `seats/<seat>/CHARTER.md` ×12: Product Strategist, Code Architect, Engineering Manager, AI Native Engineer (+SRE hat), UX Researcher & Evolutionist, DevOps & DevEx Tinkerer, Quality Architect, Mentalist, Auditor, Marketer & Documenter, Revenue Architect (a seat; its paired human sits with founder B), Founder. RACI generated from registry: one Accountable seat per verb, one owner per layer, *executes* vs *owns threshold* distinguished (Engineer executes `verify`; Quality Architect owns `CONSTRAINTS.md`; EM owns `inspect` policy, review lane executes it; `review` owned by UX; `retro` by EM; `respond` by Engineer; `audit` by Auditor; `measure` by Revenue Architect; `brief` by Mentalist; `tell` by Marketer; `release` by DevOps). Rendered into `AGENTS.md`. **`aayu seat add|remove|list`**: `add <seat> --human <login>` switches a seat from solo to paired (writes `[seats.<seat>]`, adds the login to CODEOWNERS for the seat's layers, posts the seat's charter and "how we work" digest to the human as an onboarding issue, and when it is the second human in the org flips the WP0.1 solo-mode policy off); `remove` reverses it, reassigns open stories' Technical Owner to the twin, and records the change on `ledger/seats/`. Seat changes are protected-path writes.
Accept: registry lint: every verb one A, every layer one owner; `seat add` fixture produces the expected CODEOWNERS diff and onboarding issue; `seat remove` leaves no story with a dangling human owner.

**WP5.2 Baseline adapters (M)** — PostHog, Sentry, GitHub read adapters (moved up from M7) so `grill` can pull baselines; each adapter returns `{metric, population, window, value, source_query, fetched_at}` so a baseline is reproducible.
Accept: recorded PostHog/Sentry/GitHub fixtures → the exact expected baseline tuples (golden); a missing adapter yields exit 5 with the message naming the `[integrations]` key to set; a write-scoped key is refused by WP1.3 before any call.

**WP5.3 `grill` (L)** — Question sets per type in `references/<type>.md` keyed to schema sections; baseline required before a target on Goal/Initiative; refuses Ready without validator pass; writes via `actions` under the human's identity when run by a human, bot otherwise; `grill brand` → `design/brand.yaml` (AA palette check); TUI (bubbletea) for CLI, prompt for MCP/skill; untrusted-input wrapping of any pasted material. Extends the WP4.10 markdown `grill story` in place.
Accept: tier-1 eval per type (10 fixtures, one per type) ends in an issue passing `ready` 10/10; tier-2 rubric on `grill goal` and `grill story` meets the WP3.4 bar (pass^5 ≥ 0.8, mean ≥ 3.5) and the result file is committed as the baseline; a Goal fixture with no reachable baseline is refused Ready with the message naming the missing metric.

**WP5.4 `slice`, `plan` (M)** — `slice`: XS–L stories with criteria, deps, footprint **computed from the Feature's expected paths via `LAYERS.md` globs and confirmed by the plan**; closes an XL as "split into". `plan`: checkpoints per layer, contracts, tests, instrumentation, rollback; EM plan-time review required for `autonomy.never` layers, recorded in the pack.
Accept: evals on two fixture features (one 3-layer, one 6-layer incl. L07 migration): every produced story is ≤ L and passes `ready` (tier 1, 100 %); each `plan` has exactly one checkpoint per layer in the story's footprint and a rollback step for any L07 checkpoint; tier-2 rubric meets the WP3.4 bar; an XL-sized input feature is returned as ≥ 2 stories with the XL closed "split into".

**WP5.5 `build`, `ship` (L)** — `build <story|bug>`: refuses unless Ready and body hash = snapshot; per-story **lease** (assignee + lock comment with TTL); worktree under `.aayudha/worktrees/<id>` from `origin/dev`; checkpoint commits with `Aayudha-Checkpoint: n/N`; replace-in-place progress comment and checkpoint record on `ledger`; `verify` per checkpoint; `[autonomy] budget {wall, cost, attempts}` → `status:blocked` + page on breach or second failure on a never-layer; `--resume` from the last trailer; retries 429/5xx with jitter. `ship <pr>`: green gate; human pass = approving review from the seat's `human` (solo → `[autonomy] approver`), `type != Bot`; never-layers require CODEOWNER approval regardless of streak; approval ID and streak state written to the pack; squash-merge; Complete section; status `merged`; branch/worktree cleanup. `aayu gc` prunes stale worktrees/leases. The agent writes the code; the binary owns worktree, commits, verify, PR, merge.
Accept: dogfood story Ready → merged; a bot review is rejected as a pass; a stale lease is reclaimed by `gc`.

**WP5.6 `decide`, `respond`, ops catalog (M)** — `decide`: options/evaluation/recommendation; `[decisions] authority.<type|layer>`; SLA in business hours (`[time]`): 24 h → `[notify]` page to the authority **and** escalation comment that @-mentions founder B's login (`[decisions] escalate_to`); 48 h → if `reversible: true` the recommended option is applied, the issue gets `decision:provisional`, the ratify/revert deadline is written to the body, and a Decision ratified later clears the label; if `reversible: false` the SLA lapse only re-pages and `status` keeps showing it overdue; stories `constrains`-linked show blocked; supersede chain. `decide --waiver <floor>` creates a floor-waiver Decision; `audit` counts waivers. `ops/actions.toml`: `{name, argv_template, params typed, approval auto|human, environments, max_per_hour, rollback, blast_radius}`; read only from the protected default branch at a pinned SHA; edits need two human CODEOWNER reviews and can never be `auto` on first merge. `respond <incident>`: untrusted telemetry wrapped; proposes a catalog action; `approval = human` executes only through a GitHub Environment with required reviewers and a short-lived prod credential never present in the agent env; timeline appended; closes with a linked Bug and retro entry.
Accept: non-catalog command refused; same-PR catalog edit cannot be used; human approval required and verified; a reversible Decision fixture at 48 business hours gains `decision:provisional` with the escalation comment mentioning founder B, an irreversible one does not.

**WP5.7 Autonomy, notify, halt (S)** — `aayudha-streak.yml` (protected) computes streak per layer footprint from merged-PR history; corrections detected automatically: changes-requested review, non-bot commit on the PR, post-pass `fix`/`rebuild`, revert, Incident/Bug linked within N days, plus manual `aayu:correction`; `autonomy/corrections.jsonl` on `ledger` written by CI. Uses the `[notify]` sender from WP1.6. `aayu halt` sets a repo variable that every workflow and verb checks; `aayu resume` clears it.
Accept: streak math at 19/20/21 and on each correction kind; `halt` stops a running loop within one poll.

---

## M6 — UX

**WP6.1 Design system: tokens + primitives (M)** — Semantic tokens (CSS vars, OpenUI5/Horizon-inspired), density modes, shadcn components with Phosphor swapped in, GSAP as **peerDependency** (never vendored; Storybook motion controls documented as not a visual builder), Storybook, axe on every story.
**WP6.2 Design system: floorplans (M)** — Seven Fiori-shaped floorplans as compositions (no SAP assets, icons, or fonts).
**WP6.3 Design system: platform parity (S)** — Token export for SwiftUI and Compose with identical names (test).
Accept (6.1–6.3): Storybook builds; axe clean; parity test passes; THIRD_PARTY_NOTICES lists GSAP, Phosphor, shadcn, OpenUI5 tokens.

**WP6.4 Figma + Mobbin (M)** — Figma REST + PAT primary (`get images`, nodes, variables), remote MCP opt-in; Mobbin MCP client returning **URLs and IDs only, never pixels** (terms verified by judge 3); lint blocks Mobbin-origin images anywhere.
Accept: `doctor` reachability; `research` fixture returns a reference set with URLs only.

**WP6.5 `research`, `wireframe`, `mockup`, `prototype` (L)** — As settled; artifacts under `ledger/design/<feature>/`; prototype behind OpenFeature flag with Code Connect mapping; screenshot diff vs Figma with declared tolerance (`[review] figma_diff_max`, default 2 % of pixels after anti-aliasing mask; typography and colour token mismatches are counted separately and must be zero).
Accept: eval chain on a fixture feature produces all four with pinned references; a prototype fixture at 1.5 % diff passes and one at 2.5 % fails; a fixture using a non-token colour fails regardless of pixel diff.

**WP6.6 `review`, `polish`, `baseline` (M)** — Lane A (Gemini 3.1 Pro / Fable 5.1 medium, `--deep` high) with Nielsen 10×0–4 and personas incl. screen-reader; lane B (impeccable `detect --json`, Apache-2.0, shelled out; axe); isolated; dispositions `recapture|rebuild|fix|ship`; `polish` consumes critique; `baseline` per SHA. Lane A runs k = 3 and a finding counts only when ≥ 2 of 3 runs report it at severity ≥ 2 (reduces judge noise); lane B is deterministic.
Accept: three planted defects (missing focus ring, 3.8:1 contrast on body text, 32 px tap target) are each caught by lane A in ≥ 2 of 3 runs and by lane B in 1 of 1; a clean fixture yields `ship` from both lanes; `polish` on the planted fixture removes all three on re-run.

**WP6.7 `experiment`, `readout` (M)** — GrowthBook + PostHog; minimum sample size; writes Results.
Accept: fixture lift computed; under-sample refused.

---

## M7 — Loops and explain

**WP7.1 Loops (M)** — `loops/*.toml` → Amp schedules/plugin, Cursor Automations, Actions fallback; nightly `status --digest`, weekly `retro`, monthly seat self-review, quarterly `evolve`; all honour `halt` and `[budget]`; schedules are expressed in `[time] tz`.
Accept: golden outputs per host (Amp schedule JSON, Cursor automation file, Actions cron with the tz-converted cron expression); the Actions fallback runs `status --digest` on the dogfood repo on schedule and the fake notify sink receives one digest; setting `halt` makes the next scheduled run exit 4 with no writes.

**WP7.2 `measure` (M)** — Extends WP5.2 adapters with GrowthBook, billing CSV, App Insights/AWS/GCP exports; SQLite (`modernc.org/sqlite`) snapshots, DuckDB optional shell-out; AARRR/North Star queries; Results on Goal/Initiative; triggers parent-close rule.
Accept: fixture exports → expected cohorts.

**WP7.3 `brief` core (M)** — Registers `eli5|ste|engineer|regulatory`; purposes `explain|ideate|innovate|close`; HTML artifact with grounding footer; STE100 linter; a11y check on the HTML; `close` refuses without authoritative pack and Result.
**WP7.4 `brief` visuals (M)** — `*.diagram` → SVG (static + interactive), charts from snapshots with their query, illustrations via Painter/Gemini labelled as such, annotated screens from packs; alt text states the takeaway.
Accept (7.3–7.4): register evals, tier 2 at the WP3.4 bar per register, with the `eli5` rubric additionally requiring reading grade ≤ 6 (Flesch-Kincaid) and `ste` requiring the STE lint to pass (sentence ≤ 20 words, approved-word list, one instruction per sentence); planted 30-word sentence fails STE lint; every chart carries a query and re-running the query reproduces the chart data (golden); HTML output has zero serious/critical axe findings and every image has non-empty alt text; `brief close` on a story without an authoritative pack exits 4.

**WP7.5 `tell` (M)** — `docs|guide|changelog|release|launch|brand`; tone from `brand.yaml`; claims link only to Features with authoritative packs; UTM tagging; `tell publish` human-identity only, logged.
Accept: guide references only proven Features; publish refuses bot identity.

**WP7.6 `audit`, `retro` (M)** — `audit`: floors, proof coverage, waivers, decision SLA, autonomy exceptions, reconciliation of GitHub audit events vs `actions` segments (flags unlogged writes), license scan; report to `ledger/audit/<date>/`. `retro`: estimate vs observed checkpoints, correction rate, streaks, floor misses, cost per story; files findings as **Feedback issues on `agentic-ayudha`**, never PRs. Both record their inputs (`ledger` SHA, GitHub event cursor, actions segment list, config hash) in the report header so a report can be regenerated and diffed.
Accept: dogfood outputs match fixtures; a direct `gh` write outside `actions` is flagged; a solo-mode self-approval (WP0.1) and a human-fallback write (WP1.4) each appear in the audit report; re-running `audit` with the same recorded inputs produces a byte-identical report.

**WP7.7 `evolve` (M)** — Quarterly: accepted retro Feedback → first a failing eval case that reproduces it → skill diff that passes → PR labelled `agent-authored`, human CODEOWNER review, never auto-merge; ships via `rolling`; `install --pin` rollback; per-skill correction rate tracked before/after.
Accept: an `evolve` PR without a new eval case is rejected by CI.

**WP7.8 `release` (M)** — Promotion ladder for a Release issue; artifact identity; stamps included stories `released`; `tell release`; shipping evidence; triggers `measure` schedule.
Accept: dogfood rolling snapshot with stamped stories.

---

## M8 — 0.1

**WP8.1 Dogfood (L)** — `examples/benefits-qa/` (nine-layer example) built by the pod from `grill goal` to `release`; every story with an authoritative pack.
Accept: `status` shows proof coverage 100 %; `brief close` exits 0, HTML has zero serious/critical axe findings, grounding footer lists N packs.

**WP8.2 Docs and installers (M)** — `tell docs` → GitHub Pages; first `rolling` snapshot; brew/Scoop/winget after the stable decision.
Accept: `install.sh` on clean macOS/Linux/Windows VMs; `aayu doctor` passes.

---

## 2. Cross-cutting

- **Testing**: unit; golden files (per OS); recorded GitHub fixtures; live tests behind `AAYU_LIVE=1`; evals two-tier; injection fixtures for every verb reading external text; 90 % diff coverage on aayudha itself.
- **Security**: threat model §0.1; no plaintext secrets in repo files; scrubbed child env; SHA-pinned actions; signed + attested releases; CODEOWNERS on protected paths; MCP stdio only; license scan in CI.
- **Cost**: `[budget]` + breaker; inspect cache; screen-reader floors on label/nightly; macOS/Windows runners only for L01 stories; LFS quota warnings; cost per story in `retro`.
- **Recovery**: `init --resume`/`uninit`; ledger retry/lock; per-story lease; `build --resume`; `gc`; `halt`.
- **Versioning**: SemVer; schema versions for config, pack, lock, work-items with migrations; skills `version` in frontmatter; `lock.toml` drift in `doctor`.
- **Platform**: Windows paths (`/` in ledger paths, long paths, CRLF goldens); GHES unsupported in 0.1; forks: floors run, `inspect` and secrets skipped with a visible "unreviewed" status.
- **Dependencies (Go)**: cobra, koanf, invopop/jsonschema, go-github, githubv4, modelcontextprotocol/go-sdk, go-keyring, filippo.io/age, bubbletea, modernc.org/sqlite, otel, go-git (read-only), goldmark.

## 3. Risks

| # | Risk | Mitigation |
|---|---|---|
| R1 | Issue types org-only | Label fallback; parser treats both as one |
| R2 | Go SDK MCP OAuth experimental | Figma REST primary (V5) |
| R3 | Screen-reader runners headed/expensive | L01-only, label/nightly, budget cap |
| R4 | Android needs KVM | Deferred to 0.2; skip blocks `ship` without waiver |
| R5 | LFS cost / public-repo PII | V3; private media store; retention; purge |
| R6 | Model availability drift | `models show`, doctor flags, fallback table |
| R7 | DuckDB cgo | SQLite pure Go; DuckDB optional |
| R8 | Host skill-format drift | Registry-generated frontmatter; adapters regenerated; evals on two hosts |
| R9 | Desktop authoring bypass | `build` refuses non-Ready/drifted; Ready guard; MCP exposes no writes by default |
| R10 | Relicense to MIT | DCO from first commit |
| R11 | Agent self-grading | Base-branch gate, CI-authoritative packs, CODEOWNERS, computed streaks |
| R12 | Prompt injection via issues/exports | Provenance wrapping, tool allowlists, label-only triage token, injection evals |
| R13 | Cost runaway | `[budget]` breaker, `halt` |

## 4. Sequencing and size

Path: M0 → M1 → (M2 ∥ M3) → M4 → M4.5 → (M5 ∥ M6) → M7 → M8.
WP count 61 (XS 1, S 16, M 37, L 7) plus the M4.5 run, which is not a WP. No XL. WP4.9 counts as M for its 0.2 scope; its 0.1 part (skip-and-block) is XS and ships with WP4.8.

**Stacked PRs to `dev`** (one or two WPs each):

Main stack: PR1 WP0.1–0.3 · PR2 WP0.4–0.5 · PR3 WP1.1–1.2 · PR4 WP1.3–1.4 · PR5 WP1.5–1.6 · PR6 WP1.7–1.9 · PR7 WP2.1–2.2 · PR8 WP2.3–2.4 · PR9 WP2.5 · PR10 WP2.6 · PR11 WP2.7 · PR12 WP2.8–2.9 · PR17 WP4.1–4.2 · PR18 WP4.3–4.4 · PR19 WP4.5–4.6 · PR20 WP4.7 · PR21 WP4.8 (+ 4.9 skip-and-block) · PR22 WP4.10 · M4.5 run · then M5 and M6 stacks in parallel, M7, M8.

M3 stack, parallel off PR6: PR13 WP3.1 · PR14 WP3.2 · PR15 WP3.3 · PR16 WP3.4. PR17 is based on whichever of PR12 and PR16 merges last.

From PR11 onward (ground rule 1) every WP is a Story on the repo with a Completion Goal, and from M4.5 onward it ships with an authoritative pack.

**Founder-usable early**: after PR12 the founder can `init` a repo (human identity, day-one path), author issues through the forms, `ready` them, and see `status`; after PR14 the host skills install; after PR22 `grill story` exists as a skill and after the M4.5 run one story flows end to end.

---

## Appendix A — Judge findings and dispositions

Judge 1 (architecture): scores feasibility 3, SSOT 2, sequencing 2, testability 3, completeness 2.
Judge 2 (agentic SDLC): loop closure 2, HITL 2, proof integrity 2, seat ownership 2, self-evolution 2, early value 2.
Judge 3 (adversarial): credentials 2, supply chain 3, injection 1, ops 2, data 1, cost 1, recoverability 2, licensing 3.

| Finding | Disposition in v0.2 |
|---|---|
| J1-1 no model gateway WP | Accepted → WP1.6; rule 5 |
| J1-2 sequencing (ledger, baselines, charters, gates) | Accepted → WP2.4, WP5.2, WP3.2 charter-free, WP2.6 `--gates` |
| J1-3 M2∥M3; first PR not stacked | Accepted → §1, §4 |
| J1-4 verb registry | Accepted → rule 4, WP0.3 |
| J1-5 floors/autonomy/layers duplicated | Accepted → generated from default config and registry |
| J1-6 schema drift, embed path | Accepted → generated schema, root `embed.go` |
| J1-7 keychain headless | Accepted → WP1.3 backends |
| J1-8 S3 is not LFS; go-git no LFS | Accepted → V6, rule 2 |
| J1-9 ledger concurrency, streak file | Accepted → WP2.4, V4 |
| J1-10 MCP positional args, TUI, stdout | Accepted → WP1.1 writer, WP3.3 |
| J1-11 Figma OAuth | Accepted → V5 |
| J1-12 symlinks on Windows | Accepted → V2 |
| J1-13 LLM checks non-deterministic | Accepted → WP3.4 two-tier, k-of-n |
| J1-14 vague acceptances | Accepted → rewritten in WP1.3, 0.5, 7.6, 8.1, 5.1 |
| J1-15/16 undersized WPs | Accepted → WP4.3–4.9, 6.1–6.3, 7.3–7.4, 2.5–2.6, 1.7 L, 2.3 L, 0.4 M, 3.2 L; count 59 in v0.2, 61 in v0.3 (WP1.9, WP4.10 added) |
| J1-17 field home; REST for sub-issues | Accepted → WP2.1, WP2.3 |
| J1-18 self-update vs package managers | Accepted → WP1.1 |
| J1-19 who writes code; worktrees | Accepted → WP5.5 |
| J1-20 Go coverprofile | Accepted → WP4.3 |
| J1 gaps (Windows, API budget, scopes, workflow auth, ledger protection, idempotency, versioning, exit codes, GHES, forks, binary size) | Accepted → §2, WP0.1, WP1.1, WP1.5, WP2.3, WP2.6 |
| J2-1 agent can forge proof | Accepted → WP4.8 CI-authoritative packs, ledger ruleset |
| J2-2 Completion Goal not checked | Accepted → WP2.1 criteria, WP4.1 mapping, WP4.8 |
| J2-3 streak gameable; corrections lost | Accepted → V4, WP5.7, footprint from paths |
| J2-4 human pass undefined | Accepted → WP5.5 |
| J2-5 audit trail fragmented; no cost | Accepted → WP1.5 segments on ledger, cost fields |
| J2-6 no done beyond merge | Accepted → states merged/released/measured, parent-close in WP2.8 |
| J2-7 no checkpoint reporting/budget/blocked | Accepted → WP5.5 |
| J2-8 no paging/decision authority | Accepted → `[notify]`, `[decisions]`, V8 |
| J2-9 seat RACI | Accepted → WP5.1 |
| J2-10 eval harness weak | Accepted → WP3.4 |
| J2-11 self-evolution ungoverned | Accepted → `evolve` WP7.7, retro files Feedback not PRs |
| J2-12 desktop MCP bypass | Accepted → WP2.7 guard, WP3.3 exposure, WP5.5 refusal |
| J2-13 value back-loaded | Accepted → M4.5 thin slice, §4 early value |
| J2-14 skips and waivers | Accepted → WP4.9, `decide --waiver`, SR phrases from snapshot |
| J2-15 proof coverage definition | Accepted → WP2.8 |
| J2-16 founder digest | Accepted → `status --digest` (WP2.8); sender in WP1.6; proof/streak columns fill in at M4/M5, loop scheduling at M7 |
| J2-17 inspect re-entry | Accepted → WP4.8 |
| J2-18 bugs wiring | Accepted → `build <story\|bug>`, WP2.9 |
| J2-19 EM plan-time review | Accepted, recorded in pack (WP5.4) |
| J2-20 Revenue Architect seat | Clarified in WP5.1 |
| J3-1 identity | Accepted → V1, WP1.4 |
| J3-2 self-grading | Accepted → base-branch gate, CODEOWNERS, structured output, V4 |
| J3-3 ops catalog | Accepted → WP5.6 |
| J3-4 PII on public ledger | Accepted → V3, WP4.5 redaction, purge |
| J3-5 untrusted input | Accepted → §0.1, WP2.9, WP4.8, evals |
| J3-6 retro rewrites skills | Accepted → WP7.6/7.7 |
| J3-7 actions log not audit | Accepted → rule 6, hash chain, reconciliation in `audit` |
| J3-8 token leak to toolchain | Accepted → WP4.3 scrubbed env |
| J3-9 over-scoped token | Accepted → WP1.4 |
| J3-10 self-update/supply chain | Accepted → WP0.4, WP1.1 |
| J3-11 install writes hooks | Accepted → WP3.2 absolute path, `--apply`, hook lint |
| J3-12 MCP write verbs | Accepted → WP3.3 |
| J3-13 cost control | Accepted → `[budget]`, WP1.6, WP4.8, WP4.7 |
| J3-14 concurrency/partial failure | Accepted → WP2.4, WP5.5 lease, V4 |
| J3-15 init recovery | Accepted → WP2.5 state/resume/uninit |
| J3-16 worktree GC/resume | Accepted → `gc`, `--resume` |
| J3-17 read-only prod scopes | Accepted → WP1.3 `scope` |
| J3-18 Mobbin terms | Accepted → WP6.4 URLs only, lint |
| J3-19 GSAP in BSD-3 package | Accepted → peerDependency, notices |
| J3-20 other licences | Accepted → license scan, notices |
| J3 gap kill switch, tool incident plan | Accepted → `halt`, SECURITY.md |

Rejected: none. Modified: J2-8's provisional decision at 36 h became 48 h and only for reversible decisions (V8); J3-1's "SSH/WebAuthn-signed approval" is not adopted; GitHub review identity with `type != Bot` is sufficient for 0.1.

## Appendix B — Second-pass review of v0.2 and dispositions

| Finding | Disposition in v0.3 |
|---|---|
| P1 verb count said 31 in three places, list had 32 | V7, WP0.3 corrected to 32; plumbing list completed |
| P2 ground rule 1 tracked WPs as Stories before `init`/`ready` existed | Rule 1 starts at PR11; rule 1a defines "n/a until" |
| P3 M4.5 relied on `grill story`/`build`/`ship` from M5 | WP4.10 thin-slice verbs added before M4.5; M4.5 is a run, not a WP |
| P4 §4 PR list omitted WP2.6, hid the M2∥M3 parallelism, and claimed `grill story` after PR11 | §4 rewritten: WP2.6 is PR10, M3 is a parallel stack off PR6, early-value claims re-dated |
| P5 "label-only bot token" does not exist on GitHub | WP1.5 `capability` policy hook; WP2.9 enforces label-only in `internal/actions` |
| P6 thirteen WPs referenced later artifacts without saying so | "n/a until" markers in WP0.1, 0.5, 1.5, 2.6, 2.7, 2.8, 2.9, 3.4, 4.7; `[notify]` sender moved into WP1.6 |
| P7 V3 had no implementing WP | WP2.5 visibility check and no unconditional `.lfsconfig`; WP4.5 seed data and production refusal |
| P8 V8 had no implementing WP | WP2.1 `reversible`; WP2.8 provisional column; WP5.6 label, deadline, founder-B escalation |
| P9 audit source named the org audit log (Enterprise-only) | Rule 6 uses GitHub event history with retention documented |
| P10 Appendix A counts stale; several acceptance checks had no pass bar | Counts updated; thresholds added to WP3.4, 5.2, 5.3, 5.4, 6.5, 6.6, 7.1, 7.3/7.4, 7.6, 4.9 |
| G1 App private key on laptops | WP1.4 token broker: short-lived installation tokens minted by a dispatch workflow, key never leaves repo secrets |
| G2 no schema upgrade path | WP1.9 `aayu upgrade` with grandfathered snapshots and packs |
| G3 no multi-repo view | `[fleet] repos` in user config; `status --all`, `doctor --all` |
| G4 no seat onboarding/offboarding | `aayu seat add\|remove\|list` in WP5.1 |
| G5 product repos with existing CI/branches | `doctor` coexistence report; `init` never edits existing workflows, prefixes checks, maps branch names |
| G6 SLAs in wall-clock hours | `[time] tz business_hours`; decisions and paging use business hours; Sev0/1 exempt |
| G7 day one has no bot identity and no second human | WP1.4 human-fallback path; WP0.1 solo-mode self-approval policy, both surfaced by `audit` |

Rejected: none.

### Deviations recorded while implementing PR1 (WP0.1–WP0.3)

Live tests on `rahulnakmol/agentic-ayudha` (a public repo on a personal account, one human collaborator) on 2026-10-11. Each row is the closest enforceable alternative to a plan assumption that GitHub does not support.

| # | Plan assumption | Live result | Implemented instead |
|---|---|---|---|
| D1 | WP0.1: `ledger` is pushable only by the bot identity and CI | **Partly enforceable.** A ruleset `update` restriction works on a personal repo: the founder's own token (repo admin) and Amp's user-to-server App token were both rejected (`GH013 … Cannot update this protected ref`), and a deploy key in the bypass list pushed (rule-suite `result: bypass`). But the GitHub Actions app (15368) cannot be a bypass actor on a personal repo (`422 Actor GitHub Actions integration must be part of the ruleset source or owner organization`), so CI's built-in `GITHUB_TOKEN` can never be a ledger writer. Only a GitHub App installed on the owner account is accepted, and only its installation tokens bypass; user-to-server tokens act as the user. Individual users (a machine user) cannot be bypass actors; only repository roles can. The rule-suite list shows no actor for the deploy-key push while the single-suite endpoint shows the repo owner's login, so `audit` must identify deploy-key writes from the ledger commit, not from the actor name | Ruleset `ledger-writers`: deletion, non-fast-forward and update restricted; bypass = repository admin role (the founder, day-one rule). WP1.4 `init --identity` replaces the admin bypass with the bot App (Integration actor), and CI writes `ledger` with that App's installation token (`actions/create-github-app-token`), never `GITHUB_TOKEN`. The machine-user fallback becomes a deploy key (or a repository-role bypass, which covers every collaborator in that role) |
| D2 | WP0.1 solo mode: the founder approves their own protected-path PR on a commit other than the last push | **Not enforceable.** GitHub refuses self-approval (`Review Can not approve your own pull request`). A ruleset with `require_code_owner_review: true` and 0 required approvals did not block a `skills/` PR authored by the sole code owner (scratch PR #2 merged, rule-suite `result: pass`). Requiring ≥ 1 approval would block every PR to `dev`, not only protected paths, and turn every merge into a bypass | Workflow `protected-paths` (`pull_request_target`, so GitHub runs the default branch's copy and a PR cannot weaken it; it never checks out PR code) fails when a PR changes any path in the base branch's `.github/CODEOWNERS`. Ruleset `protected-paths-codeowner-review` requires that check (and keeps native CODEOWNER review switched on for when approvals become ≥ 1); its only bypass is the repository admin role in `pull_request` mode. A protected-path PR therefore merges only through an explicit admin bypass, which GitHub records in the rule-suite history with `result: bypass`; the hand-run reconciliation lists these until `audit` automates it (WP7.6). Because the workflow must be on the default branch, the gate covers `dev` from the PR after PR1; PR1 itself, which creates it, merged on green CI. WP1.4 extends the gate to pass on an approving review of the head SHA by a human CODEOWNER who is not the author (bot-authored PRs); WP5.1 switches to native CODEOWNER review with ≥ 1 approval when a second human seat is added. CODEOWNERS also covers `.agents/` and `.claude/` (host execution surfaces) and the whole of `.aayudha/config.toml`, because CODEOWNERS cannot protect the `[models]`/`[autonomy]` sections alone |
| D3 | WP0.1: linear history on `rolling`/`main` | Linear history forbids merge commits, but promotions are non-fast-forward merge commits (`Rolling snapshot: …`, `Release: v…`); and any rule placed in a ruleset whose bypass actor does the promotion is bypassed anyway | Ruleset `rolling-main-promotion`: deletion, non-fast-forward and update restricted; only the repository admin (bypass, always) can promote. No PRs target `rolling` or `main` |
| D4 | Rule 8: tag ruleset `v*` from `main` only | Rulesets cannot check which commit a tag points to | Ruleset `release-tags`: creation, update, deletion and non-fast-forward of `refs/tags/v*` restricted to the repository admin. WP0.4 `release.yml` must refuse a `v*` tag whose commit is not on `main` |
| D5 | WP0.2: Go 1.25 | `go.mod` says `go 1.25.0`, and CI builds and vets with Go 1.25.x. The orb setup already on `dev` (`.agents/setup`) pins Go 1.27.2, a supported toolchain that builds a `go 1.25` module unchanged; Go 1.25 itself no longer receives upstream fixes now that 1.27 is out | Keep `go 1.25.0` as the language floor and the 1.27.2 orb toolchain; `go vet`'s stdversion check stops newer standard-library APIs from creeping in |

### Deviations and decisions recorded while implementing PR2 (WP0.4–WP0.5)

Rows are named by WP so that parallel work does not collide on numbers.

| # | Plan assumption | Finding | Implemented instead |
|---|---|---|---|
| D-0.4a | Promotions `dev` → `rolling` → `main` follow a deliberate human decision (stable only after an explicit decision; WP8.2 "brew/Scoop/winget after the stable decision") | Founder policy, 2026-10-11: for this repository promotions need **no human approval**; the gates and the commit/tag conventions are unchanged | The gates are CI jobs: `ci.yml` dispatched with `gate=rolling` (PR suite + `integration` + `benchmarks`; `release-snapshot` is the production validation) or `gate=stable` (rolling + `extended-integration` + cross-OS `smoke`; `security` always runs). They run on a `promote/…` branch holding the exact `Rolling snapshot: …` / `Release: vX.Y.Z` merge commit, which is then pushed to `rolling`/`main` as a fast-forward and tagged. An annotated `v*` tag push runs `release.yml` end to end. The stable decision is now "an explicit stable-release request plus a green stable gate". Procedure: CONTRIBUTING.md, AGENTS.md |
| D-0.4b | WP0.4: CI checks DCO and Conventional Commits | GitHub writes the squash commit on `dev` itself, and promotion merges (`Rolling snapshot: …`, `Release: v…`) are not Conventional Commits by design | The `commits` job runs on pull requests: every non-merge PR commit needs a Conventional Commit subject and a `Signed-off-by` matching its author, and the PR title (the squash subject) must be a Conventional Commit. Pushes are not re-checked |
| D-0.4c | WP0.4: path filters | A workflow-level `paths:` filter leaves a required check pending forever; a job skipped by `if:` reports success | Path filters are job conditions fed by a `changes` job (`lint`, `docs`, `release-snapshot`). `build`, `generate-drift`, `security`, and `commits` always run on PRs; pushes and dispatches run everything. Fork PRs skip `release-snapshot` because they cannot mint an OIDC token for keyless signing |
| D-0.4d | WP0.4: brew tap | GoReleaser v2 deprecates Homebrew formulae for prebuilt binaries in favour of casks | The tap receives `Casks/aayudha.rb` (macOS and Linux). The tap, the Scoop bucket, the winget fork, and their environment-scoped tokens are founder actions; until they exist the `homebrew`, `scoop`, and `winget` jobs fail with a message naming the missing secret, after the GitHub release is already published |
| D-0.4e | WP0.4: skill lint in CI | The full lint (hooks, scripts, Mobbin-origin images) is WP3.1 | `TestSkillLint` (`embed_test.go`, run by `build`) checks the embedded `skills/*/SKILL.md`: frontmatter present, `name` equals the directory, ≤ 500 lines, every `aayu <command>` resolves in the registry. WP3.1 extends it |
| D-0.5a | WP0.5: `LAYERS.md` seat column reads "n/a until WP5.1" | WP0.3's registry already assigns each layer one owner seat | `LAYERS.md` keeps naming the owner seat; seat charters remain n/a until WP5.1 |
| D-0.5b | WP0.5: `CONSTRAINTS.md` generated from the embedded default config | The plan fixes some defaults (coverage 90 %, zero critical/high SAST and dependency findings, zero secrets, axe zero serious/critical at WCAG 2.2 AA, 44 px targets, 200 % text, streak 20 from WP5.7's 19/20/21 test, DAST opt-in) and names others without a value (`autonomy.never`, `autonomy.budget`, `[budget]` amounts, proof retention). WP1.2 lists `[verify]` but no `[floors]` section | `internal/config/defaults.toml` (embedded, the base layer WP1.2 adopts) holds floors under `[verify.floors.<floor>]`, plus `[autonomy]`, `[budget]`, `[proof]`. Values the plan does not fix are marked "proposed (WP0.5)" in `CONSTRAINTS.md`: never-layers L04, L07, L09; per-story build budget 8 h, 25 USD, 3 attempts; 25 USD per story, 100 USD per day, 5 USD per PR for `inspect`, 2000 CI minutes per month; 90 days retention (`retention_days`). `defaults.toml` is a protected path. `internal/config/gen` fails when a key and its documentation row disagree |

### Deviations and decisions recorded while implementing WP6.1–WP6.3

| # | Plan assumption | Finding | Implemented instead |
|---|---|---|---|
| D-6.1a | WP0.4: npm licence scan (`security` job) with the allow list Apache-2.0, BSD-2-Clause, BSD-3-Clause, ISC, MIT | The design system is a pnpm workspace with one root `pnpm-lock.yaml` (`.agents/setup` installs from it), so the scan's `npm ci` per `package.json` fails for lack of a `package-lock.json`. Radix UI, under the shadcn/ui components, depends on `tslib`, which is 0BSD | The npm step installs with `pnpm install --frozen-lockfile --ignore-scripts` and checks `pnpm licenses list --recursive --prod` against the allow list; dev tooling (Storybook, Playwright, axe-core MPL-2.0) is not distributed and not scanned. 0BSD (public-domain equivalent, no conditions) joins the allow list. GSAP stays out of the scan because it is a peer dependency only (J3-19) |

### Deviations recorded while implementing M1 (PR3–PR6)

Rows are named by WP so parallel threads do not collide on numbers.

| # | Plan assumption | Finding | Implemented instead |
|---|---|---|---|
| D-1.1a | WP1.1: `self-update` verifies the cosign signature and provenance | The sigstore Go libraries are not on the §2 dependency list, and verifying a keyless bundle (Fulcio chain, Rekor inclusion, certificate identity) by hand is not something to reimplement | `self-update` shells out, as `install.sh` does: `cosign verify-blob --bundle checksums.txt.sigstore.json` pinned to `…/.github/workflows/release.yml@refs/tags/<the exact tag>` and the Actions OIDC issuer, then the archive's SHA-256 against the signed `checksums.txt`, then `gh attestation verify --signer-workflow …/release.yml`. A missing tool is exit 5 with a pointer to `install.sh`; a missing or bad signature, checksum, or attestation is exit 4 and the binary is untouched. Release asset names are the WP0.4 GoReleaser names (`aayudha_<version>_<os>_<arch>.tar.gz`, `.zip` on Windows) |
| D-1.2a | WP1.2 accept: "a 24 h SLA started Friday 17:00 London time expires the following Monday 16:00" | With the `[time]` default of Mon–Fri 09:00–18:00 that instant is 8 business hours away (Friday 17–18 plus Monday 09–16). 24 business hours land on Wednesday 14:00. No consistent reading of "business hours" gives Monday 16:00 for 24 h | `[decisions] sla_hours` counts business hours literally (`config.Clock.Add`, local wall-clock windows in the IANA zone, so DST never shifts them). The table test pins both: 8 h from Friday 17:00 → Monday 16:00, and 24 h → Wednesday 14:00, each also across the 25 October 2026 clock change (16:00 and 14:00 local, now GMT) |
| D-1.2b | WP1.2 layer list "`AAYU_*` → flags" and section keys | The plan names no environment-variable or flag syntax, and gives keys without units | Env layer: `AAYU_<SECTION>__<KEY>` (double underscore between levels, e.g. `AAYU_BUDGET__PER_DAY_USD`); other `AAYU_*` variables (`AAYU_AGENT`, `AAYU_LIVE`, `AAYU_SECRET_*`, `AAYU_GITHUB_TOKEN`) are not configuration. Flags layer: a new global `--set key=value` (repeatable). Keys carry units: `[decisions] sla_hours provisional_after_hours` (business hours), `[proof] retention_days`, `[autonomy.budget] wall_hours cost_usd attempts`, `[lfs] quota_warn_percent`. The defaults layer is WP0.5's `internal/config/defaults.toml`, extended with `[lfs]`, `[decisions]`, and `[time]` defaults (Europe/London, Mon–Fri 09:00–18:00), which `CONSTRAINTS.md` does not render. A file without `version` is v0 (plan v0.1); v0→v1 drops `[lfs] server = "github"` and refuses any other server (V6). `[models.gateway]` (founder request) routes model calls through one AI gateway; see WP1.6 |
| D-1.3a | WP1.3: "after every secret-handling command" and a headless-Linux CI job for the env and file backends | V7 fixes the plumbing list, so there is no `secret` command; the plan does not say where the age key lives; provider scope APIs belong to the WP5.2 adapters | Secrets are handled by `aayu auth set-secret <name>` (value from stdin only, refused for agents), `auth delete-secret`, `auth list-secrets` (names and backends, never values), and `auth status`. File backend: an X25519 identity at `$XDG_CONFIG_HOME/aayudha/age-identity.txt` and the ciphertext at `$XDG_DATA_HOME/aayudha/secrets.age`, both 0600. The canary test runs each command against a store whose keychain is unavailable (as on the `build` job's headless Ubuntu runner, which is the headless-Linux CI job) and scans home, repo, temp, and output. `[integrations.<name>] scope` is enforced before any call by `secrets.CheckScope` (a read verb refuses a `write` key); verifying the key's real scope with the provider is n/a until WP5.2 |
| D-1.4a | WP1.4: token broker "returns [the token] encrypted to the caller's ephemeral age key" | A `workflow_dispatch` run has no return channel, and `actions/create-github-app-token` revokes its token when the job ends | `aayudha-token.yml` (embedded template, installed by `init --gates`, WP2.6) validates the inputs, mints with `skip-token-revoke: true`, encrypts `{token, expires_at = now + 55 min}` to the caller's one-time age key, and uploads it as a one-day artifact named by the request ID; the client finds the run by `run-name`, downloads and deletes the artifact, and refuses a token that is expired or outlives 1 h. Tested against a fake GitHub; the live round-trip is `TestBrokerLive` behind `AAYU_LIVE=1` and `AAYU_LIVE_BROKER_REPO`, which needs the founder to create the App. CI uses `AAYU_BOT_TOKEN` from `actions/create-github-app-token`; with `--ci`, bot and ledger writes are refused without it or when it equals `GITHUB_TOKEN` (D1) |
| D-1.4b | Appendix B D2: extend `protected-paths` to pass on a human CODEOWNER's approval of the head commit | `pull_request_target` does not fire on reviews, and `pull_request_review` runs the PR's own workflow file | The gate logic is Go (`internal/identity.ProtectedPathsGate`, fixture-tested) run from a checkout of the base branch (the `pull_request_target` default; never the PR). A review fires `protected-paths-review.yml` (no token, no checkout), whose completion fires `protected-paths-rerun.yml` from the default branch, which re-runs the base-branch `protected-paths` run for that head SHA. Latest decisive review per reviewer counts, as on GitHub; every protected path needs one of its own owners; `[identity] bot_logins` is read from the base branch's `.aayudha/config.toml`. Only root-anchored literal CODEOWNERS patterns are supported; any other pattern fails the gate closed. Team owners are not resolved |
| D-1.5a | WP1.5 accept: "dry-run `init` records planned actions and performs none" | `init` lands in WP2.5 | The test drives `init`'s action plan (labels, issue type, form, `ledger` ref) through `actions.Do` with `--dry-run` against a fake GitHub: every record is `planned`, the server sees zero requests, and the chain verifies; WP2.5 reuses the same path. The log is `.aayudha/telemetry/actions.jsonl` in a checkout (XDG state dir elsewhere), git-ignored; segments go to `.aayudha/ledger-outbox/actions/<date>/<host>-<run>.jsonl` until the ledger writer exists (WP2.4). A capability is enforced twice: `Do` checks the action's declared request before calling anything, and `actions.Guard` wraps the HTTP transport so a perform function cannot exceed it. `aayu actions verify-chain` exits 4 on a broken chain |
| D-1.6a | WP1.6: one client per provider (Anthropic, OpenAI, Google, xAI) | Founder, 2026-10-11: models are reached through one AI gateway (repo environment `ai-gateway`: secret `AAYU_SECRET_AI_GATEWAY`, variable `AAYU_AI_GATEWAY_URL`) that speaks Anthropic Messages, OpenAI Responses, and OpenAI Chat Completions on one base URL with a Bearer key. Model IDs verified there: `claude-opus-5-5`, `claude-sonnet-5-5`, `claude-haiku-5-5`, `claude-fable-5-1` (Anthropic, not Google), `gpt-6.1-sol`, `gpt-6-astra`, `grok-4.7`, and only `gemini-3.1-pro-low`. No JSON Schema validator is on the §2 list | `[models.gateway] base_url secret api_style styles` routes every lane through the gateway; without it each family uses its direct provider (Anthropic Messages, OpenAI Responses, Google and xAI Chat Completions; key secret named after the family). Wire format per model: a `styles` entry for the model ID (`"grok-4.7=chat"`; a list because IDs contain dots), then for its family, then `api_style`, then the family default (anthropic → messages, openai → responses, others → chat). Family is inferred from the model ID, never the endpoint, so reviewer ≠ builder holds through one gateway. The registry carries the IDs (Gemini note: `gemini-3.1-pro-low` substitutes for Gemini 3.1 Pro; Fable is an anthropic alternative, valid only when the builder is not anthropic); heavy reviews use `gpt-6-astra` at high effort; Cursor excludes the openai family, so a GPT lane on Cursor falls back to Grok and logs it; a direct call where the host default is host-managed (Amp) uses the Claude Code default. Cost: the endpoint's reported `usage.cost`, else `[[models.prices]]`, else a deliberately high fallback rate (15/75 USD per million tokens) so an unpriced model trips the breaker early. The breaker trips when spend reaches the limit (the boundary trips), writes `.aayudha/state/paused` (agents' verbs exit 6 until a human clears it; `aayu resume` is WP5.7), and pages once per scope per day. `[notify]` gains `issue`, `smtp_addr`, `from`, `smtp_secret`. Structured output validates against an in-house JSON Schema subset (`models.Schema`). Live calls: `TestGatewayLive` behind `AAYU_LIVE=1`, run by `models-live.yml` on pushes to `dev`, daily, and on dispatch with `environment: ai-gateway`, never on PRs, green-skipping while the secret is absent |
| D-1.7a | WP1.7 check list and "nonzero on floor-blocking failures" | Several checks depend on later WPs or have no GitHub API (LFS quota on personal accounts); the plan does not name the exit code or how hosts are detected | `internal/doctor` runs 20 checks as pure functions of a probe, each golden-tested on a healthy and a failing fixture (`testdata/checks.golden`). Floor-blocking (exit 3): git < 2.40 or missing, git-lfs missing, invalid config, a repository schema newer than the binary. Warnings never fail: development build or newer release, missing `gh`, no or over-scoped token, bot identity not configured (day-one path), no detected host (`AMP_THREAD_ID`, `CLAUDECODE`, `CURSOR_AGENT`/`CURSOR_TRACE_ID`, or `--host`), no writable secrets backend, missing optional tools (one row), leaked worktrees, integration secrets not set. Not yet available: skill copies vs `lock.toml` (WP3.2), MCP registrations (WP3.3), LFS quota (WP4.2), stale leases (WP5.5), provider reachability and real key scope (WP5.2). Coexistence: local workflow files and their check names, branches other than dev/rolling/main/ledger, the default branch and the `[branches]` mapping `init` will write, rulesets through the API, and collisions with `aayudha-*.yml` or `aayudha/` checks. `--fix` reports nothing to re-sync until WP3.2. `--all` checks each `[fleet]` repo through the API |
| D-1.8a | WP1.8: OTel traces to a file, OTLP opt-in | `go.opentelemetry.io/otel` v1.47 requires Go 1.26 (D5 keeps the 1.25 floor), and the upstream OTLP exporters pull in gRPC and protobuf | `go.opentelemetry.io/otel` and `otel/sdk` v1.46.0. Both exporters are in `internal/telemetry` and write OTLP JSON: one request per line in `.aayudha/telemetry/traces.jsonl`, and, only when `[telemetry] otlp_endpoint` is set, a POST to `<endpoint>/v1/traces`. The `OTEL_*` variables are ignored so nothing is sent by accident; `[telemetry] disabled = true` writes nothing. Each command run is one span with its exit code |
| D-1.9a | WP1.9 accept: v1 → v2 byte-exact diff; v1 pack passes `proof verify` on v2; v3 repo refused by a v2 binary | This binary is the first with versioned files: config is v1, and `lock.toml`, `state.json`, and proof packs (WP3.2, WP2.5, WP4.1) do not exist yet | `internal/upgrade` migrates `.aayudha/config.toml` and `config.local.toml` (`version`), `lock.toml` (`version`), and `state.json` (`schema_version`) as text, so comments survive; it never touches `ledger`. The byte-exact test runs a simulated v2 binary's migrations over a v1 fixture repo against a hand-written expected diff, then re-plans to zero changes; a v3 fixture is refused by the simulated v2 binary. In the real CLI a write verb (every verb except `doctor`, marked `ReadOnly` in the registry) on a repository newer than the binary exits 2 with the `aayu self-update` hint. `upgrade --apply` is refused for agents. Pack readers per schema version (`proof verify`), the per-story schema column in `status`, and skill-copy migration are n/a until WP4.1, WP2.8, and WP3.2 |

### Deviations recorded while implementing M3 (PR13–PR16)

Rows are named by WP so parallel threads do not collide on numbers.

| # | Plan assumption | Finding | Implemented instead |
|---|---|---|---|
| D-3.1a | WP3.1: frontmatter includes `paths`; lint checks "every `aayu` call resolves", hooks, and Mobbin-origin images | Claude Code reads `paths` as globs that limit when the model loads a skill on its own, so an empty list would hide a skill; the plan gives no globs per verb, and a product repo's layer globs are its own. Prose mentions of the binary ("the aayu binary") are not calls. A skill's hooks live in its frontmatter (Claude Code `hooks`). Mobbin terms forbid its pixels, not links to it | `paths` is a registry field (`Command.Paths`), rendered only when set; no verb sets it yet. `internal/skills.Lint` replaces the WP0.4 subset (D-0.4e) and runs over the embedded tree in `go test` (`TestSkillLint`, CI job `build`). Calls are read from inline code, fenced code, and `scripts/`; a call resolves when the command is in the registry, a bare first argument is a value of an enum first positional, and every `--flag` is the command's input or a global flag (`verbs.GlobalFlags`, moved from `internal/cli`). `scripts/` may hold only comments, `set` options, and `aayu …` lines without shell operators. Hooks must be `type: command` with the command exactly `aayu hook <name>` (optionally `--host <host>`). Mobbin: an image file whose name or bytes (embedded metadata) mention Mobbin, or an image embed whose URL does; plain links pass. A ninth rule rejects symlinks (V2). Each rule has a failing fixture in `internal/skills/lint_test.go`, with 200/201-character descriptions and 500/501-line bodies at the boundary |

### Deviations recorded while implementing M2 (PR7–PR12)

Rows are named by WP so parallel threads do not collide on numbers.

| # | Plan assumption | Finding | Implemented instead |
|---|---|---|---|
| D-2.1a | WP2.1: "Ten types", with fields per stage, parents, and links | The plan names the count and, across WPs, Goal, Initiative, Feature, Story, Bug, Decision, Feedback, Incident, Release, and the Experiment that `experiment`/`readout` work on; "Result" is something written onto a Goal, Initiative, Feature, or Experiment, not a type. It fixes no field lists or link kinds | The ten types are Goal, Initiative, Feature, Story, Bug, Decision, Experiment, Release, Feedback, Incident (`internal/workitems/schema.go`, schema v1; `grill`'s `type` input is now this enum). Result is a Complete-stage field. Parents: Initiative → Goal, Feature → Initiative, Story → Feature (required at Ready); Experiment → Feature or Initiative (required at Ready); Bug → Feature or Story, Decision → any delivery type or Incident, Release → Goal (optional); Feedback and Incident are intake with no parent. Links are typed lines `<kind> #N` in a Links field: `supersedes`, `constrains`, `depends-on`, `caused-by`, `becomes`, `includes`, `waives`. A Completion Goal criterion is one line `N. [evidence_kind] state: …; expected: …`; Measure, Threshold, Scenario, and Environment are separate Ready fields of the Completion Goal. A criterion without a valid evidence kind is unmapped. Effort is required at Ready for Story and Bug; `effort:XL` blocks Ready for every type |
| D-2.1b | WP2.1: Priority/Effort/Risk/Health/Horizon "as labels on personal repos, GitHub issue fields where available" | GitHub issue fields are organization-level and in preview, with no support in go-github v90 (the newest release whose `go` directive fits the 1.25 floor, D5) | Every repository uses labels in 0.1 (`priority:P0–P3`, `effort:XS–XL`, `risk:low/medium/high`, `health:on-track/at-risk/off-track`, `horizon:now/next/later`; values proposed here), with one label per prefix enforced by validation. The label is the only home, so these are not form fields. Delivery Status is `status:<value>`. Moving a field to issue fields later is a schema-version migration (WP1.9) |
| D-2.2a | WP2.2: "forms generated from schema"; accept "`init --dry-run` lists labels/types" | `init` itself is WP2.5. GitHub rejects a form whose `type:` names an issue type the owner does not have, and personal accounts have none | `go generate` writes one form per type to `work-items/forms/aayudha-<type>.yml` (the `aayudha-` prefix keeps them apart from a product's own templates); `init` installs them under `.github/ISSUE_TEMPLATE/` and adds `type:` only on organizations. Forms carry the Create fields (required) and, for non-intake types, the Ready fields (optional until `ready`); intake forms add the `intake` label. The WP2.2 binary implements only `init --dry-run`: it plans the label and issue-type creations through `actions` (all `planned`, zero writes) and lists them; without `--dry-run` it exits 2 until WP2.5. Forms are checked against GitHub's form rules in `TestFormsPassGitHubFormSchema` and were validated once against the SchemaStore `github-issue-forms` JSON Schema |
| D-2.3a | WP2.3: go-github and githubv4, ETag cache, primary and secondary rate limits with serialized bulk writes, pagination, call budget per verb | go-github v91+ requires Go 1.26 (D5). Creating and deleting a repository needs rights the founder's day-to-day fine-grained token lacks. The plan does not say how "every external write goes through `internal/actions`" is enforced for API calls | `github.com/google/go-github/v90` (BSD-3-Clause) and `github.com/shurcooL/githubv4` (MIT, GraphQL for Projects v2 only), with `go-querystring` and `shurcooL/graphql` (`golang.org/x/oauth2`, used only by githubv4's own tests, is held at v0.36.0 because v0.37 requires Go 1.26). One transport stack in `internal/github`: a mutating request without the context mark that only `Client.Write` (a wrapper over `actions.Do`) sets fails with `ErrUnlogged` before it leaves the process, so a write outside `actions` is impossible rather than merely discouraged; the action's capability is checked per request; writes are serialized with a 1 s gap; 403/429 responses are retried up to 3 times after `Retry-After`, the primary reset, or one minute for an unlabelled secondary limit, and fail at once when the wait exceeds 2 minutes; a per-run in-memory ETag cache revalidates repeated GETs (304s are counted as `not_modified`); a call budget (default 2000 per run, including 304s) stops a run with `ErrBudget`; `Stats` is what verbs report in `--json`. GitHub and network errors map to exit 5. Live tests create `aayu-scratch-*` repositories with `AAYU_LIVE_ADMIN_TOKEN` when set (the founder's admin token) and delete them; `TestLive` with `AAYU_RECORD=1` rewrites the recording that `TestRecordedContract` replays request by request |
| D-2.4a | WP2.4: temp worktree, fetch-rebase-retry with bounded backoff, per-repo file lock, content-addressed paths | GitHub reports a lost race between two pushes to the same ref as `[remote rejected] (failed)`, not as a non-fast-forward. A ledger commit is a set of whole-file writes and appends, so "rebase" can re-apply them to the new tip instead of replaying a patch. `flock` is not portable to Windows | `internal/ledger.Writer` fetches the tip into a private ref, builds the commit in a temporary detached worktree, and pushes it without force; on a rejected or lost-race push it waits (100 ms·2ⁿ with jitter, capped at 5 s, 10 attempts), resets the worktree to the new tip, re-applies the same files, and pushes again. A commit that changes nothing is not pushed. The lock is an `O_EXCL` file in the clone's git common dir, taken over after 10 minutes (a dead holder). `ContentPath` gives content-addressed paths. A token travels as an `http.extraHeader` in `GIT_CONFIG_*` environment variables, never on a command line; a human-fallback write carries an `Aayudha-Identity: identity:human-fallback` trailer. Each write is a `ledger.push` action, except checkpoint segment pushes (`ledger.Sink`, now the actions log's segment sink in a checkout, falling back to the local outbox when the remote has no ledger), which carry the log itself. Accept: 5 writers in separate clones plus 5 sharing one clone all land against a local bare remote (11 commits, every writer's commit an ancestor of the tip, shared appends intact), and 5 concurrent writers land on a live scratch repository (`TestLiveConcurrentWriters`) |
