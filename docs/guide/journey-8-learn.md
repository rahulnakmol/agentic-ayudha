# 8. Measure and learn

This is where most teams stop looking, and where aayudha keeps going. You set a number in step 1. Now you find out
whether it moved, and the team, agents included, learns from how the work went.

> **You are here:** step 8 of 9. [Shape](journey-1-shape.md) · [Break down](journey-2-break-down.md) ·
> [Ready](journey-3-ready.md) · [Build](journey-4-build.md) · [Prove](journey-5-prove.md) · [Ship](journey-6-ship.md) ·
> [Release](journey-7-release.md) · **Learn** · [Explain](journey-9-explain.md)

## Did it work? `measure`

`measure` computes the Goal's metric the same way the baseline was computed, over the window after the release, and
writes the **Result** into the Goal: the value, the target, the change, and exactly how it was measured. It reads
exports from your analytics (Application Insights, AWS, Google Cloud, or billing files) or pulls them live from a
connected cloud.

```sh
aayu measure <goal> --appinsights events.jsonl
```

**You** decide what the Result means and what to do next. **The agent** runs the numbers and writes them down.

> **In the story:** Goal 125's Result reads: "Target met: activation = 0.916667 against the target >= 0.6 (baseline
> 0.258, change +0.6587)." That is 110 of 120 people getting an answer within a week, up from 31 of 120.

How that number was produced matters as much as the number:

> **Honest status:** the people in this example are synthetic. The Result was measured on a seeded simulation of the
> merged product, run through the real question workflow, before the release went out. The real 28-day
> measurement after the release is scheduled for after 2026-11-09. Anyone can rerun the simulation and get the same
> numbers.

## Was it this change? `experiment` and `readout`

A before-and-after number cannot tell you whether *your change* moved it, or something else did. When that matters,
run an experiment: `experiment` designs an A/B test for a Feature, with a hypothesis, a metric, a feature flag, and
the minimum sample size it needs. `readout` reads out the lift once the sample is big enough and writes the Result.

benefits-qa did not run an experiment. Its question was "does self-serve beat the HR queue at all", and a
before-and-after comparison answered it. Reach for `experiment` when you are choosing between versions.

> **Needs setup:** experiments need PostHog (or GrowthBook) connected with `aayu init --integrations`, and enough
> traffic to reach the minimum sample.

## How did the work go? `retro`

`retro` compares what was planned with what happened: estimates against delivery, how often a human had to correct
an agent, floor misses, and cost. It files each finding as a Feedback issue that a person can accept or reject.
benefits-qa would give it plenty to chew on. For example, `inspect` ran 17 times against a plan of 10.

## Make the agents better: `evolve`

When you accept a retro finding, `evolve` turns it into a lasting fix. First it writes a failing test case for the
skill (an eval), which proves the problem is real. Then it changes the skill until that case passes, and opens a
pull request. Skills are protected files, so a person reviews that pull request before it merges.

> **Needs setup:** `retro` and `evolve` run fine from your agent. Running them on a schedule in CI needs the bot
> GitHub App; this repository does not have one yet, so its weekly retro loop cannot run there. See
> [Adopt at your own pace](adopt.md), Level 3.

## What you get back

- A Result on the Goal: the value, the target, the change, and how to reproduce it.
- With an experiment, the measured lift of your change alone.
- Feedback issues from `retro`, and skill improvements from `evolve` that you review.

> **Well done:** you closed the loop. You set a target, shipped, and checked it, with numbers anyone can reproduce.

## What's next

Results are only useful if people hear about them. [Explain it](journey-9-explain.md).
