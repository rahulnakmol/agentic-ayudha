# What six talks and posts on software factories teach aayudha

Research, 2026-10-15. Owner: the Engineering Manager (Athena) and Quality Architect (Themis) twins, at the
founder's request. Status: proposal only. Nothing here is implemented; each change becomes a WP and an Appendix B
row in [BUILD-PLAN.md](../BUILD-PLAN.md) only after the founder answers §8.

The founder's request: study three videos (two more and a blog post were added during the work), extract what they
say about patterns, tooling, reasoning, and architecture, check each idea against how aayudha works today, and
propose a measured, least-friction set of changes that never weakens the rule that a builder cannot certify its own
work. Summaries are paraphrased; quotes are short and only where the wording matters. The raw caption text stayed in
the research orb and is not in this repository.

## 1. Summary

aayudha already has the shape these talks converge on: the factory is defined as code (ground rule 4), a different
agent certifies the work (ETHOS 1, the merge gate, cross-family `inspect`), one human is accountable for each merge
(`ship`), and skills improve eval-first (`evolve`, D-7.7a). The talks add little to that shape. They add a lot about
**measuring the flow**, **closing small loops without a human**, and **proving that a test really tests the fix**.

Recommended, in this order (§6): seven extensions of existing verbs, no new verb, none of which moves a gate away
from a human or from a deterministic check.

| # | Change | Extends | Size | Verdict |
|---|---|---|---|---|
| P1 | Flow and review-quality metrics: stage times, human wait, touches per story, inspect precision and escape proxies, change mix | `retro`, `status` | M | Adapt |
| P2 | A "test value" finding category, counted and never posted | `inspect` | XS | Adapt |
| P3 | CI repair: a failed required check becomes a follow-up to the builder, inside the existing cap | `build --resume`, `--delegate` sync | S | Adapt |
| P4 | Evolve discipline: two-story evidence rule, rewrite not append, recurrence check | `evolve` skill, `retro` | S | Adapt |
| P5 | Red-on-base proof: a mapped test must fail on the base and pass on the head | `verify --ci` in the merge gate | M | Adopt |
| P6 | A deterministic risk line at the top of the proof comment | proof comment, `inspect` input | S | Adapt |
| P7 | Ratchet floors that only tighten; loosening is a waiver | `verify`, `decide --waiver` | M | Adapt |

Rejected (§5.2): auto-approving low-risk PRs outside the earned streak, learning from 👎 reactions, LLM-judge scores
as gates, production traffic capture, a context graph, an MCP gateway, scheduled maintenance-PR loops, and a second
workflow language beside the registry.

## 2. Sources and how they were transcribed

YouTube refuses this orb's IP: `yt-dlp` (every player client, with and without a JavaScript runtime),
`youtube-transcript-api`, and a headless browser all returned a sign-in or `LOGIN_REQUIRED` wall, so neither the
captions nor the audio could be downloaded, and no audio was transcribed. The published caption tracks were read
through Amp's web reader instead, which fetches them from its own servers. That reader returns at most 50,000
characters, so for long videos the middle was recovered through repeated calls aimed at named time ranges. The
caption tracks look auto-generated (misheard names, no speaker labels), so names and model versions in them are
unreliable; numbers were cross-checked where a primary source exists.

