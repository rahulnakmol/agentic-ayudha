# The feature-map contract (WP4.5b)

`aayu verify --map` checks every part of this contract; the merge gate runs the checks. Two worked examples are in
this repository: a web app (`examples/benefits-qa/.agents/skills/verify-benefits-qa/`, Node and Playwright) and a CLI
with a library (`examples/notes/.agents/skills/verify-notes/`, Go).

## Layout

```text
.agents/skills/verify-<product>/     # anywhere in the repository; a monorepo has one per product
├── SKILL.md                         # how to launch, drive, and read the product's checks
├── <control tool>                   # control.mjs, control/main.go, control.sh, …: the product's own stack
└── features/
    ├── README.md                    # one ```map block and the ordered index
    └── <id>.md                      # one capability each
```

## `features/README.md`

One `map` block of JSON, then the index: a numbered list of `[Title](<id>.md)` links in the order a reader meets the
product.

```json
{"product": "notes",
 "control": ["go", "run", "./examples/notes/.agents/skills/verify-notes/control"],
 "surfaces": [{"kind": "command", "files": ["examples/notes/cmd/notes/main.go"], "pattern": "case \"([a-z]+)\":"}]}
```

- `control` is the control tool's argv, run from the repository root.
- Each surface rule finds one kind of surface (a route, a page control, a command, a flag, an endpoint, an exported
  function) with a regular expression over the files its globs select (`exclude` removes some); the first group, or
  the whole match, is the surface. Every surface a rule finds must be listed by some feature, and every listed
  surface must still be found: a new command and a renamed route are both drift.

## `features/<id>.md`

An H1, one paragraph saying what a person or a script gets, one `feature` block, the four H2s `Sub-features`,
`How to reach it`, `Behaviour checks`, and `Gotchas` in that order, and one `check` block per check under
`Behaviour checks`.

```json
{"id": "mark-done", "layers": ["L02"], "sources": ["examples/notes/cmd/notes/main.go"],
 "surfaces": {"command": ["done"]}, "verified_at": "<commit>"}
```

- `id` is the file name. `sources` are globs; each must match a file. `verified_at` is the commit at which someone
  last ran the feature's checks and read the result; a later change to a source marks the feature stale (reported,
  not drift).

```json
{"id": "done", "kind": "cli", "evidence": "api-trace", "steps": [{"run": ["notes", "done", "1"], "exit": 0}]}
```

- `kind` is `ui`, `cli`, `api`, or `lib`. aayu launches the app with `[verify] seed` only for `ui` and `api`, never
  against production; `cli` and `lib` checks run with no network.
- `evidence` is the artifact kind a criterion mapped to the check gets: `screenshot` or `recording` for a page,
  `api-trace` for a command, a call, or a test.
- `steps` are in the control tool's vocabulary. Handles (the `name`, `label`, `within`, and `test` fields of a step's
  action) must appear in the feature's sources, so renaming a button or a test is drift.
- `requires` names a precondition the launch may not provide; the control tool answers `unreachable` without it.

## The control tool

```text
<control...> check
  stdin:  {"product", "feature", "check": {...}, "out": "<dir>", "base_url": "<url>", "root": "<repository>"}
  stdout: {"status": "passed"|"failed"|"unreachable", "reason": "...", "artifacts": [{"path": "...", "kind": "..."}]}
```

It writes its artifacts under `out` (paths relative to it), never reads the clock into them, and adds no runtime the
product does not already have: Node and Playwright for a web app, a shell or PTY driver for a CLI, plain HTTP for an
API, the product's test framework for a library. Malformed output, or a path outside `out`, blocks the floor.
`failed` fails it, which blocks the merge gate; `unreachable` proves nothing.
