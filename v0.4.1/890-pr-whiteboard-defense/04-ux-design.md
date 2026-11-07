# #890 UX design

The surfaces are the PR body a person or agent writes, the merge gate's inspect comment, `aayu verify` output, and a
planned `aayu explain` verb. No inbox card, web screen, or phone screen is in the sources for this Feature.

## Today on dev

- The PR template has six sections and no Defense (`.github/pull_request_template.md`).
- `aayu inspect` blocks an agent PR whose body misses a section (`prBodyFloor`, `internal/cli/inspect_app.go:578-`).
- `aayu verify --help` at [884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b) shows `--floor
  string  Run only this floor.` A new floor would be reachable as `aayu verify --floor contracts` once built
  (inferred).
- `aayu explain` does not exist: the binary prints its root help for it.

## Planned, not built: verify messages

These strings are the expected results in the Completion Goals of #1107 and #1108:

- `internal/mcp/schemas/ready.json: required field story removed (breaking). Declare a contract entry with its
  migration, or keep the field.` (#1107 criterion 1)
- `breaking change declared as risk 1` (#1107 criterion 2), `no contract changed` (criterion 3)
- `internal/mcp/schemas/ready.json: not generated from its contract; run go generate ./...` (#1107 criterion 4)
- `skipped: no contracts declared` (#1107 criterion 5)
- `internal/verify/coverage.go (L04) imports internal/cli (L02); only L02 may import L02 ([verify.floors.boundaries]
  inward_only)` (#1108 criterion 1)
- `skipped: boundaries are read from Go imports in v0.4.1` (criterion 2), `skipped: no boundary rules` (criterion 6)
- One fixtures finding per value: path, line, and kind (email, card, secret), value redacted (criterion 3)

## Planned, not built: the Defense section

From Decision #904, option 1, the five parts are:

1. How it works: one diagram and the interaction pattern.
2. Why X and not Y.
3. A malicious actor, citing threat-model rows by ID.
4. Data structures.
5. Where it fails.

Each part cites code that exists at the PR head. Below it, the DoD checklist has one row per item, each naming the
verify floor that checks it or the person who answers it (#890).

## Planned, not built: challenge and explain

- The cold challenge's unsupported answer becomes an inspect finding that goes through the fix loop (#904). Its
  wording is not in any source.
- `aayu explain <pr|path> "<question>"` asks the seat that owns the code. "I do not know" opens a Bug on the defense
  (#904). Flags, output, and whether it is human-only are not in any source.

## Agent-facing surface

An agent writes the Defense in the PR body, then reads inspect's disposition. AGENTS.md already tells agents to
follow `.github/pull_request_template.md`; that rule would cover the new section with no change.

## Accessibility

The sources state no accessibility requirement for this Feature.
