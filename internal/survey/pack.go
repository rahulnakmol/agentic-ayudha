package survey

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Load reads an index written by Save.
func Load(file string) (*Index, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var ix Index
	if err := json.Unmarshal(b, &ix); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	if ix.Version != Version {
		return nil, fmt.Errorf("%s: index version %d, this binary reads %d; run `aayu survey` again", file, ix.Version, Version)
	}
	ix.index()
	return &ix, nil
}

// Save writes the index as indented JSON.
func (ix *Index) Save(file string) error {
	b, err := json.MarshalIndent(ix, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	return os.WriteFile(file, append(b, '\n'), 0o644)
}

// Pack is the fact pack of one module: what a model reads, with the module's
// source files, before it writes the module's claims. It is bounded: the
// symbol and edge lists stop at the pack budget and say how many they left out.
type Pack struct {
	Module   Module    `json:"module"`
	Files    []File    `json:"files"`
	Targets  []Target  `json:"targets"`  // build targets that compile one of the module's files
	Entries  []Fact    `json:"entries"`  // entry points in the module
	Surfaces []Fact    `json:"surfaces"` // I/O surfaces in the module
	Traps    []Fact    `json:"traps"`    // trap rules that match in the module: where to check (traps.go)
	Markers  []Marker  `json:"markers"`
	Tests    []Fact    `json:"tests"`
	Symbols  []Symbol  `json:"symbols"`
	CallsIn  []Edge    `json:"calls_in"`  // calls from other modules into this one
	CallsOut []Edge    `json:"calls_out"` // calls from this module to others
	Includes []Edge    `json:"includes"`  // includes and imports from the module's files
	Omitted  []string  `json:"omitted,omitempty"`
	Read     []string  `json:"read"` // the files to read, largest last
	Others   []Summary `json:"other_modules"`
}

// Summary is another module in one line, so a pack shows where it sits.
type Summary struct {
	ID     string `json:"id"`
	Lines  int    `json:"lines"`
	FanIn  int    `json:"fan_in"`
	FanOut int    `json:"fan_out"`
}

// ModulePack builds the pack of one module. limit caps each list (0 means 200).
func (ix *Index) ModulePack(id string, limit int) (*Pack, error) {
	if limit <= 0 {
		limit = 200
	}
	var m *Module
	for k := range ix.Modules {
		if ix.Modules[k].ID == id {
			m = &ix.Modules[k]
		}
	}
	if m == nil {
		var ids []string
		for _, x := range ix.Modules {
			ids = append(ids, x.ID)
		}
		return nil, fmt.Errorf("no module %q; the index has %s", id, strings.Join(ids, ", "))
	}
	in := map[string]bool{}
	for _, f := range m.Files {
		in[f] = true
	}
	inAt := func(at string) bool { p, _ := splitAt(at); return in[p] }
	p := &Pack{Module: *m}
	for _, f := range ix.Files {
		if in[f.Path] {
			p.Files = append(p.Files, f)
		}
	}
	slices.SortStableFunc(p.Files, func(a, b File) int { return a.Bytes - b.Bytes })
	for _, f := range p.Files {
		p.Read = append(p.Read, f.Path)
	}
	for _, t := range ix.Targets {
		for _, s := range t.Sources {
			if in[s] {
				p.Targets = append(p.Targets, t)
				break
			}
		}
	}
	for _, f := range ix.Entries {
		if inAt(f.At) {
			p.Entries = append(p.Entries, f)
		}
	}
	for _, f := range ix.Surfaces {
		if inAt(f.At) {
			p.Surfaces = append(p.Surfaces, f)
		}
	}
	for _, f := range ix.Traps {
		if inAt(f.At) {
			p.Traps = append(p.Traps, f)
		}
	}
	for _, f := range ix.Tests {
		if inAt(f.At) {
			p.Tests = append(p.Tests, f)
		}
	}
	for _, mk := range ix.Markers {
		if inAt(mk.At) {
			p.Markers = append(p.Markers, mk)
		}
	}
	for _, s := range ix.Symbols {
		if inAt(s.At) {
			p.Symbols = append(p.Symbols, s)
		}
	}
	for _, e := range ix.Edges {
		from := in[fileOf(e.From)]
		switch {
		case e.Kind != "call" && from:
			p.Includes = append(p.Includes, e)
		case e.Kind == "call" && from && !in[fileOf(e.To[0])]:
			p.CallsOut = append(p.CallsOut, e)
		case e.Kind == "call" && !from && slices.ContainsFunc(e.To, func(t string) bool { return in[fileOf(t)] }):
			p.CallsIn = append(p.CallsIn, e)
		}
	}
	cut := func(name string, n int) int {
		if n > limit {
			p.Omitted = append(p.Omitted, fmt.Sprintf("%s: %d more (the full lists are in the index file)", name, n-limit))
			return limit
		}
		return n
	}
	p.Symbols = p.Symbols[:cut("symbols", len(p.Symbols))]
	p.CallsIn = p.CallsIn[:cut("calls_in", len(p.CallsIn))]
	p.CallsOut = p.CallsOut[:cut("calls_out", len(p.CallsOut))]
	p.Includes = p.Includes[:cut("includes", len(p.Includes))]
	p.Markers = p.Markers[:cut("markers", len(p.Markers))]
	for _, o := range ix.Modules {
		if o.ID != m.ID {
			p.Others = append(p.Others, Summary{o.ID, o.Lines, o.FanIn, o.FanOut})
		}
	}
	return p, nil
}
