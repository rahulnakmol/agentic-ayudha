# Cross-agent delegation and review of agent-authored PRs

Research for WP4.8b (review quality) and WP5.5b (delegation), 2026-10-15. Owner: the AI Native Engineer and
Engineering Manager twins. Status: research done; the recommendation in §8 is what 0.2 builds.

The founder's request (voice note, transcription corrected): from Claude Code or Amp, locally or in the cloud, hand a
well-defined piece of work to another agent (the Codex cloud agent, the Cursor cloud agent, Claude Code, Amp). It works
in its own branch and opens a PR against the same repository. The originating agent reviews that PR, gives feedback,
and closes the loop. Borrow from Alibaba's open-source review skill and from Greptile's review architecture, for both
the adversarial review (`inspect`) and general review of AI-written work: automatic evaluation plus a peer review. A
follow-up widened it: slash commands (`/codex`, `/cursor`, `/amp`, `/claude`) from the daily-driver host, one
delegation brief for every receiver, generated cloud environments kept in sync with one source, cloud by default
with a local CLI fallback in an aayu worktree, worktrees for every code-changing verb with a view of parallel work,
and the cheapest adequate receiver per story size.

Transcription checks: "cloud code" is Claude Code; "Khazar" is Cursor (confirmed by the founder); "court review" is
code review; "cryptile" is Greptile (the only code-review product of that sound; its published architecture matches
the request).

Labels used below: **[V]** checked in a primary source (official docs, the repository's own files, or the CLI's
`--help`) on 2026-10-15; **[I]** inferred, or from a third party. Vendor benchmark numbers are the vendor's own.

## 1. Summary and recommendation

- **The Alibaba skill is `alibaba/open-code-review` (OCR)**, Apache-2.0, a Go CLI plus a thin `SKILL.md`. Its
  ideas that matter here: findings anchored by a quoted snippet rather than a model-chosen line, a deterministic
  plan of what to review, rule checklists per language that tell the model not to repeat what linters find, and an
  **asymmetric "prove it wrong" filter pass** that drops only findings the diff contradicts. We adopt the ideas and
  write our own prompts; no text is copied.
- **Greptile publishes ideas, not code.** The ones that matter: retrieve the context of the changed symbols (callers,
  tests) instead of sending only the diff; suppress comments that resemble ones the team dismissed; measure
  "addressed rate", not volume; and **route review to a different model family from the author** ("model inversion",
  which their data supports and which `inspect` already enforces).
- **Every published reviewer converged on precision first**, with a separate verify or filter stage after
  generation (OpenAI Codex review, OCR, Greptile v3, Cursor Bugbot, Anthropic's code-review plugin). Self-rated
  confidence scores did poorly on their own (Greptile: "nearly random"; Anthropic removed its 0–100 score). So
  confidence is one weak gate in our pipeline, and the real filter is independent confirmation: deterministic
  anchoring, de-duplication, learned suppressions, and, in peer mode, a second family cross-checking the first.
- **Delegation fits `build`, not a new verb.** The Ready gate, lease, budget, halt, worktree, branch and PR rules,
  checkpoint record, and RACI are `build`'s; a separate `delegate` verb would duplicate all of them. `aayu build <n>
  --delegate codex|cursor|claude|amp|auto [--local]` (Appendix B D-5.5b.1), surfaced by four slash commands.
- **Transports differ, the brief does not.** Cursor has an official REST API; Amp has `amp -ox` (orb threads);
  Claude Code has the `@claude` GitHub mention (claude-code-action); Codex cloud has only the experimental
  `codex cloud exec` and a "Create PR" click, so its adapter returns the diff and aayu opens the PR. In every case
  **aayu, not the receiver, owns the PR's merge**: the receiver never gets merge rights, and the PR must say
  `Refs #<story>` so the merge gate, `inspect`, and `ship` find the story.
- **Cloud by default, local fallback** (`--local`, or when the receiver is not connected): the receiver's CLI
  (`codex exec`, `cursor-agent -p`, `claude -p`, `amp -x`) runs in `.aayudha/worktrees/<n>-<agent>` with a
  scrubbed environment and no GitHub token; aayu commits, verifies, pushes, and opens the PR.
- **Environments come from one source**, the repository's `.agents/setup` (pinned, idempotent, checksum-verified
  toolchain): Amp runs it natively, Cursor's `.cursor/environment.json` is generated to call it, Codex's Install
  script field is one constant line that calls it, and `doctor` reports drift per receiver.

## 2. The "Alibaba open review skill": `alibaba/open-code-review`

### 2.1 Identification

