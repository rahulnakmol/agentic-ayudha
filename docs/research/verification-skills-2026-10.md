# Repository-specific verification skills: pstack, Open Code Review, and aayudha

Research for the founder's question of 2026-10-15, relayed by the orchestrating thread: can aayudha give every
repository it sets up strong, repository-specific verification skills that an agent uses to prove a change works, at
four levels (feature, tooling, platform, repository)? Owner: the Quality Architect and AI Native Engineer twins.
Status: research and a prototype; nothing here is built into the product. The proposals in §9 are inputs to the
build plan, not decisions.

Labels: **[V]** checked in source (the file and commit are named), **[M]** measured in this study (the command or
artifact is named), **[I]** inferred. Sources were read at: `cursor/plugins` 99743e9 (pstack 0.15.25),
`poteto/verification-skill-example` (main, 2026-10-15), `alibaba/open-code-review` 11ac6d4 (v1.13.0-5), and
aayudha `dev` at 86275c2. The prototype and the comparison's raw data are on the unmerged branch
[`scratch/verify-skill-prototype`](https://github.com/rahulnakmol/agentic-ayudha/tree/scratch/verify-skill-prototype).

## 1. Summary

- **We have the proof half, not the driving half.** The merge gate builds the authoritative pack, `ship` lands it,
  and the floors are strong (§5). But the only way the gate can evidence a `screenshot` or `recording` criterion is
  to load one URL path written in backticks in the criterion and photograph it (`verify.States`,
  `internal/verify/web.go:84`; `page.goto` in `internal/verify/web/web.mjs:71`). It cannot type a question, press
  a button, or check what came back. Behaviour criteria are evidenced only by Go tests named in
  `.aayudha/evidence/<story>.toml` (`verify.TestEvidence`). No repository gets a map of its own features, a
  control CLI, or a maintained recipe for driving the app. The `verify` skill's body is a stub
  (`skills/verify/SKILL.md`: "Body lands in WP4.3").
- **pstack fills the driving half.** `/create-verification-skill` interviews the repository, writes a
  project-local skill with Launch, Doctor, Drive, Evidence, and Cleanup sections and a Feature Map (one file per
  feature, four fixed H2s), and must prove the skill end to end before handing it over.
  `/maintain-verification-skill` keeps the map honest with one source pass, one live pass, and at most one PR,
  confined to the skill's directory. Its premise differs from ours: "verification means that an agent can verify
  its own work" (Pt. 1 article). In aayudha that is a preview; only the gate's pack counts.
- **OCR adds little that inspect v2 does not already have.** WP4.8b took its main ideas (quote anchoring, a review
  plan, an asymmetric filter). Two ideas are left worth taking: a **sealed coverage manifest** (every selected file
  ends reviewed or skipped with a reason, and the run's state is derived from it) and **excluding tests from line
  findings**. Running `ocr` itself in the gate conflicts with D-4.8a (the inspect job never checks out PR code) and
  adds a second model loop.
- **The prototype works** [M]. A verify skill for `examples/benefits-qa` (five-feature map, a 409-line control
  CLI) drove nine journeys in 4.6 s. It produced Completion Goal evidence for a sample story: three criteria met
  and one reported unreachable with its reason. Its source pass caught a renamed button, a new route, and four
  stale features in 0.1 s. Its live pass caught the change the source pass missed. Two sweeps produced
  byte-identical artifacts (13 of 13, video excluded), so the gate can recompute what a preview claims.
- **OCR against inspect, on one model** [M]. We gave inspect's real prompt and OCR's delegation mode to the same
  host model, over the twelve seeded cases, PR 182 as merged, and PR 182 with three planted defects. Both found
  every planted defect (13 of 13). On the seeded set, precision was 0.91 for OCR's flow and 0.56 for inspect's.
  Every one of inspect's eight false positives was a P2 "no test for …" finding on a test file, which OCR's flow
  never reviews. Without those findings, inspect's precision is 1.00.
- **Recommendation.** Adopt pstack's skill shape. Adapt it so the gate, not the agent, runs the journeys: journeys
  are deterministic, committed, and mapped to criteria. Take OCR's coverage manifest and test-file rule as ideas.
  Reject vendoring either project. Sequence: inspect precision fixes first (cheap, measured); then a journeys floor
  and a deterministic map-drift check; then the generating and maintaining skill; then grounding for grill, plan,
  and inspect; then a nightly sweep loop.

## 2. The four levels

| Level | What "prove it works" means | Example in benefits-qa |
|---|---|---|
| Feature | A user-visible behaviour, driven the way a user drives it, with the result captured | Type a question, choose Ask, see the quoted answer and its source |
| Tooling | The harness that makes the feature level cheap and repeatable: launch, health check, drive, capture, clean up, and the review tools around it | `deploy/serve.sh`, a control CLI, the merge gate's floors, `inspect` |
| Platform | The app in an environment beyond a laptop: CI, staging, cloud, mobile, browsers | The gate's sandbox; `[verify.environments]`; DAST and k6 against a target |
| Repository | The whole product at once: every feature still works, the map still matches the code, reviews cover every change | A nightly sweep of every journey; map drift; review coverage |

## 3. pstack, from its source

### 3.1 `/create-verification-skill` [V]

Source: `pstack/skills/create-verification-skill/SKILL.md` and `references/feature-map-example/`.

1. **Interview the repository, not the user.** Surface, run command, how to drive it (existing harnesses first;
   then browser or CDP, a PTY or tmux, or plain HTTP), what to observe, and whether two instances can run side by
   side. If the checkout does not build, fix that or report it before generating.
2. **Generate** `.cursor/skills/verify-<app>/SKILL.md` with frontmatter and six sections. **Launch** must reach a
   usable state from an empty data directory ("ready means usable, not just that a process answers"). **Doctor** is
   one read-only "is this instance worth driving?" check. **Drive** uses real selectors from the repository. The
   **Evidence** rules are: the real user path, the action and the resulting state, side effects checked, and mocks
   only behind an existing production boundary. **Cleanup** never kills by process name and never deletes evidence.
   **Helpers** must be executable and documented.
3. **Seed the feature map**: `features/README.md` plus one file per top feature, each with four H2s in order:
   `Sub-features`, `How to get to it (user POV)`, `Driving it with <harness>`, `Gotchas`. "Never weaken the journey
   to make it pass." "A proof that drives one convenient entry point is incomplete when the map lists others."
4. **Prove the generated skill** once end to end (launch, doctor, one feature, evidence, cleanup), and check that
   the evidence survives cleanup. "A generated skill that was never executed is a draft, not a deliverable."
5. Offer the maintenance loop.

The example repository (`poteto/verification-skill-example`) is a fictional desktop app with 33 feature files and a
`control-atlas.mjs` whose code is withheld; only its command surface is shown. It names the traits of an agent-friendly
CLI: few composable commands, `--dry-run` on anything destructive, errors that say what to do, JSON output. The
guide (`docs/guide/06-verify-and-ship.md`) adds "build the lever" (a control CLI instead of throwaway scripts) and
"commit it, so every person and every agent drives the app the same way".

### 3.2 `/maintain-verification-skill` [V]

Outcomes are exactly `clean`, `changed` (one PR of proven corrections), or `blocked`. The edit scope is the skill's
own directory, never product code. The pass runs in order:

1. Index hygiene.
2. A **source wave**: one read-only subagent per feature file returns a summary, entry points, likely drift with
   citations, and one recipe.
3. Reconcile, and sweep recent churn for unmapped surfaces, with a concrete source path required.
4. A **live pass**, required even when the source looks clean. It holds three invariants: doctor before driving and
   after any surprise, evidence survives every cleanup, and nothing a drive started outlives it.
5. Triage. Doc drift is fixed in the map; a harness gap is fixed in the CLI; a product regression is reported and
   kept out of the PR.
6. Ship or stop.

The pstack article recommends running it daily as a cloud automation.

### 3.3 The principles it leans on [V]

| Principle | The rule | Where aayudha already agrees |
|---|---|---|
| prove-it-works | Check the real artifact; "a deterministic script that re-runs the same comparison, not a one-time eyeball" | ETHOS 1 and 2 |
| build-the-lever | Build the tool that does or proves the work; "a deterministic script turns 'trust me' into 'run this'" | Floors are levers; evidence maps are levers for tests |
| test-behavior-not-implementation | Assert what users observe; a test must fail when behaviour breaks and survive refactors | Not encoded in any aayudha skill today |
| sequence-verifiable-units | Small units, each ending in a checkable state; deliver in an order that proves itself (failing test, then fix) | Checkpoints with `Aayudha-Checkpoint` trailers (WP4.10) |
| explain-the-number | Name the limiter, rule out what else the number could measure, keep run count and spread | `brief`'s evidence rule; not applied to eval or perf numbers |
| `/swarm` | Fan out N workers by slice or race, aggregate with a declared rule, a missing result is a gap, not a pass | Peer mode (two reviewers); no fan-out of verification |

pstack's **Shipping** playbook goes further than its own definition of verification. "Only a verdict from an agent
that did not write the code counts, because an author checks what it meant to build. Green CI and an approving bot
review are not verdicts" (`playbooks/shipping.md`). That is aayudha's rule, held by a fresh agent instead of by CI.

## 4. Alibaba Open Code Review, from its source

This updates [agent-delegation-and-review.md §2](agent-delegation-and-review.md) to v1.13.0-5 (11ac6d4) [V].

- **Pipeline.** Deterministic file selection (`internal/config/rules/system_rules.json` maps 54 rule documents by
  path glob; tests (`default_path`) and Markdown (`unsupported_ext`) are excluded by default). Then grouping, a
  per-file plan pass (`PLAN_TASK`, `--no-plan` to skip), and a tool-using review loop (`code_search`, `file_read`,
  …). Then the `REVIEW_FILTER_TASK` ("remove only the comments this diff proves wrong") and de-duplication.
- **Forced coverage.** `Agent.registerCoverage` (`internal/agent/agent.go`) registers every non-deleted diff and
  seals the set before any dispatch. `session.Manifest.Finalize` derives the run's `terminal_state` (`complete`,
  `partial`, `failed`, `skipped`) from that sealed denominator, so a file the model never reached cannot be
  reported as reviewed. The delegation skill makes the same rule explicit for host agents: every reviewable file
  ends `reviewed` or `skipped` with a reason, and the summary reports `coverage_rate`.
- **Line anchoring.** A finding carries `existing_code`; a sliding-window matcher (`internal/diff/relocation.go`)
  places it, a model call relocates a miss, and the rest is posted without lines.
- **Depth levels.** `--effort low|medium|high` sets `MaxReviewRounds` to 1, 2, or 3
  (`internal/config/template/effort.go`).
- **Delegation mode.** `ocr delegate preview` and `ocr delegate rule` do the deterministic part (file list,
  exclusions, rule groups) with no model; the host agent reviews
  (`plugins/open-code-review/skills/open-code-review-delegate/SKILL.md`).
- **CI.** `action.yml` runs the CLI on a PR with inline comments, a sticky summary, and incremental posting; SARIF
  output; GitLab, GitFlic, and Gerrit recipes.
- **Benchmark.** AACR-Bench (200 PRs, 50 repositories, 1,505 annotated issues); the README reports higher precision
  and lower recall than a general agent at about a ninth of the tokens (vendor numbers).
- **Licence.** Apache-2.0, no NOTICE file.

## 5. What aayudha has today (each claim checked)

| Claim | Holds? | Evidence |
|---|---|---|
| `verify` runs deterministic floors from CONSTRAINTS.md | Yes | `internal/verify/verify.go` (`Floor`, `Execute`); thresholds in `internal/config/defaults.toml` `[verify.floors.*]`, rendered to `CONSTRAINTS.md` |
| Proof packs are built by the merge gate as run artifacts and landed by `ship` | Yes | Appendix B S-4.2b, D-4.2b.a; `gatePack` in `internal/cli/ship.go`; `aayudha proof comment --ci` |
| Completion Goals hold typed criteria | Yes | `workitems.EvidenceKinds` = screenshot, recording, sr-transcript, api-trace, metric (`internal/workitems/schema.go:124`) |
| `inspect` v2: context retrieval, plan, quote-anchored findings, noise filters, `--dismiss`, general and peer modes | Yes | `internal/inspect/context.go`, `filter.go`, `peer.go`; D-4.8b.1 to D-4.8b.7 |
| Tier-1 and tier-2 skill evals | Yes | `internal/evals/tier1.go`, `tier2.go`; `evals/eval.toml`; the `review` group with twelve seeded cases |
| `aayu init` scaffolds product repositories | Yes | `internal/initrepo/init.go` (config, `.aayudha/seed/README.md`, gates, labels) |
| `examples/benefits-qa` is a full example product | Yes | nine layers, `deploy/serve.sh` is this repository's `[verify] seed` (`.aayudha/config.toml`) |
| How a screenshot or recording criterion is evidenced | Static URL only | `verify.States` takes the backticked path from the criterion's state, else `/`; `web.mjs` calls `page.goto`, then the a11y checks. No click, type, or assertion of the result |
| How behaviour criteria are evidenced | Named Go tests | `verify.TestEvidence` reads `.aayudha/evidence/<story>.toml` `tests`, selects their `go test -json` events from the coverage floor's run |
| A repository-specific verify skill or feature map | No | `skills/verify/SKILL.md` body is a stub; `install` copies only aayudha's own skills and leaves the product's alone (`hosts.skillCopies`) |

## 6. Gap analysis

"Owner" names the part of aayudha that would own the change: a verb, a skill, a seat, config, or a workflow. "Best
source" is the project whose design fills the gap better, or neither.

### 6.1 Feature level

| # | Gap | Real? | Owner | Best source |
|---|---|---|---|---|
| G1 | The gate cannot drive a journey (type, click, submit, assert the result) to evidence a screenshot or recording criterion | **Real** (§5) | `verify`, new journeys floor; seat AI Native Engineer, standard Quality Architect | pstack (Drive and Evidence rules), made deterministic |
| G2 | No map of the product's features, entry points, and stable handles | **Real** | A generated product skill `.agents/skills/verify-<app>/`; config `[verify] skill` | pstack (Feature Map) |
| G3 | A criterion cannot name a journey as its evidence | **Real** (evidence map holds `tests` only) | `.aayudha/evidence/<story>.toml` gains `journeys`; `plan` writes the skeleton | Neither: aayudha's own evidence map |
| G4 | The builder has no cheap, faithful preview of what the gate will say | **Partial** (`aayu verify` runs locally; previews are advisory) | `build` skill; the same control CLI | pstack |
| G5 | Unit tests check implementation, not behaviour | **Partial** (coverage floor counts lines, not assertions) | `build` and `inspect` guidance; Quality Architect | pstack (test-behavior-not-implementation), as guidance only |

### 6.2 Tooling level

| # | Gap | Real? | Owner | Best source |
|---|---|---|---|---|
| G6 | No control CLI per repository: launch, doctor, drive, evidence, cleanup, JSON out | **Real** | `init --verify-skill`; DevOps & DevEx Tinkerer owns the harness | pstack (build-the-lever and CLI traits) |
| G7 | Launch has no freshness check (a stale build is driven without notice) | **Partial** (`[verify] seed`, production refusal, `serve.sh` pid check exist) | The control CLI's `doctor` | pstack (Doctor) |
| G8 | inspect does not record which changed files it reviewed and which it skipped | **Real** (`BuildContext` reads at most 40 files and 40 kB; nothing says what was left out) | `inspect`, state and comment | OCR (sealed coverage manifest) |
| G9 | inspect posts test-gap findings on test files as line findings | **Real** [M]: all 8 false positives in §8 | `inspect` filter and prompt | OCR (tests excluded from line review) |
| G10 | No per-language review checklists | **Partial** (risk patterns per area exist) | `inspect` plan | OCR, as ideas; measure first |

### 6.3 Platform level

| # | Gap | Real? | Owner | Best source |
|---|---|---|---|---|
| G11 | No journeys against a deployed non-production environment after release | **Partial** (DAST and k6 can target `[verify.environments]`) | `release` and a loop; `[verify.environments]` | pstack (the map is target-agnostic); aayudha supplies the environments and the production refusal |
| G12 | Mobile and desktop drivers | **Deferred** (WP4.9 to 0.2) | `verify` | Neither; pstack's generator is generic but proves nothing about native drivers |
| G13 | The gate cannot check that a preview's evidence matches its own | **Real** | `proof comment`, `ship` | Neither: aayudha's own, made possible by deterministic journeys (§7.4) |

### 6.4 Repository level

| # | Gap | Real? | Owner | Best source |
|---|---|---|---|---|
| G14 | No whole-product regression sweep | **Real** | A `verify-sweep` loop (`loops/`), nightly on `dev` | pstack (README order; `/swarm` by feature) |
| G15 | No way to notice that the verification recipe drifted from the code | **Real** | `aayu verify --map` (deterministic) and the maintain procedure in the `verify` skill; a loop | pstack (maintain), plus a deterministic source pass it does not have |
| G16 | grill, plan, and inspect do not know the product's features | **Real** (grill `--assess` reads routes and files, not features) | `grill --assess`, `plan`, `inspect.BuildContext` | pstack's map as the source |
| G17 | Reviewer quality is measured only on synthetic PRs | **Partial** (twelve seeded cases) | `evals` review group | OCR (AACR-Bench style real PRs; check the dataset licence first) |

### 6.5 Keeping "a builder cannot certify its own work"

Repository-specific verify skills **strengthen** the rule where:

- **The gate runs the journeys, not the agent.** Journeys are committed, deterministic steps; the gate's verify job
  runs them on the PR head in its sandbox and packs the artifacts. Criteria that today can only be evidenced by a
  static page load, or by a builder's own screenshot, become CI evidence.
- **Evidence becomes reproducible.** Two sweeps produced byte-identical screenshots, ARIA snapshots, action logs,
  and API traces [M] (§7.4). The gate can recompute a preview's hashes, and a mismatch is a finding about the
  environment or the claim.
- **The map makes coverage reviewable.** A feature lists every entry point. inspect can ask whether a PR that
  touches a feature's sources maps a journey for each criterion, and a human can see which entry points were left
  undriven ("a proof that drives one convenient entry point is incomplete").
- **The reviewer stays independent.** pstack's Shipping rule ("only a verdict from an agent that did not write the
  code counts") is the same rule. aayudha keeps it in CI, which is cheaper and harder to game than a second agent.

They could **weaken** it where:

- **The builder edits the journey with the code.** A journey can be relaxed (a removed `expect`, a changed expected
  text) in the same PR whose behaviour it proves. Today the same risk exists for the tests in the evidence map.
  Mitigation: inspect diffs `features/**` and raises a deterministic risk "journey relaxed in this PR"; the Ready
  snapshot freezes which journey IDs a criterion maps to; a founder decision (Q2) on whether maps are
  protected paths.
- **The preview is taken for proof.** The prototype marks every result `"authority": "preview"`; `ship` already
  refuses previews (S-4.2b). The generated SKILL.md says so in its first section.
- **Test-only doors.** A control CLI that calls internal setters proves nothing. The map's conventions forbid it;
  inspect can flag a new route or flag that only the CLI uses.
- **Retry until green.** A flaky journey invites reruns. The gate runs once and reports a flake as a failure;
  Talos's "never reruns a floor until it passes by chance" applies.
- **The map as an injection channel.** Feature files are repository content; when a model reads them (grill,
  inspect, maintain), they are wrapped as untrusted data like every other file (§0.1).

## 7. Prototype: a generated verify skill for benefits-qa

On `scratch/verify-skill-prototype` (commits 20b0562, 30468e3, 4affbd6, 361832a). Not merged; nothing in the product
reads it.

### 7.1 Shape

```text
examples/benefits-qa/.agents/skills/verify-benefits-qa/
  SKILL.md            launch, doctor, drive, evidence, cleanup; "a run here is a preview"
  control-bqa.mjs     up | doctor | list | journey | sweep | evidence | drift | down   (JSON on stdout)
  features/
    README.md         baseline, conventions, proof and skip rules, entry contract, sweep order
    ask-answer.md     3 journeys      refer-to-hr.md  1 journey     refusals.md  2 journeys
    api-ask.md        2 journeys      product-events.md  1 journey (unreachable through the seed launch)
```

Each feature file keeps pstack's four H2s and adds two fenced JSON blocks: one `feature` block (`id`, `layers`,
`sources`, `routes`, `verified_at`) and one `journey` block per journey. The same text is the recipe an agent reads
and the script the CLI runs, so there is one source. A journey is a list of steps: `goto`, `fill` by label, `click`
by role and name, `expect` a role and name, `expectText`, `absentText`, `count`, `screenshot`, and `api` (method,
path, body, expected status and body subset). `up` runs `deploy/serve.sh` (the repository's own `[verify] seed`) on
its own port. `doctor` checks that the pid on `/healthz` is the one `up` started and that the build is from `HEAD`.

### 7.2 Driving and Completion Goal evidence [M]

- `control-bqa sweep`: 8 journeys passed, 1 unreachable, in 4.6 s wall time. The unreachable one is the product
  events journey: `serve.sh` starts the app without `-events`, so it reports
  `unreachable` with that reason instead of a pass through another path.
- `ask-answer.typed` wrote `actions.jsonl`, `answer.png`, `recording.webm`, `aria.txt`, `api-trace.jsonl`, and
  `result.json` with every artifact's SHA-256. The ARIA snapshot shows the region "Answer" with the quoted passage
  and "Source: Dental, Deductible".
- `control-bqa evidence --story 412 --criteria criteria-412.txt` read four Completion Goal lines
  (`N. [kind] state: …; expected: …`) and an evidence map with `journeys`. It wrote `evidence.json` in `verify`'s
  artifact shape (`path`, `kind`, `sha256`, `criteria`) with `"authority": "preview"`. Criteria 1 (recording), 2
  (screenshot), and 3 (api-trace) are met. Criterion 4 is listed under `unmet` with the reason. A criterion's kind
  must match an artifact's kind: a recording criterion is not satisfied by a screenshot.

### 7.3 Maintain pass: detecting drift [M]

Commit 30468e3 simulates a product change: the Ask button becomes "Get answer", `GET /api/plans` is added, and the
referral reply time changes from 2 to 3 business days.

| Pass | What it found | Time |
|---|---|---|
| `doctor` | `STALE BUILD: built from 86275c2, HEAD is 30468e3` | 0.1 s |
| `drift` (source) | Handle "Ask" missing from the sources of 3 features; route `GET /api/plans` not in the map; 4 features stale since `verified_at` | 0.1 s |
| `sweep` (live, after a rebuild) | 3 journeys fail at "click button Ask" | 19 s |
| `sweep` after fixing the button in the map | `refer-to-hr.typed` fails at "within 2 business days": the change the source pass cannot see | not timed |
| `sweep` and `drift` after the map is corrected | 9 passed, 1 unreachable; `drift` clean | not timed |

The corrections (commit 4affbd6) touch only `features/`, the maintain pass's edit scope. The deterministic source
pass is free and fast, and the live pass is still required, as pstack says.

### 7.4 Reproducibility [M]

Two sweeps at the same commit and port (`repA`, `repB`): every non-video artifact was byte-identical, 13 of 13
(screenshots, ARIA snapshots, action logs, API traces). Recordings differ by encoding. A gate that re-runs a journey
can therefore compare its hashes with a preview's.

### 7.5 What the prototype does not prove

It ran in an orb, not in the merge gate's sandbox. It reuses the floors' Playwright (`packages/floors`) and needed
Playwright's ffmpeg for video. The map was written by hand from source, the way the create skill would, so the
quality of a model-generated map is unmeasured. It covers a web app and an API, not a CLI-only or mobile product.

## 8. OCR against inspect on the same diffs

### 8.1 Method

The orb's AI gateway key was refused (HTTP 401 on every route), so neither inspect's review lane nor OCR's own
agent loop could call a model here. Instead both pipelines ran on one host model (Amp subagents), blind to the
answers. That isolates the pipelines from the model:

- **inspect arm.** `reviewcmp prep` renders inspect's real `System`, schema, and `Prompt(in)`; for the seeded cases
  `in` comes from `evals.ReviewInput`, with the plan and retrieved context. The subagent answers as one call with
  no tools. `reviewcmp inspect` runs the reply through `inspect.Runner` (quote placement, `Filter.Clean` with
  `min_confidence` 0.5, de-duplication), so only what inspect would post is scored.
- **OCR arm.** The subagent follows OCR's delegation skill with `ocr` v1.13.0-5 built from source:
  `delegate preview`, `delegate rule`, per-file diffs, review with repository context, and a coverage checklist.
  It writes OCR's comment shape, which `reviewcmp ocr` maps to severities (critical to P0, high to P1, medium to P2,
  low to P3).
