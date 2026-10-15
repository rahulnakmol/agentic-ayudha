# The eval harness

Skills are prompts, and a prompt can get worse without anyone noticing. The **eval harness** catches that. It keeps
recorded runs of each skill as test cases, checks them on every pull request without calling a model, and on a
schedule runs the skills again on real models and has a judge from another model family score the results. A
promotion to `rolling` or `main` is refused when the scores fall below the bar or below `main`'s baseline.

![An eval case runs through tier 1 on every pull request and tier 2 after merge; a judge from another family scores tier 2, and evals-gate compares each gating group's results with main's before a promotion.](img/how-evals.svg)

*Tier 1 is cheap and runs everywhere; tier 2 costs money, so it runs where the key is and gates promotions.*

## Cases

A case is a directory, `evals/cases/<skill>/<name>/`, with a `case.toml`. It names the skill, the input given to it,
and the artifacts a recorded run produced under `output/`. There are 23 cases today, for the `stub`, `grill`, `slice`,
`plan`, `ready`, and `brief` skills. Nine of them have a `[tier2]` section and a rubric, so tier 2 runs them again.

A case can also carry:

- `schemas`: artifacts that must validate against a JSON Schema.
- `ready`: artifacts holding a work item that must pass Ready validation.
- `calls`: the recorded run's `aayu` calls, replayed against a fake GitHub that serves the case's `github/` files.
- `injection`: the input carries a prompt injection, so no recorded call may write.
- `scope`: **scope guards**, regular expressions for requirements the input never stated. When a judge once caught a
  skill inventing a requirement, the case keeps that defect as a deterministic rule, checked before any judge.
- `brief`: a `brief` case, whose draft must pass `aayu brief`'s deterministic checks (structure, citations, the
  register's lint).

## Tier 1: deterministic, every pull request

`Tier1` in `internal/evals/tier1.go` runs these checks on the recorded run: artifacts exist, schemas validate, work
items pass Ready, scope guards find nothing, brief drafts pass their checks, recorded calls exit 0, and an injection
case writes nothing. `TestTier1` runs every case inside `go test`, so the required `build` job in `ci.yml` runs tier 1
on every pull request. No model is called.

## Tier 2: model-judged, on a schedule

Tier 2 runs each `[tier2]` case `k` times (5 today) on the skill's real model: the registry lane of the verb the skill
is, or stands in for, on the direct host (Claude Code). The pinned judge, `gpt-6.1-sol`, then scores each run from 1 to
5 against the case's rubric. A run passes at a score of 4 or more.

Why a judge from another family: a model grading its own vendor's output tends to forgive the same mistakes. The
harness calls `models.CheckFamilies` with the skill model and the judge, and refuses to start when they match. The
judge's calls go through [the model gateway](how-models.md) on the `review` lane, and must return JSON that validates
against a small schema (`score`, `reason`), with one retry.

Deterministic checks still come first. A reply of the wrong shape, one that fails Ready, or one a scope guard catches
scores 1 without asking the judge. A `brief` draft that fails `aayu brief`'s checks gets the findings once to revise,
as it would in a host; if it still fails, it scores 1.

A run that cannot be completed (the judge's reply never validates, a reply stops at its token cap, the spend cap is
reached) is an `error`. Errors are kept out of the scores but counted: more than one errored run in a case fails it,
so a broken endpoint cannot pass quietly.

## Scores, groups, and the bar

For each case the harness records two numbers over its completed runs:

- **pass^k**: 1 when every completed run passed, else 0. A case with one weak run in five has pass^5 of 0.
- **mean rubric score**: the average score.

For a suite, pass^k is the mean of the cases' pass^k, and the mean is over every completed run.

Cases run in **groups**, defined by `[[groups]]` in `evals/eval.toml`. A case belongs to the first group with a prefix
of its ID. Each group runs as its own job with its own spend cap (`max_usd`), so one group cannot use up another's
money. The groups today are `brief-plain` (the `eli5` and `ste` brief cases), `brief` (`engineer-ideate` and
`regulatory-close`), and `skills` (every other case).

`Compare` in `internal/evals/tier2.go` applies the bar to one group's results:

- **Always enforced:** the mean of at least 3.5 for every case and for the suite; at most one errored run per case;
  and, against `main`'s baseline, a suite mean no more than 0.3 lower.
