package cli

import (
	"context"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/initrepo"
)

func init() { register("proof", runProof) }

// runProof implements only the V3 media refusal that init's public-repository
// setup depends on (WP2.5); packs, LFS, and the media store land in WP4.2.
func runProof(ctx context.Context, e *Env, in Values) error {
	if in.String("action") == "push" && in.Bool("media") {
		l, err := e.Config()
		if err != nil {
			return err
		}
		if l.Config.Proof.Media == "" {
			public := true
			if root := config.RepoRoot(e.Dir); root == "" {
			} else if st, err := initrepo.LoadState(root); err == nil && st != nil {
				public = st.Visibility != "private"
			}
			if public {
				return Errorf(ExitBlocked, "proof push --media: refused: this repository is public (or its visibility is unknown) and [proof] media names no private bucket; proof media never go to a public ledger or Git LFS (V3)")
			}
		}
	}
	return Errorf(ExitUsage, "proof %s: not implemented yet (lands in WP4.2)", in.String("action"))
}
