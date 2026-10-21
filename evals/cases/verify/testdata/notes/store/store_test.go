package store

import (
	"errors"
	"path/filepath"
	"testing"
)

// TestRoundTrip: notes saved and loaded again keep their text, order, and
// done mark; marking a note that does not exist is ErrNoNote.
func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.txt")
	if got, err := Load(path); err != nil || got != nil {
		t.Fatalf("missing file: %v %v", got, err)
	}
	notes := []Note{{Text: "buy milk"}, {Text: "call Ada"}}
	if err := MarkDone(notes, 2); err != nil {
		t.Fatal(err)
	}
	if err := MarkDone(notes, 3); !errors.Is(err, ErrNoNote) {
		t.Fatalf("note 3 of 2: %v", err)
	}
	if err := Save(path, notes); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil || len(got) != 2 || got[0] != (Note{Text: "buy milk"}) || got[1] != (Note{Text: "call Ada", Done: true}) {
		t.Fatalf("loaded %+v %v", got, err)
	}
}
