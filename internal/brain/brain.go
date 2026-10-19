// Package brain keeps the product brain: an Obsidian vault of linked Markdown
// notes on the orphan `brain` branch, which is never merged. People and seat
// twins write the durable product knowledge (vision, mission, principles,
// direction, objectives, objections, decisions, capabilities); issues stay
// the system of record for the work, so a note links an issue and never copies
// it. `aayu brain checkpoint` refreshes only the marked generated blocks (links
// and one-line facts from the repository's records) and keeps every word a
// person wrote; `aayu brain check` holds the vault to its rules.
package brain

import (
	"bytes"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"go.yaml.in/yaml/v4"

	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// Branch is the orphan branch that holds the vault.
const Branch = "brain"

// Type is one note type: its folder, its default owner seat, its statuses, and
// the sections a person writes (the template's headings).
type Type struct {
	Name     string // frontmatter `type`, and the tag type/<name>
	Label    string // display name, and the template's name
	Folder   string // root folder; its map of content is <Folder>/<Folder>.md
	Owner    verbs.SeatID
	Statuses []string
	Sections []string // the person-written sections, in order
	Keys     []string // type-specific frontmatter keys the template carries
	Color    int      // graph colour, 0xRRGGBB
}

var docStatuses = []string{"draft", "current", "archived"}

// Types lists every note type. Issues stay the record of the work, so there is
// no Story, Bug, or Feedback note; an Objective cites Goal and Initiative
// issues and a Capability cites Feature issues by number.
var Types = []Type{
	{Name: "home", Label: "Start here", Owner: verbs.ProductStrategist, Statuses: []string{"current"}, Color: 0xFFFFFF},
	{Name: "moc", Label: "Map of content", Owner: verbs.ProductStrategist, Statuses: []string{"current"}, Color: 0xA0A0A0},
	{Name: "vision", Label: "Vision", Folder: "Vision", Owner: verbs.Founder, Statuses: docStatuses,
		Sections: []string{"The vision", "Who it serves", "What it is not"}, Color: 0xE05252},
	{Name: "mission", Label: "Mission", Folder: "Vision", Owner: verbs.Founder, Statuses: docStatuses,
		Sections: []string{"The mission", "How we know it works"}, Color: 0xE07A52},
	{Name: "principle", Label: "Principle", Folder: "Principles", Owner: verbs.Founder, Statuses: docStatuses,
		Sections: []string{"The principle", "Why", "In practice"}, Keys: []string{"objections"}, Color: 0xE0A852},
	{Name: "guidance", Label: "Guidance", Folder: "Guidance", Owner: verbs.ProductStrategist, Statuses: docStatuses,
		Sections: []string{"The guidance", "When it applies", "Examples"}, Color: 0xC9C94F},
	{Name: "direction", Label: "Direction", Folder: "Direction", Owner: verbs.Founder, Statuses: docStatuses,
		Sections: []string{"Where we are going", "Why now", "What we will not do"}, Keys: []string{"objections"}, Color: 0x8FD14F},
	{Name: "objective", Label: "Objective", Folder: "Objectives", Owner: verbs.ProductStrategist,
		Statuses: []string{"proposed", "active", "met", "missed", "dropped", "archived"},
		Sections: []string{"The objective", "How we measure it", "Why it matters"}, Keys: []string{"direction", "issues", "objections"}, Color: 0x4FC97A},
	{Name: "objection", Label: "Objection", Folder: "Objections", Owner: verbs.ProductStrategist,
		Statuses: []string{"open", "answered", "accepted", "set-aside"},
		Sections: []string{"The concern", "Evidence", "Resolution"}, Keys: []string{"raised_by", "objects_to", "resolution"}, Color: 0xD14FA8},
	{Name: "decision", Label: "Decision", Folder: "Decisions", Owner: verbs.Founder,
		Statuses: []string{"proposed", "accepted", "superseded", "withdrawn"},
		Sections: []string{"Context", "The choice", "Rejected options", "Consequences"}, Keys: []string{"issues", "supersedes", "superseded_by", "objections"}, Color: 0x4FA8D1},
	{Name: "capability", Label: "Capability", Folder: "Capabilities", Owner: verbs.ProductStrategist,
		Statuses: []string{"planned", "live", "retired", "archived"},
		Sections: []string{"What it does", "Why it exists", "Who it serves"}, Keys: []string{"issues", "objectives"}, Color: 0x4F6BD1},
	{Name: "persona", Label: "Persona", Folder: "Personas", Owner: verbs.ProductStrategist, Statuses: docStatuses,
		Sections: []string{"Who they are", "What they try to do", "What gets in the way"}, Color: 0x8F4FD1},
	{Name: "problem", Label: "Problem", Folder: "Problems", Owner: verbs.ProductStrategist,
		Statuses: []string{"open", "solved", "archived"},
		Sections: []string{"The problem", "Who has it", "Evidence"}, Color: 0xB34FD1},
	{Name: "term", Label: "Glossary term", Folder: "Glossary", Owner: verbs.Marketer, Statuses: docStatuses,
		Sections: []string{"Meaning", "Not to be confused with"}, Color: 0x9C9C6B},
	{Name: "seat", Label: "Seat", Folder: "Seats", Owner: verbs.Founder, Statuses: docStatuses,
		Sections: []string{"How this seat works here"}, Color: 0x6BB3B3},
	{Name: "person", Label: "Person", Folder: "People", Owner: verbs.Founder, Statuses: []string{"active", "alumni"},
		Sections: []string{"Context"}, Color: 0x6B8FB3},
	{Name: "workstream", Label: "Workstream", Folder: "Workstreams", Owner: verbs.ProductStrategist,
		Statuses: []string{"active", "paused", "archived"},
		Sections: []string{"Purpose", "Scope", "Start here"}, Color: 0xD1B34F},
	{Name: "release", Label: "Release", Folder: "Releases", Owner: verbs.ProductStrategist,
		Statuses: []string{"planned", "released", "archived"},
		Sections: []string{"Intent", "Objectives served", "Out of scope", "Outcome"}, Keys: []string{"version", "release_issue", "objectives", "objections"}, Color: 0xD17A4F},
	{Name: "revenue", Label: "Revenue", Folder: "Revenue", Owner: verbs.RevenueArchitect, Statuses: docStatuses,
		Sections: []string{"Public facts only"}, Color: 0xD1D14F},
	{Name: "timeline", Label: "Timeline", Folder: "Timeline", Owner: verbs.ProductStrategist, Statuses: []string{"current"},
		Sections: []string{"Eras"}, Color: 0xB3B3B3},
}

// FindType returns the note type named name.
func FindType(name string) (Type, bool) {
	i := slices.IndexFunc(Types, func(t Type) bool { return t.Name == name })
	if i < 0 {
		return Type{}, false
	}
	return Types[i], true
}

// Marker comments delimit a generated block. Checkpoint rewrites what is
// between them and nothing else.
const (
	beginMark = "<!-- BEGIN aayudha:brain:"
	endMark   = "<!-- END aayudha:brain:"
	markClose = " -->"
)

// Block renders a generated block named name holding body.
func Block(name, body string) string {
	return beginMark + name + markClose + "\n" + strings.TrimRight(body, "\n") + "\n" + endMark + name + markClose
}

// blockRE matches a generated block: markers on lines of their own.
var blockRE = regexp.MustCompile(`(?ms)^<!-- BEGIN aayudha:brain:([a-z0-9-]+) -->\n(.*?)^<!-- END aayudha:brain:([a-z0-9-]+) -->$`)

// maskCode blanks the code of a Markdown body as CommonMark parses it
// (goldmark): fenced and indented code blocks with a fence's info string, and
// inline code spans. Lengths and newlines hold, so indexes into body stay
// valid; generated-block markers and wikilinks inside code are text.
func maskCode(body string) string {
	src := []byte(body)
	masked := []byte(body)
	_ = ast.Walk(mdParser.Parse(text.NewReader(src)), func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch c := n.(type) {
		case *ast.FencedCodeBlock:
			if c.Info != nil {
				blank(masked, c.Info.Segment.Start, c.Info.Segment.Stop)
			}
			blankLines(masked, c.Lines())
			return ast.WalkSkipChildren, nil
		case *ast.CodeBlock:
			blankLines(masked, c.Lines())
			return ast.WalkSkipChildren, nil
		case *ast.CodeSpan:
			for t := c.FirstChild(); t != nil; t = t.NextSibling() {
				if t, ok := t.(*ast.Text); ok {
					blank(masked, t.Segment.Start, t.Segment.Stop)
				}
			}
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	return string(masked)
}

var mdParser = goldmark.DefaultParser()

func blankLines(b []byte, lines *text.Segments) {
	for i := 0; i < lines.Len(); i++ {
		s := lines.At(i)
		blank(b, s.Start, s.Stop)
	}
}

func blank(b []byte, from, to int) {
	for k := from; k < to; k++ {
		if b[k] != '\n' {
			b[k] = ' '
		}
	}
}

// blockSpans returns the generated blocks of body as [start, end, nameStart,
// nameEnd, ...] submatch indexes, skipping marker text inside code.
func blockSpans(body string) [][]int {
	masked := maskCode(body)
	var out [][]int
	for _, m := range blockRE.FindAllStringSubmatchIndex(masked, -1) {
		if body[m[2]:m[3]] == body[m[6]:m[7]] {
			out = append(out, m)
		}
	}
	return out
}

// Blocks returns the generated blocks of a note body, by name.
func Blocks(body string) map[string]string {
	out := map[string]string{}
	for _, m := range blockSpans(body) {
		out[body[m[2]:m[3]]] = body[m[4]:m[5]]
	}
	return out
}

// SetBlock replaces the first generated block name in body, or appends it at
// the end when the body has none (a person deleted it, or the note predates it).
func SetBlock(body, name, content string) string {
	b := Block(name, content)
	for _, m := range blockSpans(body) {
		if body[m[2]:m[3]] == name {
			return body[:m[0]] + b + body[m[1]:]
		}
	}
	if h := blockHeadings[name]; h != "" {
		b = "## " + h + "\n\n" + b
	}
	return strings.TrimRight(body, "\n") + "\n\n" + b + "\n"
}

// blockHeadings are the headings a generated block gets when a checkpoint
// appends it to a note that lacks it.
var blockHeadings = map[string]string{
	"record": "Record", "plan": "Plan", "changes": "Changes since baseline", "delivered": "Delivered",
	"history": "Implementation history", "facts": "Facts", "roles": "Roles", "events": "Eras",
	"workstreams": "Workstreams", "releases": "Releases", "review": "Needs review",
}

// StripBlocks removes every generated block from body: what is left is what a
// person wrote.
func StripBlocks(body string) string {
	spans := blockSpans(body)
	for i := len(spans) - 1; i >= 0; i-- {
		body = body[:spans[i][0]] + body[spans[i][1]:]
	}
	return body
}

// Fact makes untrusted text (an issue title) inert in a generated block: it
// can neither end the block, add a wikilink to the graph, nor render as
// Markdown or HTML (an image that fetches a URL, a link, a tag).
func Fact(s string) string {
	s = strings.NewReplacer("\n", " ", "\r", " ").Replace(s)
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune(factEscapes, r) {
			b.WriteByte('\\') // a backslash-escaped character is literal text in Markdown
		}
		b.WriteRune(r)
	}
	return b.String()
}

// factEscapes are the Markdown and HTML metacharacters Fact escapes, so an
// issue title renders as text: no link, image, HTML, emphasis, code, table
// cell, or Obsidian comment (%%), highlight (==), or math ($).
const factEscapes = "\\`*_[]()!<>|#~&%=$"

// Note is one parsed Markdown note: ordered frontmatter and the body.
type Note struct {
	Path string
	Meta []KV
	Body string
	head string // the frontmatter's leading comment, kept on render
}

// KV is one frontmatter key with its YAML value.
type KV struct {
	Key   string
	Value *yaml.Node
	key   *yaml.Node // the parsed key node, with its comments
}

// ErrNoFrontmatter is returned for a note without a YAML frontmatter block.
var ErrNoFrontmatter = fmt.Errorf("no YAML frontmatter (a --- block at the top)")

// Parse reads a note's frontmatter and body.
func Parse(p string, src []byte) (*Note, error) {
	s := strings.ReplaceAll(string(src), "\r\n", "\n")
	if !strings.HasPrefix(s, "---\n") {
		return &Note{Path: p, Body: s}, ErrNoFrontmatter
	}
	end := -1 // the closing delimiter is a line of exactly ---
	for i := 4; i <= len(s)-3; {
		j := strings.IndexByte(s[i:], '\n')
		line := s[i:]
		if j >= 0 {
			line = s[i : i+j]
		}
		if line == "---" {
			end = i
			break
		}
		if j < 0 {
			break
		}
		i += j + 1
	}
	if end < 0 {
		return &Note{Path: p, Body: s}, ErrNoFrontmatter
	}
	fm, body := s[4:end], strings.TrimPrefix(s[end+3:], "\n")
	n := &Note{Path: p, Body: body}
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(fm), &doc); err != nil {
		return n, fmt.Errorf("frontmatter: %w", err)
	}
	if len(doc.Content) == 0 {
		return n, nil
	}
	m := doc.Content[0]
	if m.Kind != yaml.MappingNode {
		return n, fmt.Errorf("frontmatter: want a mapping of keys")
	}
	n.head = strings.TrimSpace(doc.HeadComment + "\n" + m.HeadComment)
	seen := map[string]bool{}
	for i := 0; i+1 < len(m.Content); i += 2 {
		k := m.Content[i].Value
		if seen[k] {
			return n, fmt.Errorf("frontmatter: key %q appears twice", k)
		}
		seen[k] = true
		n.Meta = append(n.Meta, KV{Key: k, Value: m.Content[i+1], key: m.Content[i]})
	}
	return n, nil
}

func (n *Note) node(key string) *yaml.Node {
	for _, kv := range n.Meta {
		if kv.Key == key {
			return kv.Value
		}
	}
	return nil
}

// Has reports whether the frontmatter has key.
func (n *Note) Has(key string) bool { return n.node(key) != nil }

// Str returns a scalar frontmatter value, or "".
func (n *Note) Str(key string) string {
	v := n.node(key)
	if v == nil || v.Kind != yaml.ScalarNode || v.Tag == "!!null" {
		return ""
	}
	return v.Value
}

// List returns a list frontmatter value (a scalar counts as a list of one).
func (n *Note) List(key string) []string {
	v := n.node(key)
	if v == nil {
		return nil
	}
	if v.Kind == yaml.ScalarNode {
		if v.Value == "" || v.Tag == "!!null" {
			return nil
		}
		return []string{v.Value}
	}
	var out []string
	for _, c := range v.Content {
		if c.Kind == yaml.ScalarNode && c.Value != "" {
			out = append(out, c.Value)
		}
	}
	return out
}

// Set writes key = value, keeping the key's place when it exists.
func (n *Note) Set(key string, value any) {
	v := &yaml.Node{}
	if err := v.Encode(value); err != nil {
		panic(err) // values are strings, ints, and string lists
	}
	if l, ok := value.([]string); ok && len(l) > 0 {
		v.Style = 0
	}
	for i, kv := range n.Meta {
		if kv.Key == key {
			n.Meta[i].Value = v
			return
		}
	}
	n.Meta = append(n.Meta, KV{Key: key, Value: v})
}

// Render writes the note back as Markdown.
func (n *Note) Render() []byte {
	m := &yaml.Node{Kind: yaml.MappingNode, HeadComment: n.head}
	for _, kv := range n.Meta {
		k := kv.key
		if k == nil {
			k = &yaml.Node{Kind: yaml.ScalarNode, Value: kv.Key}
		}
		m.Content = append(m.Content, k, kv.Value)
	}
	var b bytes.Buffer
	b.WriteString("---\n")
	if len(m.Content) > 0 {
		enc := yaml.NewEncoder(&b)
		enc.SetIndent(2)
		_ = enc.Encode(m)
		_ = enc.Close()
	}
	b.WriteString("---\n\n")
	b.WriteString(strings.TrimLeft(n.Body, "\n"))
	if !strings.HasSuffix(b.String(), "\n") {
		b.WriteString("\n")
	}
	return b.Bytes()
}

// Link renders a wikilink to a note path: [[basename]], since basenames are
// unique across the vault.
func Link(p string) string { return "[[" + Base(p) + "]]" }

// Base is a note's link name: its file name without .md.
func Base(p string) string { return strings.TrimSuffix(path.Base(p), ".md") }

// Today's date format in frontmatter.
const DateLayout = "2006-01-02"

var unsafeName = regexp.MustCompile(`[\[\]#^|\\/:*?"<>]+`)

// FileName turns a title into a safe note file name (without .md): no
// characters that break a wikilink or a path, at most 80 characters.
func FileName(title string) string {
	s := strings.Join(strings.Fields(unsafeName.ReplaceAllString(title, " ")), " ")
	if r := []rune(s); len(r) > 80 {
		s = strings.TrimSpace(string(r[:80]))
	}
	return s
}

// MOCPath is the map of content of a root folder.
func MOCPath(folder string) string { return folder + "/" + folder + ".md" }

// Folders lists the root folders that get a map of content, in Home's order.
func Folders() []string {
	var out []string
	for _, t := range Types {
		if t.Folder != "" && !slices.Contains(out, t.Folder) {
			out = append(out, t.Folder)
		}
	}
	return out
}

// WorkstreamIndex and WorkstreamMOC are a workstream's own small brain.
func WorkstreamIndex(ws string) string { return "Workstreams/" + ws + "/" + ws + " Index.md" }

// WorkstreamMOC is the map of content of workstream ws.
func WorkstreamMOC(ws string) string { return "Workstreams/" + ws + "/" + ws + " MOC.md" }

// workstreamOf returns the workstream folder name of a path, or "".
func workstreamOf(p string) string {
	rest, ok := strings.CutPrefix(p, "Workstreams/")
	if !ok {
		return ""
	}
	ws, _, ok := strings.Cut(rest, "/")
	if !ok {
		return ""
	}
	return ws
}

// TemplatePath is the Obsidian template of note type t.
func TemplatePath(t Type) string { return "Templates/" + t.Label + " template.md" }
