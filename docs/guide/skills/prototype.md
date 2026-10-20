# `prototype`: build the design as working UI behind a flag

A mockup shows what a screen should look like, but not whether the real product can render it. `prototype` builds
the mockups as working UI on the product's real routes and data, off by default behind a feature flag, and compares
each state's render with its Figma frame before it records anything.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | Builds a working prototype behind a flag and compares it with the design. |
| Journey stage | [Design the screens](../journey-4-build.md#if-your-story-has-a-screen) |
| Accountable seat | ![Hanu, UX Researcher: the leap across the sea to find what people need](../../../design/icons/seats/ux-researcher-evolutionist.svg) [UX Researcher & Evolutionist](../../../seats/ux-researcher-evolutionist/CHARTER.md) (Hanu) |
| Does the work | Hanu, the Accountable seat's twin, in the [`build`](../how-models.md) model lane, with the seat's paired human when it has one |
| Consulted | [Code Architect](../../../seats/code-architect/CHARTER.md) (Vishwa) |
| Informed | [Engineering Manager](../../../seats/engineering-manager/CHARTER.md) (Drona) |
| Layers it can touch | [L01 Experience, L02 Interaction / API boundary](../concepts-layers.md) |
| Who reasons | your agent's own model, inside the skill |
| Writes as | the bot identity (a GitHub App); without one, your identity, tagged as human fallback |
| Where it runs | the `aayu` command, a skill in your agent, an MCP tool (only with `[mcp] expose_writes` on) |
| The skill your agent reads | [skills/prototype/SKILL.md](../../../skills/prototype/SKILL.md) |

<!-- END GENERATED: skill-card -->

## When to use it

- The mockups for a Feature are on the ledger and the screens exist as frames in a Figma file.
- You want the new screens in the product, reachable on a test environment, without changing what users see today.
- You want each Figma component mapped to code with Code Connect, so a designer can copy a working example.

No mockups yet? Use [`mockup`](mockup.md). Critiquing a pull request's screens is [`review`](review.md).

## Run it

Ask your agent to build the prototype, start the app on your test environment, and run the gate.

```text
Use the prototype skill to build Feature issue 42's mockups as a flagged prototype, and run the Figma diff gate.
```

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu prototype <feature>
```

| Input | Required | What it does |
|---|---|---|
| `<feature>` | yes | Feature to prototype. |
| `--from` | yes | Directory with stage.json: the OpenFeature flag, Figma file, Code Connect sources, components, and each state's path and Figma frame. |
| `--captures` | no | Diff these captures (captures.json with PNGs and computed styles) instead of rendering the states. |

<!-- END GENERATED: skill-usage -->

The usual call is `aayu prototype 42 --from .aayudha/design/42/prototype`.

> **Needs setup:** this step needs a Figma file and a read-only Figma token. Add `[integrations.figma]` to
> `.aayudha/config.toml` with `secret = "figma"` and `scope = "read"`, then run `aayu auth set-secret figma`. It
> also reads your token file (`[design] tokens`), an OpenFeature flag file in flagd JSON (`[design] flags`, by default
> `flags.flagd.json`), and the app running on a `[verify.environments]` target, never production.
>
> **Honest status:** the Figma adapter has not been proven live against a real Figma account here.

## What you get back

The prototype code: the route, its new components, the data calls, a seed that opens each state at its own path,
the flag, and one Code Connect mapping (a `.figma.tsx` file) per component. With the flag off, the route renders
exactly what it renders today. The gate renders each state at its Figma frame's size: at most 2 % of pixels may
differ (`[review] figma_diff_max`), and colour and typography token mismatches must be zero.

**You should see** one `figma-diff` line per state ending in `0 colour and 0 typography token mismatches`, then
`written to ledger design/42/prototype/manifest.json`. A miss exits 3 and writes nothing: fix the UI, not the
tolerance. Stuck? See [everything else](../troubleshooting.md#everything-else).

## Gates

<!-- BEGIN GENERATED: skill-gates -->

It checks these before it acts, and stops when one fails:

- **figma-diff:** Screenshot diff against Figma is within tolerance, with zero token mismatches.
- **halt:** The repository is not halted (aayu halt).
- **budget:** The [budget] and [autonomy] budget limits are not exceeded.

<!-- END GENERATED: skill-gates -->

## What only a person does

<!-- BEGIN GENERATED: skill-human -->

The registry reserves no step of this verb for a person.

<!-- END GENERATED: skill-human -->

The agent never adds or edits files in the design-system package: those changes belong to the person who owns it.
It turns the flag on only in the environment it captures, never in production.

## What's next

- Critique the screens on the pull request: [`review`](review.md).
- The design stages in the story: [if your story has a screen](../journey-4-build.md#if-your-story-has-a-screen).
- [Every skill](README.md).
