# Threat model

This expands [BUILD-PLAN.md §0.1](BUILD-PLAN.md#01-trust-model) (WP0.5). It lists what `aayudha` protects, who
acts on it, where trust changes, and which control answers each threat. The status column says whether a
control exists today or which work package builds it ("n/a until", rule 1a). Report a weakness privately as
described in [SECURITY.md](../SECURITY.md).

## Assets

| Asset | Why it matters |
|---|---|
| Product source on `dev`, `rolling`, `main` | What ships to users. |
| Proof packs and Ready snapshots on `ledger` | The evidence that work is done; forging them defeats every gate. |
| Protected paths: `.github/`, `skills/`, `ops/`, `seats/`, `CONSTRAINTS.md`, `.aayudha/config.toml`, `.agents/`, `.claude/`, `internal/config/defaults.toml` | They define the gates, the floors, the agents' instructions, and the operations catalog. |
| Human tokens, the bot App private key, model and integration keys | Each can write somewhere an agent should not. |
| Release artifacts and the install path | A tampered release runs on every user's machine. |
| Proof media (screenshots, recordings, transcripts) | May contain personal data; LFS objects cannot be purged from GitHub. |
| Budgets | Model and CI spend. |

## Principals

| Principal | Identity | May |
|---|---|---|
| Human (founder, paired human) | Own GitHub account and token | Approve PRs, accept Decisions, publish, run `init`, change protected paths, promote |
| Agent lane (any host) | Bot: GitHub App installation token or machine user (n/a until WP1.4; until then the human's identity, tagged `identity:human-fallback`) | Open issues and PRs, push branches, push `ledger`, comment |
| CI (merge gate, loops, release) | GitHub Actions; the bot App token where it writes `ledger` (Appendix B D1) | Run `verify` and `inspect`, write authoritative packs, compute streaks, build and sign releases |
| Untrusted input | — | Issue and PR bodies, comments, intake forms, Figma text, Mobbin results, PostHog and Sentry exports, dependency and tool output |

## Trust boundaries

```text
 untrusted text ──wrap──▶ host model ──skill──▶ aayu verb ──internal/actions──▶ GitHub (bot identity)
                                                   │                               │
                                       scrubbed child env                rulesets + CODEOWNERS
                                       (floors, tools)                             │
                                                                       CI on the base branch
                                                                       (merge gate, release.yml)
```

1. **Text → model.** Anything a model reads from outside is data, wrapped in delimited provenance blocks.
2. **Model → binary.** A host model can only ask for verbs; the binary decides what is allowed (tool
   allowlists, MCP exposure, capabilities such as labels-only).
3. **Binary → child processes.** Floors and tools run with a scrubbed environment and, where feasible, no
   network.
4. **Agent → repository.** Rulesets, CODEOWNERS, and the protected-paths gate decide what lands; workflows that
   judge a PR run from the base branch, so a PR cannot weaken its own gate.
5. **Repository → user machine.** Only artifacts signed by `release.yml` on a `v*` tag that is on `main` are
   installable.

## Threats and controls

| # | Threat | Control | Status |
|---|---|---|---|
| T1 | An agent approves its own work, or a bot review counts as a human pass | Approvals count only from `type != Bot` accounts not in `[identity] bot_logins`; agents write as a bot identity | Partly: GitHub refuses self-approval today; identity checks n/a until WP1.4 |
| T2 | A PR changes the gates, skills, or thresholds that judge it | CODEOWNERS on protected paths; `protected-paths` check runs from the base branch (`pull_request_target`, never checks out PR code); solo-mode merges only through the recorded admin bypass (Appendix B D2) | In place (WP0.1) |
| T3 | Defaults are loosened quietly | `CONSTRAINTS.md` is generated from `internal/config/defaults.toml`; both are protected paths; `generate-drift` fails on any mismatch | In place (WP0.5) |
| T4 | An agent forges or edits a proof pack | Authoritative packs are written by the base-branch merge gate on the PR head SHA; `ledger` writes restricted by ruleset (Appendix B D1) | Ruleset in place; merge gate n/a until WP4.8 |
| T5 | An agent games the autonomy streak | Streak computed by a protected workflow from merged-PR history; corrections detected automatically; never-layers always need a human | n/a until WP5.7 |
| T6 | Prompt injection makes an agent write, ship, publish, or run an ops action | Provenance wrapping; untrusted text cannot trigger `respond`, `ship`, `tell publish`, or catalog execution; tool allowlists; labels-only capability for `triage`; injection fixtures in evals | n/a until WP1.5, WP2.9, WP3.4, WP4.8 |
| T7 | A token leaks to a floor, tool, or dependency | Scrubbed child environment; no tokens in files; App key only in repository secrets, laptops get ≤ 1 h tokens from the broker | n/a until WP1.3, WP1.4, WP4.3 |
| T8 | An over-scoped human token does damage through an agent | `doctor` warns on `workflow`, `admin:*`, `delete_repo` scopes; `init` is the only admin-scope verb and needs a TTY confirmation | n/a until WP1.4, WP1.7 |
| T9 | A compromised GitHub Action or CI tool | Every action pinned by full commit SHA; downloaded tools (gitleaks, osv-scanner, cosign in `install.sh`) checked against pinned SHA-256; workflows default to `contents: read`; checkout never persists credentials | In place (WP0.4) |
| T10 | A tampered or substituted release | goreleaser builds in `release.yml` only for an annotated `v*` tag whose commit is on `main` (`verify-tag`, Appendix B D4); `checksums.txt` signed keylessly with cosign; SLSA build provenance; SBOM per archive; `install.sh` verifies the signature against `release.yml@refs/tags/<exact tag>` and then the archive checksum; CI proves it refuses a flipped byte, a rewritten checksum, and a non-release signer | In place (WP0.4); `self-update` n/a until WP1.1 |
| T11 | A package-manager manifest is hijacked | brew, Scoop, and winget publishing run in separate jobs, each with a token scoped to its own GitHub environment; the build job never holds them | In place (WP0.4); tokens are a founder action |
| T12 | A malicious dependency or licence problem | `osv-scanner`, `go-licenses`, npm licence scan, `gitleaks`, `semgrep` in CI; new dependencies recorded in THIRD_PARTY_NOTICES.md | In place (WP0.4) |
| T13 | A malicious or compromised skill | Skills are protected paths; installed copies are hashed in `lock.toml` and checked by `doctor`; `install --pin` rolls back; hooks call only `aayu hook` | Protected path in place; lock and hook lint n/a until WP3.1, WP3.2 |
| T14 | Personal data in proof media on a public repository | Public repos refuse LFS media and require a private bucket; captures use seeded data; redaction pass blocks the pack; retention and purge | `init` and `proof push` refuse LFS media on public repositories, bucket lifecycle and `proof purge` (WP2.5, WP4.2); `[verify] seed`, refusal of production targets, and the redaction pass that blocks the pack (WP4.5) |
| T15 | Unlogged or unattributed writes | Every external write goes through `internal/actions`; `audit` reconciles it against GitHub's event history | n/a until WP1.5, WP7.6; hand-run reconciliation until then |
| T16 | Runaway model or CI spend | `[budget]` breaker pauses agent lanes and pages once a day; per-PR inspect cap; expensive floors only where the footprint needs them | n/a until WP1.6 |
| T17 | A promotion ships a regression without a human watching | Promotions need no human approval (founder policy, CONTRIBUTING.md), so the gate is the control: the candidate merge commit must pass the full rolling or stable suite before the branch moves, and only the admin identity can move `rolling`, `main`, or create `v*` tags | In place (WP0.4) |
| T18 | The tool itself is compromised | Tool-incident plan: halt, yank the release, pull bad skills, rotate credentials ([SECURITY.md](../SECURITY.md#tool-incident-plan)) | Plan in place; `aayu halt` (WP5.7) |

## Residual risks

- **One human.** Until a second human seat exists, protected-path changes are reviewed by nobody but the author;
  the control is that each such merge is an explicit, recorded bypass (Appendix B D2).
- **Admin identity is powerful.** The founder's token can bypass the promotion and tag rulesets. Agents that
  run with it (today, the human-fallback path) carry that power; WP1.4 moves agents to a bot identity that
  cannot bypass.
- **Keyless signing trusts GitHub and Sigstore.** A compromise of GitHub Actions OIDC or the Sigstore public
  good instance could mint a valid-looking signature. Provenance attestations give a second, independent check
  (`gh attestation verify`).
- **GitHub event history retention.** `audit` can only reconcile what GitHub still returns; older events are
  out of reach.
