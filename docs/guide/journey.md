# The journey at a glance

Every piece of work in aayudha travels the same path: from a fuzzy idea to a release whose effect you have
measured. There are nine steps. At each one, your agent does the heavy lifting and you make one or two decisions
that only a person should make.

![Nine steps in a column: shape the goal, break it down, get a story Ready, build, prove, ship, release, measure and learn, explain. You decide at Ready and at the merge; learning loops back to the next goal.](img/journey.svg)

*The journey. You say "Ready" before any code is written, and you give the final yes before anything merges.*

## The running story

This guide follows one real run, the **benefits-qa** example. On 12 October 2026 the Goal was filed at 07:25 UTC,
and the release went out at 14:47 UTC the same day. Here is what was created along the way:

| Level | What | Number |
|---|---|---|
| Goal | Example Co employees get benefits answers within a week | 125 |
| Initiative (the bet) | Self-serve answers quoted from the plan documents | 126 |
| Features | [Grounded answers API](https://github.com/rahulnakmol/agentic-ayudha/issues/127), [Answer analytics](https://github.com/rahulnakmol/agentic-ayudha/issues/128), [Ask page](https://github.com/rahulnakmol/agentic-ayudha/issues/129) | 127, 128, 129 |
| Stories | Load the plan documents; quote the best passage; the API; the analytics export; the Ask page | 130 to 134 |
| Pull requests | One per story, each checked in CI before it merged | 142, 148, 155, 158, 160 |
| Release | The rolling snapshot `rolling-2026-10-12.4` | 144 |

The numbers are GitHub issue and pull request numbers in this repository. Only the three Features are links: this
site links work only when every story under it shipped with proof, and these three qualify.

## The nine steps

| Step | What happens | Your part |
|---|---|---|
| [1. Shape the goal](journey-1-shape.md) | `grill goal` turns an idea into a Goal with a baseline and a target | Answer the interview |
| [2. Break it down](journey-2-break-down.md) | `slice` splits the work into small stories; `plan` plans each one | Agree the split |
| [3. Get a story Ready](journey-3-ready.md) | `grill story` and `ready` pin down exactly what "done" means | Say "Ready" |
| [4. Build it](journey-4-build.md) | `build` writes the code, one checkpoint per layer, and opens a pull request | Nothing, unless it asks |
| [5. Prove it](journey-5-prove.md) | `verify`, the merge gate, and `inspect` check the change against every promise | Read the verdict |
| [6. Ship it](journey-6-ship.md) | `ship` merges and records who approved what | Give the final yes |
| [7. Release it](journey-7-release.md) | `release` promotes the work and stamps what went out | Start the release |
| [8. Measure and learn](journey-8-learn.md) | `measure`, `experiment`, `retro`, and `evolve` close the loop | Decide what is next |
| [9. Explain it](journey-9-explain.md) | `brief` and `tell` explain the work, citing only what was proven | Share it |

> **Tip:** you do not have to type commands. Every step is also a skill: ask your agent in plain words, such as "use
> the slice skill on Feature 127". The commands are shown so you can see exactly what runs.

## What's next

Start at the beginning: [1. Shape the goal](journey-1-shape.md).
