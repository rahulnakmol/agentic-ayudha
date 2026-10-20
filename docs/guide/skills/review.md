# `review`: critique a pull request's screens

A screen can pass every test and still be hard to use with a keyboard, a screen reader, or a phone. `review` judges
a pull request's screens twice, apart from each other: once with a model, once with fixed accessibility checks. It
gives one verdict: recapture, rebuild, fix, or ship.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | Critiques a pull request's screens with a model and with fixed accessibility checks. |
| Journey stage | [Design the screens](../journey-4-build.md#if-your-story-has-a-screen) |
| Accountable seat | ![Hanu](../../../design/icons/seats/ux-researcher-evolutionist.svg) [UX Researcher & Evolutionist](../../../seats/ux-researcher-evolutionist/CHARTER.md) (Hanu) |
| Does the work | Hanu, the Accountable seat's twin, in the [`ui-review`](../how-models.md) model lane, with the seat's paired human when it has one |
| Consulted | [Quality Architect](../../../seats/quality-architect/CHARTER.md) (Yama) |
| Informed | [Engineering Manager](../../../seats/engineering-manager/CHARTER.md) (Drona) |
| Layers it can touch | [L01 Experience](../concepts-layers.md) |
| Who reasons | your agent's model, plus a direct pass by aayu (lane A direct) |
| Writes as | the bot identity (a GitHub App); without one, your identity, tagged as human fallback |
| Where it runs | the `aayu` command, a skill in your agent, an MCP tool (only with `[mcp] expose_writes` on) |
| The skill your agent reads | [skills/review/SKILL.md](../../../skills/review/SKILL.md) |

<!-- END GENERATED: skill-card -->

## When to use it

- A story's pull request changes a screen, and its Completion Goal has screenshot or recording criteria.
- You want accessibility, focus, and tap-target size checked on the real page, not on a mockup.
- `polish` applied a critique and you want a fresh verdict on the new head.

Applying the findings is [`polish`](polish.md). Whether the code may merge is [`inspect`](inspect.md).

## Run it

Check out the pull request's head and start the app on your test environment (`AAYU_ENV`, never production). Then:

```text
Use the review skill to critique the screens of pull request 57 and tell me its disposition.
```

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu review <pr>
```

| Input | Required | What it does |
|---|---|---|
| `<pr>` | yes | Pull request whose UI to review. |
| `--deep` | no | Run lane A at high effort when the lane's default effort is lower. |

<!-- END GENERATED: skill-usage -->

> **Honest status:** the model half of `review` needs a model key where it runs. This repository runs it in a CI
> workflow that holds the key.

## What you get back

The states come from the screenshot and recording criteria in the story's
[Ready snapshot](../../../GLOSSARY.md). Lane A, a model, scores Nielsen's 10 usability heuristics and walks each state
as keyboard-only, screen-reader, low-vision, and first-time users. It runs 3 times, and a finding counts only when 2
of the 3 runs report it. Lane B runs axe and fixed probes for focus indicators and target size; no lane A answer
overrides it. The critique goes to `.aayudha/review/<pr>/critique.json` and one pull request comment that each run
replaces.

**You should see** a comment headed `UI review: ship`. Any other disposition exits 4: on `fix`, run `polish`; on
`rebuild`, go back to mockup or prototype; on `recapture`, fix the app or the target and review again.

## Gates

<!-- BEGIN GENERATED: skill-gates -->

It checks these before it acts, and stops when one fails:

- **budget:** The [budget] and [autonomy] budget limits are not exceeded.

<!-- END GENERATED: skill-gates -->

It also refuses, with exit 4, when the checkout is not at the pull request's head, or when there is no UI state to
review. Stuck? See [everything else](../troubleshooting.md#everything-else).

## What only a person does

<!-- BEGIN GENERATED: skill-human -->

The registry reserves no step of this verb for a person.

<!-- END GENERATED: skill-human -->

The verdict is advice on the screens, not a merge. A person still approves the pull request before
[`ship`](ship.md) merges it.

## What's next

- Apply a `fix` critique and review again: [`polish`](polish.md).
- How review played out in the story: [if your story has a screen](../journey-4-build.md#if-your-story-has-a-screen).
- [Every skill](README.md).
