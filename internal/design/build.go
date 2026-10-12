package design

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/integrations"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/skills"
)

// SchemaVersion is the manifest format version.
const SchemaVersion = 1

// Pin is an upstream stage's manifest, pinned by the SHA-256 of its bytes.
type Pin struct {
	Stage  string `json:"stage"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// FigmaPin is a Figma frame a prototype state was diffed against, pinned by
// the SHA-256 of the PNG Figma rendered.
type FigmaPin struct {
	File   string `json:"file"`
	Node   string `json:"node"`
	Name   string `json:"name,omitempty"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	SHA256 string `json:"render_sha256"`
}

// Manifest is a stage on the ledger: design/<feature>/<stage>/manifest.json.
type Manifest struct {
	SchemaVersion int                      `json:"schema_version"`
	Repo          string                   `json:"repo"`
	Feature       int                      `json:"feature"`
	Stage         string                   `json:"stage"`
	CreatedAt     string                   `json:"created_at"`
	Inputs        []Pin                    `json:"inputs"`
	Query         string                   `json:"query,omitempty"` // research: the Mobbin search
	References    []integrations.Reference `json:"references"`
	States        []State                  `json:"states"`
	Files         []proof.Artifact         `json:"files"`
	Figma         []FigmaPin               `json:"figma,omitempty"`
	Flag          string                   `json:"flag,omitempty"`
	CodeConnect   map[string][]string      `json:"code_connect,omitempty"`
	Captures      string                   `json:"captures,omitempty"` // prototype: "browser" or "supplied"
	Gate          []GateResult             `json:"figma_diff,omitempty"`
}

// Figma is what the prototype stage reads from Figma (REST).
type Figma interface {
	// Frame returns a node's name and its PNG render at scale 1.
	Frame(ctx context.Context, file, node string) (name string, png []byte, err error)
	Variables(ctx context.Context, file string) ([]Variable, error)
}

// Capture renders prototype states (width and height from their Figma
// frames) and returns each state's screenshot and computed styles.
type Capture func(ctx context.Context, states []CaptureState) ([]Captured, error)

