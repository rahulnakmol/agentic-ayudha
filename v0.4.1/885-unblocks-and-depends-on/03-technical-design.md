# #885 Technical design

No tech baseline exists (`specs/885` is absent from the ledger). Everything under "Planned" is planned, not built.

## What exists on dev that this builds on

All paths are at `dev` [884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b).

| Part | Where | What it does now |
|---|---|---|
| Link kinds | `workitems.LinkKinds` (`internal/workitems/schema.go:137-140`); `workitems.ParseLinks` | `depends-on` is allowed on a feature, story, or bug: "This item cannot start until the target is merged." |
| Mission hold | `heldDeps` (`internal/cli/missionnext.go:478-514`), called from `listNext` (`:305`) | Returns the open and the cancelled depends-on targets of a Ready story. A target it cannot read counts as open. From Story #999 (PR [#1088](https://github.com/rahulnakmol/agentic-ayudha/issues/1088), [9f8e457](https://github.com/rahulnakmol/agentic-ayudha/commit/9f8e457b)) and Bug #1169 (PR [#1184](https://github.com/rahulnakmol/agentic-ayudha/issues/1184), [c2d6bd7](https://github.com/rahulnakmol/agentic-ayudha/commit/c2d6bd70)). |
| Mission split | `MissionItem.DependsOn` and `Outside` (`internal/cli/mission.go:164-165`); `splitMissions` (`:553-`); `dependsOn` (`:596`) | Puts a story whose outside depends-on have not merged into a later mission. |
| Loop pick | `Env.pickNext` (`internal/cli/buildnext.go:416-`), sort at `:573-581` | Baseline order, else priority then created time. No depends-on check. |
| PR body | `Env.storyDependencies` (`internal/cli/build.go:1068-`) | Reads each depends-on target's state and owner into the PR body. |
| ETag cache | `internal/github/transport.go:113`, `:144`, `:278` | Per-run cache: a repeated GET sends `If-None-Match`. |
| Sub-issue reads | `ready.Result.Parent` (`internal/ready/ready.go:41`); `workitems.Item.Parent` (`internal/workitems/validate.go:19`) | The parent of an item. No reader of children or dependents in one place. |

Not on `dev`: `internal/lineage`, `internal/workitems/lineage`, and any reader of dependents. `BuildState.Unblocked`
(`internal/cli/build.go:107`) records when a person made a blocked story Ready again; it is not "what this unblocks".

## Layers touched

From `LAYERS.md`: L02 (`internal/cli/buildnext.go`, `missionnext.go`), L03 (`internal/workitems/**`,
`internal/verbs/registry.go` for the new layer glob), L06 (`internal/github/**`, read only). A new `internal/lineage`
package is in no layer until `registry.go` adds it (`TestEveryInternalPackageInALayer`,
`internal/verbs/gen/main_test.go:20`).

## Planned, not built

1. **Lineage read (P1).** A package that, for one gather, returns parents, children (sub-issues), dependents (reverse
   `depends-on`), release, and who waits. It uses the GitHub client so the ETag cache applies (#885 Scope).
2. **Loop rule (P2).** `pickNext` skips a Ready story or Bug whose depends-on targets have not merged. The nearest
   design is to reuse `heldDeps` from `missionnext.go` and `depState` from `mission.go:1534` (inferred).
3. **Unblocks key (P3).** A sort key from P1's dependents count, added to `pickNext`'s order (and to the mission order,
   if a person agrees; see [02-spec.md](02-spec.md)).

## Data and ledger records

No ledger record is planned in the sources. The read is a cache per gather, not a stored record.

## Key decisions

| Decision | State | Effect |
|---|---|---|
| [#918](https://github.com/rahulnakmol/agentic-ayudha/issues/918) | accepted 2026-10-23 | The mission's pick (#889) returns only stories whose depends-on have merged. |
| [#868](https://github.com/rahulnakmol/agentic-ayudha/issues/868) | cancelled, superseded by #918 | Had put the depends-on rule in `pickNext`. |

Issue #885 depends on [#873](https://github.com/rahulnakmol/agentic-ayudha/issues/873) (One Need registry; not sliced).
The issue does not say which part needs it.