| Candidate | Evidence | Verdict |
|---|---|---|
| **alibaba/open-code-review** (`ocr` CLI and the `open-code-review` skill) | <https://github.com/alibaba/open-code-review>, created 2026-05-18; skill at `skills/open-code-review/SKILL.md`, installable with `npx skills add alibaba/open-code-review --skill open-code-review` [V]; InfoQ, 2026-09-20: <https://www.infoq.com/news/2026/09/alibaba-opencodereview/> [V] | **The one.** The only Alibaba-organisation project that is both "open code review" by name and a skill, and it was in the news weeks before the request |
| Qwen Code's bundled `/review` skill (QwenLM, Apache-2.0) | `packages/core/src/skills/bundled/review/SKILL.md` in <https://github.com/QwenLM/qwen-code> [V] | Related (Alibaba's Qwen team), but "Qwen Code's review", not "Alibaba's open review skill". Its de-duplication and verification rules are noted below |
| alibaba/aacr-bench (Apache-2.0) | <https://github.com/alibaba/aacr-bench>, paper <https://arxiv.org/abs/2601.19494> [V] | OCR's benchmark, not a reviewer |

No separate Ant Group, ModelScope, or Tongyi Lingma open-source review skill was found. Certainty: high.

### 2.2 Architecture [V]

OCR grew out of Alibaba's internal reviewer and calls itself a "Deterministic Engineering × Agent Hybrid". The
skill is a wrapper that runs `ocr review --audience agent -b "<business context>"`; the logic is in the binary
(paths at commit `ea5ced2`):

1. **Deterministic file selection** with include and exclude rules (`rule.json`).
2. **Grouping** of related files (module, producer and consumer, i18n variants) into groups of at most 10, only when
   at least 4 files changed; each group is reviewed by a sub-agent with its own context, 8 groups in parallel.
