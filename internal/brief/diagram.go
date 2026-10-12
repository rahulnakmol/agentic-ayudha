package brief

import (
	"fmt"
	"html"
	"math"
	"regexp"
	"strings"
)

// Diagram is a parsed `*.diagram` source. The format is line based:
//
//	# a comment
//	title: Merge gate
//	direction: right            (or down)
//	pr: Pull request | Opened by the build lane.
//	gate: Merge gate | Re-runs verify on the head commit.
//	pr -> gate: head commit
//	gate -> ledger
//
// `id: Label | note` declares a node (the note shows when the reader selects
// it in the interactive version); `a -> b: label` an edge. A node named only in
// an edge is labelled with its ID.
type Diagram struct {
	Title string
	Down  bool
	Set   bool // direction was given; otherwise a flow too wide for a page turns down
	Nodes []*DNode
	Edges []DEdge
}

// maxRightWidth is the widest a left-to-right diagram may be before an
// unset direction turns it top-to-bottom, so labels stay readable on a page.
const maxRightWidth = 760

// DNode is one node with its layout.
type DNode struct {
	ID, Label, Note string
	rank, order     int
	x, y, w, h      float64
	lines           []string
}

// DEdge is one edge.
type DEdge struct{ From, To, Label string }

const (
	maxDiagramNodes = 40
	maxDiagramEdges = 80
)

