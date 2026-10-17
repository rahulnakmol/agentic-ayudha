# Appendix B rows

Appendix B of the [build plan](../BUILD-PLAN.md) records every
deviation and decision made while implementing it. Each row is its own file here, so two branches that each add a row
add two files and merge without a conflict. `internal/appendixb` reads them; `go generate ./...` checks them.

- A directory per section, `<NNN>-<slug>/`, whose `README.md` holds the section's title (`# …`), its intro, and its
  table header (`| # | … |` and the `|---|` row).
- A file per row, `<NNN>-<ID>.md`: `# <ID>`, then one `## <column>` heading per column after `#`, each followed by
  that cell. The numbers order sections and rows.

Print the whole appendix as tables with `go run ./internal/appendixb/gen -print`. Find a row by its ID with
`ls docs/appendix-b/*/*-D-5.5c.12.md`.

## Add a row

1. Pick the section: your work package's directory, or a new `<NNN>-<slug>/` with the next free number and a
   `README.md` copied from a sibling (a new title and intro, the same header).
2. Pick an ID that no other row uses (`ls docs/appendix-b/*/*-<ID>.md` finds nothing), named after the work package
   (`D-5.9l`, `S-5.9b`).
3. Add `<NNN>-<ID>.md` with the next free number in the section. Copy a sibling row's headings and write one cell
   under each. A cell is one paragraph; write a literal `|` as `\|`. Write links relative to this file, so they work
   here and the docs job checks them.
4. Run `go generate ./...`. It fails on a duplicate ID, a wrong heading, or a bare `|`, and names the file.

Never add a row to `docs/BUILD-PLAN.md`: `go generate ./...` moves one written there into its own file and says so,
and the `generate-drift` check fails until that file is committed. When a rebase conflicts in Appendix B, keep both
sides, run `go generate ./...`, and commit what it moved.
