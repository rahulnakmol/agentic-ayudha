# When to use what

Start from where you are. Find your situation below: each one names the verb to reach for, what you do, and what you
get back. You can always ask your agent in plain words instead ("use the grill skill to …"); the verb is what it
runs.

## I have a vague idea

Use **`grill goal`**, or `grill feature` if the idea is already small. The agent interviews you until the idea has an
outcome, a beneficiary, a metric, a baseline, and a target.

**You get back** a Goal (or Feature) issue that anyone can read, with a number you can measure again later.
Walkthrough: [Shape the goal](journey-1-shape.md).

## I have a Feature and need a plan

Use **`slice`**, then **`plan`**. `slice` splits the Feature into stories sized XS to L, each with criteria,
dependencies, and the layers it touches. `plan` plans each story one layer at a time.

**You get back** small stories you can build in a day, and a plan per story with tests and a rollback.
Walkthrough: [Break it down](journey-2-break-down.md).

## We have to choose between options

Use **`decide`**. It records a Decision with the options, how each scores, and a recommendation. A person with the
authority accepts it.

**You get back** a Decision issue that says what was chosen and why, which `status` tracks until it is accepted.

## I have a bug report

Use **`grill bug`**. A Bug, like a story, gets a Completion Goal: numbered criteria that say what "fixed" looks like,
such as the steps that fail today and the result you expect. Then **`ready`** and **`build`**, the same as a story.
If reports arrive through an intake form, **`triage`** labels each one as Feedback, Bug, or Incident, and never edits
what people wrote.

**You get back** a Ready Bug, then a pull request that proves the fix against your criteria.

## A story is Ready, so build it

Use **`build`**. The agent works in its own worktree, commits one checkpoint per layer, checks each one, and opens a
pull request. It stops and asks a person when it runs out of budget.

**You get back** a pull request whose every checkpoint has already been checked.
Walkthrough: [Build it](journey-4-build.md).

## A pull request is ready

Let the **merge gate** run (it starts by itself), read **`inspect`**'s verdict, then **`ship`**. Run **`verify`**
yourself first if you want a quick local check.

**You get back** an authoritative proof pack, a verdict per criterion (`merge`, `fix`, or `rebuild`), and, once you
approve, a merged pull request with its Outcome written.
Walkthrough: [Prove it](journey-5-prove.md) and [Ship it](journey-6-ship.md).

## The pull request changes a screen

Add **`review`** for a design critique (a model pass and fixed checks for accessibility, focus, and target size),
then **`polish`** to apply it. **`baseline`** captures reference screenshots of a commit.

**You get back** a critique with a verdict, and a pull request that answers it.

## It is time to release

Use **`release`** with `--dry-run` first, then for real. It runs the full gate, tags the commit, stamps the stories,
writes release notes, and schedules measurement.

**You get back** a tagged snapshot or release, and shipping evidence on the Release issue.
Walkthrough: [Release it](journey-7-release.md).

## Something broke in production

File an Incident (**`grill incident`**, or let **`triage`** classify a report), then use **`respond`**. It proposes
an action from your team's operations catalog, keeps the timeline, and runs the action only after a person approves
it. When you are done, `respond --close` files the follow-up Bug and the retro entry.

If agents themselves are misbehaving, **`aayu halt`** stops every agent, loop, and workflow in the repository until a
person runs `aayu resume`.

**You get back** a timeline, an approved fix, and a Bug that makes sure it does not happen again.

> **Needs setup:** `respond` runs actions from `ops/actions.toml`, a catalog you write and review. Connect Sentry
> with `aayu init --integrations` if you want error data in the timeline.

## I need to explain this to my boss

Use **`brief`** with the register that fits: `eli5` for anyone, `engineer` for a technical lead. Use `--purpose
close` to wrap up finished work. For a weekly update, **`status`** rolls up every Goal.

**You get back** a short, accessible page where every claim is traced to its proof.
Walkthrough: [Explain it](journey-9-explain.md).

## I need to show an auditor

Use **`audit`**. It reports floor results, proof coverage, waivers, decision deadlines, autonomy exceptions,
solo-mode bypasses, and any write that skipped the log, from a snapshot that can be regenerated later. Add a brief
with `--register regulatory` if they want prose.

**You get back** a report on the ledger that someone else can reproduce exactly.

## I want to know if it worked

Use **`measure`** on the Goal after the release. If you need to know whether *your change* caused the difference,
plan an **`experiment`** before release and **`readout`** after.

**You get back** a Result on the Goal: the value, the target, the change, and how to reproduce it.
Walkthrough: [Measure and learn](journey-8-learn.md).

## The agents keep making the same mistake

Use **`retro`** to find the pattern, accept the Feedback it files, then **`evolve`** to turn it into a tested skill
change that a person reviews.

**You get back** a skill that no longer makes that mistake, with a test that keeps it that way.

## Something about my setup is off

Use **`doctor`**. It checks tools, sign-in, configuration, installed skills, and existing CI, and says what to fix.
`aayu doctor --fix` re-syncs skill copies after an upgrade.

## All 32 verbs, by journey stage

This map is generated from the verb registry, the same source as the command line, so it always lists every verb.
The descriptions are the registry's own one-liners, written for agents, so they are terser than the rest of this
guide. Each verb links to its skill page where it has one. Housekeeping commands such as `halt`, `resume`, and `seat`
are not verbs; the verbs reference lists them.

<!-- BEGIN GENERATED: verb-map -->