- **Scoring.** `evals.ScoreFindings`, the matcher of the `review` eval group: a finding within 3 lines of an
  unmatched planted defect in its file is a true positive; anything else is a false positive.
- **Inputs.** The twelve seeded cases (10 planted defects, 3 clean controls); PR 182 as merged (10 files, +283/−29);
  and PR 182 with three planted defects in its real code: partial build records counted as covering the PR
  (`builder.go:53`), the build lane never a candidate (`builder.go:77`), and tree SHAs instead of commit SHAs
  (`cli/inspect.go:350`).

### 8.2 Results [M]

| Set | Pipeline | Posted | TP | FP | FN | Precision | Recall |
|---|---|---|---|---|---|---|---|
| Seeded (12 cases) | inspect prompt and filters | 18 | 10 | 8 | 0 | 0.56 | 1.00 |
| Seeded (12 cases) | OCR delegation | 11 | 10 | 1 | 0 | 0.91 | 1.00 |
| Seeded, clean controls only | both | 0 | – | 0 | – | – | – |
| PR 182 with 3 planted | inspect prompt and filters | 3 | 3 | 0 | 0 | 1.00 | 1.00 |
| PR 182 with 3 planted | OCR delegation | 3 | 3 | 0 | 0 | 1.00 | 1.00 |
| PR 182 as merged | inspect prompt and filters | 1 | – | – | – | see below | – |
| PR 182 as merged | OCR delegation | 1 | – | – | – | see below | – |

