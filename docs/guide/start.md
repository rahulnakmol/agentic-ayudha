# Your first hour

In the next hour you will install aayudha, connect one GitHub repository, teach your agent the aayudha skills, and
file your first Goal. Nothing here changes your CI or needs a model key. You can stop at the end and still have
something useful: clear, checked work items on GitHub.

> **You are here:** setting up. Next comes [the journey](journey.md), from an idea to a measured release.

## What you need

- A GitHub repository you administer, and the `gh` command line signed in (`gh auth login`), or a token in
  `AAYU_GITHUB_TOKEN`.
- `git` 2.40 or newer and `git-lfs`. aayudha keeps its records, such as the agreed version of each story and the
  evidence for each change, on a separate branch called `ledger`. Screenshots and recordings stay on the CI run that
  made them and never go there.
- Until the first release: Go 1.25 or newer, to install the snapshot (see below).
- One of the agent hosts: Amp, Cursor, or Claude Code. aayudha works inside the agent you already use.

## 1. Install aayudha

> **Before the first release:** there is no stable release yet, so the install line below returns a 404 for now.
> Install the latest snapshot with Go 1.25 or newer instead:
>
> `go install github.com/rahulnakmol/agentic-ayudha/cmd/aayudha@rolling-2026-10-12.4`
>
> This installs `aayudha` into your Go bin directory. Add `alias aayu=aayudha` to your shell profile so the short
> name works too. Once v0.1.0 is out, use the line below and skip this note.

```sh
curl -fsSL https://raw.githubusercontent.com/rahulnakmol/agentic-ayudha/main/install.sh | sh
```

The script installs a signed release and checks its signature and checksum before it copies anything. You get two
names for the same program: `aayudha` and its short alias `aayu`.

## 2. Check your machine

```sh
aayu doctor
```

`doctor` checks your tools, your GitHub sign-in, and your setup, one row each. Green `PASS` rows are good. `WARN`
and `N/A` rows are optional things you can set up later, so do not worry about them today. Only a `FAIL ...
[blocking]` row needs fixing now, and it says how.

## 3. Connect your repository

In a clone of your repository, run:

```sh
aayu init
```

`init` asks you to confirm, then sets up the repository for work items: labels, a small config file in
`.aayudha/`, and the `ledger` branch for records. It adds issue forms for the ten kinds of work item: Goal,
Initiative, Feature, Story, Bug, Decision, Experiment, Release, Feedback, and Incident. It adds no CI workflows and
needs no keys. Commit the files it wrote through a pull request, like any other change.

It fits around what you have:

- **`AGENTS.md`:** `init` adds one marked section and leaves the rest of your file alone.
- **Branches:** if you have no `dev` branch, `init` maps your existing branches, such as `main`, and renames
  nothing.
- **Undo:** `aayu uninit` removes what `init` created.

## 4. Teach your agent the skills

```sh
aayu install --host amp            # or cursor, claude, or all; shows what it will write
aayu install --host amp --apply    # writes it
```

Each aayudha verb becomes a skill your agent can use: a short instruction file it reads when you ask for that
verb. The skills call `aayu` for anything that writes to GitHub, so every write is logged.

## 5. File your first Goal

Open your agent in the repository and ask in plain words:

> Use the grill skill to write a Goal for: customers wait too long for a reply to support tickets.

In Claude Code you can also type `/grill`. The agent interviews you, one question at a time: what outcome you want
and for whom, which number shows it, where that number is today, and where it should be. When every required
section is filled in and the target has a baseline, it files the Goal as a GitHub issue.

> **Tip:** a Goal needs a number you can measure again later. If you have no analytics connected yet, pick a number
> GitHub already knows, such as merged pull requests, or give a value from an export you have. You can also start
> one level down with a Feature: ask for "a Feature" instead of "a Goal".

Then mark it Ready. Ready means complete, agreed, and frozen: aayudha records exactly what was agreed, so later
changes are visible. You will see why this matters in [step 3](journey-3-ready.md).

```sh
aayu ready <issue> --check   # read-only: says what is missing, if anything
aayu ready <issue>           # labels it Ready and posts a snapshot of exactly what was agreed
```

> **Well done:** you have a Goal on GitHub that anyone on your team can read, with a baseline and a target. That
> alone beats most backlogs.

## What's next

See where this Goal goes from here: [the journey at a glance](journey.md). Or, if you want to keep things small for
now, read [Adopt at your own pace](adopt.md) for what you have already (Level 1) and what each next level costs.
