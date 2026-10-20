# `audit`: hand an auditor a report they can reproduce

An auditor will not take your word that every merge was checked. `audit` reads GitHub's own event history and the
ledger, and writes a report of floors, proof coverage, waivers, decision deadlines, bypasses, and any write that
skipped the log.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | Writes a report anyone can reproduce: checks, proof, waivers, deadlines, bypasses, and any write that skipped the log. |
| Journey stage | [Explain it](../journey-9-explain.md) |
| Accountable seat | ![Vidhi](../../../design/icons/seats/auditor.svg) [Auditor](../../../seats/auditor/CHARTER.md) (Vidhi) |
| Does the work | Vidhi, the Accountable seat's twin, in the [`review`](../how-models.md) model lane, with the seat's paired human when it has one |
| Consulted | [Quality Architect](../../../seats/quality-architect/CHARTER.md) (Yama) |
| Informed | [Founder](../../../seats/founder/CHARTER.md) (Atri) |
| Layers it can touch | [every layer](../concepts-layers.md) |
| Who reasons | no model: it is deterministic |
| Writes as | the bot identity (a GitHub App); without one, your identity, tagged as human fallback |
| Where it runs | the `aayu` command, a skill in your agent, an MCP tool (only with `[mcp] expose_writes` on) |
| The skill your agent reads | [skills/audit/SKILL.md](../../../skills/audit/SKILL.md) |

<!-- END GENERATED: skill-card -->

## When to use it

- An auditor, a customer, or your own team asks for evidence that the controls held.
- You want to see every solo-mode bypass and every story merged without proof since the last report.
- You need to check an older report: `--replay` regenerates it from the snapshot it read.

Need progress, not controls? Use [`status`](status.md). Need prose for a regulator? Add a [`brief`](brief.md) with
`--register regulatory`.

## Run it

Ask your agent. The verb is deterministic: the agent runs it, then relays the summary and every row marked **no**.

```text
Use the audit skill to write an audit report for this repository, and list every row it marks no.
```

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu audit
```

| Input | Required | What it does |
|---|---|---|
| `--since` | no | Window start (YYYY-MM-DD UTC or RFC 3339); default the cursor of the newest audit report on the ledger, else 7 days back. |
| `--until` | no | Window end and GitHub event cursor (RFC 3339, exclusive); default now. With --since and --ledger it regenerates a report. |
| `--ledger` | no | Ledger commit to read; default the ledger tip. |
| `--out` | no | Also write the report to this file. |
| `--replay` | no | Regenerate a report from the GitHub inputs snapshot at this ledger path (audit/\<date>/…inputs.json); writes nothing. |
| `--mark-merged` | no | Move these stories (e.g. 38-50) that merged without proof, before or after the merge gate, to status:merged with a comment saying which; no report. |

<!-- END GENERATED: skill-usage -->

> **Needs setup:** `audit` needs a repository admin token, because only an admin can read rule suites. Without one it
> exits 4 rather than report an incomplete reconciliation. Sign in as an admin, or set `AAYU_GITHUB_TOKEN`.

## What you get back

A report on the ledger under `audit/<date>/`, beside a snapshot of what it read from GitHub. Its header records the
inputs, so `--replay <snapshot> --out <file>` regenerates the same bytes. A story merged without an authoritative
proof pack is "merged without proof", pre-gate or post-gate. A bypass recorded after the merge is "recorded late".

**You should see** the report's summary and its path under `audit/<date>/`. Every row marked **no** needs a person's
attention. Something else off? See [everything else](../troubleshooting.md#everything-else).

## Gates

<!-- BEGIN GENERATED: skill-gates -->

It checks no gate before it acts.

<!-- END GENERATED: skill-gates -->

Its window starts where the newest report on the ledger ended, or 7 days back when there is none, unless `--since`
says otherwise. GitHub's event history is the source of record, not the agents' own log.

## What only a person does

<!-- BEGIN GENERATED: skill-human -->

The registry reserves no step of this verb for a person.

<!-- END GENERATED: skill-human -->

A person decides what to do about each **no**. Only on a person's decision does the agent move stories merged without
proof to `status:merged` with `--mark-merged`. Writing a late bypass record is the founder's call.

## What's next

- Explain the work in plain words to one audience: [`brief`](brief.md).
- See status and audit on a real day: [the telling](../journey-9-explain.md).
- [Every skill](README.md).
