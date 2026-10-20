# `wireframe`: sketch one screen per state

Arguing about colours before anyone agrees what a screen shows wastes a round. `wireframe` records one plain,
unstyled sketch per state the stories promise, built from the research, and pins it to that research on the ledger.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | Sketches the screens a Feature needs, from its research and promises. |
| Journey stage | [Design the screens](../journey-4-build.md#if-your-story-has-a-screen) |
| Accountable seat | ![Hanu, UX Researcher: the leap across the sea to find what people need](../../../design/icons/seats/ux-researcher-evolutionist.svg) [UX Researcher & Evolutionist](../../../seats/ux-researcher-evolutionist/CHARTER.md) (Hanu) |
| Does the work | Hanu, the Accountable seat's twin, in the [`build`](../how-models.md) model lane, with the seat's paired human when it has one |
| Consulted | [Product Strategist](../../../seats/product-strategist/CHARTER.md) (Arjun) |
| Informed | [Engineering Manager](../../../seats/engineering-manager/CHARTER.md) (Drona) |
| Layers it can touch | [L01 Experience](../concepts-layers.md) |
| Who reasons | your agent's own model, inside the skill |
| Writes as | the bot identity (a GitHub App); without one, your identity, tagged as human fallback |
| Where it runs | the `aayu` command, a skill in your agent, an MCP tool (only with `[mcp] expose_writes` on) |
| The skill your agent reads | [skills/wireframe/SKILL.md](../../../skills/wireframe/SKILL.md) |

<!-- END GENERATED: skill-card -->

## When to use it

- The research for a Feature is on the ledger and its stories name UI states in their Completion Goals.
- You want structure, copy, and keyboard order agreed before anyone picks colours or fonts.
- A research finding left a choice open, and you want it marked as an assumption for testing to settle.

No research yet? Start with [`research`](research.md). Wireframes agreed and ready for the design system? That is
[`mockup`](mockup.md).

## Run it

Ask your agent to draw the wireframes in a directory with a `stage.json` that lists each state, then pin them.

```text
Use the wireframe skill to draw one wireframe per UI state for Feature issue 42 from its research, and pin them.
```

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu wireframe <feature>
```

| Input | Required | What it does |
|---|---|---|
| `<feature>` | yes | Feature to wireframe. |
| `--from` | yes | Directory with stage.json and one wireframe per state (SVG, HTML, Markdown, or an image). |

<!-- END GENERATED: skill-usage -->

The usual call is `aayu wireframe 42 --from .aayudha/design/42/wireframe`.

## What you get back

One wireframe per state, with the state IDs the stories use, in SVG, HTML, or Markdown (an image is accepted but
cannot be checked). Each shows the state's real copy, the research finding behind each choice, the focus order,
what a screen reader announces, 44 × 44 px targets, and the phone layout at 390 px when the product is used on
phones. They land on the [ledger](../../../GLOSSARY.md) under `design/<feature>/wireframe/`, with a manifest that
pins the research by its SHA-256.

**You should see** a line with `pins research` and then `written to ledger design/42/wireframe/manifest.json`. It
refuses with exit 4 until the research stage is on the ledger. Stuck? See
[everything else](../troubleshooting.md#everything-else).

## Gates

<!-- BEGIN GENERATED: skill-gates -->

It checks these before it acts, and stops when one fails:

- **halt:** The repository is not halted (aayu halt).
- **budget:** The [budget] and [autonomy] budget limits are not exceeded.

<!-- END GENERATED: skill-gates -->

The budget gate stops agent work with exit 6 when a `[budget]` limit trips. A person reads why, then runs
`aayu resume` ([loops](../troubleshooting.md#loops)).

## What only a person does

<!-- BEGIN GENERATED: skill-human -->

The registry reserves no step of this verb for a person.

<!-- END GENERATED: skill-human -->

The assumptions a wireframe marks are left for a test with people to settle, not for the agent. Running that test is
a person's work.

## What's next

- Turn the wireframes into design-system screens: [`mockup`](mockup.md).
- The design stages in the story: [if your story has a screen](../journey-4-build.md#if-your-story-has-a-screen).
- [Every skill](README.md).