// CaptureState is one state to render.
type CaptureState struct {
	ID     string `json:"id"`
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// Captured is one rendered state. PNG is the screenshot (in a captures file,
// the PNG's path relative to that file).
type Captured struct {
	ID     string  `json:"id"`
	URL    string  `json:"url,omitempty"`
	PNG    string  `json:"png"`
	Styles []Style `json:"styles"`
	Error  string  `json:"error,omitempty"`
	data   []byte
}

// LoadCaptures reads captures supplied with --captures: <dir>/captures.json
// listing each state's PNG (relative to dir) and computed styles.
func LoadCaptures(dir string) ([]Captured, error) {
	b, err := os.ReadFile(filepath.Join(dir, "captures.json"))
	if err != nil {
		return nil, specErr("captures: %v", err)
	}
	var cs []Captured
	if err := json.Unmarshal(b, &cs); err != nil {
		return nil, specErr("captures.json: %v", err)
	}
	for i := range cs {
		if err := proof.CleanPath(cs[i].PNG); err != nil {
			return nil, specErr("captures.json: %v", err)
		}
		if cs[i].data, err = os.ReadFile(filepath.Join(dir, filepath.FromSlash(cs[i].PNG))); err != nil {
			return nil, specErr("captures: %v", err)
		}
	}
	return cs, nil
}

// Build is one stage run.
type Build struct {
	Repo    string
	Feature int
	Stage   string
	From    string // the host's directory for this stage ("" only for research)
	Root    string // repository root: tokens, flags, and Code Connect sources are read from here
	Now     func() time.Time

	// Upstream reads the previous stage's manifest bytes from the ledger;
	// it returns fs.ErrNotExist when there is none.
	Upstream func(ctx context.Context, path string) ([]byte, error)

	// research
	Query    string
	Platform string
	Mobbin   func(ctx context.Context, kind, query, platform string, limit int) (integrations.ReferenceSet, error) // nil: not configured

	// mockup, prototype
	Tokens []byte
	// prototype
	Flags        []byte
	FigmaDiffMax float64
	Figma        Figma
	Target       string // base URL of the [verify.environments] target
	Capture      Capture
	Captures     []Captured // supplied with --captures; Capture is not called
}

// Result is a built stage: the manifest and the files to land beside it
// (paths relative to the stage directory).
type Result struct {
	Manifest *Manifest
	Files    map[string][]byte
}

// Run checks the host's work for the stage, pins its inputs, and returns the
// manifest and files. A prototype that misses the gate returns its result
// with an error wrapping ErrGate.
func (b *Build) Run(ctx context.Context) (*Result, error) {
	spec, err := LoadSpec(b.From)
	if err != nil {
		return nil, err
	}
	m := &Manifest{SchemaVersion: SchemaVersion, Repo: b.Repo, Feature: b.Feature, Stage: b.Stage, CreatedAt: b.Now().UTC().Format(time.RFC3339),
		Inputs: []Pin{}, References: []integrations.Reference{}, States: []State{}}
	res := &Result{Manifest: m, Files: map[string][]byte{}}

	if b.From != "" {
		if fs := skills.MobbinImages(os.DirFS(b.From), "."); len(fs) > 0 {
			return nil, fmt.Errorf("%w: Mobbin-origin image in %s: %s (Mobbin gives URLs and IDs, never pixels)", ErrRefuse, b.From, fs[0])
		}
	}

	var up *Manifest
	if prev := Previous(b.Stage); prev != "" {
		p := ManifestPath(b.Feature, prev)
		raw, err := b.Upstream(ctx, p)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil, fmt.Errorf("%w: %s needs the %s stage of #%d on the ledger (%s); run `aayu %s %d` first", ErrRefuse, b.Stage, prev, b.Feature, p, prev, b.Feature)
			}
			return nil, err
		}
		up = &Manifest{}
		if err := json.Unmarshal(raw, up); err != nil || up.Stage != prev || up.Feature != b.Feature {
			return nil, fmt.Errorf("%w: %s is not a %s manifest of #%d", ErrRefuse, p, prev, b.Feature)
		}
		sum := sha256.Sum256(raw)
		m.Inputs = append(m.Inputs, Pin{Stage: prev, Path: p, SHA256: hex.EncodeToString(sum[:])})
		m.References = up.References
	}

	switch b.Stage {
	case Research:
		err = b.research(ctx, spec, res)
	case Wireframe, Mockup:
		err = b.artifacts(spec, up, res)
	case Prototype:
		err = b.prototype(ctx, spec, up, res)
	default:
		err = specErr("unknown stage %q", b.Stage)
	}
	if err != nil && !errors.Is(err, ErrGate) && !errors.Is(err, ErrTokens) {
		return nil, err
	}
	sort.Slice(m.Files, func(i, j int) bool { return m.Files[i].Path < m.Files[j].Path })
	return res, err
}

func (r *Result) add(rel, kind string, data []byte) {
	r.Files[rel] = data
	r.Manifest.Files = append(r.Manifest.Files, proof.NewArtifact(rel, kind, data))
}

