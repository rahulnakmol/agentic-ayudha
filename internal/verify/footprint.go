package verify

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// LayerGlobs are a repository's layer path globs, from the table in its
// LAYERS.md (generated on aayudha itself from the registry).
type LayerGlobs map[string][]string

var (
	layerRow  = regexp.MustCompile(`^\|\s*(L[0-9]{2})\s*\|`)
	layerGlob = regexp.MustCompile("`([^`]+)`")
)

// ReadLayers parses LAYERS.md at the repository root; a repository without
// one has no layers.
func ReadLayers(root string) LayerGlobs {
	b, err := os.ReadFile(filepath.Join(root, "LAYERS.md"))
	if err != nil {
		return nil
	}
	out := LayerGlobs{}
	for _, line := range strings.Split(string(b), "\n") {
		m := layerRow.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		cells := strings.Split(line, "|")
		last := cells[len(cells)-2]
		for _, g := range layerGlob.FindAllStringSubmatch(last, -1) {
			out[m[1]] = append(out[m[1]], g[1])
		}
	}
	return out
}

// Footprint returns the layers whose globs match any changed path, sorted.
func (l LayerGlobs) Footprint(paths []string) []string {
	var out []string
	for id, globs := range l {
		for _, g := range globs {
			if slices.ContainsFunc(paths, func(p string) bool { return Match(g, p) }) {
				out = append(out, id)
				break
			}
		}
	}
	slices.Sort(out)
	return out
}

// Match matches a slash path against a glob where ** spans directories and *
// stays within one segment. A glob without a slash or wildcard is a path.
func Match(glob, p string) bool {
	var re strings.Builder
	re.WriteString("^")
	for i := 0; i < len(glob); i++ {
		switch c := glob[i]; {
		case c == '*' && i+1 < len(glob) && glob[i+1] == '*':
			re.WriteString(".*")
			i++
			if i+1 < len(glob) && glob[i+1] == '/' {
				i++
				re.WriteString("/?")
			}
		case c == '*':
			re.WriteString("[^/]*")
		case c == '?':
			re.WriteString("[^/]")
		default:
			re.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	re.WriteString("$")
	ok, _ := regexp.MatchString(re.String(), p)
	return ok
}