3. **Rule checklists per language** (`internal/config/rules/rule_docs/*.md`), injected by a template engine. The Go
   checklist says: "Favor precision over recall… Do not duplicate findings that `go vet`, Staticcheck, `go test
   -race`, the compiler, or `gofmt` can determine reliably."
4. **A plan pass** for diffs over 50 lines (100 per group): a ranked risk list, "Do not invent issues to fill the
   list."
5. **A tool-using review loop** (`code_search`, `file_read`, `file_read_diff`, `file_find`, `code_comment`,
   `task_done`), capped at 100 tool calls and two rounds; round 2 drops the plan and receives the confirmed findings.
6. **Findings** carry `content`, `existing_code` (a quoted snippet), `suggestion_code`, `category` (bug, security,
   performance, maintainability, test, style, documentation, other), `severity` (critical, high, medium, low), and
   `path`. The model gives no line number: a sliding-window matcher maps the snippet to lines, and a separate model
   call re-locates a snippet that does not match. Anything still unmatched is emitted without lines.
7. **The review-filter pass**, deliberately asymmetric: remove "only the comments that this diff **proves** to be
   factually wrong… Removing a correct comment silently destroys a real finding." Two grounds only: the target code
   is not in the file's diff, or a diff line literally contradicts the comment. Protected subjects are never
   removed (memory safety, concurrency, linkage, behavioural or compatibility change, unused parameter).
8. **Ranking** is by severity; there is no numeric confidence. The skill tells the host to drop `low` items that look
   like false positives or nits.

Reported results (OCR's README, on AACR-Bench: 200 real PRs, 50 repositories, 10 languages, 1,505 expert-annotated
issues; vendor numbers on the vendor's benchmark): with Claude 4.6 Opus, OCR scored precision 33.9 %, recall 20.0 %,
F1 25.1 % against stock Claude Code's 7.2 %, 28.9 %, 11.6 %, at about a fifteenth of the tokens. OCR trades recall
for precision on purpose.

**Qwen Code's `/review`** (Apache-2.0) adds two rules worth keeping: merge duplicates on the same file, line range,
and root cause and keep the **highest** severity; and "an uncertain suggestion is dropped, an uncertain critical is
reported with low confidence for the verifier to decide".

### 2.3 Licence, what we adopt, what we must not copy

- Licence: Apache-2.0 (SPDX `Apache-2.0`), <https://github.com/alibaba/open-code-review/blob/main/LICENSE>; the
  skill's frontmatter says the same. Compatible with our BSD-3-Clause, but copied text keeps Apache terms (licence
  text, NOTICE, marked modifications) and would need a `THIRD_PARTY_NOTICES.md` row.
- **Adopt (ideas, credited here)**: snippet anchoring with a deterministic resolver; the asymmetric fact-check with
  protected categories; a deterministic plan per changed area; telling the model which deterministic checks
  already ran; keep-highest-severity de-duplication; precision, recall, and F1 against planted defects as the
  measure.
- **Not copied**: no prompt, rule document, or code from OCR or Qwen Code is pasted into aayudha; our prompts are
  written from the ideas. No "Alibaba", "Qwen", or "OpenCodeReview" in product naming. Not adopted: OCR's
  tool-using loop (100 tool calls per group); `inspect` stays one structured call per reviewer, with context aayu
  retrieves deterministically, which keeps cost bounded and the model away from tools that untrusted PR text could
  steer (§0.1).

## 3. Greptile's review architecture (proprietary SaaS)

All points are from Greptile's own pages [V]; dates as published.

| Topic | What Greptile says | Source |
|---|---|---|
| Codebase graph | Every file is parsed into a graph of directories, files, functions, classes, and variables, with call, import, and use edges; a review asks for a changed function's callers, dependencies, and similar patterns | <https://www.greptile.com/docs/how-greptile-works/graph-based-codebase-context.md> (undated) |
| Retrieval | Natural-language descriptions of code retrieve better than raw code (query similarity 0.815 vs 0.728); chunk per function | <https://www.greptile.com/blog/semantic-codebase-search> (2025-04-15) |
| Agentic review (v3) | A search loop with many hops; upvote/downvote ratio 1.44 → 5.13 and action rate 34.75 % → 59.24 % against v2, credited to a higher bar of "sureness" | <https://www.greptile.com/blog/greptile-v3-agentic-code-review> (2025-11-26) |
| v4 | Addressed comments per PR 0.92 → 1.60; share addressed 30 % → 43 % (A/B against v3) | <https://www.greptile.com/blog/greptile-v4> (2026-03-05) |
| Noise | Prompting with examples failed; an LLM judge rating severity 1–10 was "nearly random"; what worked was embedding past comments per team and blocking a new comment close to at least 3 distinct downvoted ones; address rate 19 % → 55 %+ in two weeks (threshold not published) | <https://www.greptile.com/blog/make-llms-shut-up> (2024-12-18) |
| Learning | 👍/👎 reactions, replies, teammates' comments, and whether a comment was addressed between the first and last commit; inferred rules | <https://www.greptile.com/docs/how-greptile-works/memory-and-learning.md> |
| Strictness and rules | Strictness 1–3 (high: never comment on P2s); comment types logic, syntax, style; ignore patterns; cascading `.greptile/config.json` and `rules.md` | <https://www.greptile.com/docs/code-review/controlling-nitpickiness>, <https://www.greptile.com/docs/code-review/greptile-config> |
| Confidence | A 0–5 score per **PR** (not per comment), correlated with time to merge | <https://www.greptile.com/blog/ai-code-review> (2025-06-16) |
| Model inversion | Each model catches the other's bugs better (500 Claude Code and 500 Codex PRs, about 1,500 bugs); authorship detected from `Co-authored-by` trailers, `[codex]` titles, and `codex/` branches | <https://www.greptile.com/blog/model-inversion> (2026-07-21) |
| Building a reviewer | Sandboxed agent, a codebase wiki that records reverts and incidents, cheap models screening reads | <https://www.greptile.com/blog/how-to-build-a-code-review-agent> (2026-10-09) |
| Evaluation | 50 real bug-fix PRs with each bug re-applied; catch rate only, false positives not scored | <https://www.greptile.com/benchmarks> (July 2025) |

**Adopt (ideas)**: context of the changed symbols (callers, related tests) rather than the diff alone; suppression
of findings that resemble dismissed ones, per repository; addressed rate as an online measure; cross-family review;
a per-PR summary score. **Not adopted in 0.2**: embeddings and a vector store (we have no vector store and a token
similarity over a short fingerprint does the job at our volume); a whole-repository graph (we index only the
changed symbols, their package, and their tests); an agentic loop. **Not copied**: code, prompts, config schema, or
the Greptile marks.

## 4. Other published designs (brief)

- **OpenAI Codex review** (<https://alignment.openai.com/scaling-code-verification/>, 2025-12-01) [V]: accepts
  "modestly reduced recall" for precision; utility = P(correct)·C_saved − C_verify − P(incorrect)·C_false_alarm;
  in GitHub it flags only P0 and P1 issues.
- **Cursor Bugbot** (<https://cursor.com/blog/building-bugbot>, 2026-01-15) [V]: eight passes over a shuffled
  diff, majority vote, a validator model, de-duplication against earlier runs; "resolution rate" judged at merge
  rose from 52 % to over 70 %.
- **Anthropic's code-review plugin**
  (<https://github.com/anthropics/claude-code/blob/main/plugins/code-review/commands/code-review.md>) [V]: parallel
  reviewers, then one validation sub-agent per issue; the earlier 0–100 confidence score with an 80 threshold was
  replaced by validators on 2025-12-06 (commit `9babb8dbbf`). Licence "All rights reserved": ideas only.
- **CodeRabbit** (context-engineering posts; dates not confirmed) [V]: code graph, linked issues, learnings, linters
  in a sandbox, verification agents.
- **PR-Agent** (<https://github.com/The-PR-Agent/pr-agent>) [V]: now MIT (AGPL-3.0 between 2025-05-22 and
  2026-05-20); asks for "issues you are confident about" and a 1–5 effort estimate.

## 5. Delegation mechanisms per receiver

Pages fetched 2026-10-15. CLIs checked with `--help` in a temporary HOME (codex-cli 0.162.1, Claude Code 2.1.300,
Amp CLI 0.0.1792022479, cursor-agent 2026.10.14).

### 5.1 OpenAI Codex

- **Trigger.** `@codex review` and `@codex <task>` in a **PR** comment are documented; "@codex fix the P1 issue" can
  push a fix to the PR branch "when it has permission" [V]. `@codex` on an **issue** opening a PR is not documented
  (the overview lists issues as a source without syntax) [unverified]. The GitHub integration runs on "Codex Cloud
  (Legacy)", which OpenAI "plan[s] to deprecate" [V].
- **Programmatic.** `codex cloud exec --env <ENV_ID> [--branch <BRANCH>] [--attempts N] "<prompt>"`, with `status`,
  `list`, `diff`, and `apply` subcommands, marked `[EXPERIMENTAL]` [V]. No public REST API for cloud tasks [V by
  absence]. `openai/codex-action` runs `codex exec` on your own runner and does not open PRs [V].
- **PR.** A cloud task ends in a diff: "You can open a PR or ask follow-up questions" (a user click) [V]. So the
  adapter returns the diff (`codex cloud diff`), and aayu applies it in its worktree and opens the PR [I: design].
- **Auth.** ChatGPT plan, the ChatGPT Codex Connector GitHub App, and a cloud environment ID [V].
- **Cost.** Included in ChatGPT plans (shared allowance; "cloud tasks may use more of your allowance"); "standard
  Codex Cloud environments have no separate VM charge" [V].
- **Limits.** 2 vCPU / 8 GiB (Plus) or 4 vCPU / 16 GiB (Pro and up); task state kept 7 days; GitHub.com only [V].
- **Uncertainty: high** (issue trigger, branch prefix, PR author, the deprecation date).

### 5.2 Cursor cloud agents (formerly background agents)

- **Trigger: the official API** (public beta) [V]: `POST https://api.cursor.com/v1/agents` with
  `{"prompt":{"text":…},"repos":[{"url":…,"startingRef":"dev"}],"autoCreatePR":true,"model":{"id":…}}`, returning
  `{agent:{id,status,url}, run:{id,status}}`. Status: `GET /v1/agents/{id}` and
  `GET /v1/agents/{id}/runs/{runId}` (`FINISHED|ERROR|CANCELLED|EXPIRED`, with `git.branches[] {branch, prUrl}`).
  Follow-up: `POST /v1/agents/{id}/runs {"prompt":{"text":…}}`. Webhooks are "coming soon" in v1 (legacy v0 has
  them, HMAC-signed). Auth `Authorization: Bearer <key>` (dashboard API key or service account).
