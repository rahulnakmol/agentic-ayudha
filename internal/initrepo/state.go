// Package initrepo sets up a product repository (WP2.5 `aayu init`, WP2.6
// extras) and takes it apart again (`aayu uninit`). Every object init creates
// or adopts is recorded by its GitHub ID in .aayudha/state.json, so a second
// run changes nothing, a partial run resumes where it stopped, and uninit
// removes exactly what init created; names and markers are only hints.
package initrepo

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

// StateVersion is the schema version of state.json (`schema_version`, WP1.9).
const StateVersion = 1

// StatePath is state.json relative to the repository root.
const StatePath = ".aayudha/state.json"

// Object is a GitHub object init created or adopted.
type Object struct {
	Kind    string `json:"kind"` // label, issue-type, ruleset, ledger, workflow, project, app, integration, secret
	Name    string `json:"name"`
	ID      string `json:"id"`                // GitHub ID (labels, types, rulesets, projects, apps) or commit SHA (ledger)
	Adopted bool   `json:"adopted,omitempty"` // existed before init: uninit leaves it alone
	// External objects live outside the repository's own settings (an App, a
	// project, a bucket); uninit --external lists them for manual removal.
	External bool `json:"external,omitempty"`
}

// File is a repository file init wrote.
type File struct {
	Path    string `json:"path"`
	SHA256  string `json:"sha256"`  // content after init wrote it
	Created bool   `json:"created"` // the file did not exist before init
	Section string `json:"section,omitempty"`
}

// State is .aayudha/state.json.
type State struct {
	SchemaVersion int      `json:"schema_version"`
	Repo          string   `json:"repo"`
	Run           string   `json:"run"` // the init run; idempotency keys are scoped to it
	Complete      bool     `json:"complete"`
	Done          []string `json:"done"` // completed steps, in order
	Login         string   `json:"login,omitempty"`
	Identity      string   `json:"identity,omitempty"` // bot kind, or human-fallback
	Visibility    string   `json:"visibility,omitempty"`
	Objects       []Object `json:"objects"`
	Files         []File   `json:"files"`
}

// ErrIncomplete means an earlier init stopped part-way; rerun with --resume.
var ErrIncomplete = errors.New("a previous init stopped part-way")

// LoadState reads state.json, or returns nil when there is none.
func LoadState(root string) (*State, error) {
	b, err := os.ReadFile(filepath.Join(root, StatePath))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var s State
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("%s: %w", StatePath, err)
	}
	if s.SchemaVersion > StateVersion {
		return nil, fmt.Errorf("%s: schema_version %d is newer than this binary's %d; run aayu self-update", StatePath, s.SchemaVersion, StateVersion)
	}
	return &s, nil
}

// Bytes renders state.json deterministically.
func (s *State) Bytes() []byte {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return b.Bytes()
}

// Save writes state.json.
func (s *State) Save(root string) error {
	p := filepath.Join(root, StatePath)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, s.Bytes(), 0o644)
}

// Object returns the recorded object of kind and name.
func (s *State) Object(kind, name string) (Object, bool) {
	for _, o := range s.Objects {
		if o.Kind == kind && o.Name == name {
			return o, true
		}
	}
	return Object{}, false
}

// Put records o, replacing an earlier record of the same kind and name.
func (s *State) Put(o Object) {
	for i, x := range s.Objects {
		if x.Kind == o.Kind && x.Name == o.Name {
			s.Objects[i] = o
			return
		}
	}
	s.Objects = append(s.Objects, o)
}

// Remove forgets the object of kind and name.
func (s *State) Remove(kind, name string) {
	s.Objects = slices.DeleteFunc(s.Objects, func(o Object) bool { return o.Kind == kind && o.Name == name })
}

// PutFile records f, keeping the first Created value for a path.
func (s *State) PutFile(f File) {
	for i, x := range s.Files {
		if x.Path == f.Path {
			f.Created = x.Created
			s.Files[i] = f
			return
		}
	}
	s.Files = append(s.Files, f)
}

// MarkDone records a completed step.
func (s *State) MarkDone(step string) {
	if !slices.Contains(s.Done, step) {
		s.Done = append(s.Done, step)
	}
}

// IsDone reports whether a step completed.
func (s *State) IsDone(step string) bool { return slices.Contains(s.Done, step) }

func hash(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
