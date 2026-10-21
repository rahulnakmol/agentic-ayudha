// Command notes keeps a list of notes in a plain-text file.
//
//	notes [-file notes.txt] add <text>   add a note
//	notes [-file notes.txt] list         list the notes, numbered
//	notes [-json] list                   list them as a JSON array
//	notes [-file notes.txt] done <n>     mark note n done
//
// The file defaults to $NOTES_FILE, else notes.txt. Exit codes: 0 done, 1 the
// command failed (say, no such note), 2 usage.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"example.com/notes/store"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("notes", flag.ContinueOnError)
	fs.SetOutput(stderr)
	def := os.Getenv("NOTES_FILE")
	if def == "" {
		def = "notes.txt"
	}
	file := fs.String("file", def, "the notes file")
	asJSON := fs.Bool("json", false, "list the notes as a JSON array")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	usage := func(msg string) int {
		fmt.Fprintf(stderr, "notes: %s\nusage: notes [-file f] [-json] add <text> | list | done <n>\n", msg)
		return 2
	}
	if fs.NArg() == 0 {
		return usage("no command")
	}
	notes, err := store.Load(*file)
	if err != nil {
		fmt.Fprintln(stderr, "notes:", err)
		return 1
	}
	rest := fs.Args()[1:]
	switch cmd := fs.Arg(0); cmd {
	case "add":
		text := strings.TrimSpace(strings.Join(rest, " "))
		if text == "" {
			return usage("add needs the note's text")
		}
		notes = append(notes, store.Note{Text: text})
		if err := store.Save(*file, notes); err != nil {
			fmt.Fprintln(stderr, "notes:", err)
			return 1
		}
		fmt.Fprintf(stdout, "added %d: %s\n", len(notes), text)
	case "list":
		if *asJSON {
			type item struct {
				N    int    `json:"n"`
				Text string `json:"text"`
				Done bool   `json:"done"`
			}
			items := []item{}
			for i, n := range notes {
				items = append(items, item{i + 1, n.Text, n.Done})
			}
			b, _ := json.Marshal(items)
			fmt.Fprintln(stdout, string(b))
			return 0
		}
		if len(notes) == 0 {
			fmt.Fprintln(stdout, "no notes yet")
		}
		for i, n := range notes {
			mark := " "
			if n.Done {
				mark = "x"
			}
			fmt.Fprintf(stdout, "%d [%s] %s\n", i+1, mark, n.Text)
		}
	case "done":
		n, err := strconv.Atoi(strings.Join(rest, ""))
		if err != nil {
			return usage("done needs a note number")
		}
		if err := store.MarkDone(notes, n); err != nil {
			fmt.Fprintln(stderr, "notes:", err)
			return 1
		}
		if err := store.Save(*file, notes); err != nil {
			fmt.Fprintln(stderr, "notes:", err)
			return 1
		}
		fmt.Fprintf(stdout, "done %d: %s\n", n, notes[n-1].Text)
	default:
		return usage(fmt.Sprintf("unknown command %q", cmd))
	}
	return 0
}