- **Other triggers** [V]: `@cursor` on a PR or issue, Slack, Linear, Automations, Bugbot.
- **PR.** Auto-created with `autoCreatePR`; branches `cursor/…`; opened as the user by default (v0 docs), so the
  author is not a reliable signal [V/I].
- **Review loop.** A follow-up run, or a prompt asking it to "respond to review comments", which subscribes the
  agent to the PR's comments and reviews [V].
- **Cost.** "Charged at API pricing for the selected model"; a spend limit is required [V].
- **Limits.** 20 requests per minute; at most 10 automatic CI follow-ups per PR [V].
- **Uncertainty: medium** (v1 PR author, v1 webhooks, beta status).

### 5.3 Claude Code

- **Trigger.** `anthropics/claude-code-action` (`@v1`): `@claude <task>` in an issue or PR comment, plus
  assignee, label, and custom trigger phrases; automation mode with `prompt:` [V]. On an issue it "always creates a
  new branch"; on an open PR it "pushes directly to the existing PR branch" [V].
- **PR.** "Claude doesn't create PRs by default… provides a link to a pre-filled PR submission page" [V]. Branch
  prefix `claude/` [V]. So aayu finds the pushed branch and opens the PR [I: design].
- **Cloud sessions.** `claude --cloud "<task>"`; Auto-fix watches a PR for CI failures and review comments [V]. No
  public REST API for sessions [V by absence].
