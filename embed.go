// Package aayudha embeds the source asset trees that ship inside the aayudha binary
// (docs/BUILD-PLAN.md ground rule 3). Owned by WP0.2.
//
// Running `go generate ./...` from the repository root regenerates every artifact
// derived from the verb registry in internal/verbs (ground rule 4) and CONSTRAINTS.md
// from the embedded default configuration in internal/config (WP0.5).
package aayudha

//go:generate go run ./internal/verbs/gen
//go:generate go run ./internal/config/gen

import "embed"

// Assets holds skills/, work-items/, hosts/, and seats/ exactly as committed.
//
//go:embed skills work-items hosts seats
var Assets embed.FS
