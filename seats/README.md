# seats

One `seats/<seat>/CHARTER.md` for each of the twelve seats (WP5.1). Each charter is hand-authored except the region
between the `BEGIN GENERATED: raci` and `END GENERATED: raci` markers, which `go generate ./...` writes from
`internal/verbs/registry.go` and `internal/verbs/raci.go`: the layers the seat owns and its role in every verb. The
full table is [docs/RACI.md](../docs/RACI.md). `aayu seat add|remove|list` pairs a seat with a human; its onboarding
issue carries the charter. Protected path: changes need a human CODEOWNER review. Embedded by the root `embed.go`.
