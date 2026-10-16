# Keep things moving with loops

Some work should happen without anyone asking: triage new reports on arrival, tidy up on Fridays, send a failed check
back to the agent that built it, measure after a release. A **loop** is one small file that runs an aayu command on a
schedule or when something happens on GitHub. It runs in your agent's own scheduler, in your own session, so your CI
never calls a model.

> **You are here:** Level 3. Loops work at any level, but they pay off once work flows steadily.
>
> **Needs aayudha 0.2.** If `aayu version` prints 0.1.x, this page is ahead of your binary: run `aayu self-update`.
> Until 0.2 is released, `go install github.com/rahulnakmol/agentic-ayudha/cmd/aayudha@dev` installs the build this
> page describes.

## What you get

`aayu init --gates` copies the default loop files into `loops/`. They do nothing until you set them up on your
agent, below.

| Loop | Starts | Does |
|---|---|---|
| `triage` | Mondays 09:30 | Lists new intake issues; your agent labels each as Feedback, Bug, or Incident |
| `triage-new` | An intake issue is opened | The same, for that issue only |
| `housekeeping` | Fridays 16:00 | Cleans up, then reports setup drift, feature-map drift, and stale Ready issues |
| `ci-failed` | A check suite fails | For a story handed to another agent, sends the failed check back to it |
| `feedback` | A Feedback issue is opened | Reports which skills now have enough evidence for `evolve` |
| `release-measure` | A release is published | Runs `measure` |
| `digest`, `retro`, `evolve` | Weekdays, Mondays, monthly | The founder digest, the retro, and skill evolution |

Times are in `[time] tz` from your config. Every run starts with a guard that stops it while the repository is halted
or over budget.

## Set them up

```sh
aayu install --loops
```

It finds the agents you have and, for each one, sets the loops up where that agent's tools allow, then prints the
exact steps for the rest:

- **Amp:** one prompt per loop, to paste into an Amp thread of your project.
- **Cursor:** with `--apply`, it runs `terraform apply` for you, if Terraform and a Cursor token are there.
- **Claude Code:** the values for the routines form.
- **Codex:** the values for a scheduled task.

When a loop is set up on a host, record it, so aayudha can tell you later if it went stale:

```sh
aayu install --loops --host amp --record triage=<the ID the host gave you>
```

**You should see** each recorded loop as active in `aayu doctor`'s `loops` row. A loop you changed after recording it
shows as stale, and one you never set up as missing.

## Not every host can do every event

Each agent can start a run on different GitHub events. Where a host has its own trigger, the loop runs there
("native"). Where it has none, the event waits for **your next session**: when you open your agent,
`aayu hook session-start` lists the events since last time and offers the loop's steps. Nothing runs between
sessions. `aayu doctor` shows such a loop as `session`.

The table follows each host's own documentation; only the Amp column has been run live so far (see the note below).

| Event (default loop) | Amp | Cursor | Claude Code | Codex |
|---|---|---|---|---|
| Intake issue opened (`triage-new`) | native | native, from the label | session | session, or a person's `@codex` |
| Feedback filed (`feedback`) | native | native, from the label | session | session, or a person's `@codex` |
| Check suite failed (`ci-failed`) | chained | native | session, or auto-fix on one pull request | session, or a person's `@codex` |
| Release published (`release-measure`) | session | session | native | session |

- **Amp, chained.** Amp cannot start a thread on a CI failure. So the `ci-failed` loop starts a thread when a pull
  request opens, and that thread subscribes itself to the pull request's checks and runs the loop when one fails.
  It covers the pull request's first commit and any later commit the thread hears about, and starts one Amp thread
  per new pull request.
- **Cursor.** The Terraform that `install --loops --apply` runs cannot express every trigger Cursor's web app
  offers. Triggers such as a review submitted are web-app only.
- **Codex.** Codex has no GitHub event trigger you can create outside its own app or chat, so its event loops rely on
  your session, or on a person posting `@codex` on the issue or pull request. aayudha never posts that mention from a
  workflow.
- **Claude Code** routines start on pull request and release events only. Auto-fix answers CI failures on one pull
  request a person turns it on for.
- **GitHub Actions** is a fallback that only records an event and never runs a model.

Stuck? See [loop problems](troubleshooting.md#loops).

> **Honest status:** "active" means recorded with the loop's current contents; no host lets aayudha read your
> automations back. On Amp, the scheduled `triage`, the `triage-new` spawner, and the chained `ci-failed` thread were
> proven live. The Cursor, Claude Code, and Codex forms are rendered from their documentation and not yet run on a
> real host.

## Under the hood

The loop format, the events and filters, each host's render, and the guard are in
[hand-offs, agent environments, and loops](how-agents.md#scheduled-and-event-loops).

## What's next

- Let a failed check go back by itself: [hand work to an agent](guide-build.md).
- See what the retro and evolve loops produce: [did it work?](guide-learn.md)
