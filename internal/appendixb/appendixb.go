// Package appendixb keeps Appendix B of docs/BUILD-PLAN.md, the deviations and
// decisions recorded while implementing the plan, as one file per row under
// docs/appendix-b/ (Story #397, Feedback #330). Parallel branches that each add
// a row add different files, so they merge without a conflict; the table is
// rendered on demand (gen -print) rather than committed, because a committed
// rendering would collide exactly as the hand-written table did.
//
// Layout: docs/appendix-b/<NNN>-<slug>/README.md holds a section's title, its
// intro, and its table header; docs/appendix-b/<NNN>-<slug>/<NNN>-<ID>.md holds
// one row: "# <ID>", then one "## <column>" heading per column after "#", each
// followed by that cell. Sections and rows render in file-name order, so the
// numbers order them; two branches that pick the same number still merge, and
// the slug or ID breaks the tie.
package appendixb

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const (
	// Dir holds the rows, relative to the repository root.
	Dir = "docs/appendix-b"
	// Plan is the build plan whose Appendix B ends in the generated pointer.
	Plan = "docs/BUILD-PLAN.md"
	// PrintCommand prints the whole appendix as tables.
	PrintCommand = "go run ./internal/appendixb/gen -print"

	begin = "<!-- BEGIN GENERATED: appendix-b (internal/appendixb/gen; DO NOT EDIT) -->"
	end   = "<!-- END GENERATED: appendix-b -->"
)

// Pointer is the generated end of docs/BUILD-PLAN.md: where the rows live and
// how to read them. It names no row, so adding a row never changes it.
const Pointer = begin + `

The deviations and decisions recorded while implementing this plan, from PR1 on, live one file per row in
[docs/appendix-b/](appendix-b/README.md): a directory per section and a file per row, ` + "`<NNN>-<ID>.md`" + `, so
parallel work adds files instead of editing one table. To add a row, follow AGENTS.md ("Appendix B rows"). Print the
whole appendix as tables with ` + "`" + PrintCommand + "`" + `; find one row by its ID with
` + "`ls docs/appendix-b/*/*-D-5.5c.12.md`" + `.

` + end + "\n"

// Section is one "### " section of the appendix.
type Section struct {
	Dir     string   // directory name under Dir, "<NNN>-<slug>"
	Order   int      // the NNN of Dir
	Title   string   // heading text
	Intro   string   // the text between the heading and the table, links relative to the plan
	Columns []string // table header; the first is "#"
	Rows    []Row
}

// Row is one table row; its ID is the "#" cell.
type Row struct {
	File  string   // file name in the section's directory, "<NNN>-<ID>.md"
	Order int      // the NNN of File
	ID    string   // the "#" cell
	Cells []string // the other cells, in column order, links relative to the plan
}

var (
	sectionName = regexp.MustCompile(`^([0-9]{3})-[a-z0-9-]+$`)
	rowName     = regexp.MustCompile(`^([0-9]{3})-(.+)\.md$`)
	link        = regexp.MustCompile(`(\]\()([^)\s]+)(\))`)
)

// Load reads and validates every section and row under root/Dir: names,
// headings, columns, cells without an unescaped "|", and IDs unique across the
// whole appendix. A missing directory is an empty appendix.
func Load(root string) ([]Section, error) {
	entries, err := os.ReadDir(filepath.Join(root, Dir))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var secs []Section
	var errs []error
	for _, e := range entries {
		if !e.IsDir() {
			if e.Name() != "README.md" {
				errs = append(errs, fmt.Errorf("%s/%s: only README.md and section directories belong here", Dir, e.Name()))
			}
			continue
		}
		s, err := loadSection(root, e.Name())
		if err != nil {
			errs = append(errs, err)
			continue
		}
		secs = append(secs, s)
	}
	errs = append(errs, checkUnique(secs)...)
	return secs, errors.Join(errs...)
}

