package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/design"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/figma"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/providers"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/verify"
)

func init() {
	for _, s := range design.Stages {
		register(s, runDesign(s))
	}
}

// DesignResult is what a design verb reports.
type DesignResult struct {
	Feature  int              `json:"feature"`
	Stage    string           `json:"stage"`
	Path     string           `json:"ledger_path,omitempty"`
	Commit   string           `json:"ledger_commit,omitempty"`
	Stores   []string         `json:"stores,omitempty"` // per file: path=store
	Manifest *design.Manifest `json:"manifest"`
	DryRun   bool             `json:"dry_run,omitempty"`
}

// figmaFrames adapts the WP6.4 Figma adapter to what the prototype stage reads.
type figmaFrames struct{ c *figma.Client }

func (f figmaFrames) Frame(ctx context.Context, file, node string) (string, []byte, error) {
	ns, err := f.c.Nodes(ctx, file, node)
	if err != nil {
		return "", nil, err
	}
	b, err := f.c.Render(ctx, file, node, 1)
	return ns[0].Name, b, err
}

func (f figmaFrames) Variables(ctx context.Context, file string) ([]design.Variable, error) {
	vs, err := f.c.Variables(ctx, file)
	out := make([]design.Variable, len(vs))
	for i, v := range vs {
		out[i] = design.Variable{Name: v.Name, Type: v.Type, Value: v.Value}
	}
	return out, err
}

// runDesign checks what the host produced for one design stage of a Feature,
// pins it to the stage before, and lands it on the ledger under
// design/<feature>/<stage>/ (WP6.5).
func runDesign(stage string) Handler {
	return func(ctx context.Context, e *Env, in Values) error {
		root := config.RepoRoot(e.Dir)
		if root == "" {
			return Errorf(ExitUsage, "%s runs in a checkout of the repository", stage)
		}
		l, err := e.Config()
		if err != nil {
			return err
		}
		repo, err := e.RepoName(ctx)
		if err != nil {
			return err
		}
		c, bot, err := e.BotGitHub(ctx, "")
		if err != nil {
			return err
		}
		feature := issueNumber(in.String("feature"))
		_, it, err := issueItem(ctx, c, repo, feature)
		if err != nil {
			return err
		}
		if it.Type != "feature" {
			return Errorf(ExitUsage, "%s: #%d is a %q; design stages belong to a Feature", stage, feature, it.Type)
		}
		ident := "bot"
		if bot.Kind == identity.KindHumanFallback {
			ident = identity.KindHumanFallback
		}
		store, err := e.proofStore(ctx, c, bot.Token, ident, bot.Tag)
		if err != nil {
			return err
		}
		inRoot := func(p string) string {
			if p == "" || filepath.IsAbs(p) {
				return p
			}
			return filepath.Join(e.Dir, p)
		}
		b := &design.Build{Repo: repo, Feature: feature, Stage: stage, From: inRoot(in.String("from")), Root: root, Now: e.Now,
			Query: in.String("query"), Platform: in.String("platform"), FigmaDiffMax: l.Config.Review.FigmaDiffMax,
			Upstream: func(ctx context.Context, p string) ([]byte, error) {
				t, err := store.Ledger.Read(ctx)
				if err != nil {
					return nil, err
				}
				defer t.Close()
				return t.File(ctx, p)
			}}
		deps := e.integrationDeps(repo)
		readRepo := func(key, p string) ([]byte, error) {
			if p == "" {
				return nil, Errorf(ExitUsage, "[design] %s is not set", key)
			}
			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
			if err != nil {
				return nil, Errorf(ExitBlocked, "[design] %s: %v", key, err)
			}
			return data, nil
		}
		switch stage {
		case design.Research:
			if b.Query != "" {
				if mob, err := providers.Mobbin(deps); err == nil {
					b.Mobbin = mob.Search
				} else if !errors.Is(err, integrations.ErrNotConfigured) {
					return err
				}
			}
		case design.Mockup:
			if b.Tokens, err = readRepo("tokens", l.Config.Design.Tokens); err != nil {
				return err
			}
		case design.Prototype:
			if b.Tokens, err = readRepo("tokens", l.Config.Design.Tokens); err != nil {
				return err
			}
			if b.Flags, err = readRepo("flags", l.Config.Design.Flags); err != nil {
				return err
			}
			fc, err := providers.Figma(deps)
			if err != nil {
				return err
			}
			b.Figma = figmaFrames{fc}
			if p := in.String("captures"); p != "" {
				if b.Captures, err = design.LoadCaptures(inRoot(p)); err != nil {
					return designErr(err)
				}
			} else {
				t, err := verify.ResolveTarget(&verify.Run{Verify: l.Config.Verify, Env: e.Getenv})
				if err != nil {
					return Wrap(ExitBlocked, err)
				}
				b.Target = t.URL
				home, err := os.MkdirTemp("", "aayudha-design-home-")
				if err != nil {
					return err
				}
				defer func() { _ = os.RemoveAll(home) }()
				work, err := os.MkdirTemp("", "aayudha-design-")
				if err != nil {
					return err
				}
				defer func() { _ = os.RemoveAll(work) }()
				b.Capture = design.BrowserCapture(&verify.Sandbox{Getenv: e.Getenv, Home: home}, root, work)
			}
		}

		res, err := b.Run(ctx)
		if err != nil && res == nil {
			return designErr(err)
		}
		out := DesignResult{Feature: feature, Stage: stage, Manifest: res.Manifest, DryRun: c.DryRun()}
		if err != nil { // the gate or the token check missed: nothing is written
			_ = e.Emit(out, func(w io.Writer) error { return renderDesign(w, out) })
			return designErr(err)
		}

		// Text lands on the ledger; media follow the proof media rules (V3):
		// the bucket, else Git LFS on a private repository, else only their
		// hashes stay in the manifest.
		dir := design.Dir(feature, stage)
		commit := ledger.Commit{Message: fmt.Sprintf("design: %s for #%d", stage, feature), Files: map[string][]byte{}}
		lfs := false
		for i := range res.Manifest.Files {
			a := &res.Manifest.Files[i]
			data := res.Files[a.Path]
			switch {
			case !proof.IsMedia(*a):
				a.Store = proof.StoreGit
				commit.Files[path.Join(dir, a.Path)] = data
			case store.Bucket != nil:
				a.Store, a.Object = proof.StoreBucket, store.Bucket.Key(repo, feature, a.SHA256)
				if !c.DryRun() {
					if err := store.Bucket.Put(ctx, a.Object, data); err != nil {
						return Wrap(ExitExternal, err)
					}
				}
			case store.Private:
				a.Store, lfs = proof.StoreLFS, true
				commit.Files[path.Join(dir, a.Path)] = data
			default:
				a.Store = proof.StoreNone
			}
			out.Stores = append(out.Stores, a.Path+"="+a.Store)
		}
		out.Path = design.ManifestPath(feature, stage)
		commit.Files[out.Path] = design.Encode(res.Manifest)
		w := *store.Ledger
		w.LFS = lfs
		if out.Commit, err = w.Write(ctx, commit); err != nil {
			return err
		}
		return e.Emit(out, func(w io.Writer) error { return renderDesign(w, out) })
	}
}