- **pass^k is a suite bar.** A pass^k of at least 0.8 is enforced for the group as a whole, and a case's pass^k may not
  fall below its own baseline. A case's pass^k below 0.8 is printed as an advisory and does not fail. Pull request 154
  made this change, because one off run in five failed whole promotions while the suite stood at pass^5 0.80 and mean
  4.80. The comment in `compare` and `TestCompareSuitePassK` hold it.
- **`pass_k = "advisory"`** (the `brief` and `brief-plain` groups): pass^k findings, at the suite and against the
  baseline, are printed as advisories. These groups pass on the mean bar, the error limit, and the drop rule. Pull
  request 143 added this; `TestGroupPassKPolicy` covers it.
- **`gate = "advisory"`** (`brief-plain` only): results are computed and printed, but never block a promotion. Pull
  request 153 added this; `TestGroupGatePolicy` and `TestCompareAdvisoryGate` cover it.

## How the gates run

- **`evals.yml`** runs tier 2, one matrix job per group, in the `ai-gateway` environment, which only `dev`, `rolling`,
  and `main` may use. It runs nightly, on dispatch, on pushes to `rolling` and `main`, and on pushes to `dev` that touch
  skills, the harness, or the model gateway. The two brief groups skip pushes to `dev`, because they cost the most. Each group writes
  `evals/results/<sha>-<group>.json` and uploads it as the artifact `eval-results-<sha>-<group>`. Its baseline is the
  results file for `main`'s commit and the same group: a committed file under `evals/results/` if one exists, else
  that commit's artifact.
- **`evals-gate`** in `ci.yml` runs when a promotion candidate is gated (`gate=rolling` or `gate=stable`). For each
  gating group (`eval -groups -gating`), it fetches the promoted commit's results and `main`'s, runs `eval -compare`,
  and fails when any group's results are missing or fail.
- **The evolve gate.** `aayu evolve` changes a skill in answer to accepted Feedback. Its pull request must first add a
  case that names the Feedback and reproduces it: the old skill's recorded run (in the `reproduce` directory) must fail
  tier 1, and the new run in `output/` must pass. The `build` job runs `eval -evolve-gate` on every pull request and
  refuses an evolve pull request (an `evolve/` branch or an `Aayudha-Evolve:` trailer) without such a case.

## Key data and file formats

A case with scope guards, from `evals/cases/stub/csv-export/case.toml` (trimmed):

```toml
skill = "stub"
description = "Draft a Ready Story for a CSV export request."
input = "input.txt"
artifacts = ["output/issue.json"]
schemas = [{ file = "output/issue.json", schema = "evals/schemas/workitem.json" }]
ready = ["output/issue.json"]
…
scope = [
  { pattern = '(?i)content[- ]?type|\btext/csv\b|\bmime\b', why = "a content type the request does not state" },
  …
]
…
[tier2]
rubric = "rubric.md"
verb = "grill" # the stub drafts a Story as `grill story` does, so it runs on grill's lane (spec)
```

The bar and the groups, from `evals/eval.toml` (comments trimmed):

```toml
judge = "gpt-6.1-sol" # openai; the stub skill runs on grill's spec lane, claude-opus-5-5 (anthropic) today
k = 5                 # runs per case
pass_score = 4        # a run passes at this rubric score (1-5) or above
min_pass_k = 0.8
min_mean = 3.5
max_drop = 0.3
…
[[groups]]
name = "brief-plain"
prefixes = ["brief/eli5-", "brief/ste-"]
max_usd = 6.0
pass_k = "advisory"
gate = "advisory"

[[groups]]
name = "brief"
prefixes = ["brief/"]
max_usd = 6.0 # engineer-ideate and regulatory-close (the plain registers moved to brief-plain, D-7.3h)
…
pass_k = "advisory"

[[groups]]
name = "skills" # every other case
max_usd = 6.0
```

The pass^k computation, from `Tier2` in `internal/evals/tier2.go`:

```go
var done []int
cr.PassK = 1
for _, r := range cr.Runs {
    if r.Status == RunError {
        cr.Errors++
        continue
    }
    done = append(done, r.Score)
    if !r.Pass {
        cr.PassK = 0
    }
}
if len(done) == 0 {
    cr.PassK = 0
}
cr.Mean = mean(done)
…
for _, cr := range res.Cases {
    res.PassK += cr.PassK / float64(len(res.Cases))
}
```

And where a group's pass^k policy is applied, from `compare` in the same file:

```go
passK := &reasons
if g, ok := cfg.Group(cur.Group); ok && g.PassK == PassKAdvisory {
    passK = &advisory
}
```

