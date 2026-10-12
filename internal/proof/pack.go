// Package proof defines proof packs (WP4.1) and the ledger operations on them
// (WP4.2): push, get, verify, list, and purge.
//
// A pack is one pack.json that names the story, the PR head commit it proves,
// who wrote it, the Completion Goal hash it was built against, which artifacts
// are the evidence for each criterion, every artifact's SHA-256 and size and
// where its bytes live, and every floor's result. Packs written by an agent are
// advisory; packs written by the base-branch merge gate in CI are
// authoritative (WP4.8).
package proof

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/models"
)

// SchemaVersion is the pack schema this binary writes. Readers for every
// version shipped since 0.1 stay in Decode (WP1.9 grandfathering).
const SchemaVersion = 1

// FileName is the pack's file name in a pack directory.
const FileName = "pack.json"

// Authority kinds.
const (
	Advisory      = "advisory"      // pushed by an agent lane or a human
	Authoritative = "authoritative" // written by the base-branch merge gate in CI (WP4.8)
)

// Artifact stores: where an artifact's bytes live.
const (
	StoreGit    = "git"    // committed to the ledger branch (data artifacts)
	StoreLFS    = "lfs"    // committed to the ledger through Git LFS (media, private repositories)
	StoreBucket = "bucket" // uploaded to the private [proof] media bucket (media; required on public repositories, V3)
	StoreNone   = "none"   // media not pushed (proof push without --media); only the hash is kept
)

// Floor results.
const (
	Pass    = "pass"
	Fail    = "fail"
	Skipped = "skipped"
	Waived  = "waived"
	Blocked = "blocked" // the floor refused to produce evidence (e.g. PII found, production target)
)

// Evidence kinds of artifacts, plus report for floor outputs.
var Kinds = []string{"screenshot", "recording", "sr-transcript", "api-trace", "metric", "report"}

// Pack is pack.json v1.
type Pack struct {
	Schema             int         `json:"schema_version"`
	Repo               string      `json:"repo"`
	Story              int         `json:"story"`
	PR                 int         `json:"pr,omitempty"`
	PRHeadSHA          string      `json:"pr_head_sha"`
	BaseSHA            string      `json:"base_sha,omitempty"`
	Checkpoint         string      `json:"checkpoint,omitempty"` // n/N from the Aayudha-Checkpoint trailer
	Seat               string      `json:"seat"`
	Host               string      `json:"host"`
	Model              string      `json:"model,omitempty"` // the builder model, when known
	Identity           string      `json:"identity"`        // bot, human, human-fallback, or ci
	Authority          Authority   `json:"authority"`
	CompletionGoalHash string      `json:"completion_goal_hash"`
	Footprint          []string    `json:"footprint,omitempty"` // layers the PR's changed paths fall in
	Criteria           []Criterion `json:"criteria"`
	Artifacts          []Artifact  `json:"artifacts"`
	Checks             []Check     `json:"checks"`
	Cost               Cost        `json:"cost"`
	Links              Links       `json:"links"`
	CreatedAt          string      `json:"created_at"`
}

// Authority says who wrote a pack and whether it counts.
type Authority struct {
	Kind     string `json:"kind"`               // advisory or authoritative
	Workflow string `json:"workflow,omitempty"` // authoritative: the merge gate workflow file
	RunID    int64  `json:"run_id,omitempty"`   // authoritative: the Actions run that produced it
	Writer   string `json:"writer,omitempty"`   // who landed it on the ledger, when not the run itself
}

// Criterion maps one Completion Goal criterion to its evidence.
type Criterion struct {
	ID       int      `json:"id"`
	Evidence []string `json:"evidence"` // artifact paths
}

// Artifact is one evidence file.
type Artifact struct {
	Path      string     `json:"path"` // relative, always "/"-separated
	Kind      string     `json:"kind"`
	SHA256    string     `json:"sha256"`
	Size      int64      `json:"size"`
	Store     string     `json:"store"`
	Object    string     `json:"object,omitempty"` // bucket: the object key
	Tombstone *Tombstone `json:"tombstone,omitempty"`
}

// Tombstone records that an artifact's bytes were purged; the hash stays.
type Tombstone struct {
	PurgedAt string `json:"purged_at"`
	Reason   string `json:"reason"`
}

// Check is one floor's result.
type Check struct {
	Floor     string  `json:"floor"`
	Result    string  `json:"result"`
	Value     float64 `json:"value"`
	Threshold float64 `json:"threshold"`
	Tool      string  `json:"tool"`
	Version   string  `json:"version"`
	Detail    string  `json:"detail,omitempty"`
	Waiver    string  `json:"waiver,omitempty"` // floor-waiver Decision, e.g. #57
	Network   string  `json:"network,omitempty"`
}

// Cost is model spend attributed to the pack's story.
type Cost struct {
	TokensIn  int     `json:"tokens_in"`
	TokensOut int     `json:"tokens_out"`
	USD       float64 `json:"usd"`
}