func loadSection(root, name string) (Section, error) {
	m := sectionName.FindStringSubmatch(name)
	if m == nil {
		return Section{}, fmt.Errorf("%s/%s: a section directory is named <NNN>-<slug> (lower-case letters, digits, hyphens)", Dir, name)
	}
	s := Section{Dir: name}
	s.Order, _ = strconv.Atoi(m[1])
	readme := path.Join(Dir, name, "README.md")
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(readme)))
	if err != nil {
		return s, fmt.Errorf("%s: %w", readme, err)
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if !strings.HasPrefix(lines[0], "# ") {
		return s, fmt.Errorf("%s: the first line is \"# <section title>\"", readme)
	}
	s.Title = strings.TrimPrefix(lines[0], "# ")
	t := slices.IndexFunc(lines, func(l string) bool { return strings.HasPrefix(l, "|") })
	if t < 0 || t+2 != len(lines) || !strings.HasPrefix(lines[t+1], "|---") {
		return s, fmt.Errorf("%s: end with the table's header row and its |---| row, and nothing after them", readme)
	}
	s.Intro = fromFile(strings.TrimSpace(strings.Join(lines[1:t], "\n")), name)
	s.Columns = splitRow(lines[t])
	if len(s.Columns) < 2 || s.Columns[0] != "#" {
		return s, fmt.Errorf("%s: the table's first column is \"#\", the row ID", readme)
	}
	files, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(path.Join(Dir, name))))
	if err != nil {
		return s, err
	}
	var errs []error
	for _, f := range files {
		if f.Name() == "README.md" {
			continue
		}
		r, err := loadRow(root, s, f.Name())
		if err != nil {
			errs = append(errs, err)
			continue
		}
		s.Rows = append(s.Rows, r)
	}
	return s, errors.Join(errs...)
}

func loadRow(root string, s Section, name string) (Row, error) {
	file := path.Join(Dir, s.Dir, name)
	m := rowName.FindStringSubmatch(name)
	if m == nil {
		return Row{}, fmt.Errorf("%s: a row file is named <NNN>-<ID>.md", file)
	}
	r := Row{File: name, ID: m[2]}
	r.Order, _ = strconv.Atoi(m[1])
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
	if err != nil {
		return r, err
	}
	lines := strings.Split(string(b), "\n")
	if lines[0] != "# "+r.ID {
		return r, fmt.Errorf("%s: the first line is %q, the ID in the file name", file, "# "+r.ID)
	}
	var heads []string
	var cell []string
	flush := func() {
		if len(heads) > 0 {
			c := strings.Join(cell, " ")
			if strings.HasPrefix(c, `\#`) { // a cell that starts with "#" (an issue number) is escaped in the file
				c = c[1:]
			}
			r.Cells = append(r.Cells, fromFile(c, s.Dir))
		}
		cell = nil
	}
	for _, l := range lines[1:] {
		if h, ok := strings.CutPrefix(l, "## "); ok {
			flush()
			heads = append(heads, h)
			continue
		}
		if t := strings.TrimSpace(l); t != "" {
			cell = append(cell, t)
		}
	}
	flush()
	if want := s.Columns[1:]; !slices.Equal(heads, want) {
		return r, fmt.Errorf("%s: the headings are %q, one \"## <column>\" per column of the section's table after \"#\", in order", file, want)
	}
	for i, c := range r.Cells {
		if hasBarePipe(c) {
			return r, fmt.Errorf("%s: the %q cell has a \"|\" that would split the table: write it as \\|", file, heads[i])
		}
	}
	return r, nil
}

func checkUnique(secs []Section) []error {
	seen := map[string]string{}
	var errs []error
	for _, s := range secs {
		for _, r := range s.Rows {
			f := path.Join(Dir, s.Dir, r.File)
			if prev, ok := seen[r.ID]; ok {
				errs = append(errs, fmt.Errorf("row ID %s is used twice in Appendix B: %s and %s; give the newer row the next free ID", r.ID, prev, f))
				continue
			}
			seen[r.ID] = f
		}
	}
	return errs
}

// Render is the appendix as the plan used to hold it: one "### " section per
// directory, each with its intro and table, links relative to the plan.
func Render(secs []Section) string {
	var b strings.Builder
	for i, s := range secs {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "### %s\n\n", s.Title)
		if s.Intro != "" {
			b.WriteString(s.Intro + "\n\n")
		}
		b.WriteString(joinRow(s.Columns) + "\n")
		b.WriteString("|" + strings.Repeat("---|", len(s.Columns)) + "\n")
		for _, r := range s.Rows {
			b.WriteString(joinRow(append([]string{r.ID}, r.Cells...)) + "\n")
		}
	}
	return b.String()
}

// Generate checks the rows under root/Dir and keeps the end of the plan, from
// the BEGIN marker on, equal to Pointer. Sections and rows written there by
// hand, the old way, move to their own files first, and Generate says so on w,
// as a GitHub error annotation when annotate is set; the generate-drift check
// then fails until the moved files are committed.
func Generate(root string, w io.Writer, annotate bool) error {
	planPath := filepath.Join(root, filepath.FromSlash(Plan))
	b, err := os.ReadFile(planPath)
	if err != nil {
		return err
	}
	plan := string(b)
	i := strings.Index(plan, begin)
	if i < 0 {
		return fmt.Errorf("%s: missing the %q marker that ends Appendix B", Plan, begin)
	}
	tail := plan[i:]
	if tail == Pointer {
		_, err := Load(root)
		return err
	}
	hand, err := parseHand(tail)
	if err != nil {
		return err
	}
	secs, err := Load(root)
	if err != nil {
		return err
	}
	moved, err := adopt(root, secs, hand)
	if err != nil {
		return err
	}
	if _, err := Load(root); err != nil {
		return err
	}
	if err := os.WriteFile(planPath, []byte(plan[:i]+Pointer), 0o644); err != nil {
		return err
	}
	return tell(w, annotate, moved)
}

