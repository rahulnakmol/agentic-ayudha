# 7. Release it

Shipping puts a change on `dev`, where the team integrates. Releasing puts it in front of people. `release` moves
your work up the ladder, from `dev` to a `rolling` snapshot for early adopters, then to a stable release on `main`,
and it runs a full gate before every step.

> **You are here:** step 7 of 9. [Shape](journey-1-shape.md) · [Break down](journey-2-break-down.md) ·
> [Ready](journey-3-ready.md) · [Build](journey-4-build.md) · [Prove](journey-5-prove.md) · [Ship](journey-6-ship.md) ·
> **Release** · [Learn](journey-8-learn.md) · [Explain](journey-9-explain.md)

## What you do, and what the agent does

**You** start the release. `release` runs only for a person, on a terminal, with a confirmation: agents and CI are
refused. You first file a Release work item (`grill release`) that lists the Features it includes.

**`release`** does the careful part. It checks that everything the Release lists has shipped. It builds the exact
merge commit that will land, runs the promotion gate on it in CI (the full test suite that must pass before a
branch moves), and moves the branch only when the gate is green.
Then it tags the commit, stamps every story it carried as `released`, writes release notes that name only proven
Features, and schedules the measurement of each Goal above those stories, 28 days out.

```sh
aayu release <release> --to rolling --dry-run   # what would ship, and which stories get stamped
aayu release <release> --to rolling             # dev to rolling, after a confirmation
aayu release <release> --to main                # rolling to main, as version vX.Y.Z
```

The ladder is explained in [dev, rolling, main](concepts-ladder.md).

## In the benefits-qa story

> **In the story:** Release 144, "benefits-qa example on the next rolling snapshot", included the three Features.

At 14:47 UTC, `release` promoted `dev` to `rolling`:

- The candidate commit was `9991cd3`, titled "Rolling snapshot: 20261012T1447Z".
- The evals (tests of the agents' own skills) and the rolling gate both passed on the first attempt.
- The commit was tagged `rolling-2026-10-12.4`.
- Stories 132, 133, and 134 were stamped `released`. Stories 130 and 131 had already gone out in the snapshot before.
- Release notes were written to the ledger.
- Measurement of Goal 125 and Initiative 126 was scheduled for after 2026-11-09.

All of it was written back to the Release issue as shipping evidence: the runs, the artifacts and their
fingerprints, the stories, the notes, and the due dates.

> **Honest status:** a stable release (`main`, version v0.1.0) has not been cut for this project yet. The `rolling`
> step above is real; the `main` step works the same way and adds a signed build, an install check, and the
> package managers.

## What you get back

- A tagged snapshot or release, built from a commit that passed the full gate.
- Every story it carried stamped `released`, including any you forgot to list (they are reported).
- Release notes that cite only proven work, and a measurement date for each Goal.

> **Well done:** your work is out. And because the measurement is already scheduled, you will not forget to check
> whether it helped.

## What's next

Did it work? [Measure and learn](journey-8-learn.md).