## The brief group's honest history

The `brief` skill writes one-page briefs in four registers: `eli5`, `ste` (Simplified Technical English), `engineer`,
and `regulatory`. Its original bar was the full one, pass^5 of at least 0.8 for every register. It failed that bar in
five consecutive repair rounds (eval runs 38200697472, 38202244333, 38207049608, 38216409539, and 38222172014; the
build record also lists a sixth charged run, 38210971285). The last scored suite pass^5 0.50 and mean 4.45: the
`regulatory` and `engineer` registers passed 5 of 5 runs, `ste` 4 of 5, and `eli5` 3 of 5. Every deterministic check
held on every judged run; what remained were judgement calls no rule decides.

Rather than hide the shortfall, the team recorded it and changed only the `brief` group's gate: it passes on the mean,
per register and for the suite, with pass^k reported as advisory (pull request 143). The next rolling candidate then
failed `eli5` and `ste` even on the mean, mostly where the rubric asks for everyday restatement and then marks it down
for fact fidelity. So pull request 153 moved those two registers into `brief-plain`, which is reported but does not
gate. The plain-language rubric conflicts are tracked as an open issue, Feedback 152, for `evolve` to fix. Every other
group keeps the full bar, which since pull request 154 applies pass^k per group at the suite level rather than to each
case on its own. The design rows are in [the build plan](../BUILD-PLAN.md); search it for "advisory".

## Invariants

1. Every recorded run passes tier 1 on every pull request, without a model call. Enforced by `TestTier1` in
   `internal/evals/evals_test.go`, run by the `build` job in `.github/workflows/ci.yml`.
2. The judge's family differs from the skill model's, and from its fallback's. Enforced in `Tier2`,
   `TestTier2RefusesSameFamilyJudge`.
3. Deterministic checks beat the judge: a wrong shape, a Ready failure, or a scope guard hit scores 1 first. Enforced
   in `internal/evals/tier2.go`, `TestTier2ScopeGuardAndOpenQuestions` and `TestTier2ReadyFloor`.
4. More than one errored run in a case fails its group, and a spend cap turns the remaining runs into recorded errors.
   Enforced by `MaxErrors` and `Tier2`, `TestTier2SpendCap`.
5. pass^k is enforced per group at the suite level, never per case; the per-case mean, the error limit, and the
   baselines stay enforced. Enforced in `compare`, `TestCompareSuitePassK`.
6. Only `brief` and `brief-plain` relax pass^k, and only `brief-plain` does not gate; an unknown group gates. Enforced
   by `Config.Gates` and `compare`, `TestGroupPassKPolicy`, `TestGroupGatePolicy`, `TestCompareAdvisoryGate`.
7. An evolve pull request adds a reproducing case before any skill diff. Enforced by `EvolveGate` in
   `internal/evals/evolve.go`, `TestEvolveGateCI` and `TestEvolveGateAfterTestMergeRecomputed`.

## Where it lives in the code

- [internal/evals/evals.go](../../internal/evals/evals.go): cases, groups, and `eval.toml` loading.
- [internal/evals/tier1.go](../../internal/evals/tier1.go): the deterministic checks.
- [internal/evals/tier2.go](../../internal/evals/tier2.go): runs, the judge, pass^k, and `Compare`.
- [internal/evals/evolve.go](../../internal/evals/evolve.go): the eval-first gate.
- [internal/evals/cmd/eval/main.go](../../internal/evals/cmd/eval/main.go): the `eval` command CI runs.
- [evals/](../../evals/): cases, schemas, `eval.toml`, and results.
- [.github/workflows/evals.yml](../../.github/workflows/evals.yml) and [.github/workflows/ci.yml](../../.github/workflows/ci.yml).

> **Honest status:** tier 2 needs the `ai-gateway` key, so it never runs on pull requests, and a fork cannot run it.
> The two results files committed under `evals/results/` are from earlier runs on `dev`, so the baseline comes from
> `main`'s workflow artifact. No evolve case exists in `evals/cases/` yet, so the evolve gate has only run in tests and
> on pull requests that were not evolve pull requests. The `eli5` and `ste` registers do not gate any promotion until
> Feedback 152 is resolved.

## Read next

- [The model gateway and lanes](how-models.md): how the judge and the skill models are chosen and paid for.
- [Releases](how-releases.md): where `evals-gate` sits in a promotion.