| Stage | Verb | What it does |
|---|---|---|
| [Set up](start.md) | `init` | Set up a product repo: labels, issue types, forms, config, ledger branch, and optionally projects, gates, identity, integrations. |
|  | `install` | Install skill copies, lock.toml, MCP registration, and hooks for each host; shows a diff unless --apply. |
|  | [`doctor`](../../skills/doctor/SKILL.md) | Check tools, tokens, identity, config, host installs, and existing CI; --fix re-syncs copies after showing a diff. |
| [Shape the goal](journey-1-shape.md) | [`grill`](../../skills/grill/SKILL.md) | Interview a human until a work item of the given type is complete and passes validation, then write it as an issue. |
| [Break it down](journey-2-break-down.md) | [`slice`](../../skills/slice/SKILL.md) | Split a Feature into XS-L stories with observable criteria, dependencies, and a layer footprint; close XL items as split. |
|  | [`plan`](../../skills/plan/SKILL.md) | Plan a story: one checkpoint per layer, contracts, tests, instrumentation, and rollback. |
|  | [`decide`](../../skills/decide/SKILL.md) | Record a Decision with options, evaluation, and recommendation; accept it, supersede it, or file a floor waiver. |
| [Get a story Ready](journey-3-ready.md) | [`ready`](../../skills/ready/SKILL.md) | Validate a work item for Ready, label it status:ready, and post the Ready snapshot; --check is read-only. |
| [Design the screens](journey-4-build.md#if-your-story-has-a-screen) | [`research`](../../skills/research/SKILL.md) | Gather pinned design references (URLs and IDs only) and user research for a Feature. |
|  | [`wireframe`](../../skills/wireframe/SKILL.md) | Produce wireframes for a Feature's states from its research and criteria. |
|  | [`mockup`](../../skills/mockup/SKILL.md) | Produce design-system mockups for a Feature from its wireframes. |
|  | [`prototype`](../../skills/prototype/SKILL.md) | Build a flagged, working prototype of a Feature with Code Connect mapping and a Figma screenshot diff. |
|  | [`review`](../../skills/review/SKILL.md) | Critique UI with a model lane and a deterministic lane; dispositions recapture, rebuild, fix, or ship. |
|  | [`polish`](../../skills/polish/SKILL.md) | Apply a review critique to a PR's UI and re-run review. |
|  | [`baseline`](../../skills/baseline/SKILL.md) | Capture visual baselines for a commit SHA. |
| [Build it](journey-4-build.md) | [`build`](../../skills/build/SKILL.md) | Build a Ready story or bug in a worktree with checkpoint commits per layer, verify each checkpoint, and open a PR. |
| [Prove it](journey-5-prove.md) | [`verify`](../../skills/verify/SKILL.md) | Run the deterministic floors (coverage, SAST, secrets, deps, a11y, perf) and record results in a proof pack. |
|  | [`inspect`](../../skills/inspect/SKILL.md) | Adversarially review a PR against its plan, pack, and criteria; return a structured disposition: merge, fix, or rebuild. |
|  | [`proof`](../../skills/proof/SKILL.md) | Push, get, verify, list, or purge proof packs on the ledger branch. |
| [Ship it](journey-6-ship.md) | [`ship`](../../skills/ship/SKILL.md) | Squash-merge a PR after a green merge gate and a human pass, write the Complete section, and clean up. |
| [Release it](journey-7-release.md) | [`release`](../../skills/release/SKILL.md) | Promote a Release along dev, rolling, main; stamp included stories released and trigger measurement. |
| [Measure and learn](journey-8-learn.md) | [`experiment`](../../skills/experiment/SKILL.md) | Design an experiment for a Feature: hypothesis, metric, flag, and minimum sample size. |
|  | [`readout`](../../skills/readout/SKILL.md) | Read out an experiment's lift once it reaches minimum sample and write the Result. |
|  | [`retro`](../../skills/retro/SKILL.md) | Compare estimates with observed delivery, corrections, floor misses, and cost; file findings as Feedback issues. |
|  | [`measure`](../../skills/measure/SKILL.md) | Measure a Goal or Initiative against its baseline from analytics exports or live cloud pulls and write the Result. |
|  | [`evolve`](../../skills/evolve/SKILL.md) | Turn accepted retro Feedback into a failing eval case, then a skill diff that passes it, opened as an agent-authored PR. |
| [Explain it](journey-9-explain.md) | [`status`](../../skills/status/SKILL.md) | Roll up status, health, risk, proof coverage, decisions, and cost per Goal, Initiative, and Feature. |
|  | [`audit`](../../skills/audit/SKILL.md) | Audit floors, proof coverage, waivers, decision SLAs, autonomy exceptions, and unlogged writes; write a reproducible report. |
|  | [`brief`](../../skills/brief/SKILL.md) | Explain work to an audience in a chosen register and purpose as a grounded, accessible HTML brief. |
|  | [`tell`](../../skills/tell/SKILL.md) | Write docs, guides, changelogs, release notes, launch copy, or brand copy that cite only proven Features; publish is human-only. |
| [Keep it running](when-to-use-what.md#something-broke-in-production) | [`triage`](../../skills/triage/SKILL.md) | Classify intake issues as Feedback, Bug, or Incident by label only; never edits bodies or touches non-intake issues. |
|  | [`respond`](../../skills/respond/SKILL.md) | Respond to an incident: propose an ops catalog action, execute it only through human approval, keep the timeline. |

<!-- END GENERATED: verb-map -->

For every input and flag, see the [project overview](../../README.md#verbs) and the verbs reference.

## What's next

- New here? Start with [your first hour](start.md).
- Want to see the verbs in action? Follow [the journey](journey.md).
