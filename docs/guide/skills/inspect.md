# `inspect`: get a second opinion from another vendor

An agent should never grade its own work. `inspect` has a model from a different vendor than the builder read the
pull request against every promise: by default the code-review environment's model, or a reviewer app you choose. It
reads the review as `merge`, `fix`, or `rebuild`.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | A model from another vendor, or an agent app you choose, reviews the pull request against every promise; aayu reads its reply as merge, fix, or rebuild. |
| Journey stage | [Prove it](../journey-5-prove.md) |
| Accountable seat | [Engineering Manager](../../../seats/engineering-manager/CHARTER.md) (Drona) |
| Does the work | Drona, the Accountable seat's twin, in the [`review`](../how-models.md) model lane, with the seat's paired human when it has one |
| Consulted | [Quality Architect](../../../seats/quality-architect/CHARTER.md) (Yama), [Code Architect](../../../seats/code-architect/CHARTER.md) (Vishwa) |
| Informed | [AI Native Engineer](../../../seats/ai-native-engineer/CHARTER.md) (Nakul) |
| Layers it can touch | [every layer](../concepts-layers.md) |
| Who reasons | aayu calls a model itself |
| Writes as | the bot identity (a GitHub App); without one, your identity, tagged as human fallback |
| Where it runs | the `aayu` command, a skill in your agent, an MCP tool (only with `[mcp] expose_writes` on) |
| The skill your agent reads | [skills/inspect/SKILL.md](../../../skills/inspect/SKILL.md) |

<!-- END GENERATED: skill-card -->

## When to use it

- The merge gate passed, and the `aayudha/inspect` check is red because the reviewer has not replied yet.
- You opened the pull request yourself, not through `build`, so nobody asked the reviewer.
- The reviewer raised a finding that does not apply to this repository, and you want to dismiss it for good.

Checks failing before review? Fix them with [`verify`](verify.md) first.

## Run it

Run it outside CI, as the builder or a person: the reviewer apps answer only the people who connected them, never a
workflow.

```text
Use the inspect skill to ask the reviewer to review pull request 148, and tell me its verdict for each criterion.
```

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu inspect <pr>
```

| Input | Required | What it does |
|---|---|---|
| `<pr>` | yes | Pull request to inspect. |
| `--mode` | no | general: one reviewer; peer: two reviewers of different families cross-check each other (default from [inspect]). One of: `general`, `peer`. |
| `--dismiss` | no | Record the finding with this ID as dismissed in this repository, so inspect stops raising findings like it. **Person only.** |
| `--approve` | no | As the [autonomy] approver, record your review of the PR's head on the ledger and post it as the human review the merge gate reads; a person-verified step, and the only way a human review counts. To approve several such PRs with one device code, use `aayu inbox`. **Person only.** |
| `--reason` | no | Why the finding is dismissed (with --dismiss), or what you checked (with --approve). **Person only.** |

<!-- END GENERATED: skill-usage -->

> **Needs setup:** the repository's `code-review` environment with its review model and key (`aayu init --gates`
> creates it; you add the key), or a reviewer app you choose in `[inspect] reviewer`. `aayu doctor`'s `review-rung` row
> shows the setup. See [set up the reviewer](../guide-review.md#before-your-first-pull-request-set-up-the-reviewer) and
> [code review in the gate](../how-review.md).

## What you get back

One comment on the pull request, replaced in place on each run, headed with the disposition, such as
`inspect: merge`. It holds a verdict per criterion (`met`, `unmet`, or `unclear`), findings rated P0 to P3, and any
check the pull request weakens. A failing floor, a P0 or P1 finding, or an unmet criterion turns `merge` into `fix`.

**You should see** the `aayudha/inspect` check turn green with `merge`. It exits 4 on anything else, so the check
stays red: no reply fails closed. See [merge gate problems](../troubleshooting.md#merge-gate).

## Gates

<!-- BEGIN GENERATED: skill-gates -->

It checks these before it acts, and stops when one fails:

- **authoritative-pack:** An authoritative (CI-written) proof pack exists.
- **budget:** The [budget] and [autonomy] budget limits are not exceeded.

<!-- END GENERATED: skill-gates -->

The reviewer reads only after the merge gate has written its proof pack. After two `fix` or `rebuild` rounds,
`inspect` stops spending and pages a person ([when it says fix](../guide-review.md#when-it-says-fix)).

## What only a person does

<!-- BEGIN GENERATED: skill-human -->

- `--dismiss`: Record the finding with this ID as dismissed in this repository, so inspect stops raising findings like it.
- `--approve`: As the [autonomy] approver, record your review of the PR's head on the ledger and post it as the human review the merge gate reads; a person-verified step, and the only way a human review counts. To approve several such PRs with one device code, use `aayu inbox`.
- `--reason`: Why the finding is dismissed (with --dismiss), or what you checked (with --approve).

<!-- END GENERATED: skill-human -->

Only a person dismisses a finding, with a reason: `aayu inspect 148 --dismiss <id> --reason "…"`. A person also
decides whether a weakened check is right, and approves the pull request before [`ship`](ship.md).

## What's next

- The verdict is `merge`: [`ship`](ship.md).
- The whole flow, from a pull request to a merge: [a pull request is ready](../guide-review.md).
- [Every skill](README.md).
