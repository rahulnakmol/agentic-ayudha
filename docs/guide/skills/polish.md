# `polish`: fix what the design review found

A critique is only useful once someone fixes every finding without hiding the problem. `polish` applies a `review`
critique to the pull request's screens with design-system tokens and components only, then runs both review lanes
again and says which findings are resolved, which remain, and which are new.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | Applies a design critique to a pull request and reviews it again. |
| Journey stage | [Design the screens](../journey-4-build.md#if-your-story-has-a-screen) |
| Accountable seat | ![Hanu](../../../design/icons/seats/ux-researcher-evolutionist.svg) [UX Researcher & Evolutionist](../../../seats/ux-researcher-evolutionist/CHARTER.md) (Hanu) |
| Does the work | Hanu, the Accountable seat's twin, in the [`build`](../how-models.md) model lane, with the seat's paired human when it has one |
| Consulted | none |
| Informed | [Engineering Manager](../../../seats/engineering-manager/CHARTER.md) (Drona) |
| Layers it can touch | [L01 Experience](../concepts-layers.md) |
| Who reasons | your agent's own model, inside the skill |
| Writes as | the bot identity (a GitHub App); without one, your identity, tagged as human fallback |
| Where it runs | the `aayu` command, a skill in your agent, an MCP tool (only with `[mcp] expose_writes` on) |
| The skill your agent reads | [skills/polish/SKILL.md](../../../skills/polish/SKILL.md) |

<!-- END GENERATED: skill-card -->

## When to use it

- `review` gave the pull request the disposition `fix`.
- Some findings remain after a polish run, and you want to work through them.

A `rebuild` disposition means the design is wrong at its root: go back to [`mockup`](mockup.md) or
[`prototype`](prototype.md). No critique yet? Run [`review`](review.md) first.

## Run it

Ask your agent to fix the findings, commit them on the pull request's branch, restart the app on the review target,
and compare.

```text
Use the polish skill to fix every finding in the review critique of pull request 57, then re-run the review.
```

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu polish <pr>
```

| Input | Required | What it does |
|---|---|---|
| `<pr>` | yes | Pull request to polish. |

<!-- END GENERATED: skill-usage -->

## What you get back

Fix commits on the pull request's branch that change only the lines a finding needs, with every changed value a
design token. A pull request note lists each finding (lane and element), its fix, that nothing was waived or hidden,
and how to re-run. Then a fresh comparison of both lanes against the critique in
`.aayudha/review/<pr>/critique.json`.

**You should see** `polish: N resolved, 0 remaining, N new` in the output and in the review comment, with exit 0.
Anything remaining exits 4: repeat for what remains. Without a critique it refuses and tells you to run
`aayu review` first. Stuck? See [everything else](../troubleshooting.md#everything-else).

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

`polish` never lowers a floor, hides an element from the capture, or waives a finding. A waiver is a Decision a person
accepts: see [`decide`](decide.md) and its `--waiver` option.

## What's next

- Once both lanes are clean, merge with a person's yes: [`ship`](ship.md).
- The whole flow for a ready pull request: [a pull request is ready](../guide-review.md).
- [Every skill](README.md).