func (b *Build) research(ctx context.Context, spec Spec, res *Result) error {
	m := res.Manifest
	seen := map[string]bool{}
	addRef := func(r integrations.Reference) error {
		if err := CheckReference(r); err != nil {
			return err
		}
		if !seen[r.URL] {
			seen[r.URL] = true
			m.References = append(m.References, r)
		}
		return nil
	}
	for _, r := range spec.References {
		if err := addRef(r); err != nil {
			return err
		}
	}
	if b.Query != "" {
		if b.Mobbin == nil {
			return fmt.Errorf("%w: [integrations.mobbin] is not set, so --query cannot search Mobbin; set it, or pass the references you gathered in %s", integrations.ErrNotConfigured, SpecFile)
		}
		m.Query = b.Query
		set, err := b.Mobbin(ctx, "screen", b.Query, b.Platform, 10)
		if err != nil {
			return err
		}
		for _, r := range set.References {
			if err := addRef(r); err != nil {
				return err
			}
		}
	}
	if len(m.References) == 0 {
		return specErr("research: no references; gather some (%s references) or search Mobbin with --query", SpecFile)
	}
	if spec.Notes != "" {
		if err := proof.CleanPath(spec.Notes); err != nil || path.Ext(spec.Notes) != ".md" {
			return specErr("research: notes %q: want a Markdown file inside --from", spec.Notes)
		}
		data, err := os.ReadFile(filepath.Join(b.From, filepath.FromSlash(spec.Notes)))
		if err != nil {
			return specErr("research: %v", err)
		}
		res.add("files/"+spec.Notes, "notes", data)
	}
	refs, _ := json.MarshalIndent(m.References, "", "  ")
	res.add("references.json", "reference-set", append(refs, '\n'))
	return nil
}

func (b *Build) artifacts(spec Spec, up *Manifest, res *Result) error {
	if err := checkStates(b.Stage, b.From, spec); err != nil {
		return err
	}
	if b.Stage == Mockup {
		if miss := covers(up.States, spec); len(miss) > 0 {
			return specErr("mockup: no mockup for wireframe state(s) %s", strings.Join(miss, ", "))
		}
	}
	var tok *Tokens
	if b.Stage == Mockup {
		var err error
		if tok, err = ParseTokens(b.Tokens); err != nil {
			return fmt.Errorf("%w: [design] tokens: %w", ErrRefuse, err)
		}
	}
	var bad []string
	for _, st := range spec.States {
		data, err := os.ReadFile(filepath.Join(b.From, filepath.FromSlash(st.File)))
		if err != nil {
			return specErr("%v", err)
		}
		res.add("files/"+st.File, b.Stage, data)
		if tok != nil {
			for _, mm := range tok.CheckLiterals(st.File, data) {
				bad = append(bad, mm.String())
			}
		}
	}
	res.Manifest.States = spec.States
	if len(bad) > 0 {
		return fmt.Errorf("%w: %d colour(s) in the mockups are not design tokens: %s", ErrTokens, len(bad), strings.Join(first(bad, 5), "; "))
	}
	return nil
}

