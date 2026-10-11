# Third-party notices

`aayudha` is licensed under BSD-3-Clause ([LICENSE](LICENSE)). This file records the licence of every
third-party component the project includes, depends on, or shells out to.

## Included today

The Go standard library is distributed with the Go toolchain under the BSD-3-Clause licence of The Go Authors
(<https://go.dev/LICENSE>). These Go modules are linked into the binary. Direct dependencies are the ones the
plan lists in §2; the rest are pulled in by them.

| Component | Version | Licence | Use | Source |
|---|---|---|---|---|
| `github.com/spf13/cobra` | v1.10.2 | Apache-2.0 | CLI frame (WP1.1), direct | <https://github.com/spf13/cobra> |
| `github.com/spf13/pflag` | v1.0.9 | BSD-3-Clause | Flag parsing, via cobra | <https://github.com/spf13/pflag> |
| `github.com/inconshreveable/mousetrap` | v1.1.0 | Apache-2.0 | Windows console detection, via cobra | <https://github.com/inconshreveable/mousetrap> |
| `github.com/knadh/koanf/v2` | v2.3.8 | MIT | Config layering (WP1.2), direct | <https://github.com/knadh/koanf> |
| `github.com/knadh/koanf/maps` | v0.1.2 | MIT | Via koanf | <https://github.com/knadh/koanf> |
| `github.com/knadh/koanf/parsers/toml/v2` | v2.2.2 | MIT | TOML config files (WP1.2), direct | <https://github.com/knadh/koanf> |
| `github.com/knadh/koanf/parsers/json` | v1.0.1 | MIT | JSON config files (WP1.2), direct | <https://github.com/knadh/koanf> |
| `github.com/knadh/koanf/providers/confmap` | v1.0.1 | MIT | Env and `--set` layers (WP1.2), direct | <https://github.com/knadh/koanf> |
| `github.com/go-viper/mapstructure/v2` | v2.4.0 | MIT | Strict decoding into config structs (with koanf) | <https://github.com/go-viper/mapstructure> |
| `github.com/mitchellh/copystructure` | v1.2.0 | MIT | Via koanf | <https://github.com/mitchellh/copystructure> |
| `github.com/mitchellh/reflectwalk` | v1.0.2 | MIT | Via copystructure | <https://github.com/mitchellh/reflectwalk> |
| `github.com/pelletier/go-toml/v2` | v2.4.3 | MIT | TOML parsing (with koanf) and `config show` output | <https://github.com/pelletier/go-toml> |
| `github.com/invopop/jsonschema` | v0.14.0 | MIT | Config JSON Schema from structs (WP1.2), direct | <https://github.com/invopop/jsonschema> |
| `github.com/pb33f/ordered-map/v2` | v2.3.1 | Apache-2.0 | Via jsonschema | <https://github.com/pb33f/ordered-map> |
| `github.com/bahlo/generic-list-go` | v0.2.0 | BSD-3-Clause | Via ordered-map | <https://github.com/bahlo/generic-list-go> |
| `github.com/buger/jsonparser` | v1.1.2 | MIT | Via ordered-map | <https://github.com/buger/jsonparser> |
| `go.yaml.in/yaml/v4` | v4.0.0-rc.2 | MIT and Apache-2.0 | Via ordered-map | <https://github.com/yaml/go-yaml> |
| `github.com/zalando/go-keyring` | v0.2.8 | MIT | OS keychain secrets backend (WP1.3), direct | <https://github.com/zalando/go-keyring> |
| `github.com/godbus/dbus/v5` | v5.2.2 | BSD-2-Clause | Secret Service on Linux, via go-keyring | <https://github.com/godbus/dbus> |
| `github.com/danieljoos/wincred` | v1.2.3 | MIT | Windows Credential Manager, via go-keyring | <https://github.com/danieljoos/wincred> |
| `filippo.io/age` | v1.3.2 | BSD-3-Clause | Encrypted secrets file (WP1.3) and token broker hand-over (WP1.4), direct | <https://github.com/FiloSottile/age> |
| `filippo.io/hpke` | v0.4.0 | BSD-3-Clause | Via age | <https://github.com/FiloSottile/hpke> |
| `golang.org/x/crypto` | v0.55.0 | BSD-3-Clause | Via age | <https://go.googlesource.com/crypto> |
| `golang.org/x/sys` | v0.47.0 | BSD-3-Clause | Via go-keyring and age | <https://go.googlesource.com/sys> |

### Shelled out by `self-update` (not bundled)

| Component | Use | Licence |
|---|---|---|
| cosign | Verifies the keyless signature bundle over `checksums.txt` (WP1.1, Appendix B D-1.1a) | Apache-2.0 |
| GitHub CLI (`gh`) | `gh attestation verify` checks the SLSA build provenance (WP1.1) | MIT |

### Run by `install.sh` (not bundled)

| Component | Version | Licence | Use |
|---|---|---|---|
| [cosign](https://github.com/sigstore/cosign) | v3.1.3 | Apache-2.0 | Verifies the release signature. Used from `PATH`; if missing, `install.sh` downloads this pinned version and checks its SHA-256 first. |

### CI and release tooling (not distributed)

These run only in this repository's GitHub Actions workflows. Versions are pinned in the workflows.

| Component | Licence | Use |
|---|---|---|
| [GoReleaser](https://github.com/goreleaser/goreleaser) | MIT | Builds, archives, and checksums releases and snapshots |
| [cosign](https://github.com/sigstore/cosign) | Apache-2.0 | Keyless signing of `checksums.txt` |
| [Syft](https://github.com/anchore/syft) | Apache-2.0 | SBOM per archive |
| [actions/attest-build-provenance](https://github.com/actions/attest-build-provenance) | MIT | SLSA build provenance |
| [golangci-lint](https://github.com/golangci/golangci-lint) | GPL-3.0 | Go lint (run as a tool, never linked) |
| [gitleaks](https://github.com/gitleaks/gitleaks) | MIT | Secret scanning |
| [Semgrep](https://github.com/semgrep/semgrep) | LGPL-2.1 | SAST (run as a tool, never linked) |
| [OSV-Scanner](https://github.com/google/osv-scanner) | Apache-2.0 | Dependency vulnerabilities |
| [go-licenses](https://github.com/google/go-licenses) | Apache-2.0 | Go licence scan |
| [pnpm](https://github.com/pnpm/pnpm) | MIT | Installs the Node workspace and runs its licence scan (`pnpm licenses list`) |
| [markdownlint-cli2](https://github.com/DavidAnson/markdownlint-cli2) | MIT | Markdown lint |
| [lychee](https://github.com/lycheeverse/lychee) | Apache-2.0 OR MIT | Link check |

### Design system (`packages/design-system/`, WP6.1–6.3)

The package is private (not published) and ships as source; versions are pinned in `pnpm-lock.yaml`.
`pnpm licenses list --prod` in `packages/design-system/` reports only MIT, Apache-2.0, and 0BSD for its runtime
dependency tree (80 MIT packages, mostly Radix UI and Floating UI; `class-variance-authority` Apache-2.0;
`tslib` 0BSD).

| Component | Version | Licence | How it is used | Source |
|---|---|---|---|---|
| GSAP | peer `^3.13.0` (3.15.0 for local development) | GSAP Standard "No Charge" License (not OSI) | **peerDependency only, never vendored or bundled**: the consuming app installs it. Used by `@aayudha/design-system/motion`. The licence prohibits using GSAP in visual, no-code animation builders that compete with Webflow, so the Storybook motion controls only preview existing presets and tokens and are documented as not a visual builder. `test/package-policy.test.ts` fails if GSAP becomes a dependency or its source appears in the package (J3-19) | <https://gsap.com/standard-license> |
| Phosphor Icons (`@phosphor-icons/react`) | 2.1.10 | MIT | Dependency. The icon set swapped into the shadcn/ui components in place of Lucide | <https://github.com/phosphor-icons/react> |
| shadcn/ui | component source as of CLI 4.21.4 | MIT | Component source copied into `src/components/ui/` and modified (Phosphor icons, density tokens, status badge variants). Not a package dependency | <https://github.com/shadcn-ui/ui> |
| OpenUI5 / SAP Horizon | n/a | OpenUI5 is Apache-2.0 | **Inspiration only** for semantic token naming (brand, shell, field, list, positive/negative/critical/informative) and the Fiori-shaped floorplans. Names, structure, and values are Aayudha's own; no SAP theme files, assets, icons, or fonts are included | <https://github.com/SAP/openui5> |
| Radix UI (`radix-ui`) | 1.7.0 | MIT | Dependency. Accessible primitives under the shadcn/ui components | <https://github.com/radix-ui/primitives> |
| `class-variance-authority` | 0.7.1 | Apache-2.0 | Dependency. Component variants | <https://github.com/joe-bell/cva> |
| `clsx` | 2.1.1 | MIT | Dependency. Class name joining | <https://github.com/lukeed/clsx> |
| `tailwind-merge` | 3.7.0 | MIT | Dependency. Class conflict resolution | <https://github.com/dcastil/tailwind-merge> |
| `tw-animate-css` | 1.4.0 | MIT | Dependency. CSS enter/exit animations used by the shadcn/ui dialog and select | <https://github.com/Wombosvideo/tw-animate-css> |
| Tailwind CSS | peer `^4.0.0` (4.3.3 for local development) | MIT | peerDependency: the consuming app compiles `styles.css` | <https://github.com/tailwindlabs/tailwindcss> |
| React, React DOM | peer `^19.0.0` | MIT | peerDependency | <https://github.com/facebook/react> |

Development only (never shipped; the static Storybook build is a CI check, not a release): Storybook and its
addons (MIT), Vite (MIT), Vitest (MIT), Playwright (Apache-2.0), TypeScript (Apache-2.0), and axe-core
(MPL-2.0, through `@storybook/addon-a11y`). The development tree also contains BSD-2-Clause, BSD-3-Clause,
ISC, BlueOak-1.0.0, and CC-BY-4.0 (browser-support data) packages.

## Policy

- A PR that adds a dependency adds its entry here in the same PR: name, version or range, licence, how it is
  used (linked, peer dependency, or shelled out), and the source URL.
- CI licence scanning gates new dependencies: `go-licenses` for the Go module and `pnpm licenses list --prod`
  for the pnpm workspace (the `security` job; allowed: 0BSD, Apache-2.0, BSD-2-Clause, BSD-3-Clause, ISC, MIT;
  0BSD per docs/BUILD-PLAN.md Appendix B D-6.1a).
  `aayu audit` repeats the scan (n/a until WP7.6).
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
| `github.com/google/go-github` | GitHub REST client (WP2.3) | BSD-3-Clause |
| `github.com/shurcooL/githubv4` | GitHub GraphQL, Projects v2 only (WP2.3) | MIT |
| `github.com/modelcontextprotocol/go-sdk` | MCP server (WP3.3) | To be confirmed |
| `github.com/charmbracelet/bubbletea` | `grill` TUI (WP5.3) | MIT |
| `modernc.org/sqlite` | `measure` snapshots, pure Go (WP7.2) | BSD-3-Clause |
| `go.opentelemetry.io/otel` | Traces (WP1.8) | Apache-2.0 |
| `github.com/go-git/go-git` | Read-only git access | Apache-2.0 |
| `github.com/yuin/goldmark` | Markdown parsing | MIT |

### Shelled out (not bundled)

| Component | Use | Expected licence |
|---|---|---|
| impeccable | `review` lane B, `detect --json` (WP6.6) | Apache-2.0 |

Other optional tools that `doctor` reports (git-lfs, gh, node, Playwright, axe, semgrep, gitleaks,
osv-scanner, ZAP, k6, Lighthouse, Guidepup, DuckDB) are installed by the user and run as separate programs.
Each is added to this file with its licence when the verb that calls it ships.