- **inspect's false positives are all one kind.** All 8 are P2 `test` findings on `*_test.go` files ("No test for
  the failing-rename case in criterion 2"); confidence 0.7 to 0.85, so `min_confidence` keeps them. OCR's flow
  excludes test files from review by default, so it cannot raise them. Without them, inspect's precision on the
  seeded set is 10/10.
- **OCR's one false positive is arguable**: a missing fsync before an atomic rename (`ignored-rename-error`), real
  in general but not planted.
- **PR 182 as merged.** Both arms raised the same single finding: the error from listing the PR's commits is
  dropped (`if cs, err := …; err == nil`), so a GitHub failure reads as "no build record" in the verdict's note. I
  checked the code: the fallback is safe (more candidate families are excluded, not fewer), but the note
  misreports its source, so the finding is real at P2–P3. Production inspect (gpt-6.1-sol, run 38696655853) posted
  no finding on this PR.
- **Same model, different results from production.** The seeded set's production numbers with gpt-6.1-sol were
  precision 0.73 and recall 0.95 (D-4.8b.6, k = 2). The host model here found more and was noisier on tests. The
  pipelines, not the models, are what this study compares.

### 8.3 Cost and time [M]

| Measure | inspect | OCR delegation |
|---|---|---|
| Model calls per review | 1 (2 in peer mode) | An agent loop (native mode: up to 100 tool calls per group, 2 rounds at medium effort, plus plan and filter passes) |
| Fixed input before any context reads, seeded cases | 6.1–7.6 k characters (system, story, criteria, diff, plan, context) | 22.0–22.6 k characters (the skill at 9.3 k, the Go rule group at 11.1 k, preview, diff) |
| Fixed input, PR 182 | 49.5 k (40 kB of retrieved context) | 38.6 k, then whatever files the agent reads |
| Wall time per seeded review, host subagent, median | 14.3 s | 10.9 s |
| Wall time, PR 182 / planted | 55 s / 49 s | 74 s / 36 s |
| Production cost, PR 182 | 0.37 USD per cycle (gpt-6.1-sol); the gate's Inspect step took 40 s | Not measured: native mode needs its own model key |

