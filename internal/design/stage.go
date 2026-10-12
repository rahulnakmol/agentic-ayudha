// Package design holds the deterministic half of the design verbs (WP6.5):
// research, wireframe, mockup, and prototype. The host's model does the design
// work in the skill; the binary checks what the host produced, pins every
// reference (upstream manifests by SHA-256, external references as URLs and
// IDs only, Figma frames by the hash of their render), applies the figma-diff
// gate to a prototype, and lands each stage on the ledger under
// design/<feature>/<stage>/.
package design

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/integrations"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/mobbin"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
)

// Stages, in order: each stage pins the manifest of the one before it.
const (
	Research  = "research"
	Wireframe = "wireframe"
	Mockup    = "mockup"
	Prototype = "prototype"
)

// Stages lists the stages in order.
var Stages = []string{Research, Wireframe, Mockup, Prototype}

// Previous is the stage a stage builds on, or "" for research.
func Previous(stage string) string {
	i := slices.Index(Stages, stage)
	if i <= 0 {
		return ""
	}
	return Stages[i-1]
}

// Dir is a stage's ledger directory: design/<feature>/<stage>.
func Dir(feature int, stage string) string { return path.Join("design", fmt.Sprint(feature), stage) }

// ManifestPath is a stage's manifest on the ledger.
func ManifestPath(feature int, stage string) string {
	return path.Join(Dir(feature, stage), "manifest.json")
}

// SpecFile is the file the host writes in a stage's --from directory.
const SpecFile = "stage.json"

// Errors the CLI maps to exit codes.
var (
	ErrSpec   = errors.New("stage spec")         // the host's stage.json or files are wrong (exit 2)
	ErrRefuse = errors.New("refused")            // an upstream stage is missing, or a policy refuses (exit 4)
	ErrGate   = errors.New("figma-diff gate")    // the prototype misses the gate (exit 3)
	ErrTokens = errors.New("design-token check") // a mockup uses a colour that is not a token (exit 3)
)

// Spec is stage.json: what the host produced for one stage.
type Spec struct {
	Notes       string                   `json:"notes,omitempty"`      // research: user-research notes (Markdown)
	References  []integrations.Reference `json:"references,omitempty"` // research: references the host gathered (URLs and IDs only)
	States      []State                  `json:"states,omitempty"`
	Flag        string                   `json:"flag,omitempty"`         // prototype: OpenFeature flag key
	FigmaFile   string                   `json:"figma_file,omitempty"`   // prototype: Figma file key
	CodeConnect []string                 `json:"code_connect,omitempty"` // prototype: Code Connect sources, repository paths
	Components  []string                 `json:"components,omitempty"`   // prototype: Figma component nodes the prototype uses
}

// State is one state of the Feature's UI.
type State struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	File      string `json:"file,omitempty"`       // wireframe, mockup: the artifact, relative to --from
	Path      string `json:"path,omitempty"`       // prototype: URL path on the [verify.environments] target
	FigmaNode string `json:"figma_node,omitempty"` // prototype: the Figma frame the state is diffed against
}

var (
	stateID   = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,40}$`)
	nodeID    = regexp.MustCompile(`^[0-9]+:[0-9]+$`)
	flagKey   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,80}$`)
	fileKey   = regexp.MustCompile(`^[A-Za-z0-9]+$`)
	imageExts = []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".avif", ".bmp", ".tif", ".tiff", ".heic"}
	textExts  = []string{".svg", ".html", ".htm", ".md"}
)

func specErr(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrSpec, fmt.Sprintf(format, args...))
}

// LoadSpec reads <dir>/stage.json. A missing --from (dir "") is an empty spec.
func LoadSpec(dir string) (Spec, error) {
	var s Spec
	if dir == "" {
		return s, nil
	}
	b, err := os.ReadFile(filepath.Join(dir, SpecFile))
	if err != nil {
		return s, specErr("%v", err)
	}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&s); err != nil {
		return s, specErr("%s: %v", SpecFile, err)
	}
	return s, nil
}

// CheckReference accepts a reference only as a URL and an ID (WP6.4, J3-18):
// an https page URL, never an image; Mobbin references must be Mobbin page
// URLs.
func CheckReference(r integrations.Reference) error {
	u, err := url.Parse(r.URL)
	switch {
	case r.ID == "" || r.Source == "" || r.Kind == "":
		return specErr("reference %q: source, kind, id, and url are required", r.URL)
	case err != nil || u.Scheme != "https" || u.Host == "":
		return specErr("reference %q: want an https URL", r.URL)
	case slices.Contains(imageExts, strings.ToLower(path.Ext(u.Path))) || strings.Contains(strings.ToLower(u.Host), "bytescale"):
		return specErr("reference %q points at an image; references are page URLs and IDs only, never pixels", r.URL)
	case (r.Source == "mobbin" || strings.HasSuffix(strings.ToLower(u.Host), "mobbin.com")) && !mobbin.Page(r.URL):
		return specErr("reference %q: a Mobbin reference must be a mobbin.com page URL", r.URL)
	}
	return nil
}

// checkStates validates the states of a wireframe, mockup, or prototype spec.
func checkStates(stage, dir string, s Spec) error {
	if len(s.States) == 0 {
		return specErr("%s: no states", stage)
	}
	seen := map[string]bool{}
	for _, st := range s.States {
		if !stateID.MatchString(st.ID) || seen[st.ID] {
			return specErr("%s: state id %q: want a unique lower-case slug", stage, st.ID)
		}
		seen[st.ID] = true
		if strings.TrimSpace(st.Title) == "" {
			return specErr("%s: state %s has no title", stage, st.ID)
		}
		switch stage {
		case Wireframe, Mockup:
			if err := proof.CleanPath(st.File); err != nil || st.File == "" {
				return specErr("%s: state %s: file %q: want a relative path inside --from", stage, st.ID, st.File)
			}
			ext := strings.ToLower(path.Ext(st.File))
			if !slices.Contains(textExts, ext) && (stage == Mockup || !slices.Contains(imageExts, ext)) {
				want := "SVG, HTML, Markdown, or an image"
				if stage == Mockup {
					want = "SVG or HTML, so its colours can be checked against the tokens"
				}
				return specErr("%s: state %s: %s: want %s", stage, st.ID, st.File, want)
			}
			if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(st.File))); err != nil {
				return specErr("%s: state %s: %v", stage, st.ID, err)
			}
		case Prototype:
			if !strings.HasPrefix(st.Path, "/") {
				return specErr("prototype: state %s: path %q: want a URL path starting with /", st.ID, st.Path)
			}
			if !nodeID.MatchString(st.FigmaNode) {
				return specErr("prototype: state %s: figma_node %q: want a node ID like 4:7", st.ID, st.FigmaNode)
			}
		}
	}
	return nil
}

// covers reports the upstream states the spec leaves out.
func covers(up []State, s Spec) []string {
	var missing []string
	for _, u := range up {
		if !slices.ContainsFunc(s.States, func(st State) bool { return st.ID == u.ID }) {
			missing = append(missing, u.ID)
		}
	}
	return missing
}
