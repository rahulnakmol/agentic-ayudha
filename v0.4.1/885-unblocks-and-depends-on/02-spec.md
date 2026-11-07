# #885 Spec

No spec baseline exists (`specs/885` is absent from the ledger) and no Story is filed. Requirements trace to #885.
Slices are proposed, not filed (see [05-implementation-plan.md](05-implementation-plan.md)).

## Functional requirements

| # | Requirement | Source | Slice |
|---|---|---|---|
| FR1 | A lineage read returns an item's parents, children, dependents (items whose `depends-on` names it), release, and who waits on it. | [#885](https://github.com/rahulnakmol/agentic-ayudha/issues/885) Summary | P1 |
| FR2 | The read is cached per gather, with ETags. | #885 Scope | P1 |
| FR3 | `pickNext` skips a story or Bug whose depends-on target has not merged. | #885 Summary and Scope | P2 |
| FR4 | "Merged" means what the mission pick already uses: closed as completed, or open with `status:merged`; a cancelled target never merges. | `depState` (`internal/cli/mission.go:1529-1534`) and `heldDeps` (`internal/cli/missionnext.go:470-514`); Bug #1169 | P2 |
| FR5 | An unblocks sort key orders work so that an item that unblocks more goes earlier. The tie-break position is not stated. | #885 Scope ("the unblocks sort key") | P3 |

## Non-functional requirements

| Kind | Requirement | Source |
|---|---|---|
| Cost and rate limits | The lineage read reuses the per-run ETag cache, so a repeated GET sends `If-None-Match`. | #885 Scope; `internal/github/transport.go:113`, `:144`, `:278` |
| Safety | A target aayu cannot read counts as open, so a pick never starts on unknown state. | `heldDeps` comment (`internal/cli/missionnext.go:470-477`) |
| Determinism | The pick makes no model call. | `aayu build --help`: `--next` is "The build-ready loop, with no model" |
| Layering | A new `internal/lineage` package needs a layer glob in `internal/verbs/registry.go`. | `TestEveryInternalPackageInALayer` (`internal/verbs/gen/main_test.go:20`) |

## Open questions

- #885's Expected paths are `internal/lineage/**` and `internal/workitems/lineage/**`. Which one is meant? The sources
  do not say.
- Does the unblocks key apply to `mission next` too? #918 says the mission pick orders by priority, slice order, then
  Ready time, and names no unblocks key. Unverified.
