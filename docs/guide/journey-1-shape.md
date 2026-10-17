# 1. The goal

> *Monday, 07:20 UTC.* You type into your agent: "Use the grill skill to write a Goal: employees should get
> benefits answers faster." The agent does not write anything. It asks you a question instead. "Faster than what?
> Which number would tell you it worked?"

## An interview that will not accept "faster"

Most projects that disappoint were never clear about what success meant. So the first thing aayudha does is refuse
to start until it is.

The `grill` skill interviews you one question at a time, the way a sharp product lead would. Who benefits? The
employees, you say. Which number shows it? You settle on the share of people who get an answer from the plan
documents within 7 days of first asking. What is that number today? You counted the inbox this morning. The agent is
polite about it, and firm: a count from the inbox is not a baseline.

That is where most goals quietly go wrong. aayudha has a rule for it: no baseline, no target. Before you may say
"60 %", the figure for today has to be measured in a way that can be repeated later.

So it is. `aayu measure` reads an export of the old process, where every question was an HR ticket, and counts: 31 of
120 people got an answer quoted from the plan documents, from an HR specialist, within 7 days of first asking.
**0.258.** Like everyone in this story's numbers, those 120 people are synthetic: a simulation with a fixed seed, so the
count can be repeated exactly. The Goal records the exact command, so anyone
can run it again in November and get the same number.

At 07:25, Goal 125 is filed:

| Section | What the interview produced |
|---|---|
| Summary | Employees get an answer within a week of first asking, quoted from the plan documents, instead of waiting on an HR ticket |
| Metric | Activation: the share who get an answer quoted from the plan documents, by the app or by HR, within 7 days of first asking |
| Baseline | 0.258: 31 of 120 people |
| Target | At least 0.60, over the 28 days after it ships |

Twenty-one seconds later comes Initiative 126, the bet: if employees can ask the plan documents directly and get a
quoted answer at once, the number rises from 0.258 to at least 0.60. The interview also made you name the riskiest
assumption: that the plan text really covers the questions people ask.

## What you do, and what the agent does

**You** answer questions. That is the whole job, and it is the most important one in the story.

**Your agent** asks, pushes back on vague answers, writes what you said into the Goal's sections, checks that every
required section is there, fetches the baseline where it can, and files the issue only when everything passes.

> **Since then:** the interview no longer waits for you to ask for it. Whenever you give any aayudha verb or hand-off
> a vague request, in Claude Code, Codex, Cursor, or Amp, it stops and unpacks the request with you first, and a
> concrete one goes straight through. [How aayudha unpacks a vague ask](unpack.md) shows a real conversation.

## What you learn

A number with a baseline turns an opinion into a bet you can win or lose. It also tells the agents what matters,
which is what every later check will measure against.

> **Tip:** do not skip the riskiest-assumption question. It tells you what to watch first.

## The verbs in this step

<!-- BEGIN GENERATED: stage-verbs -->

| Verb | What it does |
|---|---|
| [`grill`](../../skills/grill/SKILL.md) | Interviews you, one question at a time, until an idea is a complete work item, then files it on GitHub. |
| [`survey`](../../skills/survey/SKILL.md) | Reads a codebase nobody documented, says what it does with a citation for every claim, and drafts the backlog to modernise it. |

<!-- END GENERATED: stage-verbs -->

> **Under the hood:** `grill` runs inside your agent, so your agent's model does the interviewing and the binary only
> checks the draft and writes the issue. See [Architecture](how-architecture.md) for what runs where, and
> [the verb registry](how-registry.md) for how every verb is declared once and generated everywhere.

## What's next

The Goal is clear. It is also far too big to build in one go, and some of it is dangerous to get wrong:
[chapter 2, the plan](journey-2-break-down.md).

> **You are here:** chapter 1 of 9. **Goal** · [Plan](journey-2-break-down.md) · [Promises](journey-3-ready.md) ·
> [Build](journey-4-build.md) · [Gate](journey-5-prove.md) · [Yes](journey-6-ship.md) ·
> [Release](journey-7-release.md) · [Number](journey-8-learn.md) · [Telling](journey-9-explain.md)
