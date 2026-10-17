# `baseline`: capture reference screenshots of a commit

To tell whether a later change altered a page, you need a trustworthy picture of how it looked before. `baseline`
captures your configured pages for one exact commit, on a test environment, and pins the screenshots to that commit
on the ledger.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | Captures reference screenshots of a commit to compare later changes against. |
| Journey stage | [Design the screens](../journey-4-build.md#if-your-story-has-a-screen) |
| Accountable seat | [UX Researcher & Evolutionist](../../../seats/ux-researcher-evolutionist/CHARTER.md) (Iris) |
| Does the work | Iris, the Accountable seat's twin, in the [`chore`](../how-models.md) model lane, with the seat's paired human when it has one |
| Consulted | [Quality Architect](../../../seats/quality-architect/CHARTER.md) (Themis) |
| Informed | [Engineering Manager](../../../seats/engineering-manager/CHARTER.md) (Nestor) |
| Layers it can touch | [L01 Experience](../concepts-layers.md) |
| Who reasons | no model: it is deterministic |
| Writes as | the bot identity (a GitHub App); without one, your identity, tagged as human fallback |
| Where it runs | the `aayu` command, a skill in your agent, an MCP tool (only with `[mcp] expose_writes` on) |
| The skill your agent reads | [skills/baseline/SKILL.md](../../../skills/baseline/SKILL.md) |

<!-- END GENERATED: skill-card -->

## When to use it

- You are about to change screens and want a picture of the current commit for later reviews to compare against.
- A release or a known-good commit should be kept as the reference for its pages.

Judging a pull request's screens is [`review`](review.md), not a baseline.

## Run it

Ask your agent to check out the commit (in a separate worktree if you have work in progress), start the app on the
test environment, and capture it.

```text
Use the baseline skill to capture visual baselines for commit 4f2c1a9 on the preview environment.
```

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu baseline
```

| Input | Required | What it does |
|---|---|---|
| `--sha` | no | Commit to baseline; defaults to HEAD. |

<!-- END GENERATED: skill-usage -->

The usual call sets the target too: `AAYU_ENV=preview aayu baseline --sha 4f2c1a9`.

## What you get back

One screenshot per path in `[review] baseline` (by default `/`), and a manifest at `baselines/<sha>/manifest.json` on
the [ledger](../../../GLOSSARY.md). The screenshots go to the `[proof] media` bucket when one is set, to Git LFS on a
private repository without one, and otherwise only their SHA-256 hashes are kept. The agent leaves your work in
progress alone and stops the app and removes its worktree, even on failure.

> **Needs setup:** on a public repository without a `[proof] media` bucket, a later run can tell that a page
> changed, but there is no stored image of this commit to look at. Set up a bucket with `aayu init --integrations`,
> then run the baseline again.

**You should see** a line `baseline 4f2c1a9: N state(s); ledger baselines/…/manifest.json`, then one line per
screenshot with its hash and where it is stored. Stuck? See [everything else](../troubleshooting.md#everything-else).

## Gates

<!-- BEGIN GENERATED: skill-gates -->

It checks these before it acts, and stops when one fails:

- **halt:** The repository is not halted (aayu halt).

<!-- END GENERATED: skill-gates -->

It also refuses, with exit 4, when the checkout is not at `--sha`. That refusal is right: it stops other work being
recorded under that commit's name. It needs `[verify.environments.<env>]` with a URL and `[review] baseline` paths.

## What only a person does

<!-- BEGIN GENERATED: skill-human -->

The registry reserves no step of this verb for a person.

<!-- END GENERATED: skill-human -->

Nothing here needs a person. People come in later, when a pull request that changes these pages goes through
[`review`](review.md) and a person approves it before [`ship`](ship.md).

## What's next

- Critique a pull request's screens: [`review`](review.md).
- The design stages in the story: [if your story has a screen](../journey-4-build.md#if-your-story-has-a-screen).
- [Every skill](README.md).
