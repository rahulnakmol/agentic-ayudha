# Technical design: Review guidance

Status: planned, not built. The tech baseline v1 (`specs/887/baselines/tech-v1.json`, hash `d4ad6ec59119`, approved
by @rahulnakmol on 2026-11-02T17:22:31Z) pins `specs/887/design.json` (hash `1de31e16…`). That file is the design
below. The ledger also holds `.draft.json` copies of each part; the approved files are the ones without `.draft`.

## Layers

The footprint is L02 and L03 (`specs/887/design.json`, `footprint` and `layers`).

| Layer | Owner seat | Paths this Feature touches |
|---|---|---|
| L02 Interaction / API boundary | Code Architect | `internal/cli/slice.go`, `internal/cli/delegate.go`, `internal/cli/inspect.go`, `internal/cli/buildreview.go`, `internal/cli/ready.go` |
| L03 Application / workflow | Engineering Manager | `internal/workitems/`, `internal/delegate/brief.go`, `internal/inspect/`, `internal/ready/` |

Sources: LAYERS.md globs; #887 Expected paths. `internal/cli/ready.go` and `internal/ready/` are inferred from #1221
(Ready checks) and #1222 Notes (`ready.ParseSnapshot`); #887 Expected paths do not list them.

## Contracts (from `specs/887/design.json`)

1. "Review guidance stays a `#### Review guidance` block under a Story or Bug's Assumptions field: `- Check:`,
   `- Ignore:` and `- Risk:` lines; the issue forms do not change (#1221)".
2. "`workitems.ParseGuidance(body)` returns the checks, ignores and risks in order; Ready and slice refuse an Ignore
   that names security, data, concurrency, a floor, or a P0 or P1 area, and a line of another shape; a missing
   block warns (#1221)".
3. "`delegate.BriefInput` gains Guidance, read from the Ready snapshot's body, never the live issue, and rendered
   inside the untrusted quote; no guidance renders nothing (#1222)".
4. "`inspect.Input` gains Guidance from the same snapshot; it is in DiffHash only when present, so the cache of
   stories with none holds; an Ignore drops only a P2 or P3 finding outside the limits, and lists each drop in
   Dropped (#1223)".

Story order: 1221, 1222, 1223 (`story_order`). Rollback: "A revert of each story's squash undoes it: no ledger record
or issue body is rewritten." (`rollback`).

## What the code on dev has now (884d81b)

| Item | Where | What it does today |
|---|---|---|
| `workitems.ParseGuidance` | none | Does not exist. `rg -i guidance internal/workitems` finds only unrelated label text (`internal/workitems/labels.go:34` and `:92`). |
| Ready snapshot reader | `internal/ready/ready.go:345` `LedgerSnapshot`, `:358` `ParseSnapshot` | Returns `BodyHash`, `CompletionGoalHash`, and `Criteria`. It reads the body at Ready (`snapshotBody`, `:353`) but does not return it. |
| Brief input | `internal/delegate/brief.go:17` `BriefInput` | Has `Body` and `Criteria`, no `Guidance`. `Body` is commented as "the story's body at Ready", but `build` passes the live `is.GetBody()` (`internal/cli/build.go:423`); the criteria come from the snapshot (`internal/cli/delegate.go:414-418`). |
| inspect input | `internal/inspect/inspect.go:161` `Input` | Has `StoryBody` and `Criteria`, no `Guidance`. `runInspect` sets `StoryBody` from the live issue (`internal/cli/inspect.go:412`) and `Criteria` from the snapshot (`:426-427`). The session review does the same (`internal/cli/buildreview.go:251`). |
| Cache key | `internal/inspect/inspect.go:193` `DiffHash` | Hashes the diff, the pack head, the Completion Goal hash, checks, criteria, the peer mode, and a parts marker. No guidance. |
| Dropped list | `internal/inspect/filter.go` drop reasons; `internal/inspect/inspect.go:229` `Dropped` | Exists, with reasons such as `nit`, `repeat`, `pre-existing`. No `story guidance: ignore` reason. |
| Protected categories | `internal/inspect/filter.go:23` `Protected` | Security, data, and concurrency are never suppressed at P0. #864 cites this rule as the model for the Ignore limits. |
| Slice no-layer rule | `internal/planning/slice.go:248` | Refuses a story whose paths fall in no layer. #1221 adds a second refusal for a limited Ignore. |
| Writer of the block | none | #1221 Notes say the block is "the `#### Review guidance` block that slice already writes". The evidence does not show a writer: no commit on `origin/dev` adds the text "Review guidance" (`git log -S 'Review guidance' origin/dev` is empty), and neither `skills/slice/` nor `evals/schemas/slice.json` names it. The block appears in issue bodies: a GitHub search for open issues with "Review guidance" in the body returns at least 100 results (the search limit). |

## Planned design per Story

- [#1221](https://github.com/rahulnakmol/agentic-ayudha/issues/1221): add `workitems.ParseGuidance` and the
  word-list limit check; call it from Ready (`aayu ready --check`) and from `slice --draft`. Proposed, from the facts
  above: also let `ready.ParseSnapshot` return the body at Ready or its guidance, so #1222 and #1223 read the frozen
  copy without each reparsing the snapshot file. This choice is not in the design file.
- [#1222](https://github.com/rahulnakmol/agentic-ayudha/issues/1222): `BriefInput.Guidance` from
  `workitems.ParseGuidance` of the snapshot's body; a `## Review guidance` section after the Completion Goal, inside
  the untrusted quote (#1222 Notes).
- [#1223](https://github.com/rahulnakmol/agentic-ayudha/issues/1223): `inspect.Input.Guidance`, set in `runInspect`
  and in the session review; a `Story guidance` block in the prompt; an Ignore match is "the finding's file or
  quoted line naming the Ignore text"; the limits of #1221 decide what is never dropped; `DiffHash` includes the
  guidance only when present (#1223 Notes).

## Data and ledger records

No new ledger record. The guidance is read from the Ready snapshots that exist today, `specs/<n>/ready.md`
(`ready.LedgerPath`, `internal/ready/ready.go:175`, read at `internal/cli/inspect.go:425`). No config key is named by any source.

## Decisions

- [#864](https://github.com/rahulnakmol/agentic-ayudha/issues/864) (accepted 2026-10-23): option 1, the field with
  hard limits on Ignore. It is marked not reversible (#864 Reversible: false).
- Depends on Feature [#877](https://github.com/rahulnakmol/agentic-ayudha/issues/877) (#887 Links). See
  [06-sequencing.md](06-sequencing.md).
