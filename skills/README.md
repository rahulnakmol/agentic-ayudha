# skills

One directory per verb that has the skill surface in the registry. Embedded into the binary by the root `embed.go`
(ground rule 3); `aayu install` (WP3.2) copies them into each host. Protected path: changes need a human CODEOWNER review.

## Format (WP3.1)

```text
skills/<verb>/
├── SKILL.md        # generated frontmatter + hand-authored body
├── references/     # optional: detail the body links to (question sets, checklists)
└── scripts/        # optional: thin wrappers that call only `aayu <command>`
```

- **Frontmatter** is generated from `internal/verbs/registry.go` by `go generate ./...`; never edit it by hand (CI
  fails on drift). Keys: `name`, `description` (the registry summary), `disable-model-invocation` (true for verbs
  never exposed to agents), `user-invocable`, `paths` (only when the registry sets globs), `version`, and
  `aayu: {seat, lane, layers, inputs, outputs, gates, identity, mode}`.
- **Body** is everything after the closing `---`. It tells the host's model what to do and which `aayu` calls to make
  (ground rule 5: inside a host, the host's model reasons). Keep it short; move detail into `references/`.
- **Hooks** a skill declares in frontmatter are command hooks that run `aayu hook <name>` and nothing else.
- **Images**: never Mobbin-origin pixels (WP6.4). Link to the Mobbin page or ID instead.

## Lint

`internal/skills.Lint` runs over the embedded tree in `go test` (`TestSkillLint` in `embed_test.go`, CI job `build`).
Each rule has a failing fixture in `internal/skills/lint_test.go`.

| Rule | Check |
|---|---|
| `frontmatter` | `SKILL.md` exists, opens with a closed `---` block, and has every generated key |
| `name` | one lowercase word, equal to the directory name |
| `description` | non-empty, at most 200 characters |
| `body-length` | at most 500 lines after the frontmatter |
| `aayu-call` | every `aayu` call in code spans, code blocks, and scripts names a registry command, a valid first enum value, and only flags the command or the CLI defines |
| `script` | files under `scripts/` hold only comments, blank lines, `set` options, and `aayu <command>` lines without pipes, redirects, chaining, or substitution |
| `hook` | hooks are command hooks whose command is exactly `aayu hook <name>` (optionally `--host <host>`) |
| `mobbin-image` | no image file whose name or bytes mention Mobbin, and no image embed whose URL points at Mobbin |
| `symlink` | no symlinks anywhere (V2: real copies only) |