- **Auth.** The Claude GitHub App, an `ANTHROPIC_API_KEY` or `CLAUDE_CODE_OAUTH_TOKEN` secret, and the workflow [V].
  Gotcha: comments written with the workflow's `GITHUB_TOKEN` do not trigger workflows, so aayu's hand-off comment
  must come from the human or bot token (it does) [V].
- **Cost.** API tokens or plan usage, plus Actions minutes [V].
- **Limits.** Only users with write access trigger it; it cannot approve PRs [V].
- **Uncertainty: low** (issue branch name format).
- **Superseded (2026-10-16, WP5.5c).** The founder's rule is no AI key in a product repository's CI, and
  claude-code-action needs one. aayu now starts Claude Code on the web from the user's signed-in CLI:
  `claude --cloud "<brief>"`, run in the repository; follow-ups go through `claude -p "<msg>" --cloud <session>
  --output-format json`, which answers `{ok, session_id, url}`. Cloud sessions use the claude.ai subscription and
  ignore `ANTHROPIC_API_KEY`. A session pushes its branch and does not open a PR from the CLI, so aayu opens it
  (code.claude.com/docs/en/claude-code-on-the-web and /authentication, read 2026-10-16) [V; the new-session
  output format is not documented: I].

### 5.4 Amp

- **Trigger.** `amp -ox "<brief>" --project <owner/repo> [--orb-size a1.small] [--title …]` starts an orb thread and
  prints its URL; stdin is accepted; `amp threads continue <T-…> -ox "<follow-up>"` continues it [V]. The SDK
  (`execute({prompt, options:{executor:'orb'}})`) needs `AMP_API_KEY` [V]. No `@amp` mention [V by absence].
- **PR.** Pushes and PRs appear under the user's account [V]. The default Ship flow pushes to the base branch,
  which this repository forbids, so the brief says to open a PR to `dev` [V].
- **Review loop.** PR subscriptions deliver reviews and inline comments to the thread; or a follow-up [V].
- **Cost.** Credits at provider API price (no markup for non-enterprise), orbs per minute (`a1.small` $0.17/h,
  `a1.medium` $0.33/h); paused orbs are free [V].
- **Limits.** A burst of 20 orbs, then one per 5 minutes [V].
- **Uncertainty: low.**

### 5.5 Comparison

| | Codex | Cursor | Claude Code | Amp |
|---|---|---|---|---|
| aayu's cloud transport | `codex cloud exec` (experimental), then `diff` | REST `POST /v1/agents` | `@claude` comment on the story (claude-code-action) | `amp -ox --project` (orb) |
| Who opens the PR | aayu, from the returned diff | the agent (`autoCreatePR`) | aayu, from the pushed `claude/` branch | the agent, as the brief says |
| Status | `codex cloud status` | `GET …/runs/{id}` | the action's sticky comment, the branch | thread URL |
| Review follow-up | a new task on the PR branch, diff applied by aayu | follow-up run | `@claude` on the PR | `amp threads continue` |
| Founder connects | ChatGPT plan, Codex Connector app, an environment ID | Cursor GitHub App, API key, spend limit | Claude GitHub App, key or OAuth secret, workflow | Amp GitHub App, project, `AMP_API_KEY` |
| Marginal cost | plan allowance | model API price | API tokens or plan, Actions minutes | API-price credits, orb minutes |
| Local fallback | `codex exec` | `cursor-agent -p` | `claude -p` | `amp -x` |

**Design consequences.** The PR is found by `Refs #<story>` in a PR body (all four can be told to write it), or by
the receiver's branch for the two that do not open PRs; never by author login, because Cursor and Amp PRs are opened
as the human. The receiver's review comments are not trusted to steer anything (§0.1); aayu's own follow-up carries
the structured findings.

## 6. Environments for cloud agents

### 6.1 How each platform defines one