func (b *Build) prototype(ctx context.Context, spec Spec, up *Manifest, res *Result) error {
	m := res.Manifest
	if err := checkStates(Prototype, b.From, spec); err != nil {
		return err
	}
	if miss := covers(up.States, spec); len(miss) > 0 {
		return specErr("prototype: no prototype state for mockup state(s) %s", strings.Join(miss, ", "))
	}
	switch {
	case !flagKey.MatchString(spec.Flag):
		return specErr("prototype: flag %q: want the OpenFeature flag key the prototype sits behind", spec.Flag)
	case !fileKey.MatchString(spec.FigmaFile):
		return specErr("prototype: figma_file %q: want a Figma file key", spec.FigmaFile)
	case len(spec.CodeConnect) == 0:
		return specErr("prototype: code_connect lists no Code Connect sources")
	}
	if err := CheckFlag(b.Flags, spec.Flag); err != nil {
		return fmt.Errorf("%w: [design] flags: %w", ErrRefuse, err)
	}
	m.Flag, m.States = spec.Flag, spec.States
	sources := map[string][]byte{}
	for _, p := range spec.CodeConnect {
		if err := proof.CleanPath(p); err != nil {
			return specErr("prototype: code_connect %q: want a repository path", p)
		}
		data, err := os.ReadFile(filepath.Join(b.Root, filepath.FromSlash(p)))
		if err != nil {
			return specErr("prototype: code_connect: %v", err)
		}
		sources[p] = data
		res.add("code-connect/"+p, "code-connect", data)
	}
	m.CodeConnect = CodeConnect(sources)
	for _, c := range spec.Components {
		if !nodeID.MatchString(c) {
			return specErr("prototype: component %q: want a node ID like 1:2", c)
		}
	}
	if miss := Unmapped(m.CodeConnect, spec.FigmaFile, spec.Components); len(miss) > 0 {
		return fmt.Errorf("%w: no Code Connect mapping for Figma component(s) %s of file %s", ErrRefuse, strings.Join(miss, ", "), spec.FigmaFile)
	}
	tok, err := ParseTokens(b.Tokens)
	if err != nil {
		return fmt.Errorf("%w: [design] tokens: %w", ErrRefuse, err)
	}

	// Figma frames, pinned by the hash of their renders.
	frames := map[string][]byte{}
	var cstates []CaptureState
	for _, st := range spec.States {
		name, data, err := b.Figma.Frame(ctx, spec.FigmaFile, st.FigmaNode)
		if err != nil {
			return err
		}
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("%w: figma render of %s is not a PNG: %w", integrations.ErrProvider, st.FigmaNode, err)
		}
		sum := sha256.Sum256(data)
		w, h := img.Bounds().Dx(), img.Bounds().Dy()
		m.Figma = append(m.Figma, FigmaPin{File: spec.FigmaFile, Node: st.FigmaNode, Name: name, Width: w, Height: h, SHA256: hex.EncodeToString(sum[:])})
		frames[st.ID] = data
		res.add("figma/"+st.ID+".png", "figma-frame", data)
		cstates = append(cstates, CaptureState{ID: st.ID, URL: strings.TrimRight(b.Target, "/") + st.Path, Width: w, Height: h})
	}

	caps := b.Captures
	m.Captures = "supplied"
	if caps == nil {
		if b.Target == "" {
			return fmt.Errorf("%w: no prototype target: set [verify.environments.<AAYU_ENV>] url, or pass --captures", ErrRefuse)
		}
		if caps, err = b.Capture(ctx, cstates); err != nil {
			return err
		}
		m.Captures = "browser"
	}
	byID := map[string]Captured{}
	for _, c := range caps {
		byID[c.ID] = c
	}
	pass := true
	for _, st := range spec.States {
		g := GateResult{State: st.ID, Node: st.FigmaNode}
		c, ok := byID[st.ID]
		switch {
		case !ok:
			g.Error = "no capture of this state"
		case c.Error != "":
			g.Error = "capture failed: " + c.Error
		default:
			res.add("captures/"+st.ID+".png", "screenshot", c.data)
			shot, err := png.Decode(bytes.NewReader(c.data))
			if err != nil {
				g.Error = "capture is not a PNG: " + err.Error()
				break
			}
			fig, _ := png.Decode(bytes.NewReader(frames[st.ID]))
			if g.Diff, err = Diff(fig, shot, DefaultThreshold); err != nil {
				g.Error = err.Error()
			}
			g.Colour, g.Typography = tok.CheckStyles(c.Styles)
		}
		g.Judge(b.FigmaDiffMax)
		pass = pass && g.Pass
		m.Gate = append(m.Gate, g)
	}
	if vars, err := b.Figma.Variables(ctx, spec.FigmaFile); err == nil {
		g := GateResult{State: "figma-variables", Node: spec.FigmaFile}
		g.Colour, g.Typography = tok.CheckVariables(vars)
		g.Judge(b.FigmaDiffMax)
		pass = pass && g.Pass
		m.Gate = append(m.Gate, g)
	} else if !errors.Is(err, integrations.ErrProvider) {
		return err
	}
	report, _ := json.MarshalIndent(m.Gate, "", "  ")
	res.add("figma-diff.json", "report", append(report, '\n'))
	if !pass {
		var fails []string
		for _, g := range m.Gate {
			if !g.Pass {
				fails = append(fails, g.Summary())
			}
		}
		return fmt.Errorf("%w: %s", ErrGate, strings.Join(fails, "; "))
	}
	return nil
}

// Encode renders a manifest as indented JSON with a trailing newline.
func Encode(m *Manifest) []byte {
	b, _ := json.MarshalIndent(m, "", "  ")
	return append(b, '\n')
}
