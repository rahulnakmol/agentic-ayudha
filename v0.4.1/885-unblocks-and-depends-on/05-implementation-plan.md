# #885 Implementation plan

## Stories and Bugs

No Story or Bug is filed under [#885](https://github.com/rahulnakmol/agentic-ayudha/issues/885). The P rows are
proposed, not filed; effort and risk are blank because no source estimates them. #999 and #1169 are filed and merged
under Feature #889; they are listed because #885 builds on them.

| # | Title | Type | Status | Effort | Risk | Depends on | Layers or paths | Evidence |
|---|---|---|---|---|---|---|---|---|
| P1 | A lineage read gives parents, children, dependents, release, and who waits, cached with ETags (proposed, not filed) | Story | not filed | — | — | — | L03 new `internal/lineage/**` (or `internal/workitems/lineage/**`), `internal/verbs/registry.go` (layer glob) and generated `LAYERS.md` | not started |
| P2 | `aayu build --next pick` skips a story whose depends-on target has not merged (proposed, not filed) | Story | not filed | — | — | — | L02 `internal/cli/buildnext.go`, `internal/cli/buildnext_test.go`; reads `missionnext.go` `heldDeps` | not started |
| P3 | Picks order by what an item unblocks (proposed, not filed) | Story | not filed | — | — | P1, P2 | L02 `internal/cli/buildnext.go`; maybe `internal/cli/missionnext.go` | not started |
| [#999](https://github.com/rahulnakmol/agentic-ayudha/issues/999) | The runnable list holds back a story whose depends-on target is still open (under #889) | Story | merged | S | medium | — | `internal/cli/missionnext.go` | PR #1088, [9f8e457](https://github.com/rahulnakmol/agentic-ayudha/commit/9f8e457b) |
| [#1169](https://github.com/rahulnakmol/agentic-ayudha/issues/1169) | aayu mission next holds a story whose depends-on target merged but is still open (under #889) | Bug | merged | S | medium | — | `internal/cli/missionnext.go`, `mission.go` | PR #1184, [c2d6bd7](https://github.com/rahulnakmol/agentic-ayudha/commit/c2d6bd70) |

## Delivered

Nothing filed under #885 is delivered. What works on `dev` now, from #889's work: `aayu mission next` holds a
story while a depends-on target is open or cancelled (#1088, #1184).

## In review

None. No open PR refers to #885.

## Pending

- P1 to P3, after the baselines and Approve to build.
- A decision for a person: is `build --next` still a path that needs the rule, now that missions pick under #889
  and #918? The loop definition `loops/build-ready.toml` still exists on `dev`.
- No source gives an estimate.