| Platform | Definition | Cached state | Secrets | Network |
|---|---|---|---|---|
| Codex cloud | UI only: an **Install script** and a **Start skill**; Codex reads `AGENTS.md` for commands; no repository file or API defines the environment [V]. Legacy: setup and maintenance scripts, the `universal` image with `CODEX_ENV_GO_VERSION` (1.22–1.25.1 only) [V] | Published filesystem; Legacy cache up to 12 h, invalidated when scripts or variables change [V] | Network secrets substituted by the proxy for allowed domains; Legacy secrets only during setup [V] | Off for the agent by default; presets (package managers) or custom domains; methods can be limited to GET/HEAD/OPTIONS [V] |
| Cursor cloud agents | `.cursor/environment.json` on the **default branch** (`install`, `start`, `terminals`, `build.dockerfile`, `image`, `snapshot`, `egressAllowlist`, `egressMode`, …), else a dashboard environment [V] | A Build runs `install` and captures disk only; Dockerfile layers cached [V] | Dashboard: environment variable, runtime secret (redacted), build secret; OIDC/IAM [V] | Internet by default; allow-all, default + allowlist, or allowlist only [V] |
| Amp orbs | `.agents/setup` (committed, idempotent, ≤ 20 min), `.agents/resume` (no installs) [V] | Snapshot per orb size, reused up to 72 h; a setup change alone does not invalidate it [V] | Workspace, project, personal secrets; OIDC from `amp orb id-token` [V] | No documented default restriction [I] |
| Claude Code on the web | UI only: an environment (name, network access, environment variables, setup script) per account; the setup script runs as root on Ubuntu 24.04 before Claude Code starts; a `SessionStart` hook in the repository's `.claude/settings.json` runs in every cloud session (`CLAUDE_CODE_REMOTE=true`, `$CLAUDE_PROJECT_DIR`) [V]. `/remote-env` only selects an environment [V] | Setup results cached about 5 min after the first run [V] | Environment variables in the UI; no secret store [V] | Trusted by default: a package-manager and forge allowlist; Custom adds domains [V] |
| Claude Code (Action) | Not used: it needs `ANTHROPIC_API_KEY` in the product repository's CI (founder rule, 2026-10-16) | — | — | — |

### 6.2 One source, generated per receiver

Built in WP3.2c (Appendix B S-3.2c). `aayu install --env` discovers the environment from the code and writes the
spec to `.aayudha/env.toml`; every host's files are generated from it, and every host calls the same two
scripts: `.agents/setup` (pinned, checksummed, idempotent; it runs as root or as a sudo user) and `.agents/check`
(the pinned toolchains are the ones on `PATH`, services start, the build and tests pass, then a marker line).

- **Amp**: runs `.agents/setup` natively and `.agents/resume` on resume (it starts the services only).
- **Cursor**: `.cursor/environment.json` `install: bash .agents/setup` and, with services, `start:
  bash .agents/start`; aayu owns `name`, `install`, and `start`, the rest is the owner's. It must be on the
  default branch.
- **Codex**: UI only. aayu prints the Install script (`bash .agents/setup`), the Start skill (start the services),
  the extra allowed domains beyond the Package managers preset, and the variables by name, then verifies with
  `codex cloud exec --env <id>` or a recorded probe reply.
- **Claude Code on the web**: the `SessionStart` hook in `.claude/settings.json` runs `.agents/setup` when
  `CLAUDE_CODE_REMOTE` is `true`, so the UI needs no setup script; aayu prints the network allowlist additions and
  the variables.
- **Proof**: `install --env --verify` runs setup cold, setup warm, then the check in a clean container of each
  host's documented base; `doctor` reports each host as verified, stale, or missing.

The shared script holds for all four hosts: each one either runs a repository file or accepts one command, and
each of the documented bases (Debian 12, Ubuntu 24.04, codex-universal) has bash, apt, and sudo or root.

### 6.3 Principles of a good agent environment

1. **Pinned**: every tool at an exact version with a checksum; lockfiles frozen (`--frozen-lockfile`,
   `go mod download` from `go.sum`).
2. **Idempotent and fast**: safe to re-run, seconds on a warm cache; expensive steps in the cached layer (`install`,
   setup), nothing long-running there; no installs on resume (Amp, Cursor) [V].
3. **No long-lived secrets**: the agent phase gets none, or only what the platform injects and redacts; prefer OIDC
   (Amp, Cursor) or proxy substitution (Codex) [V]; never write tokens into the snapshot.
4. **Least network**: package registries and the forge only; narrow methods where offered (Codex) [V]; never
   wildcard object storage ("an exfiltration path for a prompt-injected agent", Cursor) [V].
5. **Reproducible caches**: caches keyed on the lockfiles; a cache miss changes speed, never results.
6. **One source**: every platform calls the same script, so a pin changes once.

## 7. What aayudha already has, and where each part fits

