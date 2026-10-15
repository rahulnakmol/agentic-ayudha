package proof

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
)

// ErrMediaRefused: proof media never go to the ledger or Git LFS. They stay in
// the merge gate's run artifact, and only an opt-in [proof] media bucket keeps
// them longer (Appendix B S-4.2b).
var ErrMediaRefused = errors.New("refused: [proof] media names no bucket; proof media never go to the ledger or Git LFS, they stay in the merge gate's aayudha-pack run artifact (S-4.2b)")

// ErrNoPack means the ledger holds no pack for the story.
var ErrNoPack = errors.New("no proof pack on the ledger")

// Dir is a pack's ledger directory: proof/<story>/<head sha>/<advisory|authoritative>/.
func Dir(story int, sha, kind string) string {
	return path.Join("proof", fmt.Sprint(story), sha, kind)
}

// LocalDir is where `verify` leaves the latest pack for a story in a checkout:
// .aayudha/proof/<story>/ (git-ignored runtime state), holding pack.json and artifacts/.
func LocalDir(root string, story int) string {
	return filepath.Join(root, ".aayudha", "proof", fmt.Sprint(story))
}

// Store is the proof store of one repository: its ledger, and its media
// bucket when [proof] media names one (opt-in, for retention beyond the run
// artifact's).
type Store struct {
	Repo   string
	Ledger *ledger.Writer
	Bucket Media // nil without [proof] media
	// Private allows Git LFS on the ledger for design manifests and UI
	// baselines (V3); proof packs never use it (S-4.2b).
	Private       bool
	RetentionDays int
}

// LoadLocal reads a pack directory (pack.json plus artifacts/) and checks
// that every artifact's bytes match the pack.
func LoadLocal(dir string) (*Pack, error) {
	b, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		return nil, err
	}
	p, err := Decode(b)
	if err != nil {
		return nil, err
	}
	if r := VerifyLocal(dir, p, Expect{}); !r.OK {
		return nil, fmt.Errorf("local pack %s: %s", dir, strings.Join(r.Problems, "; "))
	}
	return p, nil
}

// VerifyLocal verifies p against the pack directory dir it was read from:
// every artifact must be present there, whatever store pack.json names, and
// match its hash and size.
func VerifyLocal(dir string, p *Pack, exp Expect) Report {
	local := *p
	local.Artifacts = slices.Clone(p.Artifacts)
	for i := range local.Artifacts {
		local.Artifacts[i].Store, local.Artifacts[i].Tombstone = StoreGit, nil
	}
	return Verify(&local, exp, func(a Artifact) ([]byte, error) {
		return os.ReadFile(filepath.Join(dir, "artifacts", filepath.FromSlash(a.Path)))
	})
}

// PushResult is what Push wrote.
type PushResult struct {
	Story     int      `json:"story"`
	HeadSHA   string   `json:"pr_head_sha"`
	Authority string   `json:"authority"`
	Path      string   `json:"ledger_path"`
	Commit    string   `json:"ledger_commit"`
	Stores    []string `json:"stores"` // per artifact: path=store
	Lifecycle bool     `json:"lifecycle_changed,omitempty"`
	// LifecycleNote says why the store's retention rule is not managed here
	// (ErrLifecycleUnmanaged); the rule is then set from the setup files.
	LifecycleNote string `json:"lifecycle_note,omitempty"`
}

