# Did it work, and what do we learn?

Shipping is not the finish line. The Goal you started from has a number, and the only question that matters is
whether it moved. Then: what did the work itself teach you, and how do the agents stop repeating a mistake?

> **You are here:** after a release. You need a Goal with a baseline, which [grill](journey-1-shape.md) gave you.
> The baseline comes from data `measure` can read again: an analytics or billing export, or a live pull from
> [Azure](../getting-started-azure.md), [AWS](../getting-started-aws.md), or
> [Google Cloud](../getting-started-gcp.md).
>
> **Needs aayudha 0.2.** If `aayu version` prints 0.1.x, this page is ahead of your binary: run `aayu self-update`.
> Until 0.2 is released, `go install github.com/rahulnakmol/agentic-ayudha/cmd/aayudha@dev` installs the build this
> page describes.

## Measure the Goal

```sh
aayu measure <goal>
```

`measure` computes the Goal's metric exactly the way the baseline was computed, over a new window, and writes the
**Result** into the Goal: the value, the target, the change, and the command to reproduce it. `release` schedules it
for you when it ships work, and the `release-measure` [loop](guide-loops.md) can run it when a release is published.

**You should see** a Result section on the Goal, such as "Target met: activation = 0.916667 against the target >= 0.6"
from [the story](journey-8-learn.md).

Need to know whether *your change* caused the difference, not just whether the number moved? Plan an `experiment`
before the release and `readout` after. That needs PostHog or GrowthBook and enough traffic.

## Learn from the work: retro

```sh
aayu retro
```

`retro` compares each plan with what happened and files every lesson as a Feedback issue you accept or reject. It
also reports how the work flowed:

- the time each stage took;
- how long a green pull request waited for a person;
- how many times each story was touched;
- the share of reviewer findings that were acted on;
- the escape rate: Bugs traced back to shipped stories;
- the mix of changes.

These flow numbers are a baseline, reported and never gated. Do not change a story or a review just to move one.

## Stop repeating mistakes: evolve

When the agents keep making the same mistake, `evolve` turns accepted Feedback into a better skill. It is careful on
purpose:

- it takes only Feedback **classified as prose**: a mistake that a type, an invariant, a lint rule, or a test could
  prevent becomes a Story instead (`aayu retro --classify`), because the code is the most lasting place for a fix;
- it acts only when **at least two** accepted Feedback issues point at the same skill;
- it changes a skill **at most once a month**;
- it writes a failing test for the skill first, then **rewrites** the guidance rather than piling on rules;
- a person reviews the pull request;
- the next retro checks whether the problem came back.

```sh
aayu evolve           # lists the skills that have the evidence evolve needs
```

**You should see** a pull request that changes one skill and adds the test that keeps it fixed.

## Under the hood

How retro's flow numbers are computed and how evolve's rules are enforced are in
[the eval harness](how-evals.md#retro-evolve-and-the-coverage-ratchet). Where the measurement due dates live is in
[the ledger branch](how-ledger.md).

## What's next

- Explain the result to someone who was not there: [chapter 9, the telling](journey-9-explain.md).
- Start the next Goal: [when to use what](when-to-use-what.md#i-have-a-vague-idea).