func tell(w io.Writer, annotate bool, moved []string) error {
	rule := "Appendix B rows live one file per row in " + Dir + "/ (AGENTS.md, \"Appendix B rows\"); do not add them to " + Plan + "."
	head := Plan + "'s Appendix B ends in a generated pointer; go generate restored it. " + rule
	if len(moved) > 0 {
		head = Plan + " had Appendix B rows written by hand; go generate moved them to their own files. " + rule + " Commit the moved files."
	}
	if annotate {
		head = "::error file=" + Plan + "::" + head
	} else {
		head = "appendixb: " + head
	}
	var b strings.Builder
	b.WriteString(head + "\n")
	for _, m := range moved {
		b.WriteString("  " + m + "\n")
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// parseHand reads sections and rows written by hand in the plan's tail: a
// "### " heading, its intro, its header, and its rows. The pointer's own lines
// are skipped.
func parseHand(tail string) ([]Section, error) {
	skip := map[string]bool{}
	for _, l := range strings.Split(Pointer, "\n") {
		skip[l] = true
	}
	var secs []Section
	var intro []string
	for _, l := range strings.Split(tail, "\n") {
		switch {
		case skip[l] && l != "":
			continue
		case strings.HasPrefix(l, "### "):
			secs = append(secs, Section{Title: strings.TrimPrefix(l, "### ")})
			intro = nil
		case strings.HasPrefix(l, "|"):
			if len(secs) == 0 {
				return nil, fmt.Errorf("%s: an Appendix B row after the generated pointer has no \"### \" section; add it as a file under %s/ instead", Plan, Dir)
			}
			s := &secs[len(secs)-1]
			switch {
			case s.Columns == nil:
				s.Columns = splitRow(l)
				s.Intro = strings.TrimSpace(strings.Join(intro, "\n"))
			case strings.HasPrefix(l, "|---"):
			default:
				cells := splitRow(l)
				if len(cells) != len(s.Columns) {
					return nil, fmt.Errorf("%s: row %q has %d cells, its table %d", Plan, cells[0], len(cells), len(s.Columns))
				}
				s.Rows = append(s.Rows, Row{ID: cells[0], Cells: cells[1:]})
			}
		case len(secs) > 0 && secs[len(secs)-1].Columns == nil:
			intro = append(intro, l)
		}
	}
	return secs, nil
}

// adopt writes the hand-written sections and rows into files: a section is
// matched by title, else it is new and goes last; a row is matched by ID, else
// it is new and goes last in its section. It returns one line per file written.
func adopt(root string, secs []Section, hand []Section) ([]string, error) {
	var moved []string
	nextSec := 1
	for _, s := range secs {
		nextSec = max(nextSec, s.Order+1)
	}
	where := map[string]string{}
	for _, s := range secs {
		for _, r := range s.Rows {
			where[r.ID] = s.Title
		}
	}
	for _, h := range hand {
		if h.Columns == nil || h.Columns[0] != "#" {
			return nil, fmt.Errorf("%s: the hand-written section %q needs a table whose first column is \"#\"", Plan, h.Title)
		}
		k := slices.IndexFunc(secs, func(s Section) bool { return s.Title == h.Title })
		isNew := k < 0
		if isNew {
			secs = append(secs, Section{Dir: fmt.Sprintf("%03d-%s", nextSec, slug(h.Title)), Order: nextSec, Title: h.Title, Columns: h.Columns})
			nextSec++
			k = len(secs) - 1
		}
		s := &secs[k]
		if !slices.Equal(s.Columns, h.Columns) {
			return nil, fmt.Errorf("%s: section %q has columns %q, its files %q", Plan, h.Title, h.Columns, s.Columns)
		}
		if isNew || h.Intro != "" && h.Intro != s.Intro { // a heading copied without its intro keeps the intro // a heading copied without its intro keeps the intro
			if f, changed, err := writeSection(root, *s, h.Intro); err != nil {
				return nil, err
			} else if changed {
				moved = append(moved, f)
			}
			s.Intro = h.Intro
		}
		nextRow := 1
		for _, r := range s.Rows {
			nextRow = max(nextRow, r.Order+1)
		}
		for _, hr := range h.Rows {
			if t, ok := where[hr.ID]; ok && t != s.Title {
				return nil, fmt.Errorf("%s: Appendix B row ID %s is already used in section %q; give the new row the next free ID", Plan, hr.ID, t)
			}
			j := slices.IndexFunc(s.Rows, func(r Row) bool { return r.ID == hr.ID })
			if j < 0 {
				hr.File, hr.Order = fmt.Sprintf("%03d-%s.md", nextRow, hr.ID), nextRow
				nextRow++
				s.Rows = append(s.Rows, hr)
			} else {
				hr.File, hr.Order = s.Rows[j].File, s.Rows[j].Order
				s.Rows[j] = hr
			}
			where[hr.ID] = s.Title
			f, changed, err := writeRow(root, *s, hr)
			if err != nil {
				return nil, err
			}
			if changed {
				moved = append(moved, f)
			}
		}
	}
	return moved, nil
}

func writeSection(root string, s Section, intro string) (string, bool, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", s.Title)
	if intro != "" {
		b.WriteString(toFile(intro) + "\n\n")
	}
	b.WriteString(joinRow(s.Columns) + "\n|" + strings.Repeat("---|", len(s.Columns)) + "\n")
	return write(root, path.Join(Dir, s.Dir, "README.md"), b.String())
}

func writeRow(root string, s Section, r Row) (string, bool, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n", r.ID)
	for i, c := range r.Cells {
		fmt.Fprintf(&b, "\n## %s\n", s.Columns[i+1])
		if c != "" {
			if strings.HasPrefix(c, "#") {
				c = `\` + c // not a heading
			}
			b.WriteString("\n" + toFile(c) + "\n")
		}
	}
	return write(root, path.Join(Dir, s.Dir, r.File), b.String())
}

func write(root, file, data string) (string, bool, error) {
	p := filepath.Join(root, filepath.FromSlash(file))
	if old, err := os.ReadFile(p); err == nil && string(old) == data {
		return file, false, nil
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return file, false, err
	}
	return file, true, os.WriteFile(p, []byte(data), 0o644)
}

// toFile and fromFile move a cell's relative links between the plan's
// directory (docs/) and a section's directory, so links work, and the docs
// job's link check checks them, in both places.
func toFile(s string) string {
	return relink(s, func(t string) string { return path.Join("../..", t) })
}

func fromFile(s, dir string) string {
	return relink(s, func(t string) string { return path.Join("appendix-b", dir, t) })
}

func relink(s string, f func(string) string) string {
	return link.ReplaceAllStringFunc(s, func(m string) string {
		p := link.FindStringSubmatch(m)
		t := p[2]
		if strings.Contains(t, "://") || strings.HasPrefix(t, "#") || strings.HasPrefix(t, "/") || strings.HasPrefix(t, "mailto:") {
			return m
		}
		return p[1] + f(t) + p[3]
	})
}

// splitRow splits a table line on the "|" that are not escaped as "\|".
func splitRow(l string) []string {
	l = strings.TrimSpace(l)
	l = strings.TrimPrefix(l, "|")
	if strings.HasSuffix(l, "|") && !strings.HasSuffix(l, `\|`) {
		l = l[:len(l)-1]
	}
	var cells []string
	var cur strings.Builder
	for i := 0; i < len(l); i++ {
		switch {
		case l[i] == '\\' && i+1 < len(l) && l[i+1] == '|':
			cur.WriteString(`\|`)
			i++
		case l[i] == '|':
			cells = append(cells, strings.TrimSpace(cur.String()))
			cur.Reset()
		default:
			cur.WriteByte(l[i])
		}
	}
	return append(cells, strings.TrimSpace(cur.String()))
}

func joinRow(cells []string) string {
	return "| " + strings.Join(cells, " | ") + " |"
}

func hasBarePipe(s string) bool {
	return len(splitRow("|"+s+"|")) != 1
}

// slug names a section's directory after what its title is about: the words
// after "while <verb>ing" when the title has them ("recorded while implementing
// WP5.9 (survey)" is "wp5-9-survey").
func slug(title string) string {
	if _, after, ok := strings.Cut(title, " while "); ok {
		if verb, rest, ok := strings.Cut(after, " "); ok && strings.HasSuffix(verb, "ing") {
			title = rest
		}
	}
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(title) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
		if b.Len() >= 40 {
			cut := b.String()
			if i := strings.LastIndexByte(cut, '-'); i > 0 {
				cut = cut[:i]
			}
			return strings.Trim(cut, "-")
		}
	}
	return strings.Trim(b.String(), "-")
}
