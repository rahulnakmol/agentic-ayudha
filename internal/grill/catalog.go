// Package grill holds what makes a grilled spec concrete (WP5.3b): the
// catalogues that decompose vague qualifiers ("high quality", "fast") into
// dimensions, the checks that a Story's or Bug's criteria leave no qualifier
// unresolved and record who stated or confirmed each one, and the read-only
// assessment of the repository that grounds the questions in facts. The
// host's model asks the questions (ground rule 5); this package supplies the
// facts and the checks.
package grill

import (
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
)

// CatalogDir is where the catalogues live in the embedded assets, one
// Markdown file per domain, so a domain grows by editing data.
const CatalogDir = "skills/grill/references/vague"

// Domain is one catalogue: the qualifiers that trigger it and the dimensions
// a qualifier decomposes into, in the order to ask them.
type Domain struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	File       string      `json:"file"` // relative to skills/grill, e.g. references/vague/ux.md
	Qualifiers []string    `json:"qualifiers"`
	Dimensions []Dimension `json:"dimensions"`
}

// Dimension is one concrete question a vague qualifier breaks into.
type Dimension struct {
	ID       string   `json:"id"` // <domain>.<name>, e.g. ux.states
	Name     string   `json:"name"`
	Ask      string   `json:"ask"`
	Default  string   `json:"default"`
	Evidence string   `json:"evidence"`
	Facts    []string `json:"facts"` // fact kinds to cite when asking
	// Topics are the keys a question about this dimension is tagged with
	// (<dimension>/<topic>), so a scripted human can answer each one.
	Topics []string `json:"topics,omitempty"`
	// Products are the product types the dimension applies to (web, api,
	// cli, library); empty applies to every product. The assessment keeps
	// the dimensions of the products it finds in the repository, so "fast"
	// asks about startup time for a CLI and interaction latency for a web app.
	Products []string `json:"products,omitempty"`
}

// ProductTypes are the kinds of product a repository can be (Assess finds
// them; a dimension's Products names them).
var ProductTypes = []string{"web", "api", "cli", "library"}

// ForProducts returns the dimensions that apply to any of products, in
// order; with no products every dimension applies.
func ForProducts(ds []Dimension, products []string) []Dimension {
	if len(products) == 0 {
		return ds
	}
	var out []Dimension
	for _, d := range ds {
		if len(d.Products) == 0 || slices.ContainsFunc(d.Products, func(p string) bool { return slices.Contains(products, p) }) {
			out = append(out, d)
		}
	}
	return out
}

// Catalog is every domain, sorted by ID, and the abbreviations the gate
// unpacks (references/jargon.md).
type Catalog struct {
	Domains []Domain `json:"domains"`
	Jargon  Jargon   `json:"jargon"`
}

var (
	domainHeading = regexp.MustCompile("^# Vague: (.+) \\(`([a-z][a-z0-9-]*)`\\)\\s*$")
	dimHeading    = regexp.MustCompile("^### (.+) \\(`([a-z][a-z0-9-]*\\.[a-z][a-z0-9-]*)`\\)\\s*$")
	dimField      = regexp.MustCompile(`^- (Ask|Default|Evidence|Facts|Topics|Products): (.*)$`)
	topicKey      = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
)

// LoadCatalog reads every catalogue under CatalogDir in fsys. A catalogue
// that does not parse is an error: the files are data the checks depend on.
func LoadCatalog(fsys fs.FS) (Catalog, error) {
	var c Catalog
	names, err := fs.Glob(fsys, path.Join(CatalogDir, "*.md"))
	if err != nil {
		return c, err
	}
	if len(names) == 0 {
		return c, fmt.Errorf("grill: no catalogues under %s", CatalogDir)
	}
	seen := map[string]bool{}
	for _, n := range names {
		b, err := fs.ReadFile(fsys, n)
		if err != nil {
			return c, err
		}
		d, err := parseDomain(string(b))
		if err != nil {
			return c, fmt.Errorf("%s: %w", n, err)
		}
		d.File = strings.TrimPrefix(n, "skills/grill/")
		for _, id := range append([]string{d.ID}, dimIDs(d)...) {
			if seen[id] {
				return c, fmt.Errorf("%s: %q is defined twice", n, id)
			}
			seen[id] = true
		}
		c.Domains = append(c.Domains, d)
	}
	sort.Slice(c.Domains, func(i, j int) bool { return c.Domains[i].ID < c.Domains[j].ID })
	b, err := fs.ReadFile(fsys, JargonFile)
	if err != nil {
		return c, err
	}
	if c.Jargon, err = parseJargon(string(b)); err != nil {
		return c, fmt.Errorf("%s: %w", JargonFile, err)
	}
	return c, nil
}

func dimIDs(d Domain) []string {
	var out []string
	for _, x := range d.Dimensions {
		out = append(out, x.ID)
	}
	return out
}