The times are subagent wall times, including reading the inputs. They show the two flows are of the same order on
small diffs; they are not model latencies. Native OCR spends more calls by design, while inspect spends more
retrieved context per call.

### 8.4 What this changes

Do not add OCR as a pre-pass or a reviewer inside the merge gate. On this evidence it finds nothing that inspect's
prompt misses, and it would need its own model key, a second loop, and a PR checkout in the inspect job (D-4.8a
forbids that). Do take two of its rules: review only non-test code for line findings (test gaps become criterion
verdicts), and record a sealed coverage manifest.

## 9. Proposal

### 9.1 Adopt, adapt, reject

| Idea | Source | Verdict | Why |
|---|---|---|---|
| Project-local verify skill with Launch, Doctor, Drive, Evidence, Cleanup | pstack | **Adopt** (shape, our text) | It is the missing driving half; the prototype proved it on benefits-qa |
| Feature Map, four H2s, one file per feature | pstack | **Adopt**, plus a JSON `feature` block | The JSON block gives drift checks and grounding a deterministic source |
| Journeys as prose an agent follows | pstack | **Adapt**: fenced JSON steps the gate executes | Rule 7: CI evidence must not depend on a model following prose |
| "Prove the generated skill before handing it over" | pstack | **Adopt** as the create procedure's acceptance and a tier-1 eval | It is the eval |
| Maintain: source wave, live pass, one PR, edit scope | pstack | **Adapt**: a deterministic `verify --map` first, the model wave only for what it cannot see | 0.1 s and free against a subagent per feature |
| Agent self-verification as the definition of done | pstack article | **Reject** as proof; keep as preview | ETHOS 1, S-4.2b |
| `/swarm` fan-out | pstack | **Adapt** as a CI matrix by feature for sweeps | Deterministic fan-out needs no model |
| test-behavior-not-implementation, explain-the-number | pstack | **Adopt** as guidance in the `build` and `inspect` skills and the eval README | Cheap; no code |
| Sealed coverage manifest and terminal state | OCR | **Adopt** (idea) in inspect | Fills G8 |
| Tests excluded from line findings | OCR | **Adapt**: test gaps become criterion verdicts | Fixes all 8 false positives measured |
| Per-language rule documents | OCR | **Defer**; measure after the two fixes above | No recall gap measured |
| `ocr` as a gate pre-pass or a peer reviewer | OCR | **Reject** | D-4.8a, cost, no measured gain |
| Effort levels | OCR | **Reject** for the gate | inspect is one call; `--mode peer` is the depth switch |

