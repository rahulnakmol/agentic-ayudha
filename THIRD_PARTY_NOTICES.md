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
| [license-checker-rseidelsohn](https://github.com/RSeidelsohn/license-checker-rseidelsohn) | BSD-3-Clause | npm licence scan (once `package.json` exists) |
| [markdownlint-cli2](https://github.com/DavidAnson/markdownlint-cli2) | MIT | Markdown lint |
| [lychee](https://github.com/lycheeverse/lychee) | Apache-2.0 OR MIT | Link check |

## Policy

- A PR that adds a dependency adds its entry here in the same PR: name, version or range, licence, how it is
  used (linked, peer dependency, or shelled out), and the source URL.
- CI licence scanning gates new dependencies: `go-licenses` for the Go module and an npm licence scan for
  every `package.json` (the `security` job; allowed: Apache-2.0, BSD-2-Clause, BSD-3-Clause, ISC, MIT).
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
