# Adopt at your own pace

You do not have to adopt everything at once. aayudha comes in three levels. Each one is a complete place to stop:
it pays for itself on its own, and you can stay there as long as you like. Move up when the next level solves a
problem you actually have.

![Three steps up: Level 1, clear work items; add the gates to reach Level 2, proof and the merge gate; add the bot App to reach Level 3, autonomy, loops, and integrations.](img/levels.svg)

*Three levels. Each step up adds one piece of setup.*

## Level 1: clear work items

**What you run:** `aayu tour` (which runs `init` and `install`), then `grill`, `ready`, and `status`.

**What it gives you:** every piece of work starts as a well-formed GitHub issue: Goals with a baseline and a target,
Features with a scope, and stories with numbered criteria that say what "done" means. Ready snapshots catch
promises that quietly change. `status` rolls it all up per Goal.

**What it costs:**

| Cost | At Level 1 |
|---|---|
| Keys | None beyond your own GitHub sign-in |
| CI changes | None: `init` adds issue forms, labels, a config file, and a records branch, but no workflows |
| CI minutes | None |
| Model spend | Only what your agent already uses: `grill` runs inside Claude Code, Codex, Cursor, or Amp |

**Good for:** trying aayudha on a real project in an afternoon, and teams whose main pain is fuzzy requirements.

> **Well done:** if you finished [your first hour](start.md), you are already at Level 1.

## Level 2: proof and the merge gate

**What you run:** `init --gates`, then `verify`, `inspect`, and `ship` on every pull request. Your agent can now
`build` stories end to end, or hand them to another agent's cloud ([hand work to an agent](guide-build.md)). Add a
feature map (`init --verify-skill`) and the gate also runs behaviour checks
([prove a behaviour change](guide-prove-behaviour.md)).

**What it gives you:** every story's pull request is checked in CI against fixed floors (coverage, security,
secrets, dependencies, and, for screens, accessibility and speed). Each one gets an authoritative proof pack and a
review against its criteria, and merges only with a person's yes. This is where "trust me" becomes "here is the
evidence".

**What it costs:**

| Cost | At Level 2 |
|---|---|
| Keys | One in CI: the review model's key, as the secret `AAYU_REVIEW_MODEL_KEY` in the repository's `code-review` environment, which only the base branches and the default branch can reach. A review app can review instead, with no key in CI. Triage runs from your own agent |
| CI changes | New workflows: the merge gate, a guard that removes Ready when a story is edited, and an incident runner you trigger by hand |
| CI minutes | About 10 minutes per push to a story's pull request in this repository (8 for the floors, 2 for the review); yours depend on your tests |
| Model spend | The review model's calls, billed by its provider to your key. `[budget] per_pr_inspect_max` caps each pull request at 5 USD by default. A review app bills through your own subscription to it instead |
| Storage | None to set up: screenshots and recordings stay in each CI run's `aayudha-pack` file for `[proof] retention_days` (90 by default, GitHub's limit is 90 days on a public repository), and `ship` copies the pack without them to the ledger. A private bucket (S3, R2, Azure Blob Storage, or Google Cloud Storage) is optional, for longer retention |

> **Needs setup:** the review model's key, as the secret `AAYU_REVIEW_MODEL_KEY` in **Settings → Environments →
> code-review**, which `aayu init --gates` creates ([code review in the merge gate](how-review.md)). Or a review app
> installed on your account, named in `[inspect] reviewer` and `[inspect] apps`. Proof needs no bot App, bucket, or
> secret: the pack waits as a file on the CI run with a comment on the pull request, and
> `ship` checks it came from that run and records it under your name.

**Good for:** teams that let agents write real code and need to know it is safe to merge.

## Level 3: autonomy, loops, and integrations

**What you run:** `init --identity` for the bot App, `[autonomy]` settings, `install --loops` for scheduled and
event loops, and `init --integrations` for PostHog, Sentry, and your cloud.

**What it gives you:**

- **A bot identity.** Agents write as a GitHub App, so their work is clearly theirs and a person's approval is
  clearly a person's.
- **Earned autonomy.** After 20 correction-free merges on a layer, agents may merge there without waiting for a
  person. Risky layers (domain policy, stored data, and the platform) always need a person, however long the
  streak. A protected workflow computes the streak, so no agent can fake it.
- **Loops.** Scheduled and event-driven runs keep the system moving: triage on arrival, Friday housekeeping, a failed
  check sent back to its builder, measurement after a release, a daily founder digest, a weekly retro, and a monthly
  skill evolution. They run in your agent's own scheduler, in your own session; where a host has no trigger for an
  event, your next session picks it up ([keep things moving with loops](guide-loops.md)).
- **Integrations.** Baselines and results come straight from PostHog, Sentry, Azure, AWS, or Google Cloud, and
  experiments can run on PostHog feature flags. Cloud access is read-only and keyless (see
  [Azure](../getting-started-azure.md), [AWS](../getting-started-aws.md), and [Google Cloud](../getting-started-gcp.md)).

**What it costs:**

| Cost | At Level 3 |
|---|---|
| Keys | The GitHub App is free; `init` creates it and stores its key as a secret. Read-only tokens for PostHog and Sentry. Read-only cloud access, with no stored key when it runs in GitHub Actions |
| CI changes | Optional: a loops workflow that only records events, with no model |
| CI minutes | Little: loops run in your agent's scheduler, and the Actions fallback is capped at 2,000 minutes a month by default |
| Model spend | Loops and autonomous merges use your agent more often, billed by your agent's plan as usual. Default caps on metered (pay-as-you-go) spend: 60 USD per story and 100 USD per day per repository; subscription use is not counted. When a cap trips, agents pause and a person is paged |
| Other | PostHog bills by usage on its side; Sentry adds no cost |

> **Honest status:** this repository runs at Level 2, with some loops. It has no bot App installed yet, so its agents
> write as the founder, tagged as such, and autonomous merges stay off. Its triage loops run live in Amp. Level 3 is
> built; this repository has not switched all of it on.

**Good for:** teams with a steady flow of work who want the agents to carry more of it, with the guard rails
switched on.

## How to move up

| From | To | Run |
|---|---|---|
| Nothing | Level 1 | `aayu tour` (it runs `aayu init`, then `aayu install --host <your host> --apply`) |
| Level 1 | Level 2 | `aayu init --gates`, commit the new workflows through a pull request, and add `AAYU_REVIEW_MODEL_KEY` to the `code-review` environment (or name a review app in `[inspect] reviewer` and `[inspect] apps`) |
| Level 2 | Level 3 | `aayu init --identity`, `aayu install --loops`, then `aayu init --integrations` for the services you use |

Every step is idempotent: running it again changes nothing. And `aayu uninit` removes what `init` created.

## What's next

- At Level 1? Follow [the journey](journey.md) up to step 3, and stop there for now.
- Ready for Level 2? Read [Prove it](journey-5-prove.md) to see what the merge gate gives you.
- Want the ideas behind the levels? See [the human pass and solo mode](concepts-human-pass.md) and
  [floors](concepts-floors.md).
