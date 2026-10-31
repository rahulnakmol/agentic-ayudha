# rahulnakmol/agentic-ayudha#1210: fix(slice): a directory in Expected paths covers no file under it, and internal/inspect and internal/delegate fall in no layer

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `d85f0ad04b974bb970aa0cfd7ebdf6c32030a13219206beed6b959cd33ff9dbf` |
| Completion Goal hash | `209c09a4d2bab2688d276e1013103827ce13a5a5a5a1b0e1b413ccad68ddf0ed` |
| Technical Owner | code-architect |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1210/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: a Feature's Expected paths include `internal/workitems/` and a story's paths are `internal/workitems/validate.go`; expected: slice reports no path problem for it
2. [api-trace] state: a Feature's Expected paths include `internal/workitems` with no slash; expected: the same as with the slash
3. [api-trace] state: a story's paths are `internal/workitemsx/a.go` and the Feature names `internal/workitems/`; expected: slice reports it outside the Feature's expected paths
4. [api-trace] state: a story's paths are `internal/inspect/filter.go` or `internal/delegate/brief.go`; expected: slice names the layer the path falls in, and no path problem
5. [api-trace] state: `go generate ./...` runs; expected: LAYERS.md lists the layer of `internal/inspect/**` and `internal/delegate/**`, and a test fails when a package under `internal/` falls in no layer

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

`aayu slice 887 --draft` refuses every honest slice of Feature #887, for two reasons. First, the Feature's Expected paths name directories (`internal/workitems/`, `internal/inspect/`), and a story path such as `internal/workitems/validate.go` is "outside the Feature's expected paths". The check matches the directory as a literal glob, not as everything under it (`sample` and `verify.Match`, internal/planning/slice.go:115-178). Second, a story whose paths are `internal/inspect/` or `internal/delegate/brief.go` "falls in no layer of LAYERS.md". LAYERS.md maps `internal/cli/**` (L02) and `internal/workitems/**` (L03), but has no row for `internal/inspect/**` or `internal/delegate/**` (LAYERS.md:12-13). Seen on 2026-10-31 at 20:10 UTC while the agent sliced #887 toward the next one-code morning. #887 also needs `internal/cli/delegate.go`, `internal/cli/inspect.go` and `internal/cli/buildreview.go`, which are not in its Expected paths; Story #1179 (`aayu grill` edits a filed body before Ready) is the route to add them.

### Steps to reproduce

1. Feature #887 has Expected paths `internal/workitems/`, `internal/delegate/brief.go`, `internal/inspect/`, and `internal/cli/slice.go`.
2. Run `aayu slice 887 --draft <file>` with a story whose paths are `internal/workitems/validate.go`.
3. Run it again with a story whose paths are `internal/inspect/`.

### Expected behaviour

A file under an expected directory is inside the Feature's paths, and every path under `internal/` falls in a layer.

### Actual behaviour

Slice reports "outside the Feature's expected paths" for the file, and "the paths fall in no layer of LAYERS.md" for `internal/inspect/` and `internal/delegate/brief.go`.

### Completion Goal

1. [api-trace] state: a Feature's Expected paths include `internal/workitems/` and a story's paths are `internal/workitems/validate.go`; expected: slice reports no path problem for it
2. [api-trace] state: a Feature's Expected paths include `internal/workitems` with no slash; expected: the same as with the slash
3. [api-trace] state: a story's paths are `internal/workitemsx/a.go` and the Feature names `internal/workitems/`; expected: slice reports it outside the Feature's expected paths
4. [api-trace] state: a story's paths are `internal/inspect/filter.go` or `internal/delegate/brief.go`; expected: slice names the layer the path falls in, and no path problem
5. [api-trace] state: `go generate ./...` runs; expected: LAYERS.md lists the layer of `internal/inspect/**` and `internal/delegate/**`, and a test fails when a package under `internal/` falls in no layer

### Measure

Completion Goal criteria observed

### Threshold

all 5 criteria

### Scenario

Slice stories under an expected directory, with and without its slash, under a sibling directory with a longer name, and in internal/inspect and internal/delegate; then regenerate LAYERS.md.

### Environment

go test of internal/planning and internal/verbs, and go generate; never production

### Checkpoints

1. A directory in Expected paths covers the files under it, and only those (criteria 1, 2, 3)
2. Every package under internal/ falls in a layer (criteria 4, 5)

### Technical Owner

code-architect

### Assumptions

#### Notes

The layer of `internal/inspect/**` and `internal/delegate/**` is the Code Architect's choice in `internal/verbs/registry.go`, the source of LAYERS.md; L03 (application and workflow) is the default, beside `internal/loops/**`. Adding a row changes the footprint of PRs that touch these packages, so the PR names the layer and its owner seat.

#### Review guidance

- Check: criterion 3: a sibling with a longer name is not inside the directory.
- Check: criterion 5: the test walks every package under internal/.
- Ignore: the wording of the slice report.
- Risk: a new layer row that makes an earlier merged PR's footprint differ; footprints are read at plan time, not after the merge.

#### Behaviours

1. says: A file under an expected directory is inside the Feature's paths; rule: limit; source: criteria 1, 2, 3
2. says: Every package under internal/ falls in a layer; rule: limit; source: criteria 4, 5

### Provenance

Request: "learn from what we have built so far so we can really understand the friction here and what are are causing al lot of halts and user intervention"
1. default (scope): a directory covers the files under it
2. default (scope): the slash is optional
3. default (scope): a sibling with a longer name is outside
4. default (scope): internal/inspect and internal/delegate get a layer
5. default (scope): a test keeps every package in a layer
````
