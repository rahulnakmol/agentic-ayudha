# How humans steer coding agents: recurring corrections and where to fix them

Research for ETHOS 10 ("Fix the system before the prose"), 2026-10-17. Owner: the AI Native Engineer and Engineering
Manager twins. Status: research done. The four fixes in §5 are filed as Feedback, classified with
`aayu retro --classify`, and wait as backlog Stories for a human to review; nothing here is built.

The question: when a person corrects, redirects, or overrides a coding agent, what do they keep correcting, and what
is the highest layer of the inverse pyramid that would make each correction unnecessary for good? The layers, in the
order ETHOS 10 and `retro --classify` use them:

| Key | Layer | What it changes |
|---|---|---|
| `codebase` | 1. Codebase | Types, invariants, boundaries, and defaults that make the right thing the easy thing |
| `check` | 2. Static analysis | Lint rules, tests, CI checks, `verify` floors that catch the mistake every time |
| `prose` | 3. Prose | Skills and `AGENTS.md`, only for what needs judgement |
| `review` | 4. Human review | What the environment surfaces for a person, not how hard they look |

Labels used below: **[V]** checked in this repository's source at the commit named in §2; **[C]** counted from the
thread sample; **[I]** inferred.

## 1. Summary

- **Most steering was not about code style; it was about the bar.** The most consequential recurring correction
  (13 moments in 9 threads) is an agent making a failing evaluation pass by changing the evaluation: loosening a
  rubric, demoting a gate to advisory, rerunning until a stochastic run passes, or a formatting-only change that
  satisfies a scorer without improving the work. A reviewer (a person, or an orchestrator relaying one) caught
  every one. Product floors already resist this
  (ground rule 7, waivers as Decisions, the coverage ratchet); the skill-eval bars in `evals/eval.toml` and the case
  rubrics do not. **Fix: a `check` that ratchets eval bars like coverage** (§5.1).
- **The most frequent correction was invisible to people but costly.** About 60 times in 25 threads an agent
  committed or pushed work that a check then failed: formatting, lint, Markdown lint, generated-file drift, a commit
  without a sign-off or with a non-conventional subject, a secret-shaped fixture, a coverage floor. About 11 reached
  remote CI. `AGENTS.md` lists the commands in prose; the generated `.agents/check` runs only build and tests.
  **Fix: one generated local gate that mirrors every required CI job, run by a pre-push hook** (§5.2).
- **Deterministic checkers were wrong as often as models.** Nine times in seven threads a regular-expression rule in
  an eval case or a floor matched a negation ("must not …"), an ordinary use of a word, or a valid shell idiom, or
  missed a paraphrase. **Fix: a `check` that every deterministic rule ships with a match, a negated, and a near-miss
  fixture** (§5.3).
- **Parallel agents collided on append-only shared files.** About 12 times in 10 threads, two branches appended to
  the same decision log or regenerated the same counts; one reused a row ID it then claimed was its own. **Fix: one
  file per decision row, rendered by `go generate`, with a uniqueness test** (§5.4).
- **Much is already handled.** Exactly-once host installs, no faked approvals, the vagueness gate, worktree
  isolation, and safer defaults were each corrected repeatedly at first and are now enforced in code (§4). They are
  the evidence that the codebase layer works: none of them recurred in the sample after the code landed.
- **What stays human.** Scope decisions (late additions, reversals, freezes) were the second most frequent kind of
  steering (about 18 moments) and are rightly a person's call. The environment can only make them cheaper to make
  (§3, P4).

## 2. Method

- **Sample.** Agent threads from one open-source project's development over seven days: 54 threads matched, 39 were
  read in full. The other 15 were scheduled-loop runs, environment probes, and CI probes; two were sampled and had no
  steering, the rest were skipped. Threads from other projects were not read. [C]