func designErr(err error) error {
	switch {
	case errors.Is(err, design.ErrGate), errors.Is(err, design.ErrTokens):
		return Wrap(ExitFloor, err)
	case errors.Is(err, design.ErrRefuse), errors.Is(err, ledger.ErrNoLedger):
		return Wrap(ExitBlocked, err)
	case errors.Is(err, design.ErrSpec):
		return Wrap(ExitUsage, err)
	}
	return classify(err)
}

func renderDesign(w io.Writer, r DesignResult) error {
	m := r.Manifest
	fmt.Fprintf(w, "%s for #%d: %d reference(s), %d state(s), %d file(s)", r.Stage, r.Feature, len(m.References), len(m.States), len(m.Files))
	for _, p := range m.Inputs {
		fmt.Fprintf(w, "; pins %s %s", p.Stage, short(p.SHA256))
	}
	fmt.Fprintln(w)
	for _, g := range m.Gate {
		fmt.Fprintf(w, "  figma-diff %s\n", g.Summary())
	}
	switch {
	case r.Commit != "":
		fmt.Fprintf(w, "  written to ledger %s (%s)\n", r.Path, short(r.Commit))
	case r.DryRun:
		fmt.Fprintf(w, "  --dry-run: %s planned, not written\n", design.ManifestPath(r.Feature, r.Stage))
	case r.Path == "":
		fmt.Fprintln(w, "  nothing written")
	}
	if media := strings.Join(r.Stores, ", "); strings.Contains(media, "="+proof.StoreNone) {
		fmt.Fprintln(w, "  media kept by hash only (public repository without [proof] media): "+media)
	}
	return nil
}
