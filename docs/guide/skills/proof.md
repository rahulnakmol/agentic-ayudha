# `proof`: find and check the evidence for a change

Months after a merge, someone asks how you know a story works. `proof` lists, fetches, and checks the **proof packs**
on the ledger: the files that tie each of a story's promises to its evidence, for one exact commit.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | Stores, fetches, and checks the proof packs that hold the evidence for each change. |
| Journey stage | [Prove it](../journey-5-prove.md) |
| Accountable seat | [Quality Architect](../../../seats/quality-architect/CHARTER.md) (Themis) |
| Does the work | Themis, the Accountable seat's twin, in the [`chore`](../how-models.md) model lane, with the seat's paired human when it has one |
| Consulted | [AI Native Engineer](../../../seats/ai-native-engineer/CHARTER.md) (Talos) |
| Informed | [Auditor](../../../seats/auditor/CHARTER.md) (Argus) |
| Layers it can touch | [every layer](../concepts-layers.md) |
| Who reasons | no model: it is deterministic |
| Writes as | the bot identity (a GitHub App); without one, your identity, tagged as human fallback |
| Where it runs | the `aayu` command, a skill in your agent, an MCP tool (read-only unless `[mcp] expose_writes` is on) |
| The skill your agent reads | [skills/proof/SKILL.md](../../../skills/proof/SKILL.md) |

<!-- END GENERATED: skill-card -->

## When to use it

- You want to see every pack on the ledger, and whether its floors hold: `aayu proof list`.
- You want to check that a story's pack still matches what was agreed at Ready: `aayu proof verify <story>`.
- An agent wants to share its own preview as a record: `aayu proof push <story>` lands an advisory pack.

Want to produce the evidence in the first place? Run [`verify`](verify.md).

## Run it

Ask your agent. The ledger is a branch in your repository that holds records, never code
([glossary](../../../GLOSSARY.md)).

```text
Use the proof skill to verify the proof pack for story 42, and explain any problem it finds.
```

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu proof <action> [<story>]
```

| Input | Required | What it does |
|---|---|---|
| `<action>` | yes | Proof operation: push an advisory pack, get, verify, or list packs, or (merge gate only) comment: check the run's pack and keep its PR comment. One of: `push`, `get`, `verify`, `list`, `comment`. |
| `<story>` | no | Story whose pack to operate on. |
| `--media` | no | Include proof media; needs a bucket in [proof] media, since media never go to the ledger or Git LFS. |

<!-- END GENERATED: skill-usage -->

## What you get back

`verify` takes the story's newest authoritative pack, else its newest advisory one, and checks that every artifact
matches its hash, every criterion has evidence, the Completion Goal matches the Ready snapshot, and the head is the
pull request's current head. `push` writes `proof/<story>/<sha>/advisory/pack.json` to the ledger and posts its own
comment. `comment` runs only in the merge gate.

**You should see** a line like `authoritative pack at <sha>: N artifact(s) hashed, M not stored; floors hold: true`,
and exit 0. Each problem is listed, with exit 4. A pack can verify and still not ship: see
[ship problems](../troubleshooting.md#ship).

> **Needs setup:** `--media` needs a private bucket named in `[proof] media`. Without one, `proof push --media` is
> refused: media never go to the ledger or Git LFS. See [proof packs](../how-proof-packs.md#media-and-retention).

## Gates

<!-- BEGIN GENERATED: skill-gates -->

It checks no gate before it acts.

<!-- END GENERATED: skill-gates -->

The real line is between two kinds of pack. An advisory pack is an agent's claim; only the pack the merge gate wrote
in CI is authoritative ([proof packs](../concepts-proof.md)).

## What only a person does

<!-- BEGIN GENERATED: skill-human -->

The registry reserves no step of this verb for a person.

<!-- END GENERATED: skill-human -->

A person still makes the pack count. Only `aayu ship`, run by a person, lands the merge gate's pack on the ledger as
authoritative, after checking it came from the gate's own run on the exact head.

## What's next

- The pack is green and the review says `merge`: [`ship`](ship.md).
- What a pack holds, field by field: [proof packs in detail](../how-proof-packs.md).
- [Every skill](README.md).
