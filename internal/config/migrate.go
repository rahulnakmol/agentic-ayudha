package config

import (
	"errors"
	"fmt"
)

// ErrNewerSchema reports a config file written by a newer aayudha.
var ErrNewerSchema = errors.New("config schema is newer than this binary")

// migrations[v] rewrites a file's map from schema v to v+1 in place.
var migrations = map[int]func(m map[string]any) error{
	0: v0to1,
}

// SchemaVersion returns the `version` of a parsed config file; a file without
// one is v0 (written before config versioning, plan v0.1).
func SchemaVersion(m map[string]any) (int, error) {
	switch v := m["version"].(type) {
	case nil:
		return 0, nil
	case int64:
		return int(v), nil
	case float64: // JSON numbers
		if v == float64(int(v)) {
			return int(v), nil
		}
	case int:
		return v, nil
	}
	return 0, fmt.Errorf("version = %v: want an integer", m["version"])
}

// migrate upgrades m to Version in place and returns the version it started at.
func migrate(m map[string]any) (int, error) {
	from, err := SchemaVersion(m)
	if err != nil {
		return 0, err
	}
	if from > Version {
		return from, fmt.Errorf("%w: file is v%d, this aayu reads up to v%d; run `aayu self-update`", ErrNewerSchema, from, Version)
	}
	for v := from; v < Version; v++ {
		if err := migrations[v](m); err != nil {
			return from, fmt.Errorf("migrate config v%d to v%d: %w", v, v+1, err)
		}
	}
	m["version"] = int64(Version)
	return from, nil
}

// Migrate upgrades a parsed config file to the current schema in place and
// returns the version it started at. `aayu upgrade` (WP1.9) uses it to rewrite files.
func Migrate(m map[string]any) (int, error) { return migrate(m) }

// v0to1: plan v0.1 let [lfs] server name an S3/R2 endpoint. 0.1 supports GitHub
// LFS only (V6), so the key is removed; a non-GitHub server cannot be migrated.
func v0to1(m map[string]any) error {
	lfs, ok := m["lfs"].(map[string]any)
	if !ok {
		return nil
	}
	if server, ok := lfs["server"]; ok {
		if s, _ := server.(string); s != "" && s != "github" {
			return fmt.Errorf("[lfs] server = %q: only GitHub LFS is supported in 0.1; S3/R2 arrive with `aayu lfs-agent` in 0.2 (V6)", s)
		}
		delete(lfs, "server")
	}
	if len(lfs) == 0 {
		delete(m, "lfs")
	}
	return nil
}
