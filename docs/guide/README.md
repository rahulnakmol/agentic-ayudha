# Ship what you promised, and prove it

Your coding agent can write a feature in an afternoon. Whether that feature does what you promised, is safe to
merge, and helps anyone at all is a different question. aayudha is the delivery team around the agent you already
use: Claude Code, Codex, Cursor, or Amp. It answers that question with evidence at every step, from a fuzzy idea to a
number that moved.

> **Start here:** there are two ways in. [Read the story](journey.md): you, a team of agents, one goal, one Monday,
> and every setback is real. Or [see how it works](how-it-works.md): the architecture, the file formats, and the
> guarantees, written for engineers. Ready to try it yourself? Spend [your first hour](start.md): one install and
> `aayu tour`.

## The story in one minute

Picture an HR inbox. Of the last 120 people who asked a benefits question, 31 got an answer within a week. You want
at least 60 % to. You have one day, one repository, and a team of agents.

By the evening, five small stories had shipped, touching all nine layers of the app, from the screens to the data
store. Before that, the checks had
sent the work back again and again: five stories took 17 reviews, and four pull requests were thrown away and built
again from scratch. One check for outside email addresses failed review four times, each time on a new way for an address to slip through. A model claimed a
colour was too faint, and a measurement proved it wrong. The close-out summary was refused once, for a single
sentence it could not back up.

Every "no" made the result better. Measured on 120 simulated employees, the share answered within a week went from
0.258 to 0.917, against a target of 0.60. The real measurement is due after 9 November.

[Read the whole story](journey.md). It is told to you, in your shoes. Everything in it happened on 12 October 2026,
in an example app that lives in this repository, and the employees it serves are synthetic.

## Why it exists

Coding agents are fast. Speed alone leaves you with hard questions:

- Did the agent build what we actually asked for?
- Was it tested, and by whom? The agent itself?
- Is it safe to merge? Who decided?
- Months later: did it help anyone?

aayudha writes the answers down as the work happens, in places you already trust: GitHub issues, pull requests,
and the checks that run in CI (the automated checks GitHub runs on every pull request).

| You | Your agents |
|---|---|
| Decide what matters and how to measure it | Interview you and file clear issues |
| Agree the promises each piece of work must keep | Split the work, plan it layer by layer, build it |
| Give the final yes on each merge and release | Run the checks, collect evidence, review the change |
| Read the results | Measure, explain, and suggest what to improve |

## Will it fit, and what does it cost?

- **Price:** aayudha is free and open source, under the BSD-3-Clause licence.
- **Where it works:** GitHub repositories, with Claude Code, Codex, Cursor, or Amp. Use your existing repository: if you
  only have a `main` branch, aayudha works with it and renames nothing.
- **To start (Level 1):** no keys, no CI changes, about an hour. Your agent's own model does the interviewing.
- **For proof on every pull request (Level 2):** no AI key in your CI. The review comes from an agent app from
  another vendor that you install on your GitHub account, such as Claude or Codex, billed through your own
  subscription to it. The checks take about 10 CI minutes per push in this repository.
- **Your code:** the checks run in your own CI. The reviewer app you install sees the pull requests it reviews.
- **Changed your mind?** `aayu uninit` removes what aayudha set up.

[Adopt at your own pace](adopt.md) has the full costs and benefits of each level.

## Find your way

| If you want to | Go to |
|---|---|
| Feel what it is like to work this way | [The story](journey.md), nine short chapters |
| Set it up and try it | [Your first hour](start.md) |
| Find the right command for your situation | [When to use what](when-to-use-what.md) |
| See how a vague ask ("make it slick") becomes a spec | [How aayudha unpacks a vague ask](unpack.md) |
| Get a pull request reviewed and merged | [A pull request is ready](guide-review.md) |
| Hand a story to another agent | [Hand work to an agent](guide-build.md) |
| Check what your product actually does | [Prove a behaviour change](guide-prove-behaviour.md) |
| Let work happen on a schedule or an event | [Keep things moving with loops](guide-loops.md) |
| Find out whether it helped | [Did it work?](guide-learn.md) |
| Fix something that went wrong | [Troubleshooting](troubleshooting.md) |
| Try it without touching CI or adding keys | [Adopt at your own pace](adopt.md) |
| Understand the ideas | [Seats](concepts-seats.md), [layers](concepts-layers.md), [proof packs](concepts-proof.md), [floors](concepts-floors.md), [the human pass](concepts-human-pass.md), [dev, rolling, main](concepts-ladder.md) |
| Know exactly how it works | [How it works](how-it-works.md), fifteen technical deep dives |
| See how aayudha was built with itself | [Epilogue: built with itself](built-with-itself.md) |
| Look up a term or a flag | [Glossary](../../GLOSSARY.md), the [project reference](../README.md), and the verb pages |

## If you are an agent

Helping someone adopt aayudha? Send them to [your first hour](start.md) and let them run `aayu tour` in their own
terminal: it is for people and refuses agents. After that, `aayu next` names the next step, and every GitHub write
goes through an `aayu` verb, never `gh`. Each page here has a Markdown twin at the same path ending in `.md`, listed
in `llms.txt`.

## Where things stand

aayudha is young. Its first stable release, v0.1.0, came out on 15 October 2026, and the install script installs it.
Some parts need setup you may not have yet, such as a reviewer app on your GitHub account or a GitHub App for the
agents. Each page says so plainly where it matters, with the next step. You can get real value on day one without
any of it.

## What's next

It is Monday morning, and the HR inbox is full: [the story begins](journey.md).
