// Package upgrade migrates a repository's aayudha files from their recorded
// schema versions to the running binary's (WP1.9, `aayu upgrade`), always
// showing a diff first. It only edits working-tree files under .aayudha/;
// Ready snapshots and proof packs on the ledger branch are never rewritten
// (grandfathering) and stay readable by the versioned readers that own them.
package upgrade

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
)

// Kind is a versioned repository file kind.
type Kind string

// Kinds.
const (
	KindConfig Kind = "config" // .aayudha/config.toml, config.local.toml: `version = N`
	KindLock   Kind = "lock"   // .aayudha/lock.toml: `version = N` (written by install, WP3.2)
	KindState  Kind = "state"  // .aayudha/state.json: "schema_version": N (written by init, WP2.5)
)

// Versions maps each kind to a schema version.
type Versions map[Kind]int

// Current is this binary's schema versions.
var Current = Versions{KindConfig: config.Version, KindLock: 1, KindState: 1}

// Migration rewrites one file's bytes from version v to v+1. The version
// marker itself is updated by the engine.
type Migration func(old []byte) ([]byte, error)

// Migrations holds, per kind, the step from each version to the next.
type Migrations map[Kind]map[int]Migration

// Default is this binary's migrations.
var Default = Migrations{
	KindConfig: {0: configV0toV1},
}

// files lists the files upgrade manages, relative to the repo root.
var files = []struct {
	path  string
	kind  Kind
	local bool // the local config layer, which CI never reads (Bug #640)
}{
	{".aayudha/config.toml", KindConfig, false},
	{".aayudha/config.local.toml", KindConfig, true},
	{".aayudha/lock.toml", KindLock, false},
	{".aayudha/state.json", KindState, false},
}

// ErrNewer means a repository file was written by a newer aayudha.
var ErrNewer = errors.New("repository schema is newer than this aayu")

// Change is one file's migration.
type Change struct {
	Path     string `json:"path"`
	Kind     Kind   `json:"kind"`
	From     int    `json:"from"`
	To       int    `json:"to"`
	Old, New []byte `json:"-"`
}

var tomlVersion = regexp.MustCompile(`(?m)^version[ \t]*=[ \t]*([0-9]+)[ \t]*(#.*)?$`)

// Version reads the schema version recorded in a file of the given kind; a
// file without one is version 0.
func Version(kind Kind, b []byte) (int, error) {
	if kind == KindState {
		var v struct {
			Schema *int `json:"schema_version"`
		}
		if err := json.Unmarshal(b, &v); err != nil {
			return 0, err
		}
		if v.Schema == nil {
			return 0, nil
		}
		return *v.Schema, nil
	}
	m := tomlVersion.FindSubmatch(b)
	if m == nil {
		return 0, nil
	}
	return strconv.Atoi(string(m[1]))
}

func setVersion(kind Kind, b []byte, n int) ([]byte, error) {
	if kind == KindState {
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			return nil, err
		}
		m["schema_version"] = n
		out, err := json.MarshalIndent(m, "", "  ")
		return append(out, '\n'), err
	}
	line := fmt.Sprintf("version = %d", n)
	if tomlVersion.Match(b) {
		return tomlVersion.ReplaceAll(b, []byte(line+"${2}")), nil
	}
	// Insert after the leading comment block, before the first key or table.
	lines := strings.SplitAfter(string(b), "\n")
	i := 0
	for i < len(lines) && (strings.HasPrefix(strings.TrimSpace(lines[i]), "#") || strings.TrimSpace(lines[i]) == "") {
		i++
	}
	ins := line + "\n"
	if i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), "[") {
		ins += "\n"
	}
	return []byte(strings.Join(lines[:i], "") + ins + strings.Join(lines[i:], "")), nil
}

// Plan computes the changes that bring root's files to binary's versions. It
// fails with ErrNewer when any file is newer than the binary. In CI (ci set by
// --ci or GITHUB_ACTIONS=true, as for config.Load) it skips
// .aayudha/config.local.toml: CI never reads the local layer, so a committed
// one neither refuses nor is rewritten there (Bug #640).
func Plan(root string, binary Versions, m Migrations, ci bool) ([]Change, error) {
	var out []Change
	for _, f := range files {
		if ci && f.local {
			continue
		}
		old, err := os.ReadFile(filepath.Join(root, f.path))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		from, err := Version(f.kind, old)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.path, err)
		}
		to := binary[f.kind]
		if from > to {
			return nil, fmt.Errorf("%w: %s is %s schema v%d, this aayu supports up to v%d; run `aayu self-update`", ErrNewer, f.path, f.kind, from, to)
		}
		if from == to {
			continue
		}
		b := old
		for v := from; v < to; v++ {
			step, ok := m[f.kind][v]
			if !ok {
				continue // nothing but the version marker changes
			}
			if b, err = step(b); err != nil {
				return nil, fmt.Errorf("%s: migrate v%d to v%d: %w", f.path, v, v+1, err)
			}
		}
		if b, err = setVersion(f.kind, b, to); err != nil {
			return nil, fmt.Errorf("%s: %w", f.path, err)
		}
		out = append(out, Change{Path: f.path, Kind: f.kind, From: from, To: to, Old: old, New: b})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// Check returns ErrNewer when any of root's files is newer than binary. Write
// verbs call it before running (exit 2 with the upgrade hint); ci is as for Plan.
func Check(root string, binary Versions, ci bool) error {
	_, err := Plan(root, binary, nil, ci)
	return err
}

// Apply writes the changes.
func Apply(root string, changes []Change) error {
	for _, c := range changes {
		p := filepath.Join(root, c.Path)
		fi, err := os.Stat(p)
		if err != nil {
			return err
		}
		cur, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if !bytes.Equal(cur, c.Old) {
			return fmt.Errorf("%s changed since the diff was computed; run aayu upgrade again", c.Path)
		}
		if err := os.WriteFile(p, c.New, fi.Mode().Perm()); err != nil {
			return err
		}
	}
	return nil
}

// configV0toV1 drops `[lfs] server = "github"` (0.1 supports GitHub LFS only,
// V6) as text, so comments and layout survive. The in-memory loader applies
// the same rule (config.Migrate).
func configV0toV1(old []byte) ([]byte, error) {
	var out []string
	table := ""
	for _, line := range strings.SplitAfter(string(old), "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "[") {
			table = strings.Trim(strings.SplitN(t, "#", 2)[0], "[] \t")
		}
		if table == "lfs" && strings.HasPrefix(t, "server") {
			_, val, _ := strings.Cut(t, "=")
			val = strings.Trim(strings.TrimSpace(strings.SplitN(val, "#", 2)[0]), `"'`)
			if val != "" && val != "github" {
				return nil, fmt.Errorf("[lfs] server = %q: only GitHub LFS is supported in 0.1; S3/R2 arrive with `aayu lfs-agent` in 0.2 (V6)", val)
			}
			continue
		}
		out = append(out, line)
	}
	return []byte(strings.Join(out, "")), nil
}
