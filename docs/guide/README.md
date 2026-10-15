# Ship what you promised, and prove it

Your coding agent can write a feature in an afternoon. Whether that feature does what you promised, is safe to
merge, and helps anyone at all is a different question. aayudha is the delivery team around the agent you already
use, Amp, Cursor, or Claude Code. It answers that question with evidence at every step, from a fuzzy idea to a
number that moved.

> **Start here:** there are two ways in. [Read the story](journey.md): one small team, one goal, one Monday, and
> every setback is real. Or [see how it works](how-it-works.md): the architecture, the file formats, and the
> guarantees, written for engineers. Ready to try it yourself? Spend [your first hour](start.md).

## The story in one minute

Of the last 120 people at Example Co who asked HR a benefits question, 31 got an answer within a week. Priya, the
founder, wanted at least 60 % to. Sam, the engineer, had one day and an AI team.

By the evening, five small stories had shipped, touching all nine layers of the app, from the screens to the data
store. Before that, the checks had
sent the work back again and again: five stories took 17 reviews, and four pull requests were thrown away and built
again from scratch. One check for outside email addresses failed review four times, each time on a new way for an address to slip through. A model claimed a
colour was too faint, and a measurement proved it wrong. The summary for the boss was refused once, for a single
sentence it could not back up.

Every "no" made the result better. Measured on 120 simulated employees, the share answered within a week went from
0.258 to 0.917, against a target of 0.60. The real measurement is due after 9 November.

[Read the whole story](journey.md). Priya and Sam are illustrative; everything that happens to them happened in
this repository on 12 October 2026, and the people they serve are synthetic.

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
- **Where it works:** GitHub repositories, with Amp, Cursor, or Claude Code. Use your existing repository: if you
  only have a `main` branch, aayudha works with it and renames nothing.
- **To start (Level 1):** no keys, no CI changes, about an hour. Your agent's own model does the interviewing.
- **For proof on every pull request (Level 2):** one model key for the reviewer (by default an OpenAI model, so an
  OpenAI key or one AI gateway key), about 10 CI minutes per push in this repository, and at most 5 USD of review
  per pull request by default. One review in the story cost 0.37 USD.
- **Your code:** the checks run in your own CI. The model vendors whose keys you add see what they review.
- **Changed your mind?** `aayu uninit` removes what aayudha set up.

[Adopt at your own pace](adopt.md) has the full costs and benefits of each level.

## Find your way

| If you want to | Go to |
|---|---|
| Feel what it is like to work this way | [The story](journey.md), nine short chapters |
| Set it up and try it | [Your first hour](start.md) |
| Find the right command for your situation | [When to use what](when-to-use-what.md) |
| Try it without touching CI or adding keys | [Adopt at your own pace](adopt.md) |
| Understand the ideas | [Seats](concepts-seats.md), [layers](concepts-layers.md), [proof packs](concepts-proof.md), [floors](concepts-floors.md), [the human pass](concepts-human-pass.md), [dev, rolling, main](concepts-ladder.md) |
| Know exactly how it works | [How it works](how-it-works.md), thirteen technical deep dives |
| See how aayudha was built with itself | [Epilogue: built with itself](built-with-itself.md) |
| Look up a term or a flag | [Glossary](../../GLOSSARY.md), the [project overview](../../README.md), and the verb pages |

## Where things stand

aayudha is young. The first stable release, v0.1.0, has not been cut yet, so [your first hour](start.md) shows how
to install the latest snapshot instead. Some parts need setup you may not have yet, such as a model key for CI
reviews or a GitHub App for the agents. Each page says so plainly where it matters, with the next step. You can get
real value on day one without any of it.

## What's next

Meet Priya and Sam on a Monday morning: [the story begins](journey.md).
