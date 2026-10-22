# notes feature map

What a person, or a script, can do with the `notes` CLI, and the behaviour check that proves each capability.
There is no browser and no server: checks run the CLI, built from the checkout, or the
product's own tests. `aayu verify --map` keeps the map honest against the source; `aayu verify --check all` runs
every check.

```map
{"product": "notes",
 "control": ["go", "run", "./.agents/skills/verify-notes/control"],
 "surfaces": [{"kind": "command", "files": ["cmd/notes/main.go"], "pattern": "case \"([a-z]+)\":"},
              {"kind": "flag", "files": ["cmd/notes/main.go"], "pattern": "fs\\.\\w+\\(\"([a-z-]+)\""},
              {"kind": "func", "files": ["store/store.go"], "pattern": "(?m)^func ([A-Z]\\w*)\\("}]}
```

## Baseline preconditions

- Each check runs in a fresh working directory, so its `notes.txt` starts empty and no state carries over.
- Each CLI check first builds `cmd/notes` from the checkout (`build`); a checkout that does not build
  gives no result, and the floor is blocked.

## Conventions

- Assert what a person reads (stdout), what a script relies on (the exit code), and the message on stderr.
- A library capability is checked through its own test, with the product's test framework (`go test`).

## Features

1. [Add and list](add-and-list.md): add notes and list them, numbered; usage errors exit 2.
2. [Mark done](mark-done.md): mark a note done by its number; an unknown number exits 1.
3. [Store](store.md): the notes file format, as a library.
