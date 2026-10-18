# `ship`: merge a proven pull request

A green check is not the same as a decision to merge. `ship` is the one way a story's pull request lands: it checks
the evidence again on the exact commit, needs a person's yes, then merges and writes down who approved what.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | Merges a proven pull request after your yes, and writes down who approved what, on which evidence. |
| Journey stage | [Ship it](../journey-6-ship.md) |
| Accountable seat | [Engineering Manager](../../../seats/engineering-manager/CHARTER.md) (Drona) |
| Does the work | Drona, the Accountable seat's twin, in the [`chore`](../how-models.md) model lane, with the seat's paired human when it has one |
| Consulted | [Quality Architect](../../../seats/quality-architect/CHARTER.md) (Yama) |
| Informed | [Product Strategist](../../../seats/product-strategist/CHARTER.md) (Arjun), [Marketer & Documenter](../../../seats/marketer-documenter/CHARTER.md) (Sauti) |
| Layers it can touch | [every layer](../concepts-layers.md) |
| Who reasons | no model: it is deterministic |
| Writes as | a person's own GitHub identity |
| Where it runs | the `aayu` command, a skill in your agent |
| The skill your agent reads | [skills/ship/SKILL.md](../../../skills/ship/SKILL.md) |

<!-- END GENERATED: skill-card -->

## When to use it

- The merge gate is green on your story's pull request, `inspect` says `merge`, and you have read the change.
- You work alone and GitHub will not let you approve your own pull request: use `--solo-bypass`, in the open.

Not yet green? Let the builder fix the findings first ([`inspect`](inspect.md)). Moving merged work to a release is
[`release`](release.md), not `ship`.

## Run it

Your agent runs `aayu ship <pr>` on its own story pull requests once the merge gate is green, and never merges one with
`gh`. When everything but a person's approval holds, ship comments **Ready to ship** on the pull request, and it waits
in your [`aayu inbox`](../how-trust.md), which ships it with every gate below. Only `--solo-bypass` is yours alone.
To have your agent walk you through one, ask:

```text
Use the ship skill to tell me whether pull request 148 is ready to ship, and what I still need to do.
```

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu ship <pr>
```

| Input | Required | What it does |
|---|---|---|
| `<pr>` | yes | Pull request to ship. |
| `--solo-bypass` | no | Solo mode only (one human, no bot App): merge without a second person's approval, recorded in the PR, the ship record, and the actions log. **Person only.** |

<!-- END GENERATED: skill-usage -->

## What you get back

A squash-merge commit on the base branch, the story's Outcome (Complete) section filled in, and the story moved to
`status:merged`. The approval, the rule that let it pass, and the layer streak are recorded in `ship.json` beside
the proof pack on the ledger. The build lease, the branch, and the worktree are cleaned up.

**You should see** the pull request merged and the story's Outcome section filled in. Stuck? See
[ship problems](../troubleshooting.md#ship).

## Gates

<!-- BEGIN GENERATED: skill-gates -->

It checks these before it acts, and stops when one fails:

- **merge-gate:** The base-branch merge gate is green with an authoritative pack for the head SHA.
- **human-pass:** An approving review from a human (type != Bot, not a bot login).

<!-- END GENERATED: skill-gates -->

When the change touches a risky layer (domain policy, stored data, or the platform), a CODEOWNER of those paths must
approve too, whatever the streak, and the plan's review must be on the ledger.

## What only a person does

<!-- BEGIN GENERATED: skill-human -->

- It writes under a person's own GitHub identity.
- It is never offered to agents as an MCP tool.
- A person approves the pull request. Working alone, you record a solo-mode bypass instead; it is never counted as an approval.
- `--solo-bypass`: Solo mode only (one human, no bot App): merge without a second person's approval, recorded in the PR, the ship record, and the actions log.

<!-- END GENERATED: skill-human -->

Working alone, first post a pull request comment that starts with `Solo-mode bypass:` and says why, then run
`aayu ship <pr> --solo-bypass`. The record says "Human pass: none possible", and [`audit`](audit.md) lists every
bypass. See [the human pass and solo mode](../concepts-human-pass.md).

## What's next

- Promote merged work to a snapshot or a stable release: [`release`](release.md).
- The whole flow, from a ready pull request to a merge: [a pull request is ready](../guide-review.md).
- [Every skill](README.md).
