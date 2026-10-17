# Store

The notes file format as a library: `Load`, `Save`, and `MarkDone` in `examples/notes/store`. A file saved and loaded
again keeps each note's text, order, and done mark; a missing file holds no notes.

```feature
{"id": "store", "layers": ["L07"],
 "sources": ["examples/notes/store/store.go", "examples/notes/store/store_test.go"],
 "surfaces": {"func": ["Load", "Save", "MarkDone"]},
 "verified_at": "c514f9a54b6f60eedc10f619292e093ce314007d"}
```

## Sub-features

- `round-trip`: save then load keeps every note; marking a note that does not exist is `ErrNoNote`.

## How to reach it

- Go code imports `github.com/rahulnakmol/agentic-ayudha/examples/notes/store`.

## Behaviour checks

```check
{"id": "round-trip", "kind": "lib", "evidence": "api-trace", "steps": [
  {"test": "TestRoundTrip", "package": "./examples/notes/store"}
]}
```

## Gotchas

- A line that is neither `[ ] text` nor `[x] text` makes `Load` fail rather than drop the note.
