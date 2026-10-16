# Add and list

A person adds notes and lists them, numbered in the order they were added. A missing command or an empty note is a
usage error (exit 2) with a message that says what is wrong.

```feature
{"id": "add-and-list", "layers": ["L02"],
 "sources": ["examples/notes/cmd/notes/main.go", "examples/notes/store/store.go"],
 "surfaces": {"command": ["add", "list"], "flag": ["file"]},
 "verified_at": "c514f9a54b6f60eedc10f619292e093ce314007d"}
```

## Sub-features

- `add-then-list`: two notes are added and listed in order, not done.
- `empty`: listing with no notes says so.
- `usage`: no command, or `add` with no text, exits 2.

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
  {"run": ["notes", "-file"], "exit": 2}
]}
```

## Gotchas

- The words after `add` are joined with one space, so `add buy milk` and `add "buy milk"` add the same note.
