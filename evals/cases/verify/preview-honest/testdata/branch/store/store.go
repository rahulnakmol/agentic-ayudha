// Package store keeps the notes of the notes CLI:
// a plain-text file, one note per line, "[ ] text" or "[x] text".
package store

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

// Note is one note.
type Note struct {
	Text string
	Done bool
}

// ErrNoNote is returned for a note number that does not exist.
var ErrNoNote = errors.New("no such note")

// Load reads the notes in path; a missing file has none.
func Load(path string) ([]Note, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var out []Note
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "[ ] "):
			out = append(out, Note{Text: line[4:]})
		case strings.HasPrefix(line, "[x] "):
			out = append(out, Note{Text: line[4:], Done: true})
		case line != "":
			return nil, fmt.Errorf("%s: not a note: %q", path, line)
		}
	}
	return out, sc.Err()
}

// Save writes the notes to path.
func Save(path string, notes []Note) error {
	var b strings.Builder
	for _, n := range notes {
		mark := "[ ] "
		if n.Done {
			mark = "[x] "
		}
		b.WriteString(mark + n.Text + "\n")
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// MarkDone marks note n (counted from 1) done.
func MarkDone(notes []Note, n int) error { return mark(notes, n, true) }

// MarkUndone marks note n (counted from 1) not done.
func MarkUndone(notes []Note, n int) error { return mark(notes, n, false) }

func mark(notes []Note, n int, done bool) error {
	if n < 1 || n > len(notes) {
		return fmt.Errorf("%w: %d", ErrNoNote, n)
	}
	notes[n-1].Done = done
	return nil
}
