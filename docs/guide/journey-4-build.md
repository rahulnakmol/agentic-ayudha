# 4. The build

> *Monday, just after 07:30 UTC.* You type `aayu build 130` and leave for coffee. By the time you are back, pull
> request 135 is open, and a comment on the story reads like a flight log: checkpoint, layer, result, budget used.

## Small steps, each one checked

The agent writes the code in its own copy of the repository (a git worktree), so your checkout is untouched. It
takes a lease on the story, so no other agent picks it up. Then it follows the plan one layer at a time. After each
layer it commits a **checkpoint** and runs `verify`, the same fixed checks CI will run later. A mistake shows up
minutes after it is made, not days later in review.

The stories go in order. Story 131 waits for 130 to merge, so its pull request, 148, opens at 09:12. Its plan had
two checkpoints; watch the count change when a review asks for more:

| Checkpoint | Layers | Result |
|---|---|---|
| 1 of 2 | L04 Domain and policy | Checks hold |
| 2 of 2 | L05 Intelligence | Checks hold |
| 3 of 3 | L04 and L05 | Checks hold |

The first review asked for a change, so a third checkpoint answered it, and "of 2" became "3 of 3". The flight log on the story showed the budget: 0.5 of 8 hours used, 0 of 3 failed attempts.

Some checks could not run on the building machine, and the log said so in plain words: "not run here (tool not
installed; the merge gate runs them)". That is fine. The checks that count run in CI, in the next chapter.

## A budget that keeps watch

Every build has limits: by default 8 hours, 25 USD of model spend that aayudha can see, and 3 failed attempts. If
the budget runs out, or a risky layer fails twice, the agent stops, marks the story blocked, and pages a person
through Slack, email, or a GitHub mention. The code your agent writes is billed by your Amp, Cursor, or Claude Code
plan as usual.

On your Monday the brake was pulled once, and the alert landed on you. Story 134's local checks failed twice on its
platform layer, a risky one, so the agent stopped and marked the story blocked. You looked. The cause was not the
code: it was a stale local copy of `aayu`, built before a fix to the accessibility check, and CI, on the current
version, had already passed every check. You rebuilt the binary, ran `aayu ready 134` to restart the budget, and the
build carried on. The brake did its job: it stopped an agent that was failing and handed the question to you.

## If your story has a screen

The Ask page, story 134, had screens, so it also went through design review. `review` critiques a pull request's
screens twice: with a model, and with fixed checks for accessibility, focus, and the size of things you tap. The
first review asked for two small visual fixes. The second said "ship". Hold on to that second review: it contains a
disagreement that chapter 5 settles. The other design verbs in the table below, from research to a working prototype,
are there for when a Feature starts from a blank page; the Ask page did not need them.

> **Honest status:** the model half of `review` needs a model key where it runs. This repository runs it in a CI
> workflow that holds the key.

## What you do, and what the agent does

**You** do almost nothing. You step in only when the agent stops and asks, as it did on story 134.

**Your agent** writes the code and its tests, commits one checkpoint per layer, checks each, pushes, and opens the
pull request. It maps each promise to the tests that prove it, which the next chapter turns into evidence.

## What you learn

Checking every small step is cheaper than reviewing one big change. And a clear budget lets you walk away.

## The verbs in this step

<!-- BEGIN GENERATED: stage-verbs -->

| Verb | What it does |
|---|---|
| [`build`](../../skills/build/SKILL.md) | Writes the code for a Ready story in small checked steps, one layer at a time, and opens a pull request. |
| [`research`](../../skills/research/SKILL.md) | Collects design references and user research for a Feature. |
| [`wireframe`](../../skills/wireframe/SKILL.md) | Sketches the screens a Feature needs, from its research and promises. |
| [`mockup`](../../skills/mockup/SKILL.md) | Turns wireframes into mockups that use your design system. |
| [`prototype`](../../skills/prototype/SKILL.md) | Builds a working prototype behind a flag and compares it with the design. |
| [`review`](../../skills/review/SKILL.md) | Critiques a pull request's screens with a model and with fixed accessibility checks. |
| [`polish`](../../skills/polish/SKILL.md) | Applies a design critique to a pull request and reviews it again. |
| [`baseline`](../../skills/baseline/SKILL.md) | Captures reference screenshots of a commit to compare later changes against. |

<!-- END GENERATED: stage-verbs -->

> **Under the hood:** budgets, the breaker, and model lanes are in [the model gateway](how-models.md); what `verify`
> runs and when each check is skipped is in [floors in detail](how-floors.md).

## What's next

The pull request is open. The agent says it is done. Now CI finds out whether that is true, and the answer, at
first, is no: [chapter 5, the gate says no](journey-5-prove.md).

> **You are here:** chapter 4 of 9. [Goal](journey-1-shape.md) · [Plan](journey-2-break-down.md) ·
> [Promises](journey-3-ready.md) · **Build** · [Gate](journey-5-prove.md) · [Yes](journey-6-ship.md) ·
> [Release](journey-7-release.md) · [Number](journey-8-learn.md) · [Telling](journey-9-explain.md)
