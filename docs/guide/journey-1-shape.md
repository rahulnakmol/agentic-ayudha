# 1. Shape the goal

Most projects that disappoint were never clear about what success meant. This step fixes that before any code
exists. You leave it with a Goal: a GitHub issue that says who benefits, which number shows it, where that number
is today, and where it should get to.

> **You are here:** step 1 of 9. **Shape** · [Break down](journey-2-break-down.md) · [Ready](journey-3-ready.md) ·
> [Build](journey-4-build.md) · [Prove](journey-5-prove.md) · [Ship](journey-6-ship.md) ·
> [Release](journey-7-release.md) · [Learn](journey-8-learn.md) · [Explain](journey-9-explain.md)

## What you do, and what the agent does

**You** answer questions. That is the whole job, and it is the most important one in the journey.

**Your agent**, using the `grill` skill, interviews you like a good product lead: one question at a time, pushing
back on vague answers. It writes what you said into the Goal's sections, checks that each required section is
there, and fetches the current value of your number where it can. When everything passes, it files the issue.

```sh
aayu grill goal          # on a terminal: a short interview
```

Or, in your agent: "use the grill skill to write a Goal for …".

## Why a baseline comes first

aayudha refuses a target without a baseline: "no baseline, no target". A target such as "60 % of people get an
answer within a week" means nothing unless you know today's figure and can measure it the same way again. The
baseline also records how it was measured, so the result at the end of the journey is an honest comparison.

## In the benefits-qa story

> **In the story:** Goal 125 reads: "Example Co employees get an answer to their benefits questions within a week
> of first asking, quoted from the plan documents, instead of waiting on an HR ticket."

| Section | What the interview produced |
|---|---|
| Beneficiaries | Employees of the fictional Example Co |
| Metric | Activation: the share of people who get a grounded answer within 7 days of their first visit |
| Baseline | 0.258: 31 of 120 people, when every question was an HR ticket |
| Target | At least 0.60, over the 28 days after the example ships |

The baseline came from `aayu measure` on a seeded export of the old HR-ticket process, and the Goal records the exact
command. Anyone can run it again and get 0.258.

Right after the Goal, the same interview produced Initiative 126, the bet: "if employees can ask the plan documents
directly and get a quoted, cited answer at once, activation rises from 0.258 to at least 0.60". It even names its
riskiest assumption: that the plan text covers the questions people really ask.

> **Tip:** you will be tempted to skip the riskiest-assumption question. Do not. It is the one that tells you what
> to measure first.

## What you get back

- A Goal issue on GitHub, with a baseline you can measure again and a target with a direction.
- Optionally an Initiative under it: the bet you are making, and why you think it will pay off.
- After `aayu ready <goal>`, a Ready snapshot: a comment that fingerprints exactly what was agreed, so later edits
  are visible.

> **Well done:** you have a measurable outcome, written down, before anyone wrote a line of code. Most teams never
> get this far.

## What's next

A Goal is too big to build in one go. Next, [break it down](journey-2-break-down.md) into small, provable stories.
