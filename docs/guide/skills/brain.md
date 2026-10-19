# `brain`

Keeps the product's lasting knowledge as linked notes that link to issues and outlive any one person.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | Keeps the product's lasting knowledge as linked notes that link to issues and outlive any one person. |
| Journey stage | [Explain it](../journey-9-explain.md) |
| Accountable seat | [Product Strategist](../../../seats/product-strategist/CHARTER.md) (Arjun) |
| Does the work | Arjun, the Accountable seat's twin, in the [`chore`](../how-models.md) model lane, with the seat's paired human when it has one |
| Consulted | [Founder](../../../seats/founder/CHARTER.md) (Atri), [Revenue Architect](../../../seats/revenue-architect/CHARTER.md) (Kuber) |
| Informed | [Marketer & Documenter](../../../seats/marketer-documenter/CHARTER.md) (Sauti) |
| Layers it can touch | none: it changes no product code |
| Who reasons | no model: it is deterministic |
| Writes as | the bot identity (a GitHub App); without one, your identity, tagged as human fallback |
| Where it runs | the `aayu` command, a skill in your agent, an MCP tool (read-only unless `[mcp] expose_writes` is on) |
| The skill your agent reads | [skills/brain/SKILL.md](../../../skills/brain/SKILL.md) |

<!-- END GENERATED: skill-card -->

## When to use it

Use it to keep what the product is and why: the vision, the principles, the direction, the objectives and the
objections against them, the decisions and their reasons, and what each capability does. Issues stay the record of
the work; a note links an issue by number and never copies it. When a person leaves, the notes stay, each owned by a
seat.

- `aayu brain init` once, to create the `brain` branch: an Obsidian vault with a Home note, a map for each folder,
  and a template for each note type. `--workstream <name>` adds a workstream with its own index and map.
- `aayu brain open` to see how to open it in Obsidian (a `git worktree add` of the branch).
- `aayu brain check` before you push a note: links resolve, frontmatter is valid, every note cites its sources,
  sentences stay short, and no note holds a pasted Completion Goal line or issue label. The rest of the no-copy rule
  is yours to keep.
- `aayu brain checkpoint` after each release (`aayu release` runs it) and on demand. It refreshes only the marked
  generated blocks and lists the notes a person should review.

On a public repository the `brain` branch is public: write only what is already public.

## Run it

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu brain <action>
```

| Input | Required | What it does |
|---|---|---|
| `<action>` | yes | init: create the brain branch and the vault; checkpoint: refresh the generated blocks from the repository's records and keep every word a person wrote; check: links, frontmatter, sources, STE sentence length, and no pasted Completion Goal line or issue label; open: print how to open the vault in Obsidian. One of: `init`, `checkpoint`, `check`, `open`. |
| `--workstream` | no | With init: add a workstream folder with its own index and map of content, linked up into the root vault. |
| `--dir` | no | With check: check a local folder (a worktree of the brain branch) instead of the branch's tip on the remote. |

<!-- END GENERATED: skill-usage -->

## What you get back

`init` writes the vault to the `brain` branch. `checkpoint` writes one commit with the notes it changed. `check`
writes nothing and reports each problem. `open` prints the steps to open the vault.

**You should see**, for each action:

- `init`: `brain: created (brain <sha>)`. On a second run you see `exists`. With `--workstream` you see
  `workstream added` or `workstream exists`.
- `checkpoint`: `brain checkpoint: N notes changed (brain <sha>)`, then one `needs review: <note>` line for each
  note whose facts changed after its last review.
- `check`: one `error:` or `warning:` line for each problem, then a summary such as
  `brain check brain@<sha>: N notes, 0 errors, W warnings`, and exit 0. With errors it exits 3: fix each note it
  names.
- `open`: numbered steps: fetch `brain` into `origin/brain`, then `git worktree add` it at `../<repo>-brain`
  (a new local `brain` branch, or the existing one brought up to date with `merge --ff-only`), then open that
  folder as a vault in Obsidian.

Exit 4 means that the branch is missing or that it is not an aayudha vault. The message names the next step. See
[exit codes](../troubleshooting.md) and [everything else](../troubleshooting.md#everything-else).

## Gates

<!-- BEGIN GENERATED: skill-gates -->

It checks these before it acts, and stops when one fails:

- **halt:** The repository is not halted (aayu halt).

<!-- END GENERATED: skill-gates -->

## What only a person does

<!-- BEGIN GENERATED: skill-human -->

The registry reserves no step of this verb for a person.

<!-- END GENERATED: skill-human -->

## What's next

See [every skill](README.md).
