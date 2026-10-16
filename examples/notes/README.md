# notes

A small CLI that keeps short notes in a text file, the non-UI example product of WP4.5b (docs/BUILD-PLAN.md): its
feature map (`.agents/skills/verify-notes/`) proves a CLI's commands and a library's functions with behaviour checks,
the same way `benefits-qa` proves a web app.

```sh
go run ./examples/notes/cmd/notes add buy milk
go run ./examples/notes/cmd/notes list
go run ./examples/notes/cmd/notes done 1
```

`-file` or `NOTES_FILE` picks the file (default `notes.txt`). Exit codes: 0 done, 1 the command failed, 2 usage.
