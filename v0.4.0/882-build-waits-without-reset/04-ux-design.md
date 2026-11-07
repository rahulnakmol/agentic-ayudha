# UX design: a build waits on a person without a reset

This Feature adds no command or flag (#882 Scope). It changes what `aayu build` reports while a build waits.

## The agent today (884d81b)

- `aayu build <n> --stop "<reason>"`: "Stop this build for good: move the story to status:blocked with this reason
  and page its owner; the lease stays until it expires." (`aayu build --help`).
- The output on a block: `#<n>: status:blocked: <why> (page <sent>)` then `next: a human looks at #<n>; aayu ready
  <n> lets the build continue with a fresh budget` (`internal/cli/build.go:1198-1201`).
- The budget line: `<wall> of <max> h, <spend>, <attempts> of <max> failed attempts` (`Usage.String`,
  `internal/cli/build.go:93`).

## Planned

- While a build waits on an answer, its state says `waiting` or `paused`, and the budget line does not count the
  wait (#882 Scope). The words on screen are not specified.
- After a clarifying answer, the next `aayu build <n>` goes on with the same budget (#882 Scope).
- No person-facing screen is named. The inbox ask card belongs to #880.
