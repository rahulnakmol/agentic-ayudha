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
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
)

// ErrMediaRefused is the V3 refusal: proof media never go to Git LFS or the
// ledger of a public repository; they need a private bucket.
var ErrMediaRefused = errors.New("refused: this repository is public (or its visibility is unknown) and [proof] media names no private bucket; proof media never go to a public ledger or Git LFS (V3)")

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
// bucket when [proof] media names one.
type Store struct {
	Repo          string
	Ledger        *ledger.Writer
	Bucket        Media // nil without [proof] media
	Private       bool  // only a private repository may keep media in Git LFS
	RetentionDays int
	Now           func() time.Time
}

func (s *Store) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
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
	local := *p
	local.Artifacts = slices.Clone(p.Artifacts)
	for i := range local.Artifacts {
		local.Artifacts[i].Store, local.Artifacts[i].Tombstone = StoreGit, nil // every local artifact must be present
	}
	r := Verify(&local, Expect{}, func(a Artifact) ([]byte, error) {
		return os.ReadFile(filepath.Join(dir, "artifacts", filepath.FromSlash(a.Path)))
	})
	if !r.OK {
		return nil, fmt.Errorf("local pack %s: %s", dir, strings.Join(r.Problems, "; "))
	}
	return p, nil
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
	// (ErrLifecycleUnmanaged); `proof purge` still enforces retention.
	LifecycleNote string `json:"lifecycle_note,omitempty"`
}

// Push lands the pack in dir on the ledger. Data artifacts are committed to
// the ledger; media go to the bucket, or to Git LFS on a private repository
// without a bucket, and only with media set: otherwise only their hashes are
// kept (store "none"). A pack is authoritative only when the caller is the
// merge gate in CI; every other push is advisory.
func (s *Store) Push(ctx context.Context, dir string, media bool, auth Authority) (PushResult, error) {
	p, err := LoadLocal(dir)
	if err != nil {
		return PushResult{}, err
	}
	hasMedia := slices.ContainsFunc(p.Artifacts, IsMedia)
	if media && hasMedia && s.Bucket == nil && !s.Private {
		return PushResult{}, ErrMediaRefused
	}
	if auth.Kind == "" {
		auth.Kind = Advisory
	}
	p.Authority = auth
	base := Dir(p.Story, p.PRHeadSHA, auth.Kind)
	c := ledger.Commit{Message: fmt.Sprintf("proof: %s pack for #%d at %s", auth.Kind, p.Story, short(p.PRHeadSHA)), Files: map[string][]byte{}}
	res := PushResult{Story: p.Story, HeadSHA: p.PRHeadSHA, Authority: auth.Kind, Path: path.Join(base, FileName)}
	lfs := false
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
		case s.Bucket != nil:
			a.Store, a.Object = StoreBucket, s.Bucket.Key(s.Repo, p.Story, a.SHA256)
			if err := s.Bucket.Put(ctx, a.Object, data); err != nil {
				return res, err
			}
		default: // private repository, no bucket: Git LFS on the ledger
			a.Store, lfs = StoreLFS, true
			c.Files[path.Join(base, "artifacts", a.Path)] = data
		}
		res.Stores = append(res.Stores, a.Path+"="+a.Store)
	}
	b, err := Encode(p)
	if err != nil {
		return res, err
	}
	c.Files[res.Path] = b
	w := *s.Ledger
	w.LFS = lfs
	if res.Commit, err = w.Write(ctx, c); err != nil {
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
		case StoreGit, StoreLFS:
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

// PurgeResult is what Purge removed.
type PurgeResult struct {
	Packs   []string `json:"packs"`   // pack.json paths rewritten with tombstones
	Objects []string `json:"objects"` // bucket objects deleted
	Files   []string `json:"files"`   // LFS files removed from the ledger tip
	Commit  string   `json:"ledger_commit,omitempty"`
}

// Purge removes proof media: every media artifact of the story's packs when
// story > 0, else media of packs older than the retention TTL. Bucket objects
// are deleted; LFS files are removed from the ledger tip (GitHub keeps LFS
// objects until the repository is deleted, which is why public repositories
// may not use LFS, V3). Each pack.json is rewritten with tombstones, keeping
// every hash.
func (s *Store) Purge(ctx context.Context, t *ledger.Tree, story int, reason string) (PurgeResult, error) {
	var res PurgeResult
	entries, err := s.Packs(ctx, t, story)
	if err != nil {
		return res, err
	}
	now := s.now().UTC()
	c := ledger.Commit{Message: "proof: purge media", Files: map[string][]byte{}}
	if story > 0 {
		c.Message = fmt.Sprintf("proof: purge media of #%d", story)
	}
	for _, e := range entries {
		if story == 0 {
			created, err := time.Parse(time.RFC3339, e.Pack.CreatedAt)
			if s.RetentionDays <= 0 || err != nil || now.Sub(created) < time.Duration(s.RetentionDays)*24*time.Hour {
				continue
			}
		}
		changed := false
		base := path.Dir(e.Path)
		for i := range e.Pack.Artifacts {
			a := &e.Pack.Artifacts[i]
			if !IsMedia(*a) || a.Tombstone != nil || a.Store == StoreNone {
				continue
			}
			switch a.Store {
			case StoreBucket:
				if s.Bucket == nil {
					return res, fmt.Errorf("%s: media in a bucket, but [proof] media is not set here", e.Path)
				}
				if err := s.Bucket.Delete(ctx, a.Object); err != nil {
					return res, err
				}
				res.Objects = append(res.Objects, a.Object)
			case StoreLFS:
				f := path.Join(base, "artifacts", a.Path)
				c.Delete = append(c.Delete, f)
				res.Files = append(res.Files, f)
			default:
				continue
			}
			a.Tombstone = &Tombstone{PurgedAt: now.Format(time.RFC3339), Reason: reason}
			changed = true
		}
		if !changed {
			continue
		}
		b, err := Encode(e.Pack)
		if err != nil {
			return res, err
		}
		c.Files[e.Path] = b
		res.Packs = append(res.Packs, e.Path)
	}
	if len(c.Files) == 0 {
		return res, nil
	}
	res.Commit, err = s.Ledger.Write(ctx, c)
	return res, err
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