func parseDomain(s string) (Domain, error) {
	var d Domain
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	var quals []string
	inQuals := false
	var cur *Dimension
	var field *string
	for i, line := range lines {
		if m := domainHeading.FindStringSubmatch(line); m != nil {
			d.Name, d.ID = m[1], m[2]
			continue
		}
		if q, ok := strings.CutPrefix(line, "Qualifiers:"); ok {
			quals, inQuals = append(quals, q), true
			continue
		}
		if inQuals {
			if strings.TrimSpace(line) == "" {
				inQuals = false
			} else {
				quals = append(quals, line)
			}
			continue
		}
		if m := dimHeading.FindStringSubmatch(line); m != nil {
			d.Dimensions = append(d.Dimensions, Dimension{Name: m[1], ID: m[2]})
			cur, field = &d.Dimensions[len(d.Dimensions)-1], nil
			continue
		}
		if strings.HasPrefix(line, "#") {
			cur, field = nil, nil
			continue
		}
		if cur == nil {
			continue
		}
		if m := dimField.FindStringSubmatch(line); m != nil {
			switch m[1] {
			case "Ask":
				field = &cur.Ask
			case "Default":
				field = &cur.Default
			case "Evidence":
				field = &cur.Evidence
			case "Facts", "Topics", "Products":
				field = nil
				for _, f := range strings.Split(m[2], ",") {
					if f = strings.TrimSpace(f); f == "" {
						continue
					}
					if m[1] == "Facts" {
						cur.Facts = append(cur.Facts, f)
					} else if m[1] == "Products" {
						if !slices.Contains(ProductTypes, f) {
							return d, fmt.Errorf("line %d: product %q of %s: want one of %s", i+1, f, cur.ID, strings.Join(ProductTypes, ", "))
						}
						cur.Products = append(cur.Products, f)
					} else if !topicKey.MatchString(f) {
						return d, fmt.Errorf("line %d: topic %q of %s: want a kebab-case key", i+1, f, cur.ID)
					} else {
						cur.Topics = append(cur.Topics, f)
					}
				}
				continue
			}
			*field = strings.TrimSpace(m[2])
			continue
		}
		if field != nil && strings.HasPrefix(line, "  ") && strings.TrimSpace(line) != "" {
			*field += " " + strings.TrimSpace(line) // a wrapped bullet
			continue
		}
		if strings.TrimSpace(line) != "" {
			return d, fmt.Errorf("line %d: unexpected %q under %s", i+1, strings.TrimSpace(line), cur.ID)
		}
		field = nil
	}
	if d.ID == "" {
		return d, fmt.Errorf("no \"# Vague: <name> (`<id>`)\" heading")
	}
	for _, q := range strings.Split(strings.Join(quals, " "), ",") {
		if q = strings.ToLower(strings.Join(strings.Fields(q), " ")); q != "" {
			d.Qualifiers = append(d.Qualifiers, q)
		}
	}
	if len(d.Qualifiers) == 0 || len(d.Dimensions) == 0 {
		return d, fmt.Errorf("%s: needs a Qualifiers line and at least one dimension", d.ID)
	}
	for _, x := range d.Dimensions {
		if !strings.HasPrefix(x.ID, d.ID+".") || x.Ask == "" || x.Default == "" || x.Evidence == "" {
			return d, fmt.Errorf("dimension %s: needs the id prefix %s., Ask, Default, and Evidence", x.ID, d.ID)
		}
	}
	return d, nil
}

// Dimension returns the dimension with id, if a catalogue has it.
func (c Catalog) Dimension(id string) (Dimension, bool) {
	for _, d := range c.Domains {
		for _, x := range d.Dimensions {
			if x.ID == id {
				return x, true
			}
		}
	}
	return Dimension{}, false
}

// Domain returns the domain with id.
func (c Catalog) Domain(id string) (Domain, bool) {
	for _, d := range c.Domains {
		if d.ID == id {
			return d, true
		}
	}
	return Domain{}, false
}

// Hit is one vague qualifier found in a text.
type Hit struct {
	Qualifier string `json:"qualifier"` // as the catalogue writes it
	Text      string `json:"text"`      // as the text writes it
	Domain    string `json:"domain"`
	Pos       int    `json:"-"`
}

// quoted spans are literal wording (UI copy, paths), never qualifiers.
var quoted = regexp.MustCompile("\"[^\"\n]*\"|“[^”\n]*”|`[^`\n]*`")

// Unquoted returns s with every quoted span blanked out, keeping offsets.
func Unquoted(s string) string {
	return quoted.ReplaceAllStringFunc(s, func(q string) string { return strings.Repeat(" ", len(q)) })
}

// Find returns the qualifiers in text outside quotes, in order of position.
// Longer qualifiers win over the shorter ones they contain ("easy to use"
// over "easy"); "high quality" also matches "high-quality".
func (c Catalog) Find(text string) []Hit {
	type q struct{ qual, domain string }
	var qs []q
	for _, d := range c.Domains {
		for _, x := range d.Qualifiers {
			qs = append(qs, q{x, d.ID})
		}
	}
	sort.SliceStable(qs, func(i, j int) bool { return len(qs[i].qual) > len(qs[j].qual) })
	s := Unquoted(text)
	taken := make([]bool, len(s))
	var hits []Hit
	for _, x := range qs {
		re := qualifierRE(x.qual)
		for _, m := range re.FindAllStringIndex(s, -1) {
			if slices.Contains(taken[m[0]:m[1]], true) {
				continue
			}
			for i := m[0]; i < m[1]; i++ {
				taken[i] = true
			}
			hits = append(hits, Hit{Qualifier: x.qual, Text: text[m[0]:m[1]], Domain: x.domain, Pos: m[0]})
		}
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Pos < hits[j].Pos })
	return hits
}

var reCache sync.Map // qualifier → *regexp.Regexp

func qualifierRE(q string) *regexp.Regexp {
	if re, ok := reCache.Load(q); ok {
		return re.(*regexp.Regexp)
	}
	parts := strings.FieldsFunc(q, func(r rune) bool { return r == ' ' || r == '-' })
	for i, p := range parts {
		parts[i] = regexp.QuoteMeta(p)
	}
	re := regexp.MustCompile(`(?i)\b` + strings.Join(parts, `[\s-]+`) + `\b`)
	reCache.Store(q, re)
	return re
}