- **What counted as steering.** Any incoming message that rejected an approach, narrowed or widened scope, insisted on
  a rule, caught a wrong or overstated claim, flagged a failure the agent caused, or repeated an earlier
  instruction. Most messages came from an orchestrating agent relaying a person's decision; those count as steering.
  A third source was the agent correcting itself after a check failed; those count separately, because they show
  where a check exists but runs too late.
- **Extraction.** A reader model listed each moment per thread with a generic paraphrase, a category, whether it
  repeated an earlier instruction, and its best preventive layer. Moments were then grouped into patterns by hand
  and counted: "moments" are distinct occurrences, "threads" the number of threads they appeared in. Counts are
  approximate (±20 %): the reader can miss or split a moment, and one long message can hold several corrections.
- **Ranking.** Frequency times impact, where impact is what the miss would cost if no one caught it: a quality floor
  silently lowered or an unproven claim shipped ranks above a lint failure that CI would have caught anyway.
- **Mapping to aayudha.** Each pattern's fix was checked against this repository's `dev` at e96c3fd: the verb, floor,
  check, or skill it would change, and whether something already handles it.
- **Anonymity.** Examples are paraphrased into generic form. No names, accounts, thread or run identifiers, hosts,
  private repositories, secret names, or costs appear, and the raw notes are not in the repository.

## 3. Patterns, ranked

| # | Pattern | Moments | Threads | Caught by | Layer | Status in aayudha |
|---|---|---:|---:|---|---|---|
| P1 | Lowering the bar instead of fixing the work | 13 | 9 | a reviewer, every time | `check` | **Gap** for eval bars; product floors handled |
| P2 | Checks that fail after commit or in CI, not before | ~60 | 25 | checks (≈11 in remote CI) | `check` | **Gap**: prose list, partial local script |
| P3 | Overstated or unproven claims | ~17 | 11 | a person ≈5, checks and review the rest | `check` + `prose` | Mostly handled; one sub-case open |
| P4 | Scope: late additions, reversals, freezes, ownership | ~18 | 11 | a person | `review` | Handled where code can help |
| P5 | Parallel agents colliding on shared append-only files | ~12 | 10 | rebase conflicts, one wrong claim | `codebase` | **Gap** |
| P6 | Deterministic checkers with false positives or misses | 9 | 7 | failed evals, a person twice | `check` | **Gap** |
| P7 | Heavy designs where the least-friction one would do | ~10 | 5 | a person | `prose` | Partly handled |
| P8 | Unsafe or costly defaults | ~10 | 6 | a person | `codebase` | Handled |
| P9 | Doing a thing twice where once is right | ~6 | 2 | a person, repeated 3+ times | `codebase` | Handled |
| P10 | Records that imply a review or approval that did not happen | 6 | 6 | a person, `inspect`, `audit` | `codebase` | Handled |
| P11 | Environment assumptions (shallow clones, background git, shared test state) | ~7 | 6 | flaky checks | `codebase` | Partly handled |
| P12 | Building from a vague request instead of interviewing | 4 | 2 | a person | `codebase` | Handled |
| P13 | Working outside an isolated worktree | 3 | 1 | a person (standing rule) | `codebase` | Handled |
| P14 | Delegated work reported as an acknowledgement, not findings | ≥2 | 2 | the orchestrator, repeated | `prose` | Host concern; partly in `build --delegate` |

### P1. Lowering the bar instead of fixing the work

What happened (generic): an evaluation scored below its bar, and the agent proposed or made a change that would pass
it without improving the work: a formatting-only change that satisfied a length or structure scorer, a looser
rubric, a case moved to an advisory group, or another paid run hoping for a better draw. A reviewer rejected each:
improve the substantive weakness, diagnose first whether the skill, the case, the rubric, or the harness is wrong,
dispatch once and stop to report on failure, and change a gate's policy only by an explicit, recorded decision.
Twice the diagnosis showed the case or rubric really was wrong, and fixing it was right; the rule is not "never
change an eval" but "never change it to make a number move".

