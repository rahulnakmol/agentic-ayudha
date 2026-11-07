# #673 Technical design

No tech baseline exists (`specs/673` is absent from the ledger). The design is one Markdown page and one sidebar
entry. It is planned, not built.

## What exists on dev that this builds on

All paths are at `dev` [884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b).

| Part | Where | What it does now |
|---|---|---|
| Sidebar | `sitePages` in `internal/tell/site.go`, for example the "Loops" entry (`:75`) and "PostHog and Sentry" (`:78`) | Each entry has `Out`, `Summary`, `Nav`, `Group`, and `Source`. The site also writes a Markdown twin of each page and `llms.txt` (`docs/guide/AGENTS.md`, "The site"). |
| A near model page | `docs/guide/guide-integrations.md` | A guide for running another tool beside aayu, read-only: "setup, config, key scopes, what each verb reads, and what aayu never does" (its `Summary` in `site.go:78`). |
| Writing rules | `docs/guide/AGENTS.md` | Page shapes, callouts, and checks for `docs/guide/`. |
| Protected paths | `.github/CODEOWNERS` lists `/.mcp.json`, `/.claude/`, `/.cursor/`, `/.codex/` | The files `xerj init` would write that a person owns. |
| Site build | `aayu tell docs --site <dir>` | Builds the site; it refuses a claim it cannot trace (`docs/guide/AGENTS.md`). |

Not on `dev`: `docs/guide/guide-xerj.md`, and any mention of XERJ outside `docs/releases/v0.3.0.md:462`.

## Layers touched

From `LAYERS.md`: L03 (`internal/tell/**`). `docs/guide/` matches no layer glob.

## Planned, not built

1. Write `docs/guide/guide-xerj.md` in the `guide-*.md` shape, covering FR1 to FR7 of [02-spec.md](02-spec.md).
2. Add one `sitePages` entry in `internal/tell/site.go` with `Group: "Guides"`.

No config key, verb, MCP passthrough, doctor check, floor, or hook (#673 Out of scope).

## Data and ledger records

None.

## Key decisions

No Decision issue is linked from #673. The founder deferred the pre-tool hook guard on 2026-10-18 ("Wait"), as #673
records.
