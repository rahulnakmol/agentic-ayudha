# `counsel`

Asks the founder's sage for a steer on anything, with the evidence behind it and the next step; it advises and never approves or decides.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | ![counsel](../../../design/icons/verbs/counsel.svg) Asks the founder's sage for a steer on anything, with the evidence behind it and the next step; it advises and never approves or decides. |
| Journey stage | [Explain it](../journey-9-explain.md) |
| Accountable seat | ![Atri](../../../design/icons/seats/founder.svg) [Founder](../../../seats/founder/CHARTER.md) (Atri) |
| Does the work | Atri, the Accountable seat's twin, in the [`spec`](../how-models.md) model lane, with the seat's paired human when it has one |
| Consulted | [Product Strategist](../../../seats/product-strategist/CHARTER.md) (Arjun), [Code Architect](../../../seats/code-architect/CHARTER.md) (Vishwa), [Engineering Manager](../../../seats/engineering-manager/CHARTER.md) (Drona) |
| Informed | [Auditor](../../../seats/auditor/CHARTER.md) (Vidhi) |
| Layers it can touch | [every layer](../concepts-layers.md) |
| Who reasons | your agent's own model, inside the skill |
| Writes as | the bot identity (a GitHub App); without one, your identity, tagged as human fallback |
| Where it runs | the `aayu` command, a skill in your agent, an MCP tool (read-only unless `[mcp] expose_writes` is on) |
| The skill your agent reads | [skills/counsel/SKILL.md](../../../skills/counsel/SKILL.md) |

<!-- END GENERATED: skill-card -->

## When to use it

When you want a second opinion before a call: a question ("should we split the survey into sections?"), an issue or
Decision, a file or folder, or a pull request. Atri, the founder's sage, frames the call as a one-way or a two-way
door, reads it from operations, technology, and product, says which reading dominates, and recommends one thing. It
also says what not to do, what would change the advice, the next `aayu` command, and the one question you must
answer. Every claim cites its evidence (a file and line, a proof pack, a run, a Decision); anything it cannot cite is
marked unverified.

Your agent can call it on its own, because it only reads and advises.

## Run it

Ask your agent with the question, the issue, the path, or the pull request. It reads the grounding through `aayu`,
writes Atri's counsel, and checks it before it shows you.

```text
Use the counsel skill on PR 520: should this go in as it is?
```

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu counsel [<topic>]
```

| Input | Required | What it does |
|---|---|---|
| `<topic>` | no | A question in quotes, an issue or Decision as #N, or a path in this checkout; or use --pr or --issue. |
| `--pr` | no | Pull request to counsel on: its changed files by layer, its proof pack, and its builder family. |
| `--issue` | no | Issue or Decision to counsel on: it, its parent, and its links. |
| `--draft` | no | Markdown counsel the host's model wrote; counsel checks every part, the stage's artefact, and every citation. Without it, counsel prints the grounding and the outline. |
| `--post` | no | Post the checked counsel on the PR or issue as one comment labelled counsel, replaced in place on a rerun (through the actions log); needs --draft and --pr or --issue. Never a review or an approval. |

<!-- END GENERATED: skill-usage -->

## What you get back

Without `--draft`, the grounding: the cited facts (the issue and its links, a path's layer and owner seat, or a pull
request's changed files by layer, its proof pack's floors, and its builder family), the open Decisions and floor
waivers, the `CONSTRAINTS.md` floors, the outline, and Atri's persona. With `--draft`, either "complete" or the list
of findings to fix. With `--post`, one comment on the pull request or issue.

**You should see** a counsel titled with the team stage's artefact, such as `# Three-hat line: …` at a startup,
with seven parts in order, ending with `## Your question`. A posted counsel opens with "Counsel from Atri, the
Founder's Sage. Advice only: not a review, an approval, or a Decision" and says same-family or different-family.

## Gates

<!-- BEGIN GENERATED: skill-gates -->

It checks these before it acts, and stops when one fails:

- **halt:** The repository is not halted (aayu halt).

<!-- END GENERATED: skill-gates -->

## What only a person does

<!-- BEGIN GENERATED: skill-human -->

The registry reserves no step of this verb for a person.

<!-- END GENERATED: skill-human -->

Counsel is never a gate. It does not approve, accept, merge, or count as a review, and a posted counsel says so. You
make the call, and a reviewer still reviews. Posting is opt-in: `--post` puts one comment labelled counsel on the PR
or issue, and a rerun replaces it.

## What's next

- Record a call the counsel framed: [`decide`](decide.md).
- Sharpen a story it questioned: [`grill`](grill.md).
- [Every skill](README.md).
