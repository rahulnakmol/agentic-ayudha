package ready

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// Feature baselines (Feature #874, core design S5): an agent drafts a part of
// a Feature (`aayu ready <f> --part <p> --request`), and only a person
// approves it (`aayu ready <f> --part <p>`), through GitHub's device flow, at
// the hash of the pins the draft holds. Both records live on the ledger under
// specs/<f>/baselines/.

// BaselineSchema is the schema of a Feature baseline record.
const BaselineSchema = "aayudha.feature.baseline/v1"

// Parts are the parts of a Feature that get a baseline.
var Parts = []string{"intent", "spec", "tech", "ux"}

// partOwner is the seat that owns each part.
var partOwner = map[string]verbs.SeatID{
	"intent": verbs.ProductStrategist, "spec": verbs.ProductStrategist,
	"tech": verbs.CodeArchitect, "ux": verbs.UXResearcher,
}

// Pin is one thing a baseline freezes: a reference and its hash.
type Pin struct {
	Ref  string `json:"ref"`
	Hash string `json:"hash"`
}

// Baseline is a part of a Feature as an agent drafted it and, once
// ApprovedBy is set, as a person approved it.
type Baseline struct {
	Schema        string `json:"schema"`
	Feature       int    `json:"feature"`
	Part          string `json:"part"`
	Version       int    `json:"version"` // k, from 1
	Owner         string `json:"owner"`   // the seat that owns the part
	Pins          []Pin  `json:"pins"`
	Hash          string `json:"hash"` // PinsHash(Pins): what the person approves
	Summary       string `json:"summary"`
	SourceSession string `json:"source_session,omitempty"` // the host session that drafted it
	DraftedBy     string `json:"drafted_by"`
	DraftedAt     string `json:"drafted_at"`
	ApprovedBy    string `json:"approved_by,omitempty"`
	ApprovedAt    string `json:"approved_at,omitempty"`
	Presence      string `json:"presence,omitempty"` // how the person was proven present: device-flow
}

// Check reports whether b is version v of part of Feature f as aayu writes
// it: its schema, its place, and a hash that is the hash of its pins.
func (b Baseline) Check(f int, part string, v int) error {
	if b.Schema != BaselineSchema || b.Feature != f || b.Part != part || b.Version != v || len(b.Pins) == 0 || b.Hash != PinsHash(b.Pins) {
		return fmt.Errorf("not %s v%d of #%d with the hash of its pins", part, v, f)
	}
	return nil
}

// Owner is the seat that owns part.
func Owner(part string) string { return string(partOwner[part]) }

// BaselinePath is where version v of a Feature part lives on the ledger: the
// person's approval, or with draft the agent's draft.
func BaselinePath(f int, part string, v int, draft bool) string {
	ext := ".json"
	if draft {
		ext = ".draft.json"
	}
	return fmt.Sprintf("%s/%s-v%d%s", BaselineDir(f), part, v, ext)
}

// BaselineDir holds every baseline record of Feature f.
func BaselineDir(f int) string { return fmt.Sprintf("specs/%d/baselines", f) }

// PinsHash is the hash of a baseline: SHA-256 of one "<ref> <hash>" line per pin.
func PinsHash(pins []Pin) string {
	var b strings.Builder
	for _, p := range pins {
		fmt.Fprintf(&b, "%s %s\n", p.Ref, p.Hash)
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// Pins is what part pins on Feature f now, or false while that part has no
// pins (the spec, technical design, and UX pins are n/a until #992).
func Pins(f int, part, body string) ([]Pin, bool) {
	if part != "intent" {
		return nil, false
	}
	return []Pin{{Ref: fmt.Sprintf("#%d body", f), Hash: workitems.BodyHash(body)}}, true
}

var baselineFile = regexp.MustCompile(`/([a-z]+)-v([0-9]+)(\.draft)?\.json$`)

// Versions reads the ledger paths of a Feature's baselines: the newest
// version of part a person approved, and the newest draft, each 0 when none.
func Versions(paths []string, part string) (approved, drafted int) {
	for _, p := range paths {
		m := baselineFile.FindStringSubmatch(p)
		if m == nil || m[1] != part {
			continue
		}
		v, _ := strconv.Atoi(m[2])
		if m[3] != "" {
			drafted = max(drafted, v)
		} else {
			approved = max(approved, v)
		}
	}
	return approved, drafted
}

// Label is the part as a sentence names it: intent, spec, tech, UX.
func Label(part string) string {
	if part == "ux" {
		return "UX"
	}
	return part
}

// Title is Label at the start of a sentence.
func Title(part string) string { l := Label(part); return strings.ToUpper(l[:1]) + l[1:] }

// BaselineMarker marks the one comment on a Feature that says where a part's
// baseline stands.
func BaselineMarker(part string) string { return "<!-- aayudha:baseline part=" + part + " -->" }

// Waits is what the marker comment and the terminal say about a draft.
func Waits(b Baseline) string {
	return fmt.Sprintf("%s v%d of #%d waits for a person: `aayu ready %d --part %s`", Title(b.Part), b.Version, b.Feature, b.Feature, b.Part)
}

// Baselined is what the marker comment and the terminal say about an approval.
func Baselined(b Baseline) string {
	return fmt.Sprintf("%s v%d of #%d baselined by @%s · hash %s", Title(b.Part), b.Version, b.Feature, b.ApprovedBy, b.Hash[:12])
}