| ID | Source | Length | Method | Coverage and reliability |
|---|---|---|---|---|
| S1 | [From coding agents to software factories with Uber and Faire](https://youtu.be/0gDHnH89aQs) (Warp webinar: Adam Huda, Uber; Chris Handel, Faire; Zach Lloyd, Warp) | about 60 min | Published captions via the web reader | About 95 %: continuous 0–18:12 and 41:05–end; the middle from nine targeted calls; about 3 min of short gaps. High |
| S2 | [Uber's AI infra, from the inside](https://youtu.be/MDUYSMJmi8A) (The Pragmatic Engineer: Gergely Orosz with Uday Kiran Medisetty, Uber) | 1:34:17 | Published captions via the web reader | Continuous 0–13:35 and 1:20:34–end; the middle from 13 targeted calls, about 70–75 % seen. Medium; many system names misheard |
| S3 | [How to build self-improving agents (full walkthrough)](https://youtu.be/FqafKuH8F3k) (Warp, Ben Holmes, 2026-10-13) | 1:00:55 | Published captions via the web reader | Continuous 0–18:12 and 44:24–end; the middle from seven targeted calls; about 16 short gaps. High |
| S4 | [The AI factory playbook for engineering teams](https://youtu.be/4_SHhSMHzNo) (How I AI: Claire Vo with Zach Lloyd, Warp, 2026-09-21) | 46:51 | Published captions via the web reader | Continuous 0–15:05 and 31:05–end; the middle from five targeted calls; about 11 short gaps. Medium-high |
| S5 | [Claude's "engineer mode" loop](https://youtu.be/0UQme-kv6zg) (AICodeKing, a commentary on B1) | 11:22 | Published captions via the web reader | Complete in one call. High as a transcript; it is commentary, so B1 is the primary source |
| B1 | [How we made claude.ai 3x faster in two weeks](https://claude.dev/blog/how-we-made-claude-ai-faster/) (Raymond Wang, Sam Attard, Issac G., 2026-09-23) | 15 min read | Read in full | Primary text. Self-reported numbers, no confidence intervals |

Vendor framing is heavy in S1, S3, and S4 (all Warp) and sponsored segments run in S2 and S4. Warp's dashboard
figures and every "faster and better" claim without numbers are treated below as claims, not evidence.

## 3. What each source said

### 3.1 S1: the software factory roundtable (Uber, Faire, Warp)

- **Patterns.** Not a "dark factory": the plumbing goes dark, humans stay where judgement matters (Uber, Warp).
  Build *stations* that work well when invoked by hand, then connect them with a *conveyor* (Faire); adopt
  crawl-walk-run, starting with one hero automation, then a full loop on a low-risk project (Warp). Run validation in
  the inner loop, then self-healing CI, and only then ask a human, because human review time is the scarce resource
  (Uber). Triage decides whether a task needs a spec at all, by ambiguity and risk (Warp). Code review is framed as
  precision (comments nobody acts on) and recall (defects a human had to catch), and human reactions are treated as
  labelled data for self-evolving review skills (Uber, Faire).
- **Tooling.** Uber: a Builder Infra org whose charter is the factory; a Neo4j "context graph" of people, services,
  docs, and code locations queried through a skill; a curated "golden" skills list after skill sprawl into the
  thousands; Minion (cloud agent); an inner-loop validator; an opt-in self-healing CI toggle; uReview with confidence
  scores that suppress low-signal comments. Faire: a "refiner" that turns a request into a better spec, CI autopilot,
  an in-house review agent built after a vendor reviewer's token bill triggered weekly finance meetings. Warp:
  Factories with scores such as skill adherence and efficiency.
- **Thought process.** The SDLC was always a machine; the engineer's job moves to "the thing that builds the thing",
  which needs an SRE-like factory-ops role (Uber). Because every component is non-deterministic, the only way to
  reason about the factory is to measure its outputs and tune experimentally (Faire). Zach admitted spec-driven
  development has not been A/B-proven; Uber is still testing embeddings against grep.
- **Architecture.** The factory is a graph with loops; each stage gets success metrics (cycle time, throughput) and
  guardrail metrics (reverts, incidents). Review has a central budget and model choice. Maintenance loops run off-peak
  with a cap on how many diffs they send.
- **Disagreements.** Authorship: Warp lets the agent author the PR and the person who prompted it review it, and
  called a human-labelled author a fiction; Faire and Uber keep a human as author because the label shapes felt
  accountability ("we want humans to be accountable"). Specs: durable in the repo (Warp) against documentation debt
  (Faire), settled as "historical intent, not live docs". Code representation: graph (Uber) against grep (Faire,
  Warp).

### 3.2 S2: Uber's AI infrastructure from the inside

- **Patterns.** Build foundations that do not depend on any one harness or model, because both change monthly. Reuse
  the pre-AI developer platform (cloud dev pods since 2017, the IDL registry, tracing, the monorepo) as the agents'
  substrate. Humans and agents use the same one-command build and test per module. Agents verify their own changes in
  real environments, and every PR is still merged by a human, with AI gathering the evidence (review, screenshots,
  risk score, blast radius) in one place. Auto-landing waits until risk scoring is back-tested on years of history.
  Measure feature velocity, not diff counts.
- **Tooling.** One OpenAI- and Anthropic-compatible model gateway with project-scoped keys, billing, and AI Guard
  (fine-tuned output filters); an MCP gateway that generates servers from the IDL registry; a skills monorepo
  (about 30,000 skill runs a day); a context graph with 100+ node types; Minion (about 15 % of production PRs);
  Code Inbox (risk-scored review queue); Slate (a per-branch staging instance routed by trace baggage); Capri
  (production capture and replay); Shepherd (migration campaigns as YAML policies); scheduled per-service
  maintenance PRs; Wisdom and debug assist (bug report to root-cause analysis to PR).
- **Thought process.** The host's line, "the harness is the easy part", frames the episode: production-ready code
  depends on what surrounds the harness. Uber errs on the safe side because incidents are expensive, so humans still
  review every PR. The engineer's job is now context engineering, writing specs and constraints, running many sessions
  at once, and verifying quickly. PRs are categorised (feature, bug fix, maintenance, test, docs) and by complexity, and
  P50 feature lead time per half-year shows where the bottleneck sits.
- **Architecture.** Single entry points everywhere (one model API, one tool gateway, one skills repository, one
  design-doc hub), each the place where cost, policy, and observability are enforced. Traceability from product doc to
  design doc to ticket to PR, with AI filling missing links.

### 3.3 S3: Warp, self-improving agents

- **Patterns.** Session transcripts are scored by separate scorer agents against rubrics (verbosity, task compliance
  measured by how often a human had to steer, efficiency, code quality); passing sessions are ignored and failing ones
  go to one improver agent, which opens a PR against skills citing the failed runs. A human maintainer merges it.
  Rules for the improver: rewrite a skill as a whole instead of appending to it, never patch from a single instance
  (it needs "a decent amount of evidence"), and name the owning surface (missing, wrong, or unclear instruction).
  A custom scorer flags redundant tests, because agents over-test.
- **Tooling.** Skill Doctor, an open-source local tool that samples a developer's own Codex, Claude, and other
  sessions and writes an HTML report with a grade, skill coverage, and suggested diffs; it does not redact secrets.
  Warp Factories keeps scorer definitions (rubric, model, threshold, which agents) as YAML under PR review. The judge
  is a cheap model chosen on cost; scoring costs about 0.10–0.20 USD per conversation.
- **Thought process.** Most business logic ends up in skills, so skills are the main edit target; AGENTS.md is a
  "leaning tower" of rules. One agent per rubric keeps each focused. Scorers stay independent of the improver so the
  scores can show whether the improver helps. Cross-family judging is "intuitively right" but unstudied.
- **Architecture.** A stateless weekly loop. Progress is a per-criterion trend; if the same failure returns a week
  after a patch, revert it. No shadow runs; replay benchmarks are future work. No guard against gaming the scorers was
  described beyond human review of the PRs. The only outcome claimed was a small rise in task compliance, confounded
  with model changes.

### 3.4 S4: the AI factory playbook (How I AI, Zach Lloyd)

- **Patterns.** A request (a Slack mention, a Sentry crash, a Linear issue) enters a fixed pipeline: triage, issue,
  implementation, PR, QA, merge. A QA agent records a video of the finished feature as evidence. "Human interactions
  per PR" (reprompts, comments, review corrections) is the throughput signal, by analogy with line stops in a car
  plant. Every PR still gets human review; Warp relaxed one rule, letting the person who prompted the agent review its
  code. The host described risk-scored review: low-risk PRs get a bot approval and a human click, the rest full
  review.
- **Tooling.** An agent cost dashboard, a multi-dimension scoring view, an observer agent that studies 20–25 failed
  runs and proposes edits to a factory agent's instructions; replays of past tasks against a changed factory to build
  a cost-quality Pareto chart for model choice.
- **Thought process.** Humans are both the bottleneck and the creative force, so the goal is fewer touches per PR, not
  fewer people. Cycle time is the number: in Warp's dashboard the wait from PR to first human review (3.5 hours)
  dwarfs the build (35 minutes). Model choice is the biggest cost lever. Choose models on your own tasks, not public
  leaderboards.
- **Architecture.** The factory is a noun: repositories, MCP servers, configuration, and agents defined in code,
  versioned so that a past state can be frozen and replayed. One scoring system serves both live runs and replays.

### 3.5 S5 and B1: making claude.ai 3x faster in two weeks

B1 is the primary source; S5 is a faithful commentary on it that adds one caution: agent activity and diff volume
are not value, and parallel agents add coordination and review cost.

- **Patterns.** Make the goal measurable, then hill-climb it. Every count that can be measured becomes a lab number for
  Claude to drive down **and** a CI ceiling that can only move down (a ratchet); a daily job lowers the ceiling when the
  count drops. A deterministic proxy (instruction counts under Valgrind) must first be shown to track wall-clock time;
  flaky or uncorrelated benchmarks were removed so the agent would not climb the wrong hill. Proof is red/green over
  repeated runs: a layout-shift test failed 20 of 20 runs on `main` and passed 20 of 20 on the PR. About a third of
  PRs added telemetry or guardrails that protect a win after it lands.
- **Tooling.** Claude in one shared Slack channel with standing instructions; a Datadog MCP server for field data;
  Valgrind instruction counts; deterministic browser counts (React commits, layout recalculations); a 120 Hz headless
  Chrome rig that makes "did this frame fit" a yes or no; about 200 short-lived flags, each classified as a kill switch
  or a ramp.
- **Thought process.** Humans supplied ambition (the agent defaulted to caution), taste (a named owner judged
  before/after recordings), and direction (narrow threads, merging overlaps, rejecting a 900-line change that saved
  about 2 ms). A metric can pass while the experience fails: shifts under the standard "good" threshold still annoyed
  users, so they built finer telemetry.
- **Architecture.** Lab benchmark, proxy tied to elapsed time, CI ceiling, staged rollout (employees, 1 %, everyone),
  then confirmation in real-user p75 data. Every PR had automated review plus at least one human approval; more than
  3,000 changes merged with no reported customer-facing incident (the authors' claim).

## 4. Cross-source synthesis

### 4.1 What recurs

```text
            ┌──────────────────────────────── recurring across S1–S5, B1 ────────────────────────────────┐
 intake ──▶ triage / spec ──▶ build ──▶ inner validation ──▶ self-heal CI ──▶ evidence on PR ──▶ human merge
                                  ▲                                                                   │
                                  └──── scorer / judge ──▶ improver ──▶ skill PR (human merges) ◀──────┘
                                         measured by: cycle time, human touches, reverts, cost per PR
```

1. **Verification, not generation, is the bottleneck** (S1, S2, S4, B1). Every source spends its effort on what
   happens after the code is written.
2. **The builder does not certify itself** (all). A separate validator, reviewer, judge, or human decides. No source
   lets the building agent mark its own work done.
3. **One human stays accountable per merge** (S1, S2, S4, B1). Even Warp, which lets agents author PRs, keeps a human
   review on every PR.
4. **Evidence travels with the PR**: screenshots, video, validation results, risk scores (S1, S2, S4, B1).
5. **Measure the factory as a system**: cycle time split by stage, the human's wait, touches per PR, guardrails such as
   reverts and incidents, cost per PR (S1, S2, S4).
6. **Self-improvement edits skills from evidence, and a human merges the edit** (S1, S3, S4). Scorers are kept apart
   from the improver.
7. **Close small loops automatically** before involving a human: failing CI, merge conflicts, review findings (S1, S2).
8. **Deterministic, ratcheted guardrails protect wins after they land** (B1, S5; S2's back-testing before any green
   lane is the same instinct).

### 4.2 Where they conflict

| Topic | One side | Other side | Where aayudha stands |
|---|---|---|---|
| Who judges quality | LLM-as-judge scores every session (S3, S4) | Deterministic proxies, validated against the outcome; wall-clock too noisy to gate (B1) | Ground rule 7: deterministic floors beat any model; the judge only scores eval cases offline |
| PR authorship | Agent authors, prompter reviews (Warp) | Human is the labelled author for accountability (Faire, Uber) | Builder is an agent; certification is CI plus a cross-family `inspect`; `ship` is human-only |
| Review's future | "Slowly dying" (S2 host's opinion) | Every PR human-reviewed, risk tells where to look (S2 guest, S4, B1) | Human pass on the head commit; autonomy earned per layer by a computed streak |
| Context | Graph of people, services, docs (Uber) | grep is enough; a second representation goes stale (Faire, Warp) | AST context per PR, `LAYERS.md`; repository index deferred |
| Specs | Durable in the repository (Warp) | Documentation debt (Faire) | Ready snapshots are immutable history on the ledger |
| Scale | 150 parallel threads, 200 merges a day (B1) | Parallel agents multiply coordination and review cost (S5) | Worktree per story, `status --work`; cost breaker |
| Self-healing CI | Opt-in toggle (Uber) | Considering opt-out (Faire) | Retries inside `build`; failed checks not fed back yet |

## 5. Candidate ideas against aayudha today

Verdicts: **Have** (no change), **Adopt**, **Adapt**, **Watch**, **Reject**. "Fit" checks ground rules 1–8, §0.1
(trust model), and the founder's least-friction preference.

### 5.1 The table

| ID | Idea (sources) | aayudha today | Fit | Cost | Verdict |
|---|---|---|---|---|---|
| I1 | Factory defined as code, run by a runtime (S1, S4) | The registry is the single source (ground rule 4, `internal/verbs/registry.go`); skills; `loops/*.toml` | Fits | — | Have. A second pipeline language would break rule 4 |
| I2 | Stations first; crawl, walk, run (S1) | Levels 1–3 in `docs/guide/adopt.md`; each verb runs alone | Fits | — | Have |
| I3 | A different agent certifies; evidence on the PR (all) | ETHOS 1; merge gate on `pull_request_target` (how-merge-gate.md); cross-family `inspect`; proof packs (S-4.2b) | Fits | — | Have |
| I4 | One human accountable per merge (S1, S2, S4, B1) | `ship` is human-only and never exposed (`registry.go:645`); human pass excludes bots and the author (how-trust.md) | Fits | — | Have |
| I5 | Flow metrics: stage times, human wait, touches per PR, change mix, cost per PR (S1, S2, S4) | `retro` has checkpoints vs Effort, corrections (incl. reverts and linked Bugs), cost per story, per-skill correction rate (D-7.6b, `internal/autonomy/autonomy.go`); no times or touches | Fits; read-only | M | **Adapt → P1** |
| I6 | Review precision and recall on live PRs (S1) | Offline seeded-defect eval only (D-4.8b.6: general P 0.73, R 0.95); human-only dismissals (D-4.8b.2) | Fits if computed from aayu's own records | S (inside P1) | **Adapt → P1** |
| I7 | Learn from 👎 reactions and replies (S1) | Rejected: in solo mode every agent writes as the founder, so reactions cannot be attributed (D-4.8b.2) | Breaks attribution | — | Reject |
| I8 | Confidence-gated review comments (S1) | `[inspect] min_confidence = 0.5` for P2/P3 (`internal/config/defaults.toml`) | Fits | — | Have. Research found self-rated confidence weak alone |
| I9 | Self-healing CI, "babysit PR" (S1, S2) | `build` retries a checkpoint up to `[autonomy.budget] attempts`; `inspect` fix becomes a follow-up, cap 2 (D-5.5b.3); failed required checks are not fed back | Fits; the gate still certifies | S | **Adapt → P3** |
| I10 | Improver opens skill PRs from evidence; human merges; rollback on recurrence (S3, S4) | `evolve`: accepted Feedback, a failing eval case first, then a passing skill diff, CODEOWNER review, never auto-merged, `install --pin` rollback (D-7.7a) | Fits | S | **Adapt → P4** (guidance and a recurrence rule) |
| I11 | LLM judge scores every real session (S3, S4) | The judge scores eval cases only (`evals/eval.toml`, cross-family enforced) | Model opinion, so never a gate (rule 7); host transcripts hold secrets; per-host formats; cloud sessions out of reach | M–L | Watch |
| I12 | Redundant-test scorer (S3, S4) | None; the diff-coverage floor (90 %) may even reward over-testing | Fits as a count | XS | **Adapt → P2** |
| I13 | Ratchet floors that only tighten; validate the proxy first (B1, S5) | Static floors in `CONSTRAINTS.md`; performance and API budgets are product-specific with no default (CONSTRAINTS.md) | Fits rule 7 | M | **Adapt → P7** |
| I14 | Red on base, green on head as proof (B1) | The agent chooses which tests prove each criterion; `inspect` judges the mapping (how-proof-packs.md, "How it works") | Strengthens the separation | M | **Adopt → P5** |
| I15a | Risk score tells the reviewer where to look (S1, S2, S4) | `inspect` builds a review plan with layer and pattern risks (D-4.8b.1); never-layers L04, L07, L09; `risk:` labels feed `status` only | Fits | S | **Adapt → P6** |
| I15b | Auto-approve low-risk PRs (S4 host, S2 "green lane") | Autonomy is earned per layer by a 20-merge streak computed by a protected workflow (V4, how-trust.md) | A second auto path would bypass the streak | — | Reject |
| I16 | Triage decides if a spec is needed (S1) | Every Story needs Ready; `[team] stage` sets depth | Unclear benefit | — | Watch: let P1 show whether shaping is a real wait |
| I17 | Specs as history and as tests (S1) | Ready snapshot immutable on the ledger; Completion Goal criteria are what proof checks | Fits | — | Have |
| I18 | Context graph (S1, S2) | Per-PR Go AST context; repository index deferred (D-4.8b.1) | Scale mismatch; the panel itself split | L | Reject for 0.x; watch |
| I19 | Model gateway, cost tracking, budgets (S2) | `[models.gateway]`, per-lane resolution, price table, breaker (how-models.md) | Fits | — | Have |
| I20 | MCP gateway and output filters (S2) | aayu is a stdio MCP server with per-verb allowlists; `[mcp] expose_writes` | Enterprise-scale problem | L | Reject for 0.x |
| I21 | Per-branch preview environments (S2) | `[verify.environments]`, production refused (WP4.5) | Product infrastructure, not aayu's | — | Watch |
| I22 | Capture and replay production traffic (S2) | Proof uses seeded data only; production capture exits 4 (V3, WP4.5) | Breaks V3 (PII) | — | Reject |
| I23 | Scheduled maintenance loops that open PRs (S1, S2) | Loops for digest, decisions, retro, seat review, evolve (`loops/`) | Adds review load on a solo founder | M | Reject for now |
| I24 | Choose models by replaying your own tasks; cost-quality Pareto (S4) | Tier 2 runs each skill on its lane model with cost per group (`evals/results/`) | Fits | S | Watch: a dispatch option later |
| I25 | Agent as labelled author; prompter as sole reviewer (S1, S4) | Agent builds, CI and a cross-family reviewer certify, a human ships | Weakens nothing only if certification stays separate; it already is | — | Reject the change |
| I26 | Staged rollout and field confirmation (B1) | `dev` → `rolling` → `main` with gates; `measure` for product metrics | Fits | — | Have |
| I27 | Public intake channel, champions programme (S1, S4) | GitHub issues and forms are the shared surface; solo founder | No benefit at this stage | — | Reject |
| I28 | One variable at a time; the gain must beat the noise (B1, S5) | Tier 2: k runs, pass^k, fail on a drop over 0.3 (how-evals.md) | Fits | — | Have; applied to P7 |

### 5.2 Why the rejections

- **Auto-approval outside the streak (I15b).** aayudha already has the principled version of Uber's back-tested
  "green lane": autonomy per layer earned by a correction-free streak computed by a protected workflow (V4). A risk
  score that approves on its own would be a second, model-influenced path around it.
- **👎 and reply signals (I7).** Unattributable in solo mode (D-4.8b.2). P1 measures the same precision and recall
  from records aayu writes itself.
- **Judge scores as gates (I11).** Ground rule 7. Scores may at most file Feedback, and P1's deterministic proxies
  (attempts, fix cycles, corrections, cost) already cover compliance and efficiency without reading transcripts.
- **Production capture (I22).** V3 forbids real user data in proof.
- **Context graph and MCP gateway (I18, I20).** They solve a 5,000-service organisation's discovery and policy
  problems. aayudha pods are a few repositories; the panel itself could not show the graph beats grep.
- **Maintenance-PR loops (I23).** Every loop PR is a human review. The founder asked for the least friction.
- **A workflow language (I1).** Station order already lives in the registry's gates and the skills; a second
  definition would drift (ground rule 4).

## 6. Recommended changes

```text
 Wave 1: measure            Wave 2: close loops        Wave 3: stronger proof     Wave 4: attention, ratchets
 ┌───────────────────┐      ┌───────────────────┐      ┌───────────────────┐      ┌───────────────────┐
 │ P1 retro metrics  │ ───▶ │ P3 CI repair      │ ───▶ │ P5 red-on-base    │ ───▶ │ P6 risk line      │
 │ P2 test value     │      │ P4 evolve rules   │      │   (advisory)      │      │ P7 ratchets       │
 └───────────────────┘      └───────────────────┘      └───────────────────┘      └───────────────────┘
   baseline numbers           fewer human touches        test proves the fix        faster human pass
```

P1 comes first because B1's lesson applies to us: measure before optimising, so every later change can show its
effect on human wait, touches, and escapes.

### P1: flow and review-quality metrics in `retro` (extension, M)

- **What.** `retro` (D-7.6b) adds, per story merged in its window, from data it already fetches plus ledger
  timestamps: time from Ready snapshot to first checkpoint (queue), first checkpoint to merge-gate green (build),
  merge-gate green to `ship` (**human wait**), and the total; P50 and P90 over the window. It adds **touches per
  story** (human-only actions in the actions log and corrections, `ship` excluded), loop counts (checkpoint attempts,
  `inspect` fix cycles), an `inspect` **acted-on rate** (posted finding IDs that disappear after a fix cycle, against
  human dismissals in `inspect/dismissed.jsonl`), an **escape rate** (PRs `inspect` passed that got a correction
  within `[autonomy] correction_days`), and the change mix by the squash subject's Conventional Commit type. `status`
  shows one rollup line.
- **Not.** No new findings for four weeks; the numbers are reported, not gated. No new config keys until thresholds
  are proposed from real data.
- **Proof.** A `retro --replay` test on a fixture snapshot with hand-computed, asymmetric expected values (one story
  with a long human wait, one with an escape). Metric: this repository's first four weekly reports form the baseline.
- **Owner.** Engineering Manager (Athena), with the Quality Architect for the review proxies.

### P2: a "test value" category in `inspect` (extension, XS)

- **What.** `inspect` may flag a test that only restates the implementation or duplicates another, always as P3:
  counted in the summary, never posted (D-4.8b.2 already counts P3 nits without posting them). `retro` reports the
  rate per builder family.
- **Proof.** One seeded case in `evals/cases/inspect/` with a redundant test; the `review` group keeps F1 at or above
  0.80 (baseline 0.83, D-4.8b.6).

### P3: CI repair through the existing follow-up loop (extension, S)

- **What.** When `build <n> --resume` or a `--delegate` sync finds a failed required check on the PR head, it sends one
  follow-up per head, as it already does for an `inspect` fix (D-5.5b.3): the check name and a bounded, redacted log
  tail, wrapped as untrusted data. It shares the two-cycle cap and `[autonomy.budget]`, then pages. `protected-paths`
  is excluded because it fails by design. The merge gate still decides; the builder only gets the failure sooner.
- **Proof.** A tier-1 test on a recorded check-run fixture: the follow-up names the job, quotes the tail inside a
  provenance block, and redacts a planted token. An injection eval case: a log line telling the agent to run `ship`
  changes nothing. Metric (P1): the share of agent PRs reaching green with zero human touches, before and after.

### P4: evolve discipline (extension, S)

- **What.** The `evolve` skill body gains three rules from S3: a skill change needs at least two independent stories
  as evidence (safety findings excepted); rewrite the affected section instead of appending; the PR names the owning
  surface (missing, wrong, or unclear instruction) and cites the stories. `retro` gains one finding: a Feedback that
  recurs within 30 days after its evolve PR merged is filed as "evolve ineffective" with the `install --pin` rollback.
- **Proof.** A tier-1 evolve case where a single-story Feedback must be declined; a `retro --replay` test for the
  recurrence finding. `skills/**` is a protected path, so the founder reviews the skill change as usual.

### P5: red-on-base proof (extension, M)

- **What.** For each criterion whose evidence map names tests, the merge gate's `verify --ci` also runs those tests
  against the base commit with the head's test files, in the same sandbox with no secrets. Each criterion records
  **red on base** (the test shows the fix), **green on base** (the test does not show the change), or
  **inconclusive** (the test does not build on the base, as with a new API). The pack and the proof comment carry the
  result and `inspect` reads it. A proposed key, `[verify] red_on_base = "advisory" | "bug" | "off"`, sets whether a
  green-on-base result blocks Bug stories.
- **Why.** It closes the gap the docs admit: the agent chooses which of its tests prove each promise
  (how-proof-packs.md). It is deterministic, needs no human, and makes self-certification harder, not easier.
- **Proof.** Tier-1 tests on three fixture repositories (a real fix, a vacuous test, a new-API test) with the three
  expected outcomes. Metric: the green-on-base rate on this repository's PRs over four weeks.

### P6: a risk line on the proof comment (extension, S)

- **What.** The pack step computes a deterministic tier: **high** for a never-layer footprint, a protected path, a new
  dependency, or a migration; **medium** for several layers or a large diff; **low** otherwise. The proof comment
  opens with the tier and "where to look" (the areas from `inspect`'s review plan). It approves nothing and changes
  no gate; it shortens the human pass.
- **Proof.** Table tests on footprints. Metric (P1): median human wait by tier.

### P7: ratchet floors (extension, M; this repository first)

- **What.** `[verify.ratchets.<name>]` names a command that prints one number, a direction, and a noise margin;
  ceilings live in a committed file. The gate fails when the head is worse than the base's ceiling plus the margin. A
  PR may tighten its own ceiling; loosening needs `decide --waiver` (ground rule 7's path, counted by `audit`), so no
  daily job and no extra PRs. A new ratchet stays advisory until reruns show its noise below the margin (B1's proxy
  check). First candidates: release binary size and allocations in one benchmark.
- **Proof.** Tier-1 tests: loosening without a waiver fails, tightening passes, a noisy ratchet stays advisory. Two
  weeks of dogfooding.

### 6.1 Sizes and sequence

| Wave | Change | Size | Depends on | Proof |
|---|---|---|---|---|
| 1 | P1 retro metrics | M | — | `retro --replay` fixture; four weekly baselines |
| 1 | P2 test value | XS | — | Seeded case; F1 ≥ 0.80 |
| 2 | P3 CI repair | S | P1 for the metric | Fixture, injection case, zero-touch share |
| 2 | P4 evolve rules | S | P1 for recurrence data | Evolve case, replay test |
| 3 | P5 red-on-base | M | — | Three fixtures; green-on-base rate |
| 4 | P6 risk line | S | P1 for wait by tier | Table tests; wait by tier |
| 4 | P7 ratchets | M | — | Gate tests; two-week dogfood |

## 7. Risks, and what we will not do

| ID | Risk | Earliest signal | Response |
|---|---|---|---|
| R1 | Metrics become targets (Goodhart) | A story split or merged to move a number | P1 reports only; only P7 gates, and only after its proxy check |
| R2 | P5 false alarms from flaky tests or a base that does not build | A high inconclusive or green-on-base rate on known-good fixes | The inconclusive class; advisory first (Q1) |
| R3 | CI logs carry secrets to a cloud receiver | The redaction test or gitleaks finds a token in a follow-up | Actions-log redaction, gitleaks patterns, a bounded tail |
| R4 | More review load on the founder from evolve and Feedback | Weekly count of Feedback and evolve PRs | The two-story rule, the cadence cap (Q3) |
| R5 | Missing timestamps on older stories distort P1 | Coverage below 80 % in the report | Report coverage; exclude pre-gate stories |
| R6 | Caption errors mislead this analysis | A claim that only one garbled passage supports | Numbers taken from B1 where possible; vendor claims labelled |

We will not: let any score approve or merge; add a second auto-merge path beside the streak; read host session
transcripts; capture production traffic; build a graph store, vector store, or MCP gateway; open scheduled
maintenance PRs; add a workflow language; or add a verb. Every proposal extends an existing verb, skill, or config
section.

## 8. Decisions for the founder

1. **Q1. Should a green-on-base result block Bug stories from day one?** Yes: stronger proof at once, with some false
   blocks on flaky tests. No (recommended): advisory for four weeks, then block for Bugs if false alarms are rare.
2. **Q2. Should CI repair start on its own when a check fails?** Yes: least friction, but it needs Level 3 (a bot App
   and an Actions trigger). No (recommended for now): it runs when `build --resume` or a delegate sync runs, at
   Level 2, and becomes automatic when the bot App is installed.
3. **Q3. How often should `evolve` run?** Quarterly (today): least review load, slow learning. Monthly: faster, more
   PRs. When a skill has two accepted Feedback issues, at most monthly (recommended): learning follows evidence.
4. **Q4. Should high-risk PRs get peer review automatically?** Yes: recall 1.00 but precision 0.46 at about five
   times the cost (D-4.8b.6). No (recommended): general review everywhere; the risk line points the human.
5. **Q5. Are ratchets for this repository only in 0.2, or a product feature?** Repository only (recommended): proves
   the proxy check first. Product feature: users get it sooner, with less evidence.
6. **Q6. Should aayudha ever score host session transcripts?** Never: simplest, keeps secrets out. Watch
   (recommended): revisit only if P1's deterministic proxies miss problems that Feedback keeps reporting.

## 9. What is uncertain

- About 25–30 % of S2's middle hour was not seen; an adoption figure or a guardrail detail there may be missing.
- All Warp, Uber, and Faire numbers are self-reported and unaudited; none is used as a target here.
- The absence claims in §5.1 (no trajectory capture, no lead-time code, no preview environments) rest on searches of
  this repository on 2026-10-15, not on reading every package.
- P1's human-touch count inherits the solo-mode attribution limit (how-trust.md, human fallback): actions an agent
  takes under the founder's token without the agent marker count as the founder's.
