# 7. The release

> *Monday, 14:47 UTC.* You type `aayu release 144 --to rolling` and answer "yes" at the prompt. Somewhere in CI,
> a full test run starts on a commit that does not exist on any branch yet. If it fails, nothing moves.

## Merged is not released

Shipping put your five stories on `dev`, where work integrates. Releasing puts them in front of people. aayudha
moves work up a ladder of three branches: `dev`, then a `rolling` snapshot for early adopters, then a stable release
on `main` ([dev, rolling, main](concepts-ladder.md)). Every step up runs a full gate first.

Back at 08:51, the agent had filed Release 144, listing the three Features it would carry. Now `release` checks
that everything on that list has shipped. Then it builds the exact merge commit that will land, on a temporary
branch, and runs the promotion gate on it in CI: the full test suite that must pass before a branch moves.

## What happened at 14:47

- The candidate commit was `9991cd3`, titled "Rolling snapshot: 20261012T1447Z".
- The skill evals (tests of the agents' own instructions, under the gates described in the
  [epilogue](built-with-itself.md)) and the rolling gate both passed on the first attempt.
- `rolling` moved to that exact commit, tagged `rolling-2026-10-12.4`.
- Stories 132, 133, and 134 were stamped `released`. Stories 130 and 131 had already gone out in the 10:43
  snapshot, `rolling-2026-10-12.3`.
- Release notes were written, naming only proven Features.
- The measurement of Goal 125 and Initiative 126 was scheduled for after 9 November 2026, 28 days out.

All of it was written back onto the Release issue as shipping evidence: the runs, the artifacts and their
fingerprints, the stories, the notes, and the due dates. Because the example lives inside aayudha's own repository,
that snapshot is also a release of aayudha itself: the very one [your first hour](start.md) installs.

> **Honest status:** in the real run, the seeded measurement and the closing summary in the next two chapters were
> made just before this release, between 14:29 and 14:39. The real 28-day measurement comes after 9 November. And no
> stable release, `main` at version v0.1.0, has been cut yet; that step works the same way and adds a signed build, an
> install check, and the package managers.

## What you do, and what the agent does

**You** decide when to release. Your agent may run `release` for you; at your own terminal it asks you to confirm.
CI is refused. To go to `main`, you say so on the Release issue yourself: comment `Stable release: v0.1.0` as the
first line. The agent cannot write that comment. Try `--dry-run` first: it shows what would ship and which stories
would be stamped, and writes nothing.

**`release`** does the careful part: checks the list, builds the candidate, runs the gate, moves the branch, tags it,
stamps the stories, writes the notes, and schedules the measurement.

## What you learn

Measure-later only works if "later" is on the calendar. `release` puts it there for you.

## The verbs in this step

<!-- BEGIN GENERATED: stage-verbs -->

| Verb | What it does |
|---|---|
| [`release`](../../skills/release/SKILL.md) | Moves shipped work up to a snapshot or stable release, after a full gate, and schedules the measurement. |

<!-- END GENERATED: stage-verbs -->

> **Under the hood:** the gates, the tags, goreleaser, cosign signatures, provenance, and how `install.sh` checks a
> download are in [releases and the supply chain](how-releases.md).

## What's next

The work is out. Now the only question that matters, and you already have a first answer from 14:29: did the
number move?
[Chapter 8, the number](journey-8-learn.md).

> **You are here:** chapter 7 of 9. [Goal](journey-1-shape.md) · [Plan](journey-2-break-down.md) ·
> [Promises](journey-3-ready.md) · [Build](journey-4-build.md) · [Gate](journey-5-prove.md) · [Yes](journey-6-ship.md) ·
> **Release** · [Number](journey-8-learn.md) · [Telling](journey-9-explain.md)
