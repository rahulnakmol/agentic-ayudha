package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/upgrade"
)

func init() { register("upgrade", runUpgrade) }

func runUpgrade(_ context.Context, e *Env, in Values) error {
	root := config.RepoRoot(e.Dir)
	if root == "" {
		return Errorf(ExitUsage, "upgrade: not inside a git repository")
	}
	changes, err := upgrade.Plan(root, upgrade.Current, upgrade.Default)
	if err != nil {
		return Wrap(ExitUsage, err)
	}
	apply := in.Bool("apply") && !e.DryRun
	if apply {
		if err := e.requireHuman("upgrade --apply (it rewrites protected files)"); err != nil {
			return err
		}
	}
	if apply {
		if err := upgrade.Apply(root, changes); err != nil {
			return Wrap(ExitInternal, err)
		}
	}
	type summary struct {
		Changes []upgrade.Change `json:"changes"`
		Diff    string           `json:"diff"`
		Applied bool             `json:"applied"`
	}
	diff := upgrade.Diff(changes)
	return e.Emit(summary{changes, diff, apply && len(changes) > 0}, func(w io.Writer) error {
		if len(changes) == 0 {
			_, err := fmt.Fprintln(w, "repository schema is current; nothing to upgrade")
			return err
		}
		fmt.Fprint(w, diff)
		if apply {
			_, err := fmt.Fprintf(w, "applied %d file(s)\n", len(changes))
			return err
		}
		_, err := fmt.Fprintln(w, "run `aayu upgrade --apply` to write these changes (Ready snapshots and proof packs on ledger are never rewritten)")
		return err
	})
}