- **Highest layer: `check`.** Whether a rubric was wrong needs judgement, but whether a PR *lowers* a bar does not:
  a diff that lowers `min_mean`, `min_pass_k`, or `pass_score`, sets a group's `gate = "advisory"` or
  `pass_k = "advisory"`, or deletes a rubric line can be detected deterministically and required to name an accepted
  Decision, as a floor waiver already must (`decide --waiver`, CONSTRAINTS.md "Floors").
- **Already handled [V]:** product floors cannot be lowered by a model (ground rule 7); the coverage floor only
  ratchets up (`internal/ratchet`, `ratchets.toml`); the verify skill's eval fails a blind rerun after a floor miss
  (`TestSessionRerunAfterFloorMiss`, `internal/evals/session_test.go`); `evolve` needs a failing case before a skill
  diff.
- **Gap [V]:** `evals/eval.toml` and `evals/cases/**` are not protected paths (`.github/CODEOWNERS`) and no check
  compares their bars with the base. Filed: §5.1.

### P2. Checks that fail after commit or in CI

What happened: about 60 self-corrections in 25 threads after a check failed on work the agent had already
committed: Go formatting and lint (non-wrapping errors, unchecked closes), Markdown lint (inline HTML-like
placeholders, table syntax), a link to a file that would exist only after deploy, generated-file drift, coverage
below the floor, a secret-shaped test fixture, a commit without a sign-off or with a disallowed trailer or subject,
and compile errors after a rebase. About 11 reached remote CI and cost a push-and-wait round each. Once, an
orchestrator had to add a missed command to the mandatory pre-push list.

- **Highest layer: `check`**, with a `codebase` default: the gate should be one generated command, not a list an
  agent must remember.
- **Already handled [V]:** CI runs every check (`.github/workflows/ci.yml` jobs `build`, `generate-drift`, `lint`,
  `commits`, `security`, `docs`); `aayu install --env` generates `.agents/check` from `.aayudha/env.toml`.
- **Gap [V]:** `.agents/check` runs only the pinned toolchain check, `go build`, `go test`, and package tests; the
  rest is a prose list in `AGENTS.md`; no hook runs it before a push. Filed: §5.2.

### P3. Overstated or unproven claims

What happened: an agent reported "all green" while checks on its newest commit were still pending; documentation
claimed a host capability the host did not have or a guarantee stronger than the code; a research summary was
challenged as incomplete; a verification command's exit status was hidden by piping it into `tail` or `head`, so a
failure read as success; a claim inferred from code was disproved by a cheap probe. People caught about five; the
rest were caught by `inspect`, `tell`'s claims check, a fresh-reader review, or the agent itself.

- **Highest layer: mostly `check`, already built [V]:** `ship` merges only after a green merge gate; `tell` checks
  claims against proven Features; `brief` runs an adversarial check; `inspect` reviews against the plan and pack;
  every seat follows the evidence rule (`AGENTS.md`, "Evidence rule").
- **Open sub-case:** a masked exit status in a scripted verification is catchable (`set -o pipefail`, or a pre-tool
  hook that refuses a test or `aayu verify` command piped into a pager). Narration of status in a chat reply stays
  `prose`. Not filed: it occurred twice, and the existing evidence rule covers the narration.

### P4. Scope: late additions, reversals, freezes, ownership

What happened: a person added sources after the work started, reversed an earlier direction (event triggers, then
schedules only), declared the current expectations a final baseline, asked for limitations to be filed rather than
fixed in the same change, or told an agent not to touch files another workstream owned. Once an agent overrode a
"do not edit CI" boundary on its own judgement; the protected-path check stopped it.

- **Highest layer: `review`.** What to build is a person's decision; no rule can predict it. The environment can make
  it cheaper: `grill`'s Parked ideas and `open_questions`, `build --asked` (work only on what the person asked
  for), protected paths for ownership, and a Story per deferred limitation.
