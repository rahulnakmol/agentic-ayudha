# 8. The number

> *Monday, 14:29 UTC, a few minutes before the release in chapter 7.* You run one command against the product's
> export. The terminal prints a single line. You read it twice: "Target met: activation = 0.916667 against the
> target >= 0.6." Then you ask yourself the question your boss will ask. "Met on what, exactly?"

## 0.258 became 0.917

`measure` computes the Goal's metric exactly the way the baseline was computed in chapter 1, over a new window, and
writes the **Result** into the Goal: the value, the target, the change, and how to reproduce it.

| When | Activation within 7 days | People |
|---|---|---|
| Before: every question an HR ticket | 0.258 | 31 of 120 |
| After: the Ask page and API | 0.917 | 110 of 120 |
| Target | at least 0.60 | |

## Met on what, exactly

It is the right question, and the Goal answers it in writing. The 120 employees are synthetic. Their
behaviour comes from a simulation with a fixed seed, run through the real, merged question workflow, so every
question really went through the code that shipped. Anyone can run the simulation again and get the same file, byte
for byte, and the same 0.917.

What this number is not, yet, is a measurement of real people after the release. That one is scheduled for after
9 November 2026. When it comes in, it goes into the Goal the same way, next to this one.

There is one more honest detail. When the analytics story was rebuilt in chapter 5, the simulated HR answers that
fell after the measured window were dropped. The export file changed, so its fingerprint in the Goal was updated.
The baseline did not change: still 31 of 120, because the measurement only ever counted inside the window.

## Was it this change? Experiments

A before-and-after number cannot prove *your change* moved it. When that matters, `experiment` designs an A/B test
with a hypothesis, a metric, a feature flag, and the minimum number of people it needs, and `readout` reports the
lift once enough people have taken part. You did not run one: your question was "does self-serve beat the
HR queue at all", and before-and-after answered it. Reach for an experiment when you are choosing between versions.

> **Needs setup:** experiments need PostHog or GrowthBook connected with `aayu init --integrations`, and enough
> traffic to reach the minimum sample.

## Learning from the day: retro and evolve

`retro` compares the plan with what happened and files each lesson as a Feedback issue a person can accept or
reject. This Monday would give it plenty: 17 reviews against a plan of 10, and four pull requests thrown away.
It also reports the flow without filing anything: hours from Ready to ship split by stage, how long a green pull
request waited for a person, how often a person stepped in, how much of inspect's advice was acted on, how many
shipped stories came back as Bugs, and the mix of changes.
Each lesson is first sorted by where it is fixed for good: in the code, in a check, in a skill's words, or in what
a person is shown for review. Code and check fixes become Stories, because a mistake the code cannot express or a test
always catches never needs to be warned against. `evolve` turns an accepted lesson about a skill's words into a
lasting fix for the agents themselves: a failing test for the skill first,
then a change to the skill that passes it, opened as a pull request a person reviews. Neither ran on the benefits app
that day; the [epilogue](built-with-itself.md) shows the same loop at work on aayudha itself.

> **Since then:** `evolve` waits for at least two accepted Feedback issues about the same skill, changes a skill at
> most once a month, rewrites its guidance instead of adding rules, and the next retro checks whether the problem came
> back. Loops can now run the retro, evolve, and the measurement after a release in your agent's own scheduler. See
> [did it work?](guide-learn.md) and [keep things moving with loops](guide-loops.md).

## What you do, and what the agent does

**You** decide what the Result means, and what to do next.

**Your agent** runs the numbers, the same way as before, and writes them down with everything needed to check them.

## What you learn

A result you can reproduce is worth more than a bigger one you cannot. Say what the number is, and what it is not.

## The verbs in this step

<!-- BEGIN GENERATED: stage-verbs -->

| Verb | What it does |
|---|---|
| [`experiment`](../../skills/experiment/SKILL.md) | Designs an A/B test for a Feature: what you expect, what you measure, and how many people you need. |
| [`readout`](../../skills/readout/SKILL.md) | Reads out an experiment once it has enough people, and writes down the measured lift. |
| [`retro`](../../skills/retro/SKILL.md) | Compares plans with what happened and files what to improve as feedback you can accept or reject. |
| [`measure`](../../skills/measure/SKILL.md) | Measures a Goal against its baseline, the same way as before, and writes the result into the Goal. |
| [`evolve`](../../skills/evolve/SKILL.md) | Turns accepted feedback into a test for a skill, then a skill change that passes it, for a person to review. |

<!-- END GENERATED: stage-verbs -->

> **Under the hood:** where the measurement due dates live is in [the ledger branch](how-ledger.md); how `evolve`'s
> "test first" rule and the skill evals work is in [the eval harness](how-evals.md).

## What's next

You have a number you can defend. Now you have to explain it to someone who was not there, and your first draft is
refused: [chapter 9, the telling](journey-9-explain.md).

> **You are here:** chapter 8 of 9. [Goal](journey-1-shape.md) · [Plan](journey-2-break-down.md) ·
> [Promises](journey-3-ready.md) · [Build](journey-4-build.md) · [Gate](journey-5-prove.md) · [Yes](journey-6-ship.md) ·
> [Release](journey-7-release.md) · **Number** · [Telling](journey-9-explain.md)
