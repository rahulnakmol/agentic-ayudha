# `research`: gather references and user research for a Feature

You are about to design screens for a Feature and do not yet know what people are trying to get done, or how others
solved it. `research` collects pinned references (page URLs and IDs, never copied pixels) and your notes on what
people said and did, and records them on the ledger for the design stages that follow.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | Collects design references and user research for a Feature. |
| Journey stage | [Design the screens](../journey-4-build.md#if-your-story-has-a-screen) |
| Accountable seat | [UX Researcher & Evolutionist](../../../seats/ux-researcher-evolutionist/CHARTER.md) (Hanu) |
| Does the work | Hanu, the Accountable seat's twin, in the [`spec`](../how-models.md) model lane, with the seat's paired human when it has one |
| Consulted | [Product Strategist](../../../seats/product-strategist/CHARTER.md) (Arjun) |
| Informed | [Marketer & Documenter](../../../seats/marketer-documenter/CHARTER.md) (Sauti) |
| Layers it can touch | [L01 Experience](../concepts-layers.md) |
| Who reasons | your agent's own model, inside the skill |
| Writes as | the bot identity (a GitHub App); without one, your identity, tagged as human fallback |
| Where it runs | the `aayu` command, a skill in your agent, an MCP tool (read-only unless `[mcp] expose_writes` is on) |
| The skill your agent reads | [skills/research/SKILL.md](../../../skills/research/SKILL.md) |

<!-- END GENERATED: skill-card -->

## When to use it

- A Feature starts from a blank page and its stories have screens nobody has drawn yet.
- You have support tickets, survey answers, or session notes, and want them turned into findings with their sources.
- You want to see how other products handle the same task before you sketch anything.

A pull request that changes a screen you already have needs [`review`](review.md), not research.

## Run it

Ask your agent to gather the references and write the notes. It puts both in a directory and pins them with the
command below.

```text
Use the research skill to gather design references and user research for Feature issue 42, and pin them.
```

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu research <feature>
```

| Input | Required | What it does |
|---|---|---|
| `<feature>` | yes | Feature to research. |
| `--from` | no | Directory with stage.json: the references the host gathered (URLs and IDs only) and Markdown research notes. |
| `--query` | no | Search Mobbin screens for this; needs [integrations.mobbin]. |
| `--platform` | no | Platform to search Mobbin on. One of: `ios`, `android`, `web`. |

<!-- END GENERATED: skill-usage -->

The usual call is `aayu research 42 --from .aayudha/design/42/research`.

> **Needs setup:** `--query` searches Mobbin only when `[integrations.mobbin]` is in `.aayudha/config.toml` with
> `secret = "mobbin"` and `scope = "read"`. Store the access token with `aayu auth set-secret mobbin`. Without it,
> pass the references you gathered yourself in `stage.json`.
>
> **Honest status:** the Mobbin search has not been proven live against a real Mobbin account here.

## What you get back

A notes file in plain Markdown: the job people are trying to get done, their words quoted exactly with a source,
numbers with their population ("3 of 5 sessions"), what the data does not say, and what each Completion Goal state
must make clear. A reference set where each reference is an https page URL with an ID and one line on why it was
kept. The binary refuses an image, and both land on the [ledger](../../../GLOSSARY.md) under
`design/<feature>/research/` with a manifest.

**You should see** a line that ends `written to ledger design/42/research/manifest.json`. Stuck? Run `aayu doctor`
([everything else](../troubleshooting.md#everything-else)).

## Gates

<!-- BEGIN GENERATED: skill-gates -->

It checks these before it acts, and stops when one fails:

- **halt:** The repository is not halted (aayu halt).

<!-- END GENERATED: skill-gates -->

## What only a person does

<!-- BEGIN GENERATED: skill-human -->

The registry reserves no step of this verb for a person.

<!-- END GENERATED: skill-human -->

The twin never invents a finding, so sessions with real people are a person's to run. A finding that changes a
story goes back as a Feedback issue (`aayu grill feedback`) for the story's owner to settle; the agent never edits a
Ready story itself.

## What's next

- Draw one low-fidelity screen per state from these notes: [`wireframe`](wireframe.md).
- The design stages in the story: [if your story has a screen](../journey-4-build.md#if-your-story-has-a-screen).
- [Every skill](README.md).