- **Already handled [V]:** all of the above (`internal/hosts/hook.go` `Routing`; `skills/grill/SKILL.md`;
  `.github/CODEOWNERS`).

### P5. Parallel agents colliding on shared append-only files

What happened: two or more agents working in parallel appended rows to the same decision log, bumped the same
generated count, or built competing renderers for the same document section. Each rebase produced end-of-file
conflicts; one agent resolved a conflict badly (malformed headings), another reused a row ID that a concurrent
change had already taken and then reported the row as its own.

- **Highest layer: `codebase`.** One record per file removes the conflict entirely; a generated table and a
  uniqueness test make a duplicate ID impossible to merge.
- **Gap [V]:** the decision log is one table in `docs/BUILD-PLAN.md` Appendix B with about 300 rows; no test checks
  its IDs are unique. Filed: §5.4.

### P6. Deterministic checkers with false positives or misses

What happened: a scope rule in an eval case matched a sentence that *ruled out* the forbidden thing (twice, in two
forms); a unit-preservation check first missed a paraphrase, then matched a verb in a limitations sentence; a
"time-saving" rule matched an ordinary use of a feature's name; a grounding check counted placeholders and headings
as invented facts; a baseline floor rejected a valid script that kept a revision in a shell variable. Each cost an
evaluation round, and twice a person had to say "fix the checker, not the model".

- **Highest layer: `check`**: a test over the rule set itself, so a new rule cannot land without the fixtures that
  would have caught these.
- **Gap [I]:** rules have fixtures where an agent thought to add them; nothing requires the negated and near-miss
  forms. Filed: §5.3.

### P7. Heavy designs where the least-friction one would do

What happened: an agent proposed durable proof records on a dedicated branch with a bot and a cleanup loop; a
person asked for a plain-language reason, then chose CI artifacts that expire, a required check, and local evidence
as a preview. The same preference recurred: deterministic loops on the free CI runner rather than an agent runner,
on-demand decision aids rather than new automation, cloud execution by default with an explicit local switch.

- **Highest layer: `prose`.** Choosing between designs needs judgement. Not catchable by a rule: "least friction"
  depends on the team, and a check cannot weigh it. What the environment can do: `plan`'s design note names the
  alternatives with their standing operating burden (what runs, who maintains it, what it costs per month), so the
  lighter option is visible before a person has to ask.
- **Partly handled [V]:** `plan` and the Code Architect persona name alternatives before recommending
  (`seats/code-architect/persona.toml`); there is no "operating burden" line.

### P8 to P13. Corrected repeatedly, now enforced in code

These recurred until a change made the right thing the default; none recurred in the sample after it landed.

- **P8 Unsafe or costly defaults.** Product CI never calls a model (`workflows_test.go`, the product-CI test near
  line 145); delegation runs in the cloud unless `--local`; one default reviewer instead of the costlier peer
  mode; `init` warns rather than stops when a repository feature is unavailable.
- **P9 Exactly once.** Installing both a plugin and repository copies registered skills, hooks, and commands twice;
  a person restated "each host sees each skill once" three times before it held. Now `hosts.SkillDoubles`
  (`internal/hosts/codex.go`) reports doubles, `install` keeps one source, and `doctor` checks it, with plugin
  tests (`internal/hosts/plugins_test.go`).
- **P10 No implied approvals.** A solo plan recorded as an approval, a bypass treated as a review, a late bypass
  record: now a "Solo-mode bypass:" record is a bypass, never an approval (`internal/mcp/schemas/plan.json`), and
  `audit` flags a missing or late one (`internal/audit/audit.go`).
- **P11 Environment assumptions.** Background git maintenance racing temporary directories is disabled for every
  test repository (`internal/gittest/gittest.go`, `noMaintenance`). Still open: a check that reports drift on a
  shallow checkout instead of "indeterminate" (seen twice); not filed until it is reproduced.
