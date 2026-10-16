// Command gen renders aayudha's own loops (loops/*.toml) for every host:
// .github/workflows/aayudha-loops.yml, loops/amp/<name>.json, and
// loops/cursor/aayudha-loops.tf (WP7.1). It runs from internal/loops through
// `go generate ./...`; CI fails on drift. The zone is [time] tz from the
// built-in defaults and .aayudha/config.toml only, so a developer's own
// config never changes the output.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/cli"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/loops"
)

const (
	repo   = "rahulnakmol/agentic-ayudha"
	branch = "dev"
)

func main() {
	if err := run(filepath.Join("..", "..")); err != nil {
		fmt.Fprintln(os.Stderr, "loops/gen:", err)
		os.Exit(1)
	}
}

func run(root string) error {
	l, err := config.Load(config.Sources{RepoDir: root})
	if err != nil {
		return err
	}
	ls, err := loops.Load(root)
	if err != nil {
		return err
	}
	files, err := loops.Files(ls, repo, branch, l.Config.Time.TZ, "dev", cli.Implemented)
	if err != nil {
		return err
	}
	// Remove host outputs of loops that no longer render.
	for _, dir := range []string{"amp", "cursor", "claude", "codex"} {
		old, _ := filepath.Glob(filepath.Join(root, loops.Dir, dir, "*"))
		for _, f := range old {
			rel := filepath.ToSlash(strings.TrimPrefix(f, root+string(filepath.Separator)))
			if _, keep := files[rel]; !keep {
				if err := os.Remove(f); err != nil {
					return err
				}
			}
		}
	}
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	for _, p := range paths {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, files[p], 0o644); err != nil {
			return err
		}
	}
	return nil
}
