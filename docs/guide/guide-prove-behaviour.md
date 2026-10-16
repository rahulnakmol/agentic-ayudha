# Prove a behaviour change

Tests pass. Coverage is high. And the button still does nothing. Floors check the code; a **feature map** checks what
your product actually does. It lists what your product can do, with a behaviour check for each capability, and the
merge gate runs the checks a pull request touches, with no model and no AI key.

> **You are here:** Level 2, with the merge gate running ([a pull request is ready](guide-review.md)).
>
> **Needs aayudha 0.2.** If `aayu version` prints 0.1.x, this page is ahead of your binary: run `aayu self-update`.
> Until 0.2 is released, `go install github.com/rahulnakmol/agentic-ayudha/cmd/aayudha@dev` installs the build this
> page describes.

## What a behaviour check is

A behaviour check drives your product the way a person or a client would. It comes in four kinds, so it fits any
product, not only web apps:

| Kind | Drives | Example |
|---|---|---|
| `ui` | a page, with a browser | open the Ask page, type a question, see a quoted answer |
| `cli` | a command | run `notes add`, then `notes list`, and see the note |
| `api` | an HTTP call | `POST /api/ask` returns the answer and its source |
| `lib` | a function call | the library parses a date it used to reject |

A small **control tool**, written in your product's own stack, runs each check and reports `passed`, `failed`, or
`unreachable`. It adds no runtime your product lacks: Node and Playwright for a web app, Go for a Go module, a shell
script for a command line tool.

## Create your map

```sh
aayu init --verify-skill
```

It writes a starter **verify skill** at `.agents/skills/verify-<product>/`: a `SKILL.md`, a control tool chosen from
your stack, and an empty `features/` folder. It asks which agent app should keep the map up to date, and offers only
the ones it detects. `aayu install` makes the skill available to your agent.

Then ask your agent to fill it in, in plain words:

> Use the verify skill to map this product's features, with one behaviour check per capability.

The agent writes one file per feature: what it is, how to reach it, its behaviour checks, and its gotchas.

**You should see** `aayu verify --map` exit 0. It compares the map with your code, runs nothing, and lists any
**drift**: a route, command, or function your code has that no feature lists, or one the map lists that is gone.

## What the gate does with it

On every pull request:

- **Behaviour checks.** The checks of every feature the change touches run. A failed check turns the gate red at
  once. `unreachable` means a precondition is missing: it proves nothing and fails nothing.
- **Drift.** The map is compared with the code. Drift is a warning until `[verify] drift_blocking_from`, a date in
  your config, and blocks from that date on.
- **Evidence.** A promise in the story can name checks in `.aayudha/evidence/<story>.toml`. It counts as proven only
  when every one of them passed and left evidence of the right kind.

Try a check yourself before you push:

```sh
aayu verify --check <feature>      # or <feature>.<check>, or all; a preview outside the gate
```

## Keeping the map honest

Code moves and maps drift. Two loops keep up: a nightly sweep runs every check, and a weekly maintain pass, when
something drifted or failed, posts a brief that @-mentions your maintainer app. The app opens at most one pull request
of map fixes, through the same gate. It may never weaken a check: a real regression becomes a Bug. Set the loops up
with [`aayu install --loops`](guide-loops.md).

## Bugs: proof that it was broken

For a Bug story, the gate also runs the tests that prove the fix on the base commit, before the fix. They should fail
there and pass on the pull request: "red on base, green on head". This is advisory until
`[verify] red_on_base_blocking_from`, and runs for Go tests mapped to the Bug's promises.

> **Honest status:** both dates are 13 November 2026 by default, so drift and red-on-base are warnings until then.
> aayudha's own repository has no feature map of its own yet; its two examples, benefits-qa and notes, do.

## Under the hood

The map format, the control tool's JSON contract, how checks are picked, and the maintain brief are in
[verify skills, feature maps, and behaviour checks](how-verify-maps.md).

## What's next

- See where behaviour checks fit on the day: [chapter 5 of the story](journey-5-prove.md).
- Ship the proven pull request: [a pull request is ready](guide-review.md#ship-it).