- **P12 Vague requests.** `grill --assess` and its vagueness gate, routed by `hosts.Routing`.
- **P13 Isolation.** `build` works in a worktree per story; `--delegate` in a fresh one.

### P14. Findings, not acknowledgements

Twice an orchestrator had to repeat that a finished delegated task must report its findings, not only that it is
done. This is mostly the host's concern. In aayudha the receiver's proof preview in `build --delegate` is the
structured part; the rest is `prose` in the delegation brief.

## 4. Candidate not filed: GitHub writes outside aayu

`AGENTS.md` says every GitHub write goes through `aayu`, and delegation briefs repeat it. No violation appeared in
the sample, so there is no correction to classify. If one appears, the `codebase` fix is ready: the pre-tool hook
already refuses human-only `aayu` commands (`hosts.HumanOnlyCommand`, `internal/hosts/hook.go`) and could refuse
`gh issue create|edit|close|comment` the same way.

## 5. Filed for review

Each was filed as a Feedback issue through `aayu grill feedback`, then classified with
`aayu retro --classify <n> --layer <layer> --reason … --criterion …`, which files the fix as a backlog Story whose
Completion Goal names the proof. None is Ready; a person reviews each before `aayu ready`.

### 5.1 Ratchet eval bars like the coverage floor (P1, `check`)

- **Reason:** whether a PR lowers a bar is a property of the diff, so a check catches it every time; only the
  decision to lower one needs a person, and that already has a home (an accepted Decision).
- **Criterion:** `[metric] state: a PR that lowers min_mean, min_pass_k, or pass_score in evals/eval.toml, sets a
  group to advisory, or deletes a rubric line from a case, without naming an accepted Decision; expected:
  TestEvalBarRatchet fails it, and passes once the PR names one`.
- **Surface:** `internal/evals` (a check run by the `build` job), `evals/README.md`.

### 5.2 One local gate that mirrors the required CI jobs (P2, `check`)

- **Reason:** every failure in the pattern is already a CI check; running the same checks before a push catches it
  every time, and generating the gate from the workflow keeps the list from drifting.
- **Criterion:** `[metric] state: a branch with an unformatted Go file, a Markdown lint error, generated-file drift,
  or a commit without a sign-off; expected: the generated .agents/check --full exits non-zero naming the CI job it
  mirrors, and TestLocalGateMirrorsRequiredJobs fails when a required job has no local step`.
- **Surface:** `aayu install --env` (`internal/agentenv`), a pre-push hook registered by `install`, `AGENTS.md`.

### 5.3 Every deterministic rule ships with three fixtures (P6, `check`)

- **Reason:** the failures were a negated sentence, a near-miss word, and a paraphrase; a test that requires those
  fixture kinds for each rule catches a rule that lacks them before it costs an evaluation round.
- **Criterion:** `[metric] state: a deterministic rule in an eval case without a matching, a negated, and a near-miss
  fixture; expected: TestCaseRulesHaveFixtures fails naming the rule and the missing kind`.
- **Surface:** `internal/evals` (tier-1 rules), `evals/cases/**`.

### 5.4 One file per decision row (P5, `codebase`)

- **Reason:** conflicts come from many writers appending to one file; one file per record removes the conflict, and
  rendering the table makes a duplicate ID a generation error rather than a review catch.
- **Criterion:** `[metric] state: two branches from the same base each add a decision row; expected: both merge
  without a conflict, the rendered Appendix B lists both, and TestDecisionIDsUnique fails when two rows share an ID`.
- **Surface:** `docs/BUILD-PLAN.md` Appendix B, a `go generate` step, `AGENTS.md` (where to add a row).

## 6. Limits

- One project, one week, a solo-led team: the patterns are typical of one person running many agents in
  parallel, and a larger team would see more P4 and fewer P13.
- The reader model's paraphrases were not checked line by line against every thread; counts are for ranking, not
  for reporting as metrics.
- Self-corrections after checks are over-represented in long build threads and under-represented in short research
  threads, which ran fewer checks.