var diagramID = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]*$`)

// ParseDiagram reads a diagram source.
func ParseDiagram(src string) (*Diagram, error) {
	d := &Diagram{}
	idx := map[string]*DNode{}
	node := func(id string) (*DNode, error) {
		if !diagramID.MatchString(id) || id == "title" || id == "direction" {
			return nil, fmt.Errorf("%q is not a node ID (letters, digits, _ . -; not title or direction)", id)
		}
		if n, ok := idx[id]; ok {
			return n, nil
		}
		n := &DNode{ID: id, Label: id}
		idx[id] = n
		d.Nodes = append(d.Nodes, n)
		return n, nil
	}
	for i, raw := range strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fail := func(format string, args ...any) error {
			return fmt.Errorf("diagram line %d: %s", i+1, fmt.Sprintf(format, args...))
		}
		if strings.Contains(line, "->") {
			path, label, _ := strings.Cut(line, ":")
			ids := strings.Split(path, "->")
			if len(ids) > 2 && strings.TrimSpace(label) != "" {
				return nil, fail("a chain of edges takes no label; write one edge per line")
			}
			for j := 0; j+1 < len(ids); j++ {
				a, err := node(strings.TrimSpace(ids[j]))
				if err != nil {
					return nil, fail("%v", err)
				}
				b, err := node(strings.TrimSpace(ids[j+1]))
				if err != nil {
					return nil, fail("%v", err)
				}
				d.Edges = append(d.Edges, DEdge{From: a.ID, To: b.ID, Label: strings.TrimSpace(label)})
			}
			continue
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fail("want `id: Label | note` or `a -> b: label`, got %q", line)
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		switch k {
		case "title":
			d.Title = v
			continue
		case "direction":
			if v != "right" && v != "down" {
				return nil, fail("direction is right or down")
			}
			d.Down, d.Set = v == "down", true
			continue
		}
		n, err := node(k)
		if err != nil {
			return nil, fail("%v", err)
		}
		label, note, _ := strings.Cut(v, "|")
		if strings.TrimSpace(label) == "" {
			return nil, fail("node %s has no label", k)
		}
		n.Label, n.Note = strings.TrimSpace(label), strings.TrimSpace(note)
	}
	switch {
	case len(d.Nodes) == 0:
		return nil, fmt.Errorf("diagram has no nodes")
	case len(d.Nodes) > maxDiagramNodes || len(d.Edges) > maxDiagramEdges:
		return nil, fmt.Errorf("diagram has %d nodes and %d edges; at most %d and %d keep it readable", len(d.Nodes), len(d.Edges), maxDiagramNodes, maxDiagramEdges)
	}
	return d, nil
}

// layout ranks nodes by the longest path from a source (back edges of a
// cycle are ignored), orders each rank by the mean position of its
// predecessors, and places them in columns (right) or rows (down).
func (d *Diagram) layout() (width, height float64) {
	w, h := d.place()
	if !d.Set && !d.Down && w > maxRightWidth {
		d.Down = true
		w, h = d.place()
	}
	return w, h
}

func (d *Diagram) place() (width, height float64) {
	for _, n := range d.Nodes {
		n.rank, n.order = 0, 0
	}
	idx := map[string]*DNode{}
	for _, n := range d.Nodes {
		idx[n.ID] = n
	}
	out := map[string][]string{}
	in := map[string][]string{}
	for _, e := range d.Edges {
		out[e.From] = append(out[e.From], e.To)
		in[e.To] = append(in[e.To], e.From)
	}
	state := map[string]int{} // 1 visiting, 2 done
	back := map[[2]string]bool{}
	var dfs func(id string)
	dfs = func(id string) {
		state[id] = 1
		for _, t := range out[id] {
			switch state[t] {
			case 1:
				back[[2]string{id, t}] = true
			case 0:
				dfs(t)
			}
		}
		state[id] = 2
	}
	for _, n := range d.Nodes {
		if state[n.ID] == 0 {
			dfs(n.ID)
		}
	}
	// Longest path over forward edges, relaxed in node order until stable.
	for changed := true; changed; {
		changed = false
		for _, e := range d.Edges {
			if back[[2]string{e.From, e.To}] || e.From == e.To {
				continue
			}
			if r := idx[e.From].rank + 1; r > idx[e.To].rank {
				idx[e.To].rank, changed = r, true
			}
		}
	}
	var ranks [][]*DNode
	for _, n := range d.Nodes {
		for len(ranks) <= n.rank {
			ranks = append(ranks, nil)
		}
		n.order = len(ranks[n.rank])
		ranks[n.rank] = append(ranks[n.rank], n)
	}
	for r := 1; r < len(ranks); r++ {
		bary := map[string]float64{}
		for _, n := range ranks[r] {
			sum, k := 0.0, 0
			for _, p := range in[n.ID] {
				if idx[p].rank < r {
					sum += float64(idx[p].order)
					k++
				}
			}
			bary[n.ID] = float64(n.order)
			if k > 0 {
				bary[n.ID] = sum / float64(k)
			}
		}
		rk := ranks[r]
		for i := 1; i < len(rk); i++ { // stable insertion sort by barycentre
			for j := i; j > 0 && bary[rk[j].ID] < bary[rk[j-1].ID]; j-- {
				rk[j], rk[j-1] = rk[j-1], rk[j]
			}
		}
		for i, n := range rk {
			n.order = i
		}
	}
	for _, n := range d.Nodes {
		n.lines = wrap(n.Label, 22)
		longest := 0
		for _, l := range n.lines {
			longest = max(longest, len([]rune(l)))
		}
		n.w = math.Max(112, float64(longest)*7.6+32)
		n.h = math.Max(48, float64(len(n.lines))*18+26)
	}
	labelW := 0.0
	for _, e := range d.Edges {
		labelW = math.Max(labelW, float64(len([]rune(e.Label)))*6.8+20)
	}
	const pad = 16.0
	gap, rankGap := 28.0, math.Max(64, labelW+24) // edge labels sit between ranks
	if d.Down {
		// Labels of edges that fan out sit halfway between their ends, so
		// siblings need room for two labels side by side.
		gap, rankGap = math.Max(28, labelW), 72
	}
	// Extent of each rank across the flow, and its size along the flow.
	along := make([]float64, len(ranks))
	across := make([]float64, len(ranks))
	for r, rk := range ranks {
		for i, n := range rk {
			if d.Down {
				along[r] = math.Max(along[r], n.h)
				across[r] += n.w
			} else {
				along[r] = math.Max(along[r], n.w)
				across[r] += n.h
			}
			if i > 0 {
				across[r] += gap
			}
		}
	}
	maxAcross := 0.0
	for _, a := range across {
		maxAcross = math.Max(maxAcross, a)
	}
	pos := pad
	for r, rk := range ranks {
		off := pad + (maxAcross-across[r])/2
		for _, n := range rk {
			if d.Down {
				n.x, n.y = off, pos+(along[r]-n.h)/2
				off += n.w + gap
			} else {
				n.x, n.y = pos+(along[r]-n.w)/2, off
				off += n.h + gap
			}
		}
		pos += along[r] + rankGap
	}
	pos += pad - rankGap
	loop := 0.0
	for _, e := range d.Edges {
		if idx[e.To].rank <= idx[e.From].rank {
			loop = 56 // room for back edges, routed around the far side
		}
	}
	if d.Down {
		return maxAcross + 2*pad + loop, pos
	}
	return pos, maxAcross + 2*pad + loop
}

// wrap breaks a label into lines of at most n runes, at spaces, three lines at most.
func wrap(s string, n int) []string {
	var lines []string
	cur := ""
	for _, w := range strings.Fields(s) {
		if cur != "" && len([]rune(cur))+1+len([]rune(w)) > n {
			lines = append(lines, cur)
			cur = w
			continue
		}
		if cur != "" {
			cur += " "
		}
		cur += w
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	if len(lines) > 3 {
		lines = append(lines[:2], clip(strings.Join(lines[2:], " "), n-1))
	}
	return lines
}

// Diagram colours: every text and line passes WCAG contrast on white.
const (
	dFill   = "#eef4fb"
	dStroke = "#1f4e79"
	dText   = "#102a43"
	dEdge   = "#4a5a6a"
	dLabel  = "#243b53"
)

// SVG renders the diagram. id prefixes element IDs (unique per page); alt is
// the accessible name. The static form is an image; the interactive form is
// a group of focusable nodes that a page script highlights with their edges.
func (d *Diagram) SVG(id, alt string, interactive bool) string {
	w, h := d.layout()
	idx := map[string]*DNode{}
	for _, n := range d.Nodes {
		idx[n.ID] = n
	}
	e := html.EscapeString
	var b strings.Builder
	role := `role="img" aria-labelledby="` + id + `-title ` + id + `-desc"`
	if interactive {
		role = `role="group" aria-labelledby="` + id + `-title" class="aayu-diagram"`
	}
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %.0f %.0f" width="%.0f" height="%.0f" %s font-family="system-ui, -apple-system, 'Segoe UI', sans-serif">`, w, h, w, h, role)
	fmt.Fprintf(&b, `<title id="%s-title">%s</title><desc id="%s-desc">%s</desc>`, id, e(alt), id, e(d.Text()))
	fmt.Fprintf(&b, `<defs><marker id="%s-arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="8" markerHeight="8" orient="auto-start-reverse"><path d="M0,0 L10,5 L0,10 z" fill="%s"/></marker></defs>`, id, dEdge)
	b.WriteString(`<g class="edges">`)
	for _, ed := range d.Edges {
		a, c := idx[ed.From], idx[ed.To]
		var p [4][2]float64
		switch {
		case c.rank > a.rank && d.Down:
			p[0], p[3] = [2]float64{a.x + a.w/2, a.y + a.h}, [2]float64{c.x + c.w/2, c.y}
			dy := (p[3][1] - p[0][1]) / 2
			p[1], p[2] = [2]float64{p[0][0], p[0][1] + dy}, [2]float64{p[3][0], p[3][1] - dy}
		case c.rank > a.rank:
			p[0], p[3] = [2]float64{a.x + a.w, a.y + a.h/2}, [2]float64{c.x, c.y + c.h/2}
			dx := (p[3][0] - p[0][0]) / 2
			p[1], p[2] = [2]float64{p[0][0] + dx, p[0][1]}, [2]float64{p[3][0] - dx, p[3][1]}
		case d.Down: // back edge: around the right side
			p[0], p[3] = [2]float64{a.x + a.w, a.y + a.h/2}, [2]float64{c.x + c.w, c.y + c.h/2}
			far := math.Max(a.x+a.w, c.x+c.w) + 48
			p[1], p[2] = [2]float64{far, p[0][1]}, [2]float64{far, p[3][1]}
		default: // back edge: below
			p[0], p[3] = [2]float64{a.x + a.w/2, a.y + a.h}, [2]float64{c.x + c.w/2, c.y + c.h}
			far := math.Max(a.y+a.h, c.y+c.h) + 48
			p[1], p[2] = [2]float64{p[0][0], far}, [2]float64{p[3][0], far}
		}
		fmt.Fprintf(&b, `<g class="edge" data-from="%s" data-to="%s"><path d="M%.1f,%.1f C%.1f,%.1f %.1f,%.1f %.1f,%.1f" fill="none" stroke="%s" stroke-width="1.6" marker-end="url(#%s-arrow)"/>`,
			e(ed.From), e(ed.To), p[0][0], p[0][1], p[1][0], p[1][1], p[2][0], p[2][1], p[3][0], p[3][1], dEdge, id)
		if ed.Label != "" {
			mx := (p[0][0] + 3*p[1][0] + 3*p[2][0] + p[3][0]) / 8
			my := (p[0][1] + 3*p[1][1] + 3*p[2][1] + p[3][1]) / 8
			lw := float64(len([]rune(ed.Label)))*6.8 + 12
			fmt.Fprintf(&b, `<rect x="%.1f" y="%.1f" width="%.1f" height="20" rx="4" fill="#ffffff"/><text x="%.1f" y="%.1f" text-anchor="middle" font-size="12" fill="%s">%s</text>`,
				mx-lw/2, my-10, lw, mx, my+4, dLabel, e(ed.Label))
		}
		b.WriteString(`</g>`)
	}
	b.WriteString(`</g><g class="nodes">`)
	for _, n := range d.Nodes {
		attrs := ""
		if interactive {
			name := n.Label
			if n.Note != "" {
				name += ": " + n.Note
			}
			attrs = fmt.Sprintf(` tabindex="0" role="button" aria-label="%s" data-note="%s"`, e(name), e(n.Note))
		}
		fmt.Fprintf(&b, `<g class="node" data-id="%s" transform="translate(%.1f,%.1f)"%s><rect width="%.1f" height="%.1f" rx="8" fill="%s" stroke="%s" stroke-width="1.5"/>`,
			e(n.ID), n.x, n.y, attrs, n.w, n.h, dFill, dStroke)
		top := n.h/2 - float64(len(n.lines)-1)*9 + 5
		fmt.Fprintf(&b, `<text text-anchor="middle" font-size="14" fill="%s">`, dText)
		for i, l := range n.lines {
			fmt.Fprintf(&b, `<tspan x="%.1f" y="%.1f">%s</tspan>`, n.w/2, top+float64(i)*18, e(l))
		}
		b.WriteString(`</text></g>`)
	}
	b.WriteString(`</g></svg>`)
	return b.String()
}

// Text describes the diagram in words: its nodes and edges, for the SVG
// description and the text alternative.
func (d *Diagram) Text() string {
	label := map[string]string{}
	for _, n := range d.Nodes {
		label[n.ID] = n.Label
	}
	var parts []string
	for _, e := range d.Edges {
		s := label[e.From] + " leads to " + label[e.To]
		if e.Label != "" {
			s += " (" + e.Label + ")"
		}
		parts = append(parts, s)
	}
	if len(parts) == 0 {
		for _, n := range d.Nodes {
			parts = append(parts, n.Label)
		}
	}
	return strings.Join(parts, "; ") + "."
}