| aayudha today | Role in delegation and review | Gap |
|---|---|---|
| `grill` / `ready`: Completion Goal criteria, Ready snapshot on the ledger with body hash | The brief's goal, and the gate (`build` refuses a story that is not Ready or drifted) | None |
| `plan`: checkpoints per layer with tests and rollbacks (`specs/<n>/plan.json`) | The brief's plan and stop conditions | None |
| Evidence map `.aayudha/evidence/<n>.toml` | The brief tells the receiver which tests prove which criterion; the merge gate fails without it | Receivers must be told to write it |
| `build`: lease, budget, worktree from `origin/dev`, DCO checkpoint commits, `Refs #n` PR | Delegation runs inside `build`; the local fallback reuses its checkpoint, verify, push, and PR code | Delegation mode and receiver adapters |
| Merge gate and `inspect`: authoritative pack, floors as facts, cross-family reviewer, `merge`, `fix`, or `rebuild`, 2-cycle cap and page | Reviews every delegated PR; its disposition drives the follow-up | Builder family of a delegated PR; review quality (§8.2) |
| `review` lanes A/B (UI), `polish` | Unchanged; `polish` gets a worktree | `polish` used the current checkout |
| `ship`: human pass, solo-mode bypass | Unchanged: delegated PRs merge only through `ship` | None |
| Model gateway: lanes, families, budget breaker | Reviewer selection (two families for peer mode), spend | A second reviewer family |
| Actions log, ledger, `[budget]`, `halt` | Every hand-off and follow-up is an action; delegation state on the ledger | New action kinds |
| Personas, another thread (parallel work) | The brief is persona-neutral; seats unchanged | None |

## 8. Recommendation: what 0.2 builds, what waits, what not to build

### 8.1 Delegation (WP5.5b)

**Build in 0.2:**

1. `aayu build <n> --delegate codex|cursor|claude|amp|auto [--local]`: the same gates as `build` (Ready, body hash,
   lease, budget, halt). The first run writes the brief, hands it off through the receiver's adapter, records the
   hand-off (actions log, `delegate/<n>/state.json` on the ledger, a replace-in-place comment on the story), and
   returns. Each later run syncs: receiver status, the PR (by `Refs #n`, or opened by aayu from the receiver's branch
   or diff), the PR's `inspect` disposition, and, on `fix` within the cycle cap, a follow-up carrying the structured
   findings; at `merge` it says "ready for `ship`"; at the cap it pages and stops.
2. **One brief** (Markdown, same for every receiver): the Ready snapshot and Completion Goal, the plan's checkpoints,
   the evidence map to write, `AGENTS.md` conventions, branch and PR rules (Conventional Commits, DCO sign-off,
   `Refs #<n>` first in the body, PR to `dev`), the local checks, stop conditions (scope, cap, never merge, never
   touch protected paths without saying so), and the untrusted-input rule. No secret, token, or environment value.
3. **Adapters** (transport only): Cursor REST, Amp CLI, Claude mention, Codex `cloud exec` with diff return; each
   with recorded fixtures and a fake, live tests behind `AAYU_LIVE=1` that skip cleanly without credentials.
4. **Local fallback**: `.aayudha/worktrees/<n>-<agent>`, the receiver's CLI with a scrubbed environment (no
   `GITHUB_*`, `GH_*`, `AAYU_*`, or token variables except the receiver's own key), no GitHub credentials, then
   `build`'s checkpoint, verify, push, and PR.
5. **`auto` picks the cheapest adequate connected receiver** by Effort and says why: XS–S, plan-included receivers
   first (Codex, then Claude), then Amp, then Cursor; M–L, receivers with native follow-ups first (Cursor, then Amp,
   then Claude, then Codex); none connected → local.
6. **Slash commands** `/codex`, `/cursor`, `/amp`, `/claude` (Markdown command sources under `hosts/commands/`),
   wired into the plugins the CLI and host-plugins thread builds; each takes a Ready story (or a goal, which it first
   takes through `grill story` and `ready`) and runs `aayu build <n> --delegate <agent>`.
7. **Environments**: §6.2, with a `doctor` check per receiver and `doctor --fix` for `.cursor/environment.json`.
8. **Worktrees and parallel work**: `polish` prepares a worktree of the PR head instead of using the current
   checkout; `aayu status --work` lists active worktrees with agent, story, PR, and age; `gc` also cleans
   `<n>-<agent>` worktrees.

**Later (0.3+)**: Cursor v1 webhooks when they ship (polling until then); a loop that runs the sync step on a
schedule or on `check_run` events (today the founder's host, or the receiver's own PR subscription, drives it);
Codex `@codex` on issues once documented; Claude cloud sessions once they have an API.

**Not built**: giving any receiver merge rights or a token (aayu merges only via `ship`); trusting a receiver's PR
text or comments as instructions; per-receiver briefs; a delegation verb separate from `build`; writing
`.github/workflows/` automatically.

### 8.2 Review quality (WP4.8b), for every agent-authored PR

**Build in 0.2:**

