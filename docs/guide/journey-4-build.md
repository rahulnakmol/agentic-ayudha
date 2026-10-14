# 4. Build it

Now the agent writes the code. It works in small, checked steps, so a mistake shows up minutes after it is made,
not days later in review. You can go and do something else.

> **You are here:** step 4 of 9. [Shape](journey-1-shape.md) · [Break down](journey-2-break-down.md) ·
> [Ready](journey-3-ready.md) · **Build** · [Prove](journey-5-prove.md) · [Ship](journey-6-ship.md) ·
> [Release](journey-7-release.md) · [Learn](journey-8-learn.md) · [Explain](journey-9-explain.md)

## What you do, and what the agent does

**You** do nothing, unless the agent gets stuck and asks for help.

**Your agent** runs `build`. It takes a lease on the story, so no other agent picks it up, and works in its own copy
of the repository (a git worktree), so your checkout is untouched. It follows the plan one layer at a time. After each
layer it commits a **checkpoint** and runs `verify`, the same fixed checks CI will run later. When a checkpoint holds,
it pushes the branch and opens or updates the pull request.

```sh
aayu build <story>            # start, or commit the next checkpoint
aayu build <story> --resume   # carry on from another machine, or after a review asks for a fix
```

`build` refuses a story that is not Ready, or whose text changed after its Ready snapshot.

## Guard rails while you are away

Every build has a budget: by default 8 hours of wall time, 25 USD of model spend, and 3 failed attempts. The spend
counts the model calls aayudha makes for the story, plus any spend your host reports to it; the code your agent
writes is billed by your Amp, Cursor, or Claude Code plan as usual. If the budget runs out, or a risky layer fails
twice, the story is marked blocked and a person is paged through the channel you choose: Slack, email, or a GitHub
mention. The agent stops. You look, help if needed, and run `aayu ready <story>` to let it carry on.

## In the benefits-qa story

> **In the story:** story 131 was built as pull request 148, in three checkpoints.

| Checkpoint | Layers | Result |
|---|---|---|
| 1 of 2 | L04 Domain and policy | Checks hold |
| 2 of 2 | L05 Intelligence | Checks hold |
| 3 of 3 | L04 and L05 | Checks hold |

The plan had two checkpoints. The first review asked for a fix, so a third was added, and the count became "3 of
3". The story's progress comment showed the budget used: 0.5 of 8 hours and 0 of 3 failed attempts.

The pull request was opened at 09:12 UTC. Local checks could not run two of the security tools on that machine and
said so, plainly: "not run here (tool not installed; the merge gate runs them)". That is fine: the next step runs
everything again in CI, and that run is the one that counts.

## If your story has a screen

Stories with a user interface have extra verbs, owned by the UX Researcher seat. Use them when you want design
help, not by default:

- `research`, `wireframe`, `mockup`, and `prototype` take a Feature from references to a working, flagged prototype.
- `review` critiques a pull request's screens twice: with a model, and with fixed checks for accessibility, focus,
  and target size. `polish` applies the critique. `baseline` captures reference screenshots for a commit.

Story 134, the Ask page, was reviewed twice this way. The first review asked for two small visual fixes; the second
said "ship".

> **Needs setup:** `review`'s model pass needs a model key wherever it runs. Without one, its fixed checks still
> run in the merge gate.

## What you get back

- A pull request whose commits are checkpoints, one layer each, each one already checked.
- A progress comment on the story, with each checkpoint and the budget used so far.

> **Well done:** the code exists, and every layer was checked as it was written. Next, you find out whether it keeps
> its promises.

## What's next

A pull request is not done until it is proven. [Prove it](journey-5-prove.md).