// Push lands the pack in dir on the ledger: pack.json and its data artifacts
// (text). Media never go to the ledger: with media set they go to the
// [proof] media bucket, which must exist; otherwise only their hashes are kept
// (store "none") and the bytes stay in the merge gate's run artifact. Only
// `aayu ship` pushes an authoritative pack, after checking that the merge gate
// run made it; every other push is advisory.
func (s *Store) Push(ctx context.Context, dir string, media bool, auth Authority) (PushResult, error) {
	p, err := LoadLocal(dir)
	if err != nil {
		return PushResult{}, err
	}
	hasMedia := slices.ContainsFunc(p.Artifacts, IsMedia)
	if media && hasMedia && s.Bucket == nil {
		return PushResult{}, ErrMediaRefused
	}
	if auth.Kind == "" {
		auth.Kind = Advisory
	}
	p.Authority = auth
	base := Dir(p.Story, p.PRHeadSHA, auth.Kind)
	c := ledger.Commit{Message: fmt.Sprintf("proof: %s pack for #%d at %s", auth.Kind, p.Story, short(p.PRHeadSHA)), Files: map[string][]byte{}}
	res := PushResult{Story: p.Story, HeadSHA: p.PRHeadSHA, Authority: auth.Kind, Path: path.Join(base, FileName)}
	for i := range p.Artifacts {
		a := &p.Artifacts[i]
		data, err := os.ReadFile(filepath.Join(dir, "artifacts", filepath.FromSlash(a.Path)))
		if err != nil {
			return res, err
		}
		a.Object = ""
		switch {
		case !IsMedia(*a):
			a.Store = StoreGit
			c.Files[path.Join(base, "artifacts", a.Path)] = data
		case !media:
			a.Store = StoreNone
		default:
			a.Store, a.Object = StoreBucket, s.Bucket.Key(s.Repo, p.Story, a.SHA256)
			if err := s.Bucket.Put(ctx, a.Object, data); err != nil {
				return res, err
			}
		}
		res.Stores = append(res.Stores, a.Path+"="+a.Store)
	}
	b, err := Encode(p)
	if err != nil {
		return res, err
	}
	c.Files[res.Path] = b
	if res.Commit, err = s.Ledger.Write(ctx, c); err != nil {
		return res, err
	}
	if s.Bucket != nil && media && hasMedia {
		res.Lifecycle, err = s.Bucket.EnsureLifecycle(ctx, s.Repo, s.RetentionDays)
		switch {
		case errors.Is(err, ErrLifecycleUnmanaged):
			res.LifecycleNote = err.Error()
		case err != nil:
			return res, fmt.Errorf("pack pushed, but the bucket's retention rule could not be set: %w", err)
		}
	}
	return res, nil
}

// Entry is one pack on the ledger.
type Entry struct {
	Path string `json:"ledger_path"`
	Pack *Pack  `json:"pack"`
}

// Packs returns the story's packs on the ledger (every story's when story is
// 0), best first: authoritative before advisory, then newest first.
func (s *Store) Packs(ctx context.Context, t *ledger.Tree, story int) ([]Entry, error) {
	prefix := "proof"
	if story > 0 {
		prefix = path.Join("proof", fmt.Sprint(story))
	}
	files, err := t.List(ctx, prefix)
	if err != nil {
		return nil, err
	}
	var out []Entry
	for _, f := range files {
		if path.Base(f) != FileName || strings.Count(f, "/") != 4 {
			continue
		}
		b, err := t.File(ctx, f)
		if err != nil {
			return nil, err
		}
		p, err := Decode(b)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		out = append(out, Entry{Path: f, Pack: p})
	}
	slices.SortStableFunc(out, func(a, b Entry) int {
		if ka, kb := a.Pack.Authority.Kind == Authoritative, b.Pack.Authority.Kind == Authoritative; ka != kb {
			if ka {
				return -1
			}
			return 1
		}
		return strings.Compare(b.Pack.CreatedAt, a.Pack.CreatedAt)
	})
	return out, nil
}

// Best returns the story's best pack: the newest authoritative one, else the
// newest advisory one.
func (s *Store) Best(ctx context.Context, t *ledger.Tree, story int) (Entry, error) {
	ps, err := s.Packs(ctx, t, story)
	if err != nil {
		return Entry{}, err
	}
	if len(ps) == 0 {
		return Entry{}, fmt.Errorf("#%d: %w", story, ErrNoPack)
	}
	return ps[0], nil
}

// Opener reads a ledger pack's artifacts from wherever they are stored.
func (s *Store) Opener(ctx context.Context, t *ledger.Tree, e Entry) Opener {
	base := path.Dir(e.Path)
	return func(a Artifact) ([]byte, error) {
		switch a.Store {
		case StoreGit, StoreLFS: // LFS: media of packs pushed before 0.2
			return t.Content(ctx, path.Join(base, "artifacts", a.Path))
		case StoreBucket:
			if s.Bucket == nil {
				return nil, errors.New("stored in a media bucket, but [proof] media is not set here")
			}
			return s.Bucket.Get(ctx, a.Object)
		}
		return nil, fmt.Errorf("unknown store %q", a.Store)
	}
}

// NewArtifact describes bytes as an artifact of a pack.
func NewArtifact(rel, kind string, data []byte) Artifact {
	sum := sha256.Sum256(data)
	return Artifact{Path: rel, Kind: kind, SHA256: hex.EncodeToString(sum[:]), Size: int64(len(data)), Store: StoreNone}
}

// WriteLocal writes a pack and its artifacts to dir, replacing what was there.
func WriteLocal(dir string, p *Pack, files map[string][]byte) error {
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	for rel, b := range files {
		if err := CleanPath(rel); err != nil {
			return err
		}
		f := filepath.Join(dir, "artifacts", filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(f, b, 0o644); err != nil {
			return err
		}
	}
	b, err := Encode(p)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, FileName), b, 0o644)
}
