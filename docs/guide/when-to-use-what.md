# When to use what

Start from where you are. Find your situation below: each one names the verb to reach for, what you do, and what you
get back. You can always ask your agent in plain words instead ("use the grill skill to …"); the verb is what it
runs.

## I have a vague idea

Use **`grill goal`**, or `grill feature` if the idea is already small. The agent interviews you until the idea has an
outcome, a beneficiary, a metric, a baseline, and a target.

**You get back** a Goal (or Feature) issue that anyone can read, with a number you can measure again later.
Walkthrough: [Shape the goal](journey-1-shape.md).

## I asked for something in words ("make it slick", "keep it simple")

Just ask: `build`, `slice`, `plan`, and the `/codex`, `/cursor`, `/claude`, and `/amp` hand-offs all take words.
When your words are vague, use abbreviations, or name nothing anyone could check, aayudha stops and unpacks them
with you first: a few questions, riskiest first, then "here is what done looks like" for you to confirm. A concrete
ask goes straight through with no questions.

**You get back** a Ready Story with numbered criteria, the meaning of each abbreviation, and the checkpoints you
will see as it is built. Walkthrough: [How aayudha unpacks a vague ask](unpack.md).

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

If agents themselves are misbehaving, **`aayu halt`** stops agent lanes, loops, and the workflows that check the halt
switch, until a person runs `aayu resume`.

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
| [Set up](start.md) | `init` | Sets up a repository for aayudha: labels, issue forms, config, and the records branch; more on request. |
|  | `install` | Installs the aayudha skills, tools, and hooks into Amp, Cursor, or Claude Code. |
|  | [`doctor`](../../skills/doctor/SKILL.md) | Checks your machine and setup, and says exactly what to fix. |
| [Shape the goal](journey-1-shape.md) | [`grill`](../../skills/grill/SKILL.md) | Interviews you, one question at a time, until an idea is a complete work item, then files it on GitHub. |
| [Break it down](journey-2-break-down.md) | [`slice`](../../skills/slice/SKILL.md) | Splits a Feature into small stories, each with checkable promises, the parts of the code it touches, and its order. |
|  | [`plan`](../../skills/plan/SKILL.md) | Plans a story one layer at a time: what changes, how it is tested, and how to roll it back. |
|  | [`decide`](../../skills/decide/SKILL.md) | Records a real choice with its options and a recommendation; only a person with the authority accepts it. |
| [Get a story Ready](journey-3-ready.md) | [`ready`](../../skills/ready/SKILL.md) | Checks that a work item is complete and freezes exactly what was agreed, so later edits are visible. |
| [Design the screens](journey-4-build.md#if-your-story-has-a-screen) | [`research`](../../skills/research/SKILL.md) | Collects design references and user research for a Feature. |
|  | [`wireframe`](../../skills/wireframe/SKILL.md) | Sketches the screens a Feature needs, from its research and promises. |
|  | [`mockup`](../../skills/mockup/SKILL.md) | Turns wireframes into mockups that use your design system. |
|  | [`prototype`](../../skills/prototype/SKILL.md) | Builds a working prototype behind a flag and compares it with the design. |
|  | [`review`](../../skills/review/SKILL.md) | Critiques a pull request's screens with a model and with fixed accessibility checks. |
|  | [`polish`](../../skills/polish/SKILL.md) | Applies a design critique to a pull request and reviews it again. |
|  | [`baseline`](../../skills/baseline/SKILL.md) | Captures reference screenshots of a commit to compare later changes against. |
| [Build it](journey-4-build.md) | [`build`](../../skills/build/SKILL.md) | Writes the code for a Ready story in small checked steps, one layer at a time, and opens a pull request. |
| [Prove it](journey-5-prove.md) | [`verify`](../../skills/verify/SKILL.md) | Runs the fixed checks (tests, security, secrets, dependencies, accessibility, speed) and records the results. |
|  | [`inspect`](../../skills/inspect/SKILL.md) | An agent app from another vendor, installed on your account, reviews the pull request against every promise; aayu reads its reply as merge, fix, or rebuild. |
|  | [`proof`](../../skills/proof/SKILL.md) | Stores, fetches, and checks the proof packs that hold the evidence for each change. |
| [Ship it](journey-6-ship.md) | [`ship`](../../skills/ship/SKILL.md) | Merges a proven pull request after your yes, and writes down who approved what, on which evidence. |
| [Release it](journey-7-release.md) | [`release`](../../skills/release/SKILL.md) | Moves shipped work up to a snapshot or stable release, after a full gate, and schedules the measurement. |
| [Measure and learn](journey-8-learn.md) | [`experiment`](../../skills/experiment/SKILL.md) | Designs an A/B test for a Feature: what you expect, what you measure, and how many people you need. |
|  | [`readout`](../../skills/readout/SKILL.md) | Reads out an experiment once it has enough people, and writes down the measured lift. |
|  | [`retro`](../../skills/retro/SKILL.md) | Compares plans with what happened and files what to improve as feedback you can accept or reject. |
|  | [`measure`](../../skills/measure/SKILL.md) | Measures a Goal against its baseline, the same way as before, and writes the result into the Goal. |
|  | [`evolve`](../../skills/evolve/SKILL.md) | Turns accepted feedback into a test for a skill, then a skill change that passes it, for a person to review. |
| [Explain it](journey-9-explain.md) | [`status`](../../skills/status/SKILL.md) | Rolls up progress, risk, proof, open decisions, and cost for each Goal, Initiative, and Feature. |
|  | [`audit`](../../skills/audit/SKILL.md) | Writes a report anyone can reproduce: checks, proof, waivers, deadlines, bypasses, and any write that skipped the log. |
|  | [`brief`](../../skills/brief/SKILL.md) | Explains a piece of work to one audience in a short page, where every claim is checked against its sources. |
|  | [`tell`](../../skills/tell/SKILL.md) | Writes docs, release notes, and launch copy in your brand's voice, linking only work that is proven. |
| [Keep it running](when-to-use-what.md#something-broke-in-production) | [`triage`](../../skills/triage/SKILL.md) | Sorts incoming reports into feedback, bugs, and incidents by label, without editing what people wrote. |
|  | [`respond`](../../skills/respond/SKILL.md) | Helps handle an incident: proposes a fix from your runbook, runs it only after a person approves, and keeps the timeline. |

<!-- END GENERATED: verb-map -->

For every input and flag, see the [project overview](../../README.md#verbs) and the verbs reference.

## What's next

- New here? Start with [your first hour](start.md).
- Want to see the verbs in action? Follow [the journey](journey.md).
