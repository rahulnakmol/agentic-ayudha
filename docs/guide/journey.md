# Prologue: your Monday

> *Monday 12 October 2026, just after 07:00 UTC.* You scroll through the HR inbox again. A question about
> the dental deductible, asked twelve days ago, is still open. Someone wants to know how much the company puts into
> their health savings account. Someone else asks about a colleague's claim, which HR must never answer. You count. Of
> the last 120 people who asked HR a benefits question, 31 got an answer from the plan documents within a week. You
> have one day, one repository, and a team of agents. Can you fix this, this week?

You are going to try. By tonight, you want at least six in ten people who ask to get an answer within a week, quoted
straight from the plan documents, and you want to be able to show that it worked.

The stakes are not only speed. A slow answer costs people time. A wrong answer is worse: someone picks a dental plan
on a guess. And an answer about another person's claim would be a breach. Whatever you build has to be fast, right,
and private, and you need to know it is, not hope.

You are not alone, but you are the only person: the founder, and the whole human side of the team. aayudha gives
you fourteen agent seats, a small delivery team, each with a name: Arjun, the product strategist, who interviews you;
Vishwa, the architect, who plans; Nakul, the engineer, who builds; Drona, the engineering manager, who has every
change reviewed by a model from another vendor; Yama, the quality architect, whose checks no model can talk its way
past; and nine more ([seats](concepts-seats.md)). They do the work. You make the calls only a person can make.

> **In the story:** you stand in for the one person who ran this, in this repository, on Monday 12 October 2026.
> Everything that happens to you happened. The issues, pull requests, reviews, checks, and numbers are real, linked
> or numbered so you can check them. The app you build, benefits-qa, lives in this repository's
> [examples/benefits-qa](../../examples/benefits-qa) folder, so its pull requests and releases are this repository's.
> The employer, its 120 employees, and their data are synthetic, generated from a fixed seed. Because there really was
> only one person, solo mode shows up in chapters 2 and 6.

## The arc

![Nine chapters in a column, from the goal to the telling. Proof can send the build back for a fix; you say Ready before building and give the yes before anything merges; the number loops back to the next goal.](img/journey.svg)

*The arc of your day. Two decisions belong to you: "Ready" and "yes". The gate can send work back as often as it
needs to.*

## What you will have by the evening

| Level | What | Number |
|---|---|---|
| Goal | Employees get benefits answers within a week | 125 |
| Initiative (the bet) | Self-serve answers quoted from the plan documents | 126 |
| Features | [Grounded answers API](https://github.com/rahulnakmol/agentic-ayudha/issues/127), [Answer analytics](https://github.com/rahulnakmol/agentic-ayudha/issues/128), [Ask page](https://github.com/rahulnakmol/agentic-ayudha/issues/129) | 127, 128, 129 |
| Stories | Load the plan documents; quote the best passage; the API; the analytics export; the Ask page | 130 to 134 |
| Pull requests that merged | One per story, each proven in CI | 142, 148, 155, 158, 160 |
| Pull requests thrown away | Sent back twice by review, closed, and rebuilt | 135, 140, 151, 156 |
| Release | The snapshot `rolling-2026-10-12.4` | 144 |

The numbers are GitHub issue and pull request numbers in this repository. Only the three Features are links: this
site links work only when every story under it shipped with proof, and those three qualify.

## The chapters

1. [The goal](journey-1-shape.md). 07:20. An interview that will not accept "faster" as an answer.
2. [The plan](journey-2-break-down.md). Five stories, nine layers, and three plans nobody else could review.
3. [The promises](journey-3-ready.md). What "done" means, written down before a line of code, including what the app
   must refuse to answer.
4. [The build](journey-4-build.md). The agent works; you go for coffee. A budget keeps watch.
5. [The gate says no](journey-5-prove.md). Seventeen reviews. Four pull requests thrown away. One email check that
   would not die.
6. [The yes](journey-6-ship.md). You reach for the approve button, and GitHub says no.
7. [The release](journey-7-release.md). 14:47. A gate, a tag, and a date in November.
8. [The number](journey-8-learn.md). 0.258 became 0.917. What that number is, and what it is not yet.
9. [The telling](journey-9-explain.md). Your close-out summary is refused, for one sentence.

Then an [epilogue](built-with-itself.md): how aayudha was built the same way, with its own gates saying no to it.

> **Tip:** every chapter ends with an "Under the hood" box for engineers who want the mechanics, and a "What's next"
> that leads into the next chapter. You can also read the [How it works](how-it-works.md) track on its own.

## What's next

07:20. You ask your agent to write the Goal, and instead of writing, it starts asking questions:
[chapter 1, the goal](journey-1-shape.md).
