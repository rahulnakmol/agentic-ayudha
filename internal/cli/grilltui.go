package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// grillStep is one question of the interactive interview (WP5.3): a body
// section of the work item, or the title, parent, and labels around it.
type grillStep struct {
	ID       string   // field ID, or title, parent, labels
	Label    string   // what the human sees
	Hints    []string // the questions from skills/grill/references/<type>.md
	Multi    bool     // a free-text field: Enter on an empty line (or Ctrl+D) finishes it
	Required bool
	Value    string
}

// grillModel is the bubbletea model of `aayu grill <type>` on a terminal.
// check runs the same validation as the flag path and returns its problems,
// each as "field: message"; the review screen shows them and offers to jump
// to the first one, or to create when there are none.
type grillModel struct {
	name     string
	steps    []grillStep
	i        int // current step; len(steps) is the review screen
	check    func(map[string]string) (preview string, problems []string)
	preview  string
	problems []string
	create   bool
	done     bool
}

func (m *grillModel) values() map[string]string {
	out := map[string]string{}
	for _, s := range m.steps {
		out[s.ID] = strings.TrimSpace(s.Value)
	}
	return out
}

func (m *grillModel) Init() tea.Cmd { return nil }

func (m *grillModel) review() {
	m.i = len(m.steps)
	m.preview, m.problems = m.check(m.values())
}

func (m *grillModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.PasteMsg:
		if m.i < len(m.steps) {
			m.steps[m.i].Value += msg.Content // pasted material is data: it only becomes the answer's text
		}
		return m, nil
	case tea.KeyPressMsg:
		k := msg.String()
		if k == "ctrl+c" || k == "esc" {
			m.done = true
			return m, tea.Quit
		}
		if m.i == len(m.steps) {
			return m.updateReview(k)
		}
		s := &m.steps[m.i]
		switch k {
		case "enter":
			if s.Multi && !strings.HasSuffix(s.Value, "\n") && s.Value != "" {
				s.Value += "\n"
				return m, nil
			}
			m.next()
		case "ctrl+d", "tab":
			m.next()
		case "shift+tab":
			if m.i > 0 {
				m.i--
			}
		case "backspace":
			if r := []rune(s.Value); len(r) > 0 {
				s.Value = string(r[:len(r)-1])
			}
		default:
			if t := msg.Key().Text; t != "" {
				s.Value += t
			}
		}
	}
	return m, nil
}

func (m *grillModel) next() {
	m.steps[m.i].Value = strings.TrimRight(m.steps[m.i].Value, "\n")
	if m.i+1 < len(m.steps) {
		m.i++
		return
	}
	m.review()
}

func (m *grillModel) updateReview(k string) (tea.Model, tea.Cmd) {
	switch k {
	case "c", "enter":
		if len(m.problems) == 0 {
			m.create, m.done = true, true
			return m, tea.Quit
		}
	case "e":
		m.i = 0
		if len(m.problems) > 0 {
			field, _, _ := strings.Cut(m.problems[0], ":")
			for j, s := range m.steps {
				if s.ID == field || (s.ID == "labels" && isLabelField(field)) {
					m.i = j
					break
				}
			}
		}
	case "shift+tab", "up":
		m.i = len(m.steps) - 1
	case "q":
		m.done = true
		return m, tea.Quit
	}
	return m, nil
}

func isLabelField(f string) bool {
	for _, lf := range workitems.LabelFields {
		if lf.Prefix == f {
			return true
		}
	}
	return false
}

func (m *grillModel) View() tea.View {
	var b strings.Builder
	if m.i == len(m.steps) {
		fmt.Fprintf(&b, "%s: review\n\n%s\n\n", m.name, strings.TrimSpace(m.preview))
		if len(m.problems) == 0 {
			b.WriteString("The draft passes validation.\n\n[c] create  [shift+tab] back  [q] quit without writing\n")
		} else {
			fmt.Fprintf(&b, "%d problem(s):\n", len(m.problems))
			for _, p := range m.problems {
				fmt.Fprintf(&b, "  - %s\n", p)
			}
			b.WriteString("\n[e] edit the first problem  [shift+tab] back  [q] quit without writing\n")
		}
		return tea.NewView(b.String())
	}
	s := m.steps[m.i]
	req := "optional"
	if s.Required {
		req = "required"
	}
	fmt.Fprintf(&b, "%s: question %d of %d\n\n%s (%s)\n", m.name, m.i+1, len(m.steps), s.Label, req)
	for _, h := range s.Hints {
		fmt.Fprintf(&b, "  %s\n", h)
	}
	fmt.Fprintf(&b, "\n> %s█\n\n", strings.ReplaceAll(s.Value, "\n", "\n  "))
	if s.Multi {
		b.WriteString("[enter] new line, twice to finish  [ctrl+d] next  [shift+tab] back  [esc] quit\n")
	} else {
		b.WriteString("[enter] next  [shift+tab] back  [esc] quit\n")
	}
	return tea.NewView(b.String())
}

