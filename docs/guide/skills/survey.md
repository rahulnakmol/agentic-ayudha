# `survey`: understand code nobody documented

An inherited codebase with no docs is risky to change and riskier to rewrite, because nobody can say what it
promises. `survey` indexes it, writes what it does with a citation for every claim, asks you only what the code
cannot answer, and drafts a backlog to modernise it.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | Reads a codebase nobody documented, says what it does with a citation for every claim, and drafts the backlog to modernise it. |
| Journey stage | [Shape the goal](../journey-1-shape.md) |
| Accountable seat | [Code Architect](../../../seats/code-architect/CHARTER.md) (Daedalus) |
| Does the work | Daedalus, the Accountable seat's twin, in the [`plan`](../how-models.md) model lane, with the seat's paired human when it has one |
| Consulted | [Product Strategist](../../../seats/product-strategist/CHARTER.md) (Metis), [Quality Architect](../../../seats/quality-architect/CHARTER.md) (Themis) |
| Informed | [Engineering Manager](../../../seats/engineering-manager/CHARTER.md) (Nestor) |
| Layers it can touch | [every layer](../concepts-layers.md) |
| Who reasons | your agent's own model, inside the skill |
| Writes as | the bot identity (a GitHub App); without one, your identity, tagged as human fallback |
| Where it runs | the `aayu` command, a skill in your agent, an MCP tool (read-only unless `[mcp] expose_writes` is on) |
| The skill your agent reads | [skills/survey/SKILL.md](../../../skills/survey/SKILL.md) |

<!-- END GENERATED: skill-card -->

## When to use it

- You inherited a legacy system, a library, or a prototype that became a product, and nobody can say what it does.
- You plan to rebuild it on a modern stack and need to know which interfaces others depend on.
- Before a rewrite, you want the old program's outputs captured so the new one can be proven to match.

Starting from an idea, not from code? Use [`grill`](grill.md). Planning one story in code you already know is
[`plan`](plan.md).

## Run it

Ask your agent in plain words, naming the target if you want one. It indexes the code, reads it module by module,
asks you a few questions a round, riskiest first, and shows you the backlog before anything is filed.

```text
Use the survey skill on this repository: tell me what it does and who it serves, and draft a backlog to rebuild it
in Go.
```

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu survey [<path>]
```

| Input | Required | What it does |
|---|---|---|
| `<path>` | no | Checkout to survey; default the repository root. |
| `--out` | no | Directory for the survey's files (index.json, SURVEY.md, backlog.md, golden/); default .aayudha/survey under the surveyed checkout. |
| `--budget` | no | Module size in tokens (about four bytes each) when grouping files; default 20000. |
| `--module` | no | Print one module's fact pack (files, targets, entry points, surfaces, markers, symbols, calls in and out) from the index; read-only. |
| `--draft` | no | Survey draft to check (JSON: claims at the engineering, capability, and product levels, trap findings, the do-not-carry list, the compatibility contract, unknowns, answers, modernisation path): every claim cites the index or a probe, or is marked unknown; renders SURVEY.md. |
| `--backlog` | no | Backlog draft to check (JSON: Goal, Initiatives, Features, Stories, Bugs, and do-not-carry Decisions) against the work-item schema, the survey draft, and its compatibility contract; renders backlog.md and its review digest. |
| `--create` | no | With --backlog: file the reviewed backlog as issues through the actions log, skipping items already filed; needs --reviewed. |
| `--reviewed` | no | With --create: the review digest of the exact backlog the person reviewed (printed by --backlog). |
| `--limit` | no | With --create: the most issues one run files; default 25. |
| `--golden` | no | Characterisation cases (JSON) to run against the legacy build (--bin), writing each case's stdout, stderr, and exit code with SHA-256 under golden/. |
| `--parity` | no | Characterisation cases (JSON) to replay against the new build (--bin) and compare with the golden outputs; exit 3 on any difference. |
| `--compat` | no | A legacy test suite as cases (JSON, each test with its suite and area) to run against the new build (--bin) after --golden ran it on the legacy build: scores passes per area, appends to compat-history.jsonl, exit 3 when a test the legacy build passes fails. |
| `--bin` | no | With --golden, --parity, or --compat: the program the cases run as {bin}. |
| `--run` | no | With --golden, --parity, or --compat: run the cases' program; without it the cases file is only checked. |
| `--shadow` | no | With --golden --bin \<legacy> --run: the new program, run beside the legacy one on the same inputs; reports every difference without golden files, exit 3 on any. |
| `--probe` | no | With --golden --bin --run: also run each case twice and under a changed time zone, locale, and HOME (and under strace when installed), writing non-determinism, environment dependencies, and files read and written to probes.json. |
| `--check-bin` | no | With --probe: a sanitiser, checker, or differently optimised build of the legacy program; cases whose stderr carries a checker report, or whose stdout or exit code differs from the capture, are recorded. |
| `--coverage` | no | With --probe: an lcov tracefile from running the cases under coverage; records the index's functions the cases never reach. |

<!-- END GENERATED: skill-usage -->

Proving a rewrite matches is `aayu survey --parity cases.json --bin <new program> --run`.

## What you get back

Files under `.aayudha/survey/`: `index.json`, then `SURVEY.md` with every claim, its basis, and its citation, the
traps a port must check, what not to carry over, and the compatibility contract. `backlog.md` holds the draft Goal,
Initiatives, Features, Stories, and Bugs, with a review digest. Once you confirm it, the issues are filed with the
`survey:<product>` label, at most 25 a run by default. `golden/` holds the legacy program's captured outputs.

**You should see** `aayu survey --draft survey.json` exit 0 once every citation resolves; it exits 4 while a problem
remains. `--parity` exits 3 and names the first differing line when the new build does not match.

## Gates

<!-- BEGIN GENERATED: skill-gates -->

It checks these before it acts, and stops when one fails:

- **validator:** The work item passes stage validation for its type.
- **halt:** The repository is not halted (aayu halt).

<!-- END GENERATED: skill-gates -->

A backlog item never rests on a guess: an unknown becomes a question for you, and the item it would unlock waits.
In the merge gate, the `parity` floor replays the captured outputs, so a rewrite cannot merge while its contract
fails.

## What only a person does

<!-- BEGIN GENERATED: skill-human -->

The registry reserves no step of this verb for a person.

<!-- END GENERATED: skill-human -->

You answer what only you know (who runs it, which interfaces must stay exact) and review `backlog.md`: filing needs
the digest of the exact backlog you reviewed. You accept each do-not-carry Decision with [`decide`](decide.md).

## What's next

- Make the first filed story Ready: [`ready`](ready.md).
- Turn a goal into work items from scratch instead: [the goal](../journey-1-shape.md).
- [Every skill](README.md).
