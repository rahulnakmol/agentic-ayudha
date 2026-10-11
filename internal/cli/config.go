package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
)

func init() { register("config", runConfig) }

// Config loads the layered configuration for this invocation. Errors are
// configuration errors (exit 2).
func (e *Env) Config() (*config.Loaded, error) {
	if e.config != nil {
		return e.config, nil
	}
	l, err := config.Load(config.DefaultSources(e.Getenv, e.Dir, e.Environ, e.Set))
	if err != nil {
		return nil, Wrap(ExitUsage, fmt.Errorf("config: %w", err))
	}
	for _, n := range l.Notices {
		fmt.Fprintf(e.Err, "aayu: %s\n", n)
	}
	e.config = l
	return l, nil
}

func runConfig(_ context.Context, e *Env, in Values) error {
	switch in.String("action") {
	case "validate":
		if _, err := e.Config(); err != nil {
			return err
		}
		return e.Emit(map[string]any{"valid": true}, func(w io.Writer) error {
			_, err := fmt.Fprintln(w, "config is valid")
			return err
		})
	case "init":
		return configInit(e)
	default:
		return configShow(e, in.Bool("origin"))
	}
}

func configShow(e *Env, origin bool) error {
	l, err := e.Config()
	if err != nil {
		return err
	}
	if !origin {
		return e.Emit(l.Raw(), func(w io.Writer) error {
			b, err := toml.Marshal(l.Raw())
			if err != nil {
				return err
			}
			_, err = w.Write(b)
			return err
		})
	}
	type entry struct {
		Key    string `json:"key"`
		Value  any    `json:"value"`
		Layer  string `json:"layer"`
		Source string `json:"source"`
	}
	var entries []entry
	for _, k := range l.Keys() {
		layer, source := l.Origin(k)
		entries = append(entries, entry{k, l.Get(k), layer, source})
	}
	return e.Emit(entries, func(w io.Writer) error {
		for _, en := range entries {
			v, err := toml.Marshal(map[string]any{"v": en.Value})
			if err != nil {
				return err
			}
			fmt.Fprintf(w, "%s = %s  # %s: %s\n", en.Key, strings.TrimSpace(strings.TrimPrefix(string(v), "v = ")), en.Layer, en.Source)
		}
		return nil
	})
}

const starterConfig = `#:schema https://raw.githubusercontent.com/rahulnakmol/agentic-ayudha/dev/internal/config/schema.json
# aayudha repo configuration. Layers, lowest precedence first: built-in defaults,
# ~/.config/aayudha/config.toml, this file, .aayudha/config.local.toml (not committed),
# AAYU_<SECTION>__<KEY> environment variables, then --set key=value.
# This file is a protected path: changes need a human CODEOWNER review.
# Show the merged result and where each value came from: aayu config show --origin
version = 1
`

func configInit(e *Env) error {
	root := config.RepoRoot(e.Dir)
	if root == "" {
		return Errorf(ExitUsage, "config init: not inside a git repository")
	}
	path := filepath.Join(root, ".aayudha", "config.toml")
	if _, err := os.Stat(path); err == nil {
		return Errorf(ExitBlocked, "config init: %s already exists; edit it, or run `aayu upgrade` to migrate it", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	result := map[string]any{"path": path, "written": !e.DryRun}
	if !e.DryRun {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(starterConfig), 0o644); err != nil {
			return err
		}
	}
	return e.Emit(result, func(w io.Writer) error {
		verb := "wrote"
		if e.DryRun {
			verb = "--dry-run: would write"
		}
		_, err := fmt.Fprintf(w, "%s %s\n", verb, path)
		return err
	})
}