### 9.2 Changes

Names in the WP column are suggestions for the build plan.

| # | Change | WP | Size | Proof it works | Licence |
|---|---|---|---|---|---|
| P1 | inspect: test-gap observations become criterion verdicts (`unclear` or `unmet`) or plan notes, never line findings on `*_test.go`, when the pack maps the criterion to passing tests | WP4.8c | S | Re-run the `review` eval group (k = 2): general-mode precision ≥ 0.85 at recall ≥ 0.95; a tier-1 case with a test-gap reply posts no line finding | None (idea) |
| P2 | inspect: sealed coverage manifest: every changed file is reviewed, or skipped with a reason (generated, test, over the 40-file or size cap); the comment and state show it, and the run is `complete` or `partial` | WP4.8c | S | Tier-1: a 45-file PR lists 5 skipped with reasons and is `partial`; a clean 3-file PR is `complete` | None (idea; credit OCR in this doc) |
| P3 | `verify` journeys floor: criteria mapped to `journeys` in `.aayudha/evidence/<n>.toml` run through the repository's control CLI contract (`<cli> journey <id> --out <dir>` returning `result.json`) in the sandbox; kind must match; unmet blocks like a missing test | WP4.5b | M | Tier-1 with a fixture app: pass gives hashed artifacts; failed and kind-mismatched criteria are unmet; production refusal still exits 4. Live: one benefits-qa story through the gate | Playwright already recorded (Apache-2.0) |
| P4 | `aayu verify --map`: deterministic drift (index, H2 contract, handles in sources, routes not in the map, sources changed since `verified_at`), exit 1 on drift | WP4.5b | S | Tier-1 with the §7.3 scenario: rename, new route, stale features named; clean map exits 0 | None |
| P5 | The `verify` skill gains two procedures, create and maintain, written for aayudha | WP3.5 | M | Tier-1 case: a recorded create run on a fixture repository yields a map that passes `verify --map` and one proved journey. Tier-2 rubric case for maintain on the §7.3 scenario. Both added before the skill text (eval-first) | MIT credit only if text is copied; the plan is to write our own, so none |
| P6 | `aayu init --verify-skill` scaffolds `.agents/skills/verify-<app>/` (README, entry contract, a CLI stub per surface: web, HTTP, or PTY) and `[verify] skill`; `install` mirrors product-owned `verify-*` skills to every host directory, recorded in `lock.toml` as product-owned; `doctor` gains a `verify-skill` row | WP2.6b | S | `TestInstallIntoExistingRepo` extended: a product `verify-*` skill is mirrored and never overwritten; doctor reports missing, stale, and drift | None |
| P7 | Grounding: `grill --assess` emits `feature:<id>` facts from the map; `plan` writes `journeys` into the evidence-map skeleton for criteria that name a mapped feature; inspect's plan lists the touched features and raises "journey relaxed in this PR" when an `expect` step is removed or its text changes | WP5.3c | S | grill tier-1 case cites a feature fact; plan test; inspect tier-1 case with a relaxed journey | None |
| P8 | `verify-sweep` loop: nightly on `dev`, every journey against a non-production `[verify.environments]` target, one CI matrix leg per feature; failures go to `triage` as Bugs; drift opens one maintain PR limited to the skill's directory | WP7.1c | S | Loop dry-run test; one live nightly on benefits-qa with a seeded regression caught | None |
| P9 | Preview and gate hash comparison in the proof comment (G13) | WP4.2c | S | Tier-1: matching and mismatching previews | None |