1. **Context retrieval** (deterministic, through the contents API at the PR head; no PR code runs): the changed
   symbols of each file (Go by AST, other languages by the diff's hunk headers), their callers in the same package,
   related tests, whether the PR changed tests in that area, and the `LAYERS.md` footprint.
2. **A review plan per changed area** (a package or directory): files, symbols, tests, layers, and the risks to check
   (state on L07, policy on L04, concurrency, external input), rendered into the prompt and the comment.
3. **Findings v2**: severity, category, a quoted snippet for anchoring, file and line (resolved by aayu from the
   snippet), detail, a suggested fix, confidence.
4. **Noise filtering**: findings on files outside the diff dropped (OCR's ground A); `style` findings and findings on
   generated files dropped (the linters and the drift check own them); a confidence threshold for P2–P3 only;
   de-duplication keeping the highest severity; suppression of findings that resemble ones a human dismissed
   (`inspect --dismiss`, recorded per repository on the ledger), never for P0 or protected categories (security,
   data, concurrency).
5. **Two modes**: general (one reviewer) and peer (two reviewers of different families, both different from the
   builder; each one's single-source findings go to the other for an asymmetric "prove it wrong" check; refuted
   findings are dropped unless P0 in a protected category). Delegated PRs default to general (founder decision,
   2026-10-15, after the eval in BUILD-PLAN D-4.8b.6; Appendix B D-4.8b.7); `inspect --mode peer` stays available.
6. **Inline review comments** for posted findings (one PR review per run, new findings only), so receivers that
   watch review comments address them, plus the existing summary comment and state.
7. **Evaluation**: seeded-defect PRs (planted bugs of several kinds and clean controls) with precision and recall per
   mode, before (the 0.1 prompt) and after, through the eval harness as an advisory, dispatch-only tier-2 group.
   Deterministic checks keep beating model judgement (ground rule 7): floors and P0/P1 rules are unchanged.

**Later**: embeddings for suppression and retrieval; a whole-repository symbol graph; per-language rule checklists;
addressed-rate tracking from PR history in `retro`; AACR-Bench as an external benchmark.

**Not built**: an agentic tool loop inside `inspect`; a model-rated severity filter on its own; letting `inspect`
approve or merge; dismissals by agents (dismissing is human-only).

## 9. Safety

- Everything a receiver writes (PR text, commits, comments, status) is untrusted input (§0.1): it reaches a model
  only inside `<untrusted>` blocks and never triggers `ship`, `respond`, or a follow-up by itself.
- Briefs carry no secrets: the brief renderer takes only the story, plan, evidence map, and repository rules, and its
  output is run through the actions log's redaction before it is sent.
- Receivers never get merge rights: aayu merges only via `ship` (human pass); the local fallback gets no GitHub
  credential at all.
- Every hand-off, follow-up, and review is an action in the actions log, segmented to the ledger.
- `[budget]`, `[autonomy.budget]`, and `halt` apply: `build`'s budget checks run before every hand-off and
  follow-up, and the follow-up cycle cap is `inspect`'s.

## 10. What the founder must connect

| Receiver | Cloud | Local fallback |
|---|---|---|
| Cursor | Install the Cursor GitHub App on the repository; create an API key (dashboard, or a service account) and store it with `aayu auth set-secret cursor-api-key` (value on stdin); set a spend limit; merge the generated `.cursor/environment.json` to `dev` | `cursor-agent` installed and logged in (`agent login`) or `CURSOR_API_KEY` |
| Codex | ChatGPT plan with Codex; install the ChatGPT Codex Connector app on the repository; create a cloud environment with Install script `bash .agents/setup` and the package-managers internet preset; put its ID in `[delegate.codex] env`; `codex login` on the machine that delegates | `codex` CLI logged in, or `CODEX_API_KEY` |
| Claude Code | Since WP5.5c: a Pro, Max, Team, or Enterprise claude.ai plan; `claude auth login` with that claude.ai account (an API-key login cannot start cloud sessions); the Claude GitHub App on the repository, or `/web-setup`; a cloud environment whose setup script runs `bash .agents/setup`. No workflow and no repository secret | `claude` CLI signed in, or `CLAUDE_CODE_OAUTH_TOKEN` (`claude setup-token`); `ANTHROPIC_API_KEY` is not passed to it |
| Amp | Connect GitHub in Amp (Amp GitHub App), create the Amp project for the repository, store `AMP_API_KEY` (`sgamp_…`) for the CLI | `amp` CLI with `AMP_API_KEY` |

## 11. Open questions and uncertainty

- Codex: whether `@codex` on an issue opens a PR, the Legacy deprecation date, and `codex cloud` leaving
  experimental status. The adapter is built against recorded fixtures of the CLI's documented output only.
- Cursor: whether v1 PRs are opened as the user or the app, and when v1 webhooks ship.
- Amp: the orb egress policy is not documented.
- Vendor benchmark numbers (OCR, Greptile, Bugbot) are self-reported; our own seeded-defect evaluation is the number
  this repository relies on.
