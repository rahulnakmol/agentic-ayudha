# Add and list

A person adds notes and lists them, numbered in the order they were added. A missing command or an empty note is a
usage error (exit 2) with a message that says what is wrong.

```feature
{"id": "add-and-list", "layers": ["L02"],
 "sources": ["examples/notes/cmd/notes/main.go", "examples/notes/store/store.go"],
 "surfaces": {"command": ["add", "list"], "flag": ["file"]},
 "verified_at": "45a18e746769649c356d4dc95b92be3bf83e473d"}
```

## Sub-features

- `add-then-list`: two notes are added and listed in order, not done.
- `empty`: listing with no notes says so.
- `usage`: no command, an unknown command, or `add` with no text, exits 2.
- `file`: `-file` and `NOTES_FILE` pick another file; the default `notes.txt` is left alone.

## How to reach it

- `notes add <text>`, `notes list`; `-file` or `NOTES_FILE` picks the file.

## Behaviour checks

```check
{"id": "add-then-list", "kind": "cli", "evidence": "api-trace", "steps": [
  {"build": "./examples/notes/cmd/notes"},
  {"run": ["notes", "add", "buy", "milk"], "exit": 0, "stdout": ["added 1: buy milk"]},
  {"run": ["notes", "add", "call Ada"], "exit": 0, "stdout": ["added 2: call Ada"]},
  {"run": ["notes", "list"], "exit": 0, "stdout": ["1 [ ] buy milk", "2 [ ] call Ada"]}
]}
```

```check
{"id": "empty", "kind": "cli", "evidence": "api-trace", "steps": [
  {"build": "./examples/notes/cmd/notes"},
  {"run": ["notes", "list"], "exit": 0, "stdout": ["no notes yet"]}
]}
```

```check
{"id": "usage", "kind": "cli", "evidence": "api-trace", "steps": [
  {"build": "./examples/notes/cmd/notes"},
  {"run": ["notes"], "exit": 2, "stderr": ["no command"]},
  {"run": ["notes", "add"], "exit": 2, "stderr": ["add needs the note's text"]},
  {"run": ["notes", "remove", "1"], "exit": 2, "stderr": ["unknown command \"remove\""]},
  {"run": ["notes", "-file"], "exit": 2}
]}
```

```check
{"id": "file", "kind": "cli", "evidence": "api-trace", "steps": [
  {"build": "./examples/notes/cmd/notes"},
  {"run": ["notes", "-file", "work.txt", "add", "ship it"], "exit": 0, "stdout": ["added 1: ship it"]},
  {"run": ["notes", "list"], "exit": 0, "stdout": ["no notes yet"]},
  {"run": ["notes", "list"], "env": {"NOTES_FILE": "work.txt"}, "exit": 0, "stdout": ["1 [ ] ship it"]},
  {"run": ["notes", "-file", "work.txt", "list"], "exit": 0, "stdout": ["1 [ ] ship it"]}
]}
```

## Gotchas

- The words after `add` are joined with one space, so `add buy milk` and `add "buy milk"` add the same note.
