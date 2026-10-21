# Mark done

A person marks a note done by the number `list` shows; `list` then shows it with an `x`. A number that is not on the
list fails with exit 1 and leaves the file as it was.

```feature
{"id": "mark-done", "layers": ["L02"],
 "sources": ["cmd/notes/main.go", "store/store.go"],
 "surfaces": {"command": ["done"]},
 "verified_at": "c514f9a54b6f60eedc10f619292e093ce314007d"}
```

## Sub-features

- `done`: a note marked done is listed with `[x]`.
- `missing`: an unknown number exits 1 with "no such note".

## How to reach it

- `notes done <n>`, with `n` from `notes list`.

## Behaviour checks

```check
{"id": "done", "kind": "cli", "evidence": "api-trace", "steps": [
  {"build": "./cmd/notes"},
  {"run": ["notes", "add", "buy milk"], "exit": 0},
  {"run": ["notes", "add", "call Ada"], "exit": 0},
  {"run": ["notes", "done", "1"], "exit": 0, "stdout": ["done 1: buy milk"]},
  {"run": ["notes", "list"], "exit": 0, "stdout": ["1 [x] buy milk", "2 [ ] call Ada"]}
]}
```

```check
{"id": "missing", "kind": "cli", "evidence": "api-trace", "steps": [
  {"build": "./cmd/notes"},
  {"run": ["notes", "add", "buy milk"], "exit": 0},
  {"run": ["notes", "done", "5"], "exit": 1, "stderr": ["no such note: 5 (there are 1)"]},
  {"run": ["notes", "list"], "exit": 0, "stdout": ["1 [ ] buy milk"]}
]}
```

## Gotchas

- Numbers count from 1, as `list` shows them.