### 9.3 Sequencing

1. P1 and P2: cheap, measured, and independent of the rest.
2. P3 and P4: the deterministic core. The gate can execute journeys before any model writes them.
3. P5 and P6: the generating and maintaining skill, and the scaffold that puts it in every repository.
4. P7: grounding, once maps exist.
5. P8, then P9.

### 9.4 Licence notes

- Nothing is vendored by this proposal. Ideas are credited here, as the earlier research did for OCR.
- If any pstack text is copied (for example the four-H2 entry contract wording), add a THIRD_PARTY_NOTICES row:
  pstack, MIT, "Copyright (c) 2026 Lauren Tan", with the licence text kept with the copied file.
- If any OCR rule document or prompt is copied, Apache-2.0 §4 applies: the licence text, a notice of modification
  in each changed file, and a THIRD_PARTY_NOTICES row. OCR has no NOTICE file to carry.
- The generated control CLI uses the product repository's own Playwright; aayudha already records Playwright
  (Apache-2.0) for its floors.

## 10. Decisions for the founder

| # | Question | If yes / option A | If no / option B |
|---|---|---|---|
| Q1 | Should a journey mapped to a criterion **block** the merge gate when it fails, like a missing test does today? | Recommended. Criteria get CI evidence of behaviour; a flaky journey blocks until fixed | Journeys are advisory in the pack; screenshot criteria stay static page loads in the gate |
| Q2 | May a PR change a feature map's journeys together with the code they verify? | Recommended: yes, and inspect flags a relaxed journey. Least friction | No: `features/**` becomes a protected path needing a human CODEOWNER review. Stronger, slower, and solo mode needs the bypass |
| Q3 | Which language does the generated control CLI use? | Recommended: Node with the product's Playwright for web, plain HTTP for APIs, a PTY for CLIs. Matches the floors | One Go driver inside `aayu`: one binary, but a browser-automation dependency in the CLI and less fit to each repository |
| Q4 | How often does the maintain pass run with a model? | Recommended: deterministic `verify --map` on every PR (free), the model pass weekly and on drift | Daily, as pstack suggests: fresher maps at about seven times the model spend |
| Q5 | What role does OCR play? | Recommended: ideas only (P1, P2), credited | An optional `ocr` reviewer in peer mode, with its own key and a PR checkout in the review job (reopens D-4.8a) |

