# aayudha: a delivery team for your AI agent that shows its work

aayudha turns the coding agent you already use, Amp, Cursor, or Claude Code, into a small delivery team. It takes
an idea all the way to a measured release. You make the calls. The agents do the legwork. And nothing counts as
done until there is evidence that it is.

> **Start here:** new to aayudha? Spend [your first hour](start.md) installing it and filing your first Goal.
> Then follow [the journey](journey.md), one short page per step.

## The problem it solves

Coding agents are fast. Speed alone leaves you with hard questions, though:

- Did the agent build what we actually asked for?
- Was it tested, and by whom? The agent itself?
- Is it safe to merge? Who decided?
- Months later: did it help anyone?

Usually the answers live in a chat log, in someone's head, or nowhere. aayudha writes them down as the work
happens, in places you already trust: GitHub issues, pull requests, and checks that run in CI (the automated checks
GitHub runs on every pull request).

- **Every piece of work starts as a clear promise.** A GitHub issue with numbered, checkable criteria, written with
  you in a short interview.
- **Every change is checked by fixed rules, not by the agent's say-so.** Tests, security scans, and accessibility
  checks run in CI and must pass. A model cannot talk its way past them.
- **Every release is measured** against the number you chose at the start.

## What a day with it looks like

You start the morning with an idea: "employees wait too long for answers about their benefits". You ask your agent
to use the `grill` skill. (A skill is a short instruction file that aayudha installs into your agent, one for each
of its commands.) For ten minutes it interviews you, like a sharp product lead would: who benefits, what you
will measure, what the number is today, what it should become. Then it files a Goal on GitHub.

Before lunch, the agent breaks the Goal into a handful of small stories. Each one has a short list of promises such
as "a question the plan does not cover is sent to HR, and nothing is quoted". You read them, tweak one, and say
they are Ready.

In the afternoon the agent builds the first story and opens a pull request. CI runs the checks. A second model,
from a different vendor than the one that wrote the code, reviews the change against each promise. You read a
one-page verdict ("merge: all three criteria met, with evidence") and ship it. Not every review says "merge" the
first time. When it asks for a fix, the agent fixes it and the checks run again. That is normal: in the real example
below, five stories took 17 reviews in all.

Weeks later, one command measures the outcome against your target, and another writes the one-page summary for your
boss.

| You | Your agent |
|---|---|
| Decide what matters and how to measure it | Interviews you and files clear issues |
| Approve the plan and the promises | Splits the work, plans it layer by layer, builds it |
| Give the final yes on each merge and release | Runs the checks, collects evidence, reviews the change |
| Read the results | Measures, explains, and suggests what to improve |

## A real example, start to finish

Everything in this guide follows one real piece of work in this repository: **benefits-qa**, a small app where
employees of a fictional company, Example Co, ask benefits questions and get the passage of the plan that answers
them.

- **The Goal:** more employees get an answer within a week of first asking. The number was 0.258 (31 of 120
  people). The target was 0.60.
- **The work:** one Goal, three Features, five stories, each merged with a proof pack (the evidence file CI writes
  for a change).
- **The result so far:** 0.917 (110 of 120), target met, measured on a seeded simulation of the shipped code. The
  real 28-day measurement after the release is due after 9 November 2026.

The people and the data are synthetic, generated from a fixed seed so that anyone can reproduce every number. The
code, the checks, the reviews, and the records are real, and the guide links to them.

## Will it fit, and what does it cost?

- **Price:** aayudha is free and open source, under the BSD-3-Clause licence.
- **Where it works:** GitHub repositories, with Amp, Cursor, or Claude Code. Use your existing repository: if you
  only have a `main` branch, aayudha works with it and renames nothing.
- **To start (Level 1):** no keys, no CI changes, about an hour. Your agent's own model does the interviewing.
- **For proof on every pull request (Level 2):** one model key for the reviewer (by default an OpenAI model, so an
  OpenAI key or one AI gateway key), about 10 CI minutes per push in this repository, and at most 5 USD of review
  per pull request by default. One review in the example cost 0.37 USD.
- **Your code:** the floors run in your own CI. The model vendors whose keys you add see what they review.
- **Changed your mind?** `aayu uninit` removes what aayudha set up.

[Adopt at your own pace](adopt.md) has the full costs and benefits of each level.

## Find your way

| If you want to | Go to |
|---|---|
| Set it up and try it | [Your first hour](start.md) |
| See the whole path from idea to measured release | [The journey at a glance](journey.md) |
| Find the right command for your situation | [When to use what](when-to-use-what.md) |
| Try it without touching CI or adding keys | [Adopt at your own pace](adopt.md) |
| Understand the ideas behind it | [Seats](concepts-seats.md), [layers](concepts-layers.md), [proof packs](concepts-proof.md), [floors](concepts-floors.md), [the human pass](concepts-human-pass.md), [dev, rolling, main](concepts-ladder.md) |
| Look up a term | [Glossary](../../GLOSSARY.md) |
| Read the technical reference | [Project overview](../../README.md) and the verb pages |

## Where things stand

aayudha is young. The first stable release, v0.1.0, has not been cut yet, so [your first hour](start.md) shows how
to install the latest snapshot instead. Some parts need setup you may not have yet, such as a model key for CI
reviews or a GitHub App for the agents. Each page says so plainly where it matters, with the next step. You can
get real value on day one without any of it.

## What's next

Head to [your first hour](start.md). By the end of it, you will have aayudha installed, your repository connected,
and your first Goal on GitHub.
