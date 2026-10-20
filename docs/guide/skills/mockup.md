# `mockup`: style the wireframes with your design system

A screen that invents its own colours drifts from the product and fails contrast later, in review. `mockup` turns
each agreed wireframe into a styled screen built only from your design system's components and tokens, and refuses
any colour that is not a token.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | Turns wireframes into mockups that use your design system. |
| Journey stage | [Design the screens](../journey-4-build.md#if-your-story-has-a-screen) |
| Accountable seat | ![Hanu](../../../design/icons/seats/ux-researcher-evolutionist.svg) [UX Researcher & Evolutionist](../../../seats/ux-researcher-evolutionist/CHARTER.md) (Hanu) |
| Does the work | Hanu, the Accountable seat's twin, in the [`build`](../how-models.md) model lane, with the seat's paired human when it has one |
| Consulted | [Product Strategist](../../../seats/product-strategist/CHARTER.md) (Arjun) |
| Informed | [Engineering Manager](../../../seats/engineering-manager/CHARTER.md) (Drona) |
| Layers it can touch | [L01 Experience](../concepts-layers.md) |
| Who reasons | your agent's own model, inside the skill |
| Writes as | the bot identity (a GitHub App); without one, your identity, tagged as human fallback |
| Where it runs | the `aayu` command, a skill in your agent, an MCP tool (only with `[mcp] expose_writes` on) |
| The skill your agent reads | [skills/mockup/SKILL.md](../../../skills/mockup/SKILL.md) |

<!-- END GENERATED: skill-card -->

## When to use it

- The wireframes for a Feature are on the ledger and agreed.
- You want each state shown as a person would see it, in your design system, before anyone writes product code.

No wireframes yet? Use [`wireframe`](wireframe.md). Ready to build the screens as working UI? That is
[`prototype`](prototype.md).

## Run it

Ask your agent to make one mockup per wireframe state, in SVG or HTML, and pin them.

```text
Use the mockup skill to make design-system mockups of every wireframe state for Feature issue 42, and pin them.
```

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu mockup <feature>
```

| Input | Required | What it does |
|---|---|---|
| `<feature>` | yes | Feature to mock up. |
| `--from` | yes | Directory with stage.json and one SVG or HTML mockup per wireframe state, coloured only with design tokens. |

<!-- END GENERATED: skill-usage -->

The usual call is `aayu mockup 42 --from .aayudha/design/42/mockup`.

> **Needs setup:** mockups are checked against your design system's token file, read from `[design] tokens` in
> `.aayudha/config.toml` (by default `packages/design-system/tokens/tokens.json`). Without that file the command
> stops. Point `[design] tokens` at your own token file first.

## What you get back

One mockup per wireframe state, with the same state IDs, items, and copy, styled with tokens only (on the web, the
`--ay-*` custom properties). The floors are designed in: body text at 4.5:1 contrast, a visible focus ring, 44 × 44
px targets, and a name for every control. They land on the [ledger](../../../GLOSSARY.md) under
`design/<feature>/mockup/`, pinned to the wireframes.

**You should see** a line with `pins wireframe` and then `written to ledger design/42/mockup/manifest.json`. A colour
that is not a token, or a missing state, fails with exit 3 and writes nothing. Without the wireframe stage it
refuses with exit 4. Stuck? See [everything else](../troubleshooting.md#everything-else).

## Gates

<!-- BEGIN GENERATED: skill-gates -->

It checks these before it acts, and stops when one fails:

- **halt:** The repository is not halted (aayu halt).
- **budget:** The [budget] and [autonomy] budget limits are not exceeded.

<!-- END GENERATED: skill-gates -->

## What only a person does

<!-- BEGIN GENERATED: skill-human -->

The registry reserves no step of this verb for a person.

<!-- END GENERATED: skill-human -->

The design system itself belongs to a person. A mockup uses what the system has; a change to the system is its
owner's to make.

## What's next

- Build the mockups as working UI behind a flag: [`prototype`](prototype.md).
- The design stages in the story: [if your story has a screen](../journey-4-build.md#if-your-story-has-a-screen).
- [Every skill](README.md).