## 11. Sources

- pstack: <https://github.com/cursor/plugins/tree/main/pstack> at 99743e9: `skills/create-verification-skill/`,
  `skills/maintain-verification-skill/SKILL.md`, `skills/principle-*`, `skills/swarm/SKILL.md`,
  `skills/poteto-mode/playbooks/shipping.md`, `docs/guide/06-verify-and-ship.md`,
  `automations/benny/skills/reproduce-and-fix-issues/references/control-adapter.md`.
- Example output: <https://github.com/poteto/verification-skill-example>.
- Article: "The Complete Guide to pstack Pt. 1", <https://x.com/poteto/article/2094457600259842065> (2026-08-31),
  read through search excerpts; the full page needs a sign-in.
- OCR: <https://github.com/alibaba/open-code-review> at 11ac6d4: `internal/agent/agent.go`,
  `internal/session/manifest.go`, `internal/config/template/effort.go`, `internal/config/rules/system_rules.json`,
  `cmd/opencodereview/delegate_cmd.go`, `plugins/open-code-review/skills/open-code-review-delegate/SKILL.md`,
  `action.yml`.
- aayudha: `internal/verify/web.go`, `internal/verify/evidence.go`, `internal/verify/web/web.mjs`,
  `internal/inspect/inspect.go`, `internal/evals/review.go`, `internal/hosts/install.go`, `skills/verify/SKILL.md`,
  docs/BUILD-PLAN.md Appendix B (S-4.2b, D-4.8b.*).
- This study's data: branch `scratch/verify-skill-prototype`, `research/verification-skills-2026-10/` (scores,
  timings, replies, the PR 182 cases, the prototype's evidence) and `internal/evals/cmd/reviewcmp`.
