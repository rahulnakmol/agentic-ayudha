# skills

One directory per verb: `skills/<verb>/SKILL.md`, plus optional `scripts/` (which call only `aayu <verb>`) and `references/`.
The YAML frontmatter of every SKILL.md is generated from `internal/verbs/registry.go` by `go generate ./...` (WP0.3);
the body below the frontmatter is hand-authored. Skill format and lint land in WP3.1; bodies land with each verb's WP.
Embedded into the binary by the root `embed.go` (ground rule 3).
