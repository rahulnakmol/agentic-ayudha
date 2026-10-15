package config

import _ "embed"

// DefaultsTOML is the embedded base configuration layer (defaults.toml). CONSTRAINTS.md is generated from it
// by internal/config/gen (WP0.5); WP1.2 layers the other configuration sources on top of it.
//
//go:embed defaults.toml
var DefaultsTOML []byte
