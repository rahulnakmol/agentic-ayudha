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

**WP1.1b CLI experience (M; scope addition, founder request 2026-10-15; Appendix B S-1.1b)** — "The CLI commands should be fun to use." `aayu` alone in a checkout is a home screen: where the repository is on the journey (Set up → Shape → Ready → Build → Prove → Ship → Release → Learn), its adoption level, Ready stories, stories in progress, story PRs waiting for a person, blocked stories, failing floors in local packs, measurements due on the ledger, and the next suggested command; `aayu next` prints only that command and why, and never offers an agent a human-only verb. The verb's Accountable seat speaks a one-line intro and a sign-off read from its charter's persona section (owned by the persona work), while facts stay plain. On a person's terminal: colour themes, a tidy table for `status` and `doctor`, live progress for `build`, `verify`, `release`, `doctor`, and `status`, and a short celebration for the first Ready story, the first proven ship, every release, and reaching Level 1. Every error ends with `next: <the exact command>`, in text and in `--json`. `aayu tour` walks a person to Level 1 with the real `init` and `install`. `aayu completion bash|zsh|fish|powershell` is generated from the registry, enum values included. Fun never costs correctness: `--json`, MCP, agents (`AAYU_AGENT=1`), CI, and pipes get the existing stable text; `--plain`, `[ui] plain`, `AAYU_PLAIN=1`, and `TERM=dumb` give words only (no colour, motion, emoji, or box drawing); `NO_COLOR` removes colour; no status is carried by colour or a symbol alone; exit codes do not change; binary under 60 MB with `CGO_ENABLED=0`.
Accept: the home screen golden in nine modes (pipe, `--json`, `--plain`, `[ui] plain`, `TERM=dumb`, `NO_COLOR`, Rich, agent, CI) with no non-ASCII and no escape sequences outside Rich; `next` for an agent skips `ship`; errors end with `next:` in text and JSON with the exit code unchanged; completion scripts for four shells and registry enums through `__complete`; `tour` refused for agents and without a terminal, and on a terminal runs `init` then `install` for the chosen host.

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

**WP3.2b Native host plugins (M; scope addition, founder request 2026-10-15; Appendix B S-3.2b)** — "Same is the plugin on Claude Code or Codex and Amp." OpenAI Codex (CLI and IDE extension) joins the registry's hosts: `install --host codex` writes `.agents/skills/` copies, `AGENTS.md`, a marked `[mcp_servers.aayudha]` block in `.codex/config.toml`, and `.codex/hooks.json`, and `doctor` checks that registration. The repository is a plugin marketplace (`.claude-plugin/marketplace.json`) for one plugin, `aayudha`, which Claude Code and Codex both install: the skills from `skills/` as `/aayudha:<verb>`, the host-neutral slash commands in `hosts/commands/` (`/aayudha:home`, `/aayudha:next`), the hooks, and the MCP server. The Amp plugin stub becomes a real plugin: palette commands for the main verbs and the shared commands, journey state at session start, and the hooks. `install --host claude` adds a status line with journey state. Cursor keeps its rules (`AGENTS.md`), MCP, and hooks. Safety is the same everywhere: an absolute binary path checked against its version wherever install writes the files, and a skills-version check where a marketplace plugin calls `aayu` from `PATH`; no host offers a human-only verb, and the `pre-tool` hook blocks shell commands that run one; write verbs follow `[mcp] expose_writes`; hooks call only `aayu hook <name>`.
Accept: golden install trees for every host on Unix and Windows (CRLF, no symlinks, re-plan empty), Codex included; the plugin manifests carry this binary's skills version and only `aayu hook` commands; a shell command running `ship`, `release`, `decide`, `respond`, `init`, `install --apply`, a human-only flag, or a person's plumbing is blocked by `pre-tool` on Claude Code, Codex, and Cursor; a smoke test per host loads the plugin and lists its commands with the host's own CLI (Claude Code `plugin validate`/`install`/`details`, Codex `plugin add`/`mcp list`/`debug prompt-input`, Amp `plugins list`), skipping a host whose CLI is absent.

**WP3.2c Agent environments (M; scope addition, founder request 2026-10-16; Appendix B S-3.2c)** — `aayu install --env` sets up the environment every agent host's cloud works in, for any repository: it discovers what the environment needs from the code (toolchains and versions, package managers, services, browsers, system packages, variables and secrets by name, the build and test commands), asks only what the code cannot say (grill style: few questions, a default with its reason, a readback), writes the spec to `.aayudha/env.toml`, and generates from it one shared, pinned, idempotent `.agents/setup` and an `.agents/check` that every host calls through its native configuration (Amp natively, Cursor `.cursor/environment.json`, Claude Code on the web a `SessionStart` hook, Codex the Install script it prints). `--verify` proves each host in a clean container of its documented base (setup cold, setup warm, then the check), fixing and retrying; `--cloud` and `--record` prove the live cloud environment. `.aayudha/env-verified.toml` records both. `doctor` reports each host as verified, stale, or missing (`agent-env`), and which agent CLIs and clouds this machine has (`agents`). `internal/agentenv` owns discovery, generation, verification, and detection; delegation calls `Detect` and `Ensure`.
Accept: discovery, generation, and the container proof on this repository (Go and a Node docs toolchain) and on a repository with another stack; each host's generated configuration; a live cloud proof for every host the build machine can reach, and generated files plus documentation for the rest; no AI key in a product repository's CI.

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

