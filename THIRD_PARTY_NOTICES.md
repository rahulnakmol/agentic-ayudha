# Third-party notices

`aayudha` is licensed under BSD-3-Clause ([LICENSE](LICENSE)). This file records the licence of every
third-party component the project includes, depends on, or shells out to.

## Included today

None. The Go module uses only the Go standard library. The Go standard library is distributed with the Go
toolchain under the BSD-3-Clause licence of The Go Authors (<https://go.dev/LICENSE>).

## Policy

- A PR that adds a dependency adds its entry here in the same PR: name, version or range, licence, how it is
  used (linked, peer dependency, or shelled out), and the source URL.
- CI licence scanning gates new dependencies: `go-licenses` for the Go module and an npm licence scan for
  `packages/design-system/` (n/a until WP0.4). `aayu audit` repeats the scan (n/a until WP7.6).
- Licences must be compatible with redistributing the binary under BSD-3-Clause. Anything else needs a
  recorded decision before it is added.
- Components that are shelled out to (run as a separate program) are not bundled; they are listed so users
  know what they are running.

## Planned, not yet included

Licences in this section are what the plan expects. Each is confirmed from the upstream licence file and the
CI scan when the component is actually added.

### Go modules (docs/BUILD-PLAN.md §2)

| Component | Use | Expected licence |
|---|---|---|
| `github.com/spf13/cobra` | CLI frame (WP1.1) | Apache-2.0 |
| `github.com/knadh/koanf` | Config layering (WP1.2) | MIT |
| `github.com/invopop/jsonschema` | Config schema from structs (WP1.2) | MIT |
| `github.com/google/go-github` | GitHub REST client (WP2.3) | BSD-3-Clause |
| `github.com/shurcooL/githubv4` | GitHub GraphQL, Projects v2 only (WP2.3) | MIT |
| `github.com/modelcontextprotocol/go-sdk` | MCP server (WP3.3) | To be confirmed |
| `github.com/zalando/go-keyring` | OS keychain secrets backend (WP1.3) | MIT |
| `filippo.io/age` | Encrypted secrets file, token broker (WP1.3, WP1.4) | BSD-3-Clause |
| `github.com/charmbracelet/bubbletea` | `grill` TUI (WP5.3) | MIT |
| `modernc.org/sqlite` | `measure` snapshots, pure Go (WP7.2) | BSD-3-Clause |
| `go.opentelemetry.io/otel` | Traces (WP1.8) | Apache-2.0 |
| `github.com/go-git/go-git` | Read-only git access | Apache-2.0 |
| `github.com/yuin/goldmark` | Markdown parsing | MIT |

### Design system (`packages/design-system/`, WP6.1–6.3)

| Component | Use | Expected licence |
|---|---|---|
| GSAP | Motion. Declared as a **peerDependency only, never vendored**; the product installs it. | GSAP's own licence (not OSI); confirm terms at inclusion |
| Phosphor Icons | Icon set swapped into the shadcn components | MIT |
| shadcn/ui | Component source copied into the package | MIT |
| OpenUI5 / SAP Horizon | **Inspiration only** for semantic token names and Fiori-shaped floorplans. No SAP assets, icons, or fonts are included. | OpenUI5 is Apache-2.0; nothing is copied |

### Shelled out (not bundled)

| Component | Use | Expected licence |
|---|---|---|
| impeccable | `review` lane B, `detect --json` (WP6.6) | Apache-2.0 |

Other optional tools that `doctor` reports (git-lfs, gh, node, Playwright, axe, semgrep, gitleaks,
osv-scanner, ZAP, k6, Lighthouse, Guidepup, DuckDB) are installed by the user and run as separate programs.
Each is added to this file with its licence when the verb that calls it ships.