// workItemSteps are the interview's questions for a work-item type: the
// title, every body field of the type in schema order with the reference's
// questions as hints, the parent when the type has one, and the labels.
func workItemSteps(t workitems.Type, assets fs.FS) []grillStep {
	hints := referenceHints(assets, t.ID)
	steps := []grillStep{{ID: "title", Label: "Title", Required: true, Hints: hints["Title"]}}
	for _, f := range t.Fields() {
		if f.RequiredAt == workitems.StageComplete {
			continue // Complete sections are written by ship, readout, measure, and decide
		}
		h := hints[f.Label]
		if len(h) == 0 && f.Description != "" {
			h = []string{f.Description}
		}
		steps = append(steps, grillStep{ID: f.ID, Label: f.Label, Hints: h, Required: f.RequiredAt != "",
			Multi: f.Kind != workitems.KindText && f.Kind != workitems.KindEnum})
	}
	if len(t.Parents) > 0 {
		steps = append(steps, grillStep{ID: "parent", Label: "Parent (" + strings.Join(t.Parents, " or ") + ", issue number)", Required: t.ParentAt != ""})
	}
	return append(steps, grillStep{ID: "labels", Label: "Labels (comma-separated, e.g. effort:S,priority:P2)", Required: t.Effort, Hints: hints["Labels"]})
}

// brandSteps are the questions of `grill brand`.
func brandSteps(assets fs.FS) []grillStep {
	hints := referenceHints(assets, "brand")
	return []grillStep{
		{ID: "name", Label: "Name", Required: true, Hints: hints["Name"]},
		{ID: "tone", Label: "Tone (comma-separated words)", Required: true, Hints: hints["Voice"]},
		{ID: "avoid", Label: "Avoid (comma-separated words)", Hints: hints["Voice"]},
		{ID: "palette", Label: "Palette (one per line: name #rrggbb)", Required: true, Multi: true, Hints: hints["Palette"]},
		{ID: "pairs", Label: "Pairs (one per line: fg on bg, text|large-text|ui)", Required: true, Multi: true, Hints: hints["Pairs"]},
	}
}

// referenceHints reads skills/grill/references/<name>.md: the bullet lines
// under each "### <Label> (…)" heading, keyed by the label.
func referenceHints(assets fs.FS, name string) map[string][]string {
	out := map[string][]string{}
	if assets == nil {
		return out
	}
	b, err := fs.ReadFile(assets, path.Join("skills", "grill", "references", name+".md"))
	if err != nil {
		return out
	}
	cur := ""
	for _, line := range strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
		if h, ok := strings.CutPrefix(line, "### "); ok {
			cur, _, _ = strings.Cut(h, " (")
			continue
		}
		if strings.HasPrefix(line, "## ") {
			cur = ""
			continue
		}
		if t := strings.TrimSpace(line); cur != "" && strings.HasPrefix(t, "- ") {
			out[cur] = append(out[cur], t)
		}
	}
	return out
}

// interactive reports whether grill may run its TUI: a human on a terminal,
// not an agent lane and not --json.
func (e *Env) interactive() bool {
	if e.Agent() || e.JSON {
		return false
	}
	if e.TUI != nil {
		return true
	}
	f, ok := e.In.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// runGrillTUI runs the interview on the terminal and returns the answers and
// whether the human chose to create (false when they quit).
func (e *Env) runGrillTUI(m *grillModel) (vals map[string]string, create bool, err error) {
	run := e.TUI
	if run == nil {
		run = func(m tea.Model) error {
			_, err := tea.NewProgram(m, tea.WithInput(e.In), tea.WithOutput(e.Out)).Run()
			return err
		}
	}
	if err := run(m); err != nil {
		return nil, false, err
	}
	return m.values(), m.create, nil
}