**WP4.5b Feature maps and behaviour checks (0.2; scope addition, founder decisions 2026-10-15; Appendix B S-4.5b, research in the private research note verification-skills-2026-10.md)** — Generic, not UI-only: a repository's **feature map** lists what its product can do, and each capability has a **behaviour check** of the kind that fits it: a UI flow, a CLI command, an API call, or a library or function call. The checks run in CI with no model and block the merge gate when one fails; a free drift check compares the map with the source on every PR; an agent app the user installed keeps the map honest, and no model key is ever put into a product repository's CI. The Engineer twin executes; the Quality Architect owns the floors.
**WP4.5b.1 Feature map and drift check (S)** — A map is a product-owned project skill `.agents/skills/verify-<product>/` anywhere in the repository (a monorepo has one per product): `SKILL.md`, the control tool, and `features/`. `features/README.md` holds one `map` JSON block (product, the control tool's argv, and surface rules: a kind, file globs, and a pattern whose group is one surface: a route, a command, a flag, an endpoint, an exported function) and the ordered index. Each feature file has an H1, one paragraph, one `feature` block (`id`, `layers`, `sources`, `surfaces` by kind, `verified_at`), four H2s in order (`Sub-features`, `How to reach it`, `Behaviour checks`, `Gotchas`), and one `check` block per check (`id`, `kind` ui|cli|api|lib, `evidence` kind, `steps` in the control tool's vocabulary, optional `requires`). `aayu verify --map` reads only the maps and the checkout, never runs product code, and reports index entries missing or dead, contract errors, sources that match nothing, handles (the `name`, `label`, `within`, and `test` values of a check's steps) absent from the feature's sources, surfaces the rules find that no feature lists, listed surfaces the rules no longer find, evidence maps naming unknown checks, and stale features (sources changed since `verified_at`); it exits 3 on drift. The `feature-map` floor runs the same check in every verify run, including the merge gate on every PR, where drift is reported in the pack and the proof comment but does not block: a renamed control that a check uses fails that check, which does block, and the rest goes to the maintain pass (WP4.5b.4), whose corrections arrive as their own PR.
Accept: on a fixture map, a renamed button, a new route, a removed command, a dead index entry, and an edited source are each reported by kind; a clean map exits 0; the check needs no network and starts no process but git.
**WP4.5b.2 Behaviour floor (M)** — The `behaviour` floor runs, in verify's sandbox, the checks of every feature whose sources or feature file the change touches (every check of a map whose README or control tool it touches), plus the checks `.aayudha/evidence/<story>.toml` maps to criteria (`checks = ["<feature>.<check>"]`, beside `tests`). aayu hands each check to the map's control tool (`<control> check`, the check and its output directory as JSON on stdin), which is written in the product's own stack and adds no runtime the product lacks: Node and Playwright for web apps, a shell or PTY driver for CLIs, plain HTTP for APIs, the product's test framework for libraries. aayu starts the app with `[verify] seed` only for ui and api checks, never against production (WP4.5), and gives cli and lib checks no network. The tool reports `passed`, `failed`, or `unreachable` (with the precondition it lacks) and its artifacts; aayu hashes them into the pack. Any failed check fails the floor, so the merge gate blocks at once; a criterion mapped to checks is evidenced only by artifacts of its own kind; an unreachable check proves nothing. `aayu verify --check <feature>[.<check>]|all` runs a selection on a person's or agent's checkout, as a preview (S-4.2b). A PR that serves no story runs the two map floors only.
Accept: with a fixture control tool, a passing check yields hashed artifacts mapped to its criterion; a failing one fails the floor (exit 3); a kind mismatch leaves the criterion without evidence; a production target exits 4 with nothing run; a no-story PR that touches a feature runs that feature's checks and no other floor; cli and lib checks run without network.
**WP4.5b.3 Create and maintain (M)** — The `verify` skill gains its body: what a run proves (a preview; only the gate's pack counts), how to drive the product through its map, and two procedures. **Create**: read the repository, list its capabilities, choose each one's check kind, write the map and the control tool in the product's stack, prove one check of each kind with `aayu verify --check` and leave `aayu verify --map` clean. **Maintain**: read the drift report and the failing checks; fix doc drift in the map, a harness gap in the control tool, and report a product regression as a Bug; never weaken a check to make it pass; edit only the skill's directory; at most one PR. `aayu init --verify-skill <web|cli|api|lib>` scaffolds `.agents/skills/verify-<product>/` (SKILL.md, the README with the entry contract, and a control tool stub in the product's stack for that kind) for the create procedure to fill. `install` mirrors product-owned `verify-*` skills to every host's skill directory, never overwrites them with aayudha's, and records them in `lock.toml` as product-owned; `doctor` reports a map that is missing, malformed, or drifted. `grill`, `plan`, and `inspect` read the map as grounding (WP4.5b.5).
Accept: an init golden per kind whose scaffold passes `verify --map`; install mirrors a product skill to each host and a second apply is a no-op; doctor rows for each state.
**WP4.5b.4 Maintain hand-off and sweep loops (S)** — No model keys in a product repository's CI: the model-driven maintain pass runs in an agent app the user installed. `aayu verify --maintain drift|always` posts the maintain brief (the drift report, the failing checks, the procedure, the edit scope, the one-PR rule) on one tracking issue with an @-mention of `[verify] maintainer` (`amp`, `claude`, `codex`, or `cursor`), through the actions log; the app's map corrections arrive as one PR through the normal gate. Two default loops: `verify-sweep` nightly runs every behaviour check deterministically and hands off when the map drifted or a check failed; `verify-maintain` weekly hands off always.
Accept: the hand-off golden for each app, replaced in place; a clean nightly posts nothing; the loops render for Actions, Amp, and Cursor and honour `halt`.
**WP4.5b.5 Grounding (S)** — `grill --assess` reports a `feature:<id>` fact for each mapped feature the idea's routes, commands, or words touch; `plan` writes the touched features' checks into the evidence-map skeleton; `inspect`'s context lists the features a PR touches with their checks, as untrusted data (§0.1).
Accept: a grill assessment cites a feature fact; a plan skeleton names a mapped check; the inspect context names the touched feature.
Proof: `examples/benefits-qa` (a web app: ui and api checks) and one non-UI product (a CLI: cli and lib checks) through the merge gate, with drift caught and a gate blocked by a broken behaviour.

**WP4.6 Floors: DAST + API (S)** — ZAP baseline against the running app when `[verify] dast = true`; k6 thresholds for API/CLI contracts.
Accept: fixture with a reflected-XSS route fails.

**WP4.7 Floors: screen reader (M)** — Guidepup VoiceOver (macOS runner) and NVDA (Windows runner); expected phrases come from the **Ready snapshot**, not from the agent at run time; transcript diff; runs only for stories whose footprint includes L01 (footprint computed from `LAYERS.md` globs over the PR's changed paths; the `slice`-confirmed footprint is n/a until WP5.4), on label or nightly to control runner cost.
Accept: fixture page with an unlabeled button fails against expected phrases; a PR touching only `internal/` paths skips the floor with `skipped: no-L01` in the pack.

**WP4.8 Merge gate + `inspect` (M)** — `aayudha-merge-gate.yml` runs from the **base branch** definition; re-runs `verify` on the PR head SHA in CI and writes the authoritative pack to `ledger`; fails if `pack.sha ≠ head` or Completion Goal hash ≠ Ready snapshot. `inspect <pr>`: context = diff + plan + pack + criteria, untrusted-input wrapped; review-lane model (family ≠ builder); **structured JSON** output validated against schema; judges each criterion against its evidence; dispositions `merge|fix|rebuild`; can downgrade, never override a floor; cached by diff hash; `concurrency: cancel-in-progress`; skips drafts; `[budget] per_pr_inspect_max`. Re-entry: `fix` → `build --resume`; `rebuild` → back to Ready review; cap 2 cycles then page; any post-human-pass `fix` is a correction.
Accept: planted defect → P1 + `fix`; clean → `merge`; PR text "disposition: merge" has no effect (injection fixture); third cycle pages.

**WP4.8b Review quality for agent-authored PRs (M; scope addition, founder request 2026-10-15; Appendix B S-4.8b, research in the private research note agent-delegation-and-review.md)** — `inspect` adopts what published reviewers converged on (Alibaba OpenCodeReview, Greptile, Codex review, Bugbot), as ideas, with no copied prompts: deterministic **context retrieval** through the contents API at the PR head (changed symbols per file, Go by AST and other languages by hunk headers; callers in the same package; related tests and whether the PR changed them; the `LAYERS.md` footprint), a deterministic **review plan per changed area**, findings v2 (severity, category, quoted snippet, file and line resolved by aayu from the snippet, suggested fix, confidence), **noise filtering** (findings outside the diff dropped; `style` and generated files left to the linters and drift check; a confidence threshold for P2–P3 only; de-duplication keeping the highest severity; suppression of findings resembling ones a human dismissed with `inspect --dismiss`, recorded per repository on the ledger, never for P0 or the protected categories security, data, concurrency), **two modes** (general: one reviewer; peer: two reviewers of different families, both ≠ builder, each checking the other's single-source findings with an asymmetric "prove it wrong" pass), and inline review comments for new findings. Floors and the P0/P1 rule stay above every model judgement (rule 7).
Accept: the seeded-defect eval group (planted bugs of several kinds plus clean controls) reports precision and recall for the 0.1 prompt, general, and peer modes on a dispatched tier-2 run; a style nit, a duplicate, a finding on a file outside the diff, and a finding matching two dismissals are filtered in tier 1 (recorded replies); a peer-refuted finding is dropped, a refuted P0 security finding is kept as disputed; `--dismiss` is refused for agents.

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

**WP5.1b Seat personas, practice playbooks, and team stage (scope addition, founder request 2026-10-15; Appendix B S-5.1b)** — Each of the twelve seat twins gets a character and a soul: `seats/<seat>/persona.toml` (one source) holds a name (a figure from Greek mythology, chosen by the founder; Appendix B D-5.1e), purpose, temperament and voice, values, signature habits and questions, how it pushes back and praises, what it never does, how it grounds itself, how it defers to a paired human, and a practice playbook drawn from published large-scale software practice (each entry with public sources and its depth at three team stages). `go generate` renders it into the charter's Persona region, a marked block at the top of the SKILL.md body of every verb the seat is Accountable for (saying whose voice speaks and whose standard applies where they differ: `verify`), and the AGENTS.md seat section; the MCP `persona` prompt renders it at run time. `[team] stage` (`startup`, `scaleup`, `enterprise`) and `[team] size` scale each practice's ceremony, never the floors, the evidence rule, or human-only gates. The evidence rule is mandatory for every twin: claims about code, architecture, risk, or progress cite their source; anecdotes illustrate and never replace evidence.
On-demand decision help (founder, same day, superseding a request for blocker automation): `decide --pack` (no new verb) routes a stuck human's issue or question to the owning seat's twin, which works the decision with them in its persona: the question in one sentence, grounded facts, two or three options with trade-offs, a recommendation and its door, the cost of waiting, the Mentalist's pre-mortem, the questions that would change the answer, and the one thing to decide. Read-only; nothing is applied; recording stays `decide --accept`. Every twin values helping a human decide quickly and never letting a blocker sit, as behaviour, not clocks.
Accept: every seat has a complete persona with sourced practices (load-time validation, no company résumé or trademark); every skill body carries its seat's block and every host's copy is identical; tier 1: an EM-voice review and retro explanation whose claims are cited pass, a planted uncited architectural claim fails, startup and enterprise answers carry their own stage's artefact, and a decision pack missing any part fails; tier 2: the EM retro case at both stages and an EM-owned and an Architect-owned decision pack meet the WP3.4 bar on a judge of another family, the packs scored on "a busy human can decide from this in under five minutes".

**WP5.1g The Data & AI Architect seat (M; scope addition, founder decision 2026-10-17; Decision #283; Appendix B S-5.1g)** — A thirteenth seat, `data-ai-architect` ("Data & AI Architect", twin Mneme; D-5.1g.f), the Code Architect's counterpart for data platforms, analytics and visualisation, machine learning, retrieval and knowledge graphs, and evidence-grounded agents, which interviews before it advises. It owns L05 Intelligence and L08 Analytics / reporting (L07 stays with the AI Native Engineer), is Accountable for `measure` (whose standard, metric definitions and targets, stays the Revenue Architect's through `Owns`) and for one new verb, `chart`: `aayu chart <issue> --kind data|ai` prints the grounding and the outline, `--draft` checks a chart note (every part, what each part must name, the stage's artefact, the evidence rule; exit 4), and `--create` posts it on the issue, one comment per kind replaced in place. Consulted on `plan`, `experiment`, and `evolve`; `decide --pack` routes data and AI questions to it.
Accept: registry lint with thirteen seats and 33 verbs; the persona loads with five sourced practices; `TestChart` (grounding read-only, a draft with each missing part or an invented citation refused with nothing written, `--create` posting once and replacing in place); tier 1 on two `chart` and two `measure` persona cases with planted uncited claims; tier 2 in the advisory `data-ai` group with a mean of at least 4.0 per skill, within a tier-2 budget of 60 USD.

**WP5.2 Baseline adapters (M)** — PostHog, Sentry, GitHub read adapters (moved up from M7) so `grill` can pull baselines; each adapter returns `{metric, population, window, value, source_query, fetched_at}` so a baseline is reproducible.
Accept: recorded PostHog/Sentry/GitHub fixtures → the exact expected baseline tuples (golden); a missing adapter yields exit 5 with the message naming the `[integrations]` key to set; a write-scoped key is refused by WP1.3 before any call.

**WP5.3 `grill` (L)** — Question sets per type in `references/<type>.md` keyed to schema sections; baseline required before a target on Goal/Initiative; refuses Ready without validator pass; writes via `actions` under the human's identity when run by a human, bot otherwise; `grill brand` → `design/brand.yaml` (AA palette check); TUI (bubbletea) for CLI, prompt for MCP/skill; untrusted-input wrapping of any pasted material. Extends the WP4.10 markdown `grill story` in place.
Accept: tier-1 eval per type (10 fixtures, one per type) ends in an issue passing `ready` 10/10; tier-2 rubric on `grill goal` and `grill story` meets the WP3.4 bar (pass^5 ≥ 0.8, mean ≥ 3.5) and the result file is committed as the baseline; a Goal fixture with no reachable baseline is refused Ready with the message naming the missing metric.

**WP5.3b `grill` as a conversation (M; scope addition, founder request 2026-10-15; Appendix B S-5.3b)** — Grilling feels like working an idea through with a sharp colleague and ends in a spec agents deliver without redo cycles. The skill reflects back what it heard, asks three to five questions a round (highest rework risk first) for as many rounds as the idea needs, proposes options with a default and its reason, and ends with a plain-language readback the human confirms or edits. Vague qualifiers ("high quality", "fast", "simple", "secure", "scalable", "reliable", …) are found in the request and decomposed by catalogues kept as data (`skills/grill/references/vague/<domain>.md`: dimensions, each with its question, default, evidence kind, and the facts to cite). `aayu grill <type> --assess --idea "<words>"` is a deterministic, read-only, MCP-safe fact pass the host's model consumes (ground rule 5): the flow and routes the idea touches, missing empty/loading/error/offline states, design tokens and components in use, related tests, analytics events, related stories and their Completion Goals, configured floors and budgets, and the LAYERS footprint, each fact with an ID, all of it untrusted data (§0.1). The scope guard (D-3.4c, D-5.3a–c) holds: ideas are proposed freely, but only what the human stated, confirmed, or accepted as a default enters the criteria; the rest goes to Parked ideas or Open questions, and a Provenance section records each criterion's source. A spec is done only when every qualifier is resolved into numbered, observable criteria or explicitly deferred, assumptions are listed, and every open question is answered or deferred; `grill` and `ready` refuse a criterion that still leans on a vague word with nothing to observe. CLI TUI, MCP prompt, and host skill all follow it.
Accept: `grill story --assess` on the synthetic bakery app reports the five-screen order flow and the pay screen's missing error state and writes nothing; tier 1 on two conversation cases (bakery "high-quality UX", "make search fast"): the final spec has no unresolved qualifier, cites only facts the fixture has, every criterion's quote is the human's own words from a topic they confirmed, and no parked idea is a criterion; tier 2 on both cases (scripted human, judge family ≠ skill family) scores question quality, concreteness, and fidelity against the WP3.4 bar.

**WP5.8 The vagueness gate: unpack vague asks at every entry point (M; scope addition, founder request 2026-10-16; Appendix B S-5.8)** — A request in words ("make onboarding slick with nice transitions", "it should be simple", "the PDP needs work") is unpacked before any skill acts on it, so the human and the agent read the same Goal, Completion Goal, and checkpoints. `aayu grill <type> --assess --idea "<words>"` gains a deterministic `gate` (`verdict` unpack or pass, `reasons` of kind vague, term, or outcome, `next`), the product types it detected (web, api, cli, library), and the request's abbreviations with their usual meanings. Every entry point that takes free text (the build, slice, and plan skills, the delegate commands, the MCP `grill` and new `ask` prompts, and a CLI hint when `build` or `plan` is given words) reads the verdict: on unpack the grill skill names the vague words, asks the riskiest questions first, reads back "Here is what done looks like", and hands back to the original skill once the story is Ready; a concrete request passes with no questions. The catalogues grow without becoming UI-only: motion and transitions, a rough-idea pass (who, job, outcome, smallest provable slice, out of scope), an abbreviation and jargon list, and product-type-aware dimensions for the existing qualifiers ("fast" is startup time and p95 per command for a CLI, interaction latency for a web app). A grilled Story or Bug states its Goal (the Summary), and carries a Checkpoints section that reaches every criterion and a meaning for every abbreviation.
Accept: the gate passes a concrete request and unpacks vague, term-laden, and outcome-less ones (unit tests on each kind and its near misses); tier 1 on four new conversation cases (bakery motion, bakery terms, a "make the CLI simple" request on a Go CLI fixture, and a concrete bakery request that must get no questions) plus the two WP5.3b cases; tier 2 in the advisory, path-triggered `conversations` group.

**WP5.9 `survey`: a codebase with no product docs (L; scope addition, founder request 2026-10-17; Decision #287; Appendix B S-5.9)** — Point aayu at a repository that has code but no product documentation (often a legacy codebase), on any stack, and get evidence-backed research at three levels (engineering, capability, product), an interview for what the code cannot say, a backlog of work items, and a path to a modern version whose behaviour is proven equal to the legacy one. One new verb, `survey` (Accountable Code Architect; Consulted Product Strategist and Quality Architect; no new seat): the binary indexes and checks, the host's model reads and writes (ground rule 5). **Index**: `aayu survey [path]` writes `.aayudha/survey/index.json` with no model: files and languages, build systems and their targets with sources (CMake variables and Make object lists expanded), symbols, include and import edges, a call graph resolved by name (ambiguous edges keep every candidate), entry points, I/O surfaces, tests, known-issue markers, documents, and git churn, each fact with a stable ID and a `path:line`; files group into modules by directory, split to a token budget along file stems; `--module <id>` prints one bounded fact pack inside an untrusted block. **Analysis**: `--draft survey.json` checks every claim's basis (`observed`, `stated`, `inferred` with reasoning, `confirmed` with a quoted answer, `unknown` with a question) and citations against the index, refuses `observed` product claims, and renders `SURVEY.md`. **Backlog**: `--backlog backlog.json` checks a Goal, Initiatives, Features, Stories, and Bugs against the work-item schema and the survey, renders `backlog.md` with a review digest; `--create --reviewed <digest>` files only that reviewed draft, at most `--limit` issues a run, through the actions log, skipping keys already filed. **Parity**: `--golden cases.json --bin <legacy>` captures each characterisation case's stdout, stderr, and exit code with SHA-256; `--parity cases.json --bin <new>` replays and exits 3 on a difference.
Accept: the index's facts on a mixed-stack fixture (C++ with Make and CMake, PHP, Python) and its determinism; claim, backlog, and parity checks by unit test; pilots on two genuine open-source legacy codebases of different stacks, one C++, in draft mode with filing tried only on a scratch repository deleted afterwards; an advisory tier-2 eval group `survey` where each case scores at least 4.0 of 5 (k = 5, judge gpt-6.1-sol) with tier-1 floors for cited evidence and no invented requirements, within 100 USD of tier-2 spend.

**WP5.10 The Production Engineer seat: running software in production (L; scope addition, founder decisions 2026-10-17; Decision #324; Appendix B S-5.10)** — A fourteenth seat, `production-engineer` ("Production Engineer", twin Atlas), runs what the AI Native Engineer builds: SLOs and error budgets, investigation from telemetry to the deployed code, incident command, runbooks, blameless postmortems, safe rollouts and rollback, and production loops. The AI Native Engineer drops its SRE hat (name "AI Native Engineer"; its SRE practices, habits, and questions move to Atlas; it keeps Observability first and gains Small verifiable checkpoints and Tests before code). The new seat owns L07 Operational data / state, into which `ops/**` moves from L04; the AI Native Engineer owns no layer. It is Accountable for `respond` and three new verbs, all stage "Keep it running": `investigate` (grounding from bounded telemetry queries over sanitised OpenTelemetry exports or read-only connectors, the deployed revision and the diff behind it, and a checked investigation note), `slo` (SLOs from `ops/slos.toml`, budget and burn from telemetry, `--record` for the release gate), and `runbook` (runbooks in `ops/runbooks/` checked against the catalog and SLOs). `respond` gains a checked, blameless `--postmortem` and files its follow-up Bug from the investigation's hand-off. In a repository with SLOs, `release` refuses a stale or spent error budget (only Bugs ship) and a rollback never tried once, and `plan` requires a rollout section. Loops: `alert-triage`, `error-budget-review`, `patch-sweep`. Every mutation stays a catalog action a person approves through a GitHub Environment; telemetry credentials stay in the person's own session, never in the repository or product CI.
Accept: registry lint with fourteen seats and 37 verbs and one owner per layer; both personas load; tests for each verb's grounding, check, and refusal paths; an end-to-end test on `examples/benefits-qa`, instrumented with OpenTelemetry, where a failure injected at a specific revision is investigated from telemetry to the implicated code, mitigated and rolled back through the catalog with a human approval, recorded in the timeline, written up as a postmortem, and closed with a follow-up Bug; tier 1 on two persona cases per new or changed skill with planted failures; tier 2 in the advisory `production` group with a mean of at least 4.0 per skill.

**WP5.4 `slice`, `plan` (M)** — `slice`: XS–L stories with criteria, deps, footprint **computed from the Feature's expected paths via `LAYERS.md` globs and confirmed by the plan**; closes an XL as "split into". `plan`: checkpoints per layer, contracts, tests, instrumentation, rollback; EM plan-time review required for `autonomy.never` layers, recorded in the pack.
Accept: evals on two fixture features (one 3-layer, one 6-layer incl. L07 migration): every produced story is ≤ L and passes `ready` (tier 1, 100 %); each `plan` has exactly one checkpoint per layer in the story's footprint and a rollback step for any L07 checkpoint; tier-2 rubric meets the WP3.4 bar; an XL-sized input feature is returned as ≥ 2 stories with the XL closed "split into".

**WP5.5 `build`, `ship` (L)** — `build <story|bug>`: refuses unless Ready and body hash = snapshot; per-story **lease** (assignee + lock comment with TTL); worktree under `.aayudha/worktrees/<id>` from `origin/dev`; checkpoint commits with `Aayudha-Checkpoint: n/N`; replace-in-place progress comment and checkpoint record on `ledger`; `verify` per checkpoint; `[autonomy] budget {wall, cost, attempts}` → `status:blocked` + page on breach or second failure on a never-layer; `--resume` from the last trailer; retries 429/5xx with jitter. `ship <pr>`: green gate; human pass = approving review from the seat's `human` (solo → `[autonomy] approver`), `type != Bot`; never-layers require CODEOWNER approval regardless of streak; approval ID and streak state written to the pack; squash-merge; Complete section; status `merged`; branch/worktree cleanup. `aayu gc` prunes stale worktrees/leases. The agent writes the code; the binary owns worktree, commits, verify, PR, merge.
Accept: dogfood story Ready → merged; a bot review is rejected as a pass; a stale lease is reclaimed by `gc`.

**WP5.5b Delegation to another agent (L; scope addition, founder request 2026-10-15; Appendix B S-5.5b)** — `build <story> --delegate codex|cursor|claude|amp|auto [--local]` hands a Ready story to another coding agent under `build`'s gates (Ready and body hash, lease, budget, halt). One **delegation brief** for every receiver (Ready snapshot and Completion Goal, plan checkpoints, the evidence map to write, `AGENTS.md` conventions, branch and PR rules with `Refs #<n>`, local checks, stop conditions; never a secret). **Adapters change only the transport**: Cursor's REST API (`autoCreatePR`), Amp orb threads (`amp -ox`), Claude Code's `@claude` mention (claude-code-action; aayu opens the PR from the pushed branch), Codex cloud tasks (`codex cloud exec`; aayu applies the returned diff and opens the PR). **Cloud by default; local only on `--local`** (since WP5.5c; before it, an unconnected cloud agent fell back to the local CLI): the receiver's CLI in `.aayudha/worktrees/<n>-<agent>` with a scrubbed environment and no GitHub credential, then `build`'s checkpoint, verify, push, and PR. Each later run syncs: receiver status, the PR, its `inspect` disposition, a follow-up with the structured findings on `fix` within the cycle cap, "ready for `ship`" on `merge`, a page at the cap. The receiver never gets merge rights; `ship` and its human pass are unchanged. `auto` picks the cheapest adequate connected receiver for the story's Effort and says why. Hand-offs and follow-ups are actions; state is `delegate/<n>/state.json` on the ledger. Slash commands `/codex`, `/cursor`, `/amp`, `/claude` ship through the host plugins. Environments come from one source (`.agents/setup`): `.cursor/environment.json` generated, the Codex Install script one constant line, `doctor` checks drift per receiver. `polish` prepares a worktree instead of using the current checkout; `status --work` lists active worktrees with agent, story, PR, and age; `gc` cleans delegated worktrees.
Accept: a non-Ready story is refused (exit 4) for every receiver; on recorded fixtures, each adapter hands off, finds or opens the PR with `Refs #n`, and sends one follow-up on an `inspect` `fix`; the third cycle pages without a follow-up; the brief holds no secret (canary test); the local fallback's child environment has no `GITHUB_*`, `GH_*`, `AAYU_*`, or token variable; live tests skip cleanly without `AAYU_LIVE=1` and credentials; `doctor` reports a drifted `.cursor/environment.json`.

**WP5.5c Call any agent from any host (M; scope addition, founder request 2026-10-16; Appendix B S-5.5c)** — Every pairing works from inside the plugin: Claude Code calls Codex, Cursor, or Amp; Codex calls Claude Code, Cursor, or Amp; Cursor calls Codex, Claude Code, or Amp; Amp calls any of the three. **The commands load in every host's plugin** (Claude Code, Codex, Cursor, Amp), and no host offers a hand-off to itself. The pre-tool hook still allows the hand-off's calls. **Only a Ready story leaves:** a goal in words goes through grill's vagueness gate (unpack and confirm), then `ready`. **Cloud by default:** the receiver's cloud agent, after its environment is checked or set up (the agent-environments work owns that); its CLI or headless mode (`codex exec`, `cursor-agent -p`, `claude -p`, `amp -x`) only on the user's explicit `--local`. Both modes are supported first-party. A receiver that is neither installed nor connected is named plainly, with its exact install and sign-in steps and any connected alternative. **No AI keys in a product repository's CI:** delegation runs only through the user's own signed-in CLIs and apps, never gateway credentials; Claude Code's cloud route is `claude --cloud`, not claude-code-action. **Every hand-off is fresh:** a new aayu worktree (local) or a new branch (cloud). The brief tells the receiver to break the work down, use its own subagents, verify each Completion Goal criterion with evidence before it reports, and return what it proved. The calling host reports that summary as a preview: the merge gate's proof decides.
Accept: `TestHostPluginsLoad` passes on every host CLI on PATH, each listing the other agents' hand-offs and not its own; `TestPluginManifests` holds the Claude Code, Codex, and Cursor manifests to the same rule; the hook allows the hand-off calls on every host; a missing receiver's error names its install and sign-in steps; every pairing that can run headlessly in an orb is run live, and the rest are named as proven by golden files only.

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

**WP7.1b Native scheduled and event loops (M; scope addition, founder decisions 2026-10-16; Appendix B S-7.1b to S-7.1e)** — A loop runs on a schedule (`cron`, any five-field cron in `[time] tz`) or on a GitHub event (`on` plus a `filter`), set up in each agent host's own tooling: Amp, Cursor, Claude Code, and Codex, per repository, as the user, so no model key reaches CI. The Actions fallback stays deterministic: an event loop's job only records the event. New default loops: weekly triage and triage on arrival (replacing the retired triage workflow), weekly housekeeping, and event loops for Feedback, a failed check suite, and a published release. One entry point, `aayu install --loops`, detects the user's hosts (agentenv) and sets the loops up natively where the host allows, with the exact steps for the rest; `aayu doctor` reports each loop per host as active, stale, or missing. **Narrowed for 0.2 (founder, 2026-10-16; S-7.1e):** 0.2 ships scheduled loops only, set up in the one agent tool the user picks; event loops are deferred to a later version, with D-7.1e's research and host × event matrix as their basis.
Accept: `on` and `filter` are validated; golden renders for all four hosts and Actions from one fixture; each host's native form, or its exact steps where only a web page exists, citing the host's documentation; the guard (`aayu hook loop:<name>`) starts every run, scheduled or event; `install --loops` and `doctor` report active, stale, and missing; one Amp schedule and one Amp GitHub subscription live on this repository, and all four hosts rendered for this repository and for a product repository.

**WP7.2 `measure` (M)** — Extends WP5.2 adapters with GrowthBook, billing CSV, App Insights/AWS/GCP exports; SQLite (`modernc.org/sqlite`) snapshots, DuckDB optional shell-out; AARRR/North Star queries; Results on Goal/Initiative; triggers parent-close rule.
Accept: fixture exports → expected cohorts.

**WP7.2b Cloud integrations (scope addition, founder request 2026-10-12; Appendix B S-7.2b)** — First-class Azure, AWS, and GCP alongside PostHog, Sentry, and GrowthBook, so a product on any of the three clouds gets live baselines, live `measure` pulls, a V3 media bucket, and a guided least-privilege setup. Standard-library HTTP only (no cloud SDK: rule 2 and the binary size budget); every response is untrusted input (§0.1); every credential is read-only where the data plane is concerned (WP1.3).
**WP7.2b.1 Cloud read adapters (M)** — `[integrations.azure|aws|gcp]` with `auth = "key"|"oidc"|"cli"`. Azure: Application Insights through the Log Analytics query API (KQL), Microsoft Entra token by client credentials, GitHub Actions workload identity federation, or `az account get-access-token`. AWS: CloudWatch Logs Insights over the RUM app monitor's log group for baselines and CloudWatch RUM `GetAppMonitorData` for event pulls, SigV4 with the WP4.2 signer, keys from the secret, `AssumeRoleWithWebIdentity` from a GitHub OIDC token, or `aws configure export-credentials`. GCP: the GA4 BigQuery export through `jobs.query`, a service account key (signed JWT), workload identity federation (STS, then impersonation), or `gcloud auth print-access-token`. Each returns the WP5.2 tuple for `grill` and `measure <source>:<metric>`, and `measure --live` pulls the export readers' event shapes so cohorts are computed once.
Accept: recorded fixtures replay request by request into golden tuples, with every token exchange (Entra, Google JWT and STS, AWS STS, GitHub OIDC) served by an httptest server and every AWS signature recomputed; a live pull gives the same cohorts as the export files; a write-capable credential is refused before any data request; `AAYU_LIVE=1` tests skip cleanly without credentials.
**WP7.2b.2 Cloud media buckets (S)** — `[proof] media` also takes `azblob://` (Azure Blob Storage, SAS or Entra token) and `gs://` (Google Cloud Storage JSON API with a token; the HMAC XML API is not built, D-7.2b.2a), with WP4.2's push, get, verify, purge, tombstones, and per-repository lifecycle rule.
Accept: the WP4.2 push/verify/tamper/purge test passes against fake Azure Blob and GCS servers that check every request's authorization; lifecycle rules merge with the bucket's existing rules.
**WP7.2b.3 Cloud setup and docs (S)** — `init --integrations` offers azure, aws, and gcp with a cost/scope summary and `y/N`, writes `[integrations.<cloud>]`, and writes and prints `.aayudha/setup/<cloud>/` (least-privilege role assignments or IAM policy JSON, and the GitHub Actions OIDC federation steps); `doctor` verifies reachability and read-only scope where the provider can report it; a getting-started page per cloud on the `tell docs --site` pipeline.
Accept: init golden for each cloud (files and config section), idempotent on rerun; doctor refuses a write-capable cloud credential; the site build carries the three pages.

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
WP count 78 (XS 1, S 21, M 48, L 8; WP7.2b.1–3, WP5.3b, WP1.1b, WP3.2b, WP4.5b.1–5, WP5.5c, WP5.8, WP3.2c, WP7.1b, WP5.1g, and WP5.9 added as scope additions, Appendix B S-7.2b, S-5.3b, S-1.1b, S-3.2b, S-4.5b, S-5.5c, S-5.8, S-3.2c, S-7.1b, S-5.1g, and S-5.9)) plus the M4.5 run, which is not a WP. No XL. WP4.9 counts as M for its 0.2 scope; its 0.1 part (skip-and-block) is XS and ships with WP4.8.

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

The table below is v0.3's review. The deviations and decisions recorded from PR1 to 2026-10-17 are archived (the
end of this appendix says more).

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

### The decision log

The deviations and decisions recorded while implementing this plan, from PR1 up to 2026-10-17, are archived in a
private repository. Earlier IDs cited in code and docs, such as D-5.5c.24 or S-4.5b, refer to that archive. Since
then, a deviation from the plan is recorded in its pull request's description under "Deviation from the plan:", which
the squash commit message keeps, and real decisions are aayu Decision issues. No new rows are added here.
