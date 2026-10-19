# `verify`: run the fixed checks before the gate does

An agent saying "all tests pass" is a claim. `verify` runs the **floors**, fixed checks with fixed thresholds, on
your change, and tells you what the merge gate will find, with no model deciding the result.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | Runs the fixed checks (tests, security, secrets, dependencies, accessibility, speed) and records the results. |
| Journey stage | [Prove it](../journey-5-prove.md) |
| Accountable seat | [AI Native Engineer](../../../seats/ai-native-engineer/CHARTER.md) (Nakul) |
| Owns the thresholds | [Quality Architect](../../../seats/quality-architect/CHARTER.md) (Yama) |
| Does the work | Nakul, the Accountable seat's twin, in the [`chore`](../how-models.md) model lane, with the seat's paired human when it has one |
| Consulted | none |
| Informed | [Engineering Manager](../../../seats/engineering-manager/CHARTER.md) (Drona) |
| Layers it can touch | [every layer](../concepts-layers.md) |
| Who reasons | no model: it is deterministic |
| Writes as | the bot identity (a GitHub App); without one, your identity, tagged as human fallback |
| Where it runs | the `aayu` command, a skill in your agent, an MCP tool (only with `[mcp] expose_writes` on) |
| The skill your agent reads | [skills/verify/SKILL.md](../../../skills/verify/SKILL.md) |

<!-- END GENERATED: skill-card -->

## When to use it

- You want to know whether a story is done before you push, criterion by criterion.
- A floor missed in the merge gate and you want to reproduce it: `--floor coverage` runs one floor.
- The branch changes what the product does: `--map` shows drift, and `--check <feature>` runs its behaviour checks.

Want a reviewer's verdict on the pull request instead? Use [`inspect`](inspect.md).

## Run it

Ask your agent, in the worktree that holds the change. It runs the floors once and answers for each criterion.

```text
Use the verify skill to tell me whether issue 42 is done, criterion by criterion, and what to fix first.
```

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu verify [<pr>]
```

| Input | Required | What it does |
|---|---|---|
| `<pr>` | no | Pull request to verify; defaults to the current worktree. |
| `--floor` | no | Run only this floor. |
| `--map` | no | Only the feature-map drift check: compare every .agents/skills/verify-*/ map with the source, run nothing, and exit 3 on drift. |
| `--check` | no | Only the behaviour floor, on these feature-map checks: \<feature>, \<feature>.\<check>, or all (the sweep); a run outside the merge gate is a preview. |
| `--maintain` | no | With --map or --check: post the maintain brief on the tracking issue for the agent app [verify] maintainer names, when the map drifted or a check failed (drift), or every time (always). One of: `drift`, `always`. |
| `--security` | no | Only the security floors (sast, secrets, deps, dast) on the checked-out commit, with no PR or story: what aayudha-security.yml runs on every push and nightly. With --ci, each failing finding files or updates one aayudha:security Bug. |
| `--since` | no | With --security: scan secrets only in the commits after this revision (a push's before); without it, or with all zeros or an unknown revision, the whole history of HEAD. |

<!-- END GENERATED: skill-usage -->

For one floor on pull request 148, from a checkout of its head: `aayu verify 148 --floor coverage`.

## What you get back

One line per floor: its result, value, threshold, and the tool that measured it. A local
[proof pack](../concepts-proof.md), which ties each criterion to its evidence, and behaviour-check screenshots and
traces in `.aayudha/state/verify/`. A local run is a preview: only the merge gate's pack counts.

**You should see** each floor as `pass`, then `pack written to …` and exit 0. Exit 3 is a floor miss: fix the cause,
never the threshold. "Tool not installed"? See [merge gate problems](../troubleshooting.md#merge-gate).

> **Needs setup:** `--map` and `--check` need a feature map. Without one, `verify --map` says so; start it with
> `aayu init --verify-skill` and [prove a behaviour change](../guide-prove-behaviour.md).

## Gates

<!-- BEGIN GENERATED: skill-gates -->

It checks these before it acts, and stops when one fails:

- **halt:** The repository is not halted (aayu halt).

<!-- END GENERATED: skill-gates -->

The floors themselves are the real gate. Their thresholds live in your configuration, owned by the Quality
Architect, and a model cannot overrule them ([floors](../concepts-floors.md)).

## What only a person does

<!-- BEGIN GENERATED: skill-human -->

The registry reserves no step of this verb for a person.

<!-- END GENERATED: skill-human -->

To ship with a floor miss, a person with the authority accepts a waiver, a recorded Decision that
[`audit`](audit.md) counts. Before an agent changes a feature map, it shows you the new checks and waits for a yes.

## What's next

- The merge gate ran; read the review: [`inspect`](inspect.md).
- The whole flow, from a pull request to a merge: [a pull request is ready](../guide-review.md).
- [Every skill](README.md).