// Links point at what the pack proves.
type Links struct {
	Issue         string   `json:"issue,omitempty"`
	PR            string   `json:"pr,omitempty"`
	ReadySnapshot string   `json:"ready_snapshot,omitempty"` // ledger path of the Ready snapshot
	Actions       []string `json:"actions,omitempty"`        // ledger paths of actions segments
}

//go:embed schema/pack.v1.json
var schemaV1 []byte

// SchemaJSON returns the JSON Schema of a pack version.
func SchemaJSON(version int) ([]byte, bool) {
	if version == 1 {
		return schemaV1, true
	}
	return nil, false
}

// Decode reads pack.json of any supported schema version and validates it
// against that version's schema.
func Decode(b []byte) (*Pack, error) {
	var head struct {
		Schema int `json:"schema_version"`
	}
	if err := json.Unmarshal(b, &head); err != nil {
		return nil, fmt.Errorf("pack.json: %w", err)
	}
	sch, ok := SchemaJSON(head.Schema)
	if !ok {
		return nil, fmt.Errorf("pack.json: schema version %d is not supported by this binary (newest %d); run `aayu self-update`", head.Schema, SchemaVersion)
	}
	if errs := Validate(sch, b); len(errs) > 0 {
		return nil, fmt.Errorf("pack.json does not match schema v%d: %s", head.Schema, strings.Join(errs, "; "))
	}
	var p Pack
	dec := json.NewDecoder(bytes.NewReader(b))
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("pack.json: %w", err)
	}
	return &p, nil
}

// Validate checks a JSON document against a JSON Schema (the WP1.6 subset).
func Validate(schema, doc []byte) []string {
	var s models.Schema
	if err := json.Unmarshal(schema, &s); err != nil {
		return []string{"schema: " + err.Error()}
	}
	var v any
	if err := json.Unmarshal(doc, &v); err != nil {
		return []string{err.Error()}
	}
	return s.Validate(v)
}

// Encode renders a pack as stable, indented JSON.
func Encode(p *Pack) ([]byte, error) {
	if p.Schema == 0 {
		p.Schema = SchemaVersion
	}
	if p.Authority.Kind == "" {
		p.Authority.Kind = Advisory
	}
	slices.SortFunc(p.Artifacts, func(a, b Artifact) int { return strings.Compare(a.Path, b.Path) })
	slices.SortFunc(p.Criteria, func(a, b Criterion) int { return a.ID - b.ID })
	if p.Criteria == nil {
		p.Criteria = []Criterion{}
	}
	if p.Artifacts == nil {
		p.Artifacts = []Artifact{}
	}
	if p.Checks == nil {
		p.Checks = []Check{}
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(p); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// Artifact returns the artifact at p, if any.
func (p *Pack) Artifact(rel string) (*Artifact, bool) {
	for i := range p.Artifacts {
		if p.Artifacts[i].Path == rel {
			return &p.Artifacts[i], true
		}
	}
	return nil, false
}

// Check returns the result of a floor, if the pack has one.
func (p *Pack) Check(floor string) (*Check, bool) {
	for i := range p.Checks {
		if p.Checks[i].Floor == floor {
			return &p.Checks[i], true
		}
	}
	return nil, false
}

// FloorsHold reports whether every floor passed, was skipped for a reason
// that does not block, or was waived. A failed or blocked floor never holds,
// and no model disposition can change that (rule 7).
func (p *Pack) FloorsHold() bool {
	for _, c := range p.Checks {
		switch c.Result {
		case Pass, Waived:
		case Skipped:
			if BlocksShip(c) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// BlocksShip reports whether a skipped floor blocks ship without a waiver:
// a mobile story's skipped mobile floor does (WP4.9).
func BlocksShip(c Check) bool { return c.Result == Skipped && c.Floor == "mobile" }

// MediaExts are the media file extensions: Git LFS on a private ledger,
// the private bucket on a public one (V3). init writes them to the ledger's
// .gitattributes on private repositories.
var MediaExts = []string{"png", "jpg", "jpeg", "gif", "webp", "mp4", "webm", "mov", "wav", "mp3", "pdf"}

// IsMedia reports whether an artifact is proof media: a capture kind that can
// show people or their data, or a media file extension.
func IsMedia(a Artifact) bool {
	switch a.Kind {
	case "screenshot", "recording", "sr-transcript":
		return true
	}
	return slices.Contains(MediaExts, strings.TrimPrefix(strings.ToLower(path.Ext(a.Path)), "."))
}

// CleanPath checks an artifact path: relative, "/"-separated, no "..".
func CleanPath(p string) error {
	if p == "" || strings.HasPrefix(p, "/") || strings.Contains(p, `\`) || path.Clean(p) != p || p == ".." || strings.HasPrefix(p, "../") {
		return fmt.Errorf("artifact path %q: want a clean relative path with / separators", p)
	}
	return nil
}
