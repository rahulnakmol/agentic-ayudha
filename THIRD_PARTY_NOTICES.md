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
| `go.yaml.in/yaml/v4` | v4.0.0-rc.2 | MIT and Apache-2.0 | Via ordered-map; tests also parse the generated issue forms with it (WP2.2) | <https://github.com/yaml/go-yaml> |
| `github.com/zalando/go-keyring` | v0.2.8 | MIT | OS keychain secrets backend (WP1.3), direct | <https://github.com/zalando/go-keyring> |
| `github.com/godbus/dbus/v5` | v5.2.2 | BSD-2-Clause | Secret Service on Linux, via go-keyring | <https://github.com/godbus/dbus> |
| `github.com/danieljoos/wincred` | v1.2.3 | MIT | Windows Credential Manager, via go-keyring | <https://github.com/danieljoos/wincred> |
| `filippo.io/age` | v1.3.2 | BSD-3-Clause | Encrypted secrets file (WP1.3) and token broker hand-over (WP1.4), direct | <https://github.com/FiloSottile/age> |
| `filippo.io/hpke` | v0.4.0 | BSD-3-Clause | Via age | <https://github.com/FiloSottile/hpke> |
| `golang.org/x/crypto` | v0.55.0 | BSD-3-Clause | Via age | <https://go.googlesource.com/crypto> |
| `golang.org/x/sys` | v0.47.0 | BSD-3-Clause | Via go-keyring and age | <https://go.googlesource.com/sys> |
| `go.opentelemetry.io/otel` | v1.46.0 | Apache-2.0 | Traces of verb runs (WP1.8), direct; with `otel/sdk` and `otel/trace` | <https://github.com/open-telemetry/opentelemetry-go> |
| `go.opentelemetry.io/otel/sdk` | v1.46.0 | Apache-2.0 | Tracer provider (WP1.8), direct | <https://github.com/open-telemetry/opentelemetry-go> |
| `go.opentelemetry.io/otel/trace` | v1.46.0 | Apache-2.0 | Trace API (WP1.8), direct | <https://github.com/open-telemetry/opentelemetry-go> |
| `go.opentelemetry.io/otel/metric` | v1.46.0 | Apache-2.0 | Via otel | <https://github.com/open-telemetry/opentelemetry-go> |
| `go.opentelemetry.io/auto/sdk` | v1.2.1 | Apache-2.0 | Via otel | <https://github.com/open-telemetry/opentelemetry-go-instrumentation> |
| `github.com/go-logr/logr` | v1.4.4 | Apache-2.0 | Via otel | <https://github.com/go-logr/logr> |
| `github.com/go-logr/stdr` | v1.2.2 | Apache-2.0 | Via otel | <https://github.com/go-logr/stdr> |
| `github.com/cespare/xxhash/v2` | v2.3.0 | MIT | Via otel/sdk | <https://github.com/cespare/xxhash> |
| `github.com/google/go-github/v90` | v90.0.0 | BSD-3-Clause | GitHub REST client (WP2.3), direct | <https://github.com/google/go-github> |
| `github.com/google/go-querystring` | v1.2.0 | BSD-3-Clause | Via go-github | <https://github.com/google/go-querystring> |
| `github.com/shurcooL/githubv4` | v0.0.0-20260209031235-2402fdf4a9ed | MIT | GitHub GraphQL client for Projects v2 (WP2.3), direct | <https://github.com/shurcooL/githubv4> |
| `github.com/shurcooL/graphql` | v0.0.0-20240915155400-7ee5256398cf | MIT | Via githubv4 | <https://github.com/shurcooL/graphql> |
| `github.com/google/uuid` | v1.6.0 | BSD-3-Clause | Via otel/sdk (resource IDs) | <https://github.com/google/uuid> |
| `github.com/modelcontextprotocol/go-sdk` | v1.8.0 | Apache-2.0 (new code) and MIT | MCP stdio server (WP3.3) and the streamable-HTTP MCP clients for Mobbin and the Figma opt-in (WP6.4), direct | <https://github.com/modelcontextprotocol/go-sdk> |
| `github.com/google/jsonschema-go` | v0.4.3 | MIT | JSON Schema types, via go-sdk | <https://github.com/google/jsonschema-go> |
| `github.com/segmentio/encoding` | v0.5.4 | MIT | JSON codec, via go-sdk | <https://github.com/segmentio/encoding> |
| `github.com/segmentio/asm` | v1.1.3 | MIT | Via segmentio/encoding | <https://github.com/segmentio/asm> |
| `github.com/yosida95/uritemplate/v3` | v3.0.2 | BSD-3-Clause | Resource URI templates, via go-sdk | <https://github.com/yosida95/uritemplate> |
| `golang.org/x/oauth2` | v0.36.0 | BSD-3-Clause | Via go-sdk (its OAuth handler, which aayudha does not use; V5) and go-github | <https://go.googlesource.com/oauth2> |
| `golang.org/x/sync` | v0.22.0 | BSD-3-Clause | Via go-sdk and modernc.org/libc | <https://go.googlesource.com/sync> |
| `golang.org/x/time` | v0.15.0 | BSD-3-Clause | Rate limiting, via go-sdk | <https://go.googlesource.com/time> |
| `modernc.org/sqlite` | v1.59.0 | BSD-3-Clause (SQLite itself is public domain) | Pure-Go SQLite for `measure` snapshots (WP7.2, R7), direct; v1.60 needs Go 1.26 | <https://gitlab.com/cznic/sqlite> |
| `github.com/yuin/goldmark` | v1.8.6 | MIT | Markdown drafts of `brief` parsed and rendered to HTML (WP7.3), direct | <https://github.com/yuin/goldmark> |
| `modernc.org/libc` | v1.75.7 | BSD-3-Clause (includes MIT-licensed musl code) | C runtime for the transpiled SQLite, via sqlite | <https://gitlab.com/cznic/libc> |
| `modernc.org/mathutil` | v1.7.1 | BSD-3-Clause | Via sqlite and libc | <https://gitlab.com/cznic/mathutil> |
| `modernc.org/memory` | v1.12.1 | BSD-3-Clause | Allocator, via libc | <https://gitlab.com/cznic/memory> |
| `github.com/dustin/go-humanize` | v1.0.1 | MIT | Via libc | <https://github.com/dustin/go-humanize> |
| `github.com/remyoudompheng/bigfft` | v0.0.0-20230129092748-24d4a6f8daec | BSD-3-Clause | Via mathutil | <https://github.com/remyoudompheng/bigfft> |
| `charm.land/bubbletea/v2` | v2.0.9 | MIT | `grill` TUI on a terminal (WP5.3), direct; v2.0.10 needs Go 1.26 | <https://github.com/charmbracelet/bubbletea> |
| `charm.land/lipgloss/v2` | v2.0.6 | MIT | Colour, themes, and tables on a person's terminal (WP1.1b), direct | <https://github.com/charmbracelet/lipgloss> |
| `github.com/charmbracelet/ultraviolet` | v0.0.0-20260811164956-006e29f97886 | MIT | Terminal input and rendering, via bubbletea and lipgloss | <https://github.com/charmbracelet/ultraviolet> |
| `github.com/charmbracelet/colorprofile` | v0.4.3 | MIT | Terminal colour detection, via bubbletea | <https://github.com/charmbracelet/colorprofile> |
| `github.com/charmbracelet/x/ansi` | v0.11.8 | MIT | ANSI sequences, via bubbletea and lipgloss | <https://github.com/charmbracelet/x> |
| `github.com/charmbracelet/x/term` | v0.2.2 | MIT | Terminal detection and width for the CLI experience (WP1.1b), direct; also via bubbletea | <https://github.com/charmbracelet/x> |
| `github.com/charmbracelet/x/termios` | v0.1.1 | MIT | Terminal modes, via bubbletea | <https://github.com/charmbracelet/x> |
| `github.com/charmbracelet/x/windows` | v0.2.2 | MIT | Windows console, via bubbletea | <https://github.com/charmbracelet/x> |
| `github.com/clipperhouse/displaywidth` | v0.11.0 | MIT | Text width, via ultraviolet | <https://github.com/clipperhouse/displaywidth> |
| `github.com/clipperhouse/uax29/v2` | v2.7.0 | MIT | Text segmentation, via displaywidth | <https://github.com/clipperhouse/uax29> |
| `github.com/lucasb-eyer/go-colorful` | v1.4.1 | MIT | Colour conversion, via bubbletea and lipgloss | <https://github.com/lucasb-eyer/go-colorful> |
| `github.com/mattn/go-runewidth` | v0.0.24 | MIT | Text width, via ultraviolet | <https://github.com/mattn/go-runewidth> |
| `github.com/muesli/cancelreader` | v0.2.2 | MIT | Cancellable input, via bubbletea | <https://github.com/muesli/cancelreader> |
| `github.com/rivo/uniseg` | v0.4.7 | MIT | Grapheme clusters, via ultraviolet | <https://github.com/rivo/uniseg> |
| `github.com/xo/terminfo` | v0.0.0-20220910002029-abceb7e1c41e | MIT | Terminal capabilities, via colorprofile | <https://github.com/xo/terminfo> |
| `github.com/mattn/go-isatty` | v0.0.24 | MIT | Terminal detection, via libc (found missing here by `aayu audit`'s licence scan, WP7.6) | <https://github.com/mattn/go-isatty> |
| `github.com/ncruces/go-strftime` | v1.0.0 | MIT | Time formatting, via libc (found missing here by `aayu audit`'s licence scan, WP7.6) | <https://github.com/ncruces/go-strftime> |

### Shelled out by `self-update` (not bundled)

| Component | Use | Licence |
|---|---|---|
| cosign | Verifies the keyless signature bundle over `checksums.txt` (WP1.1, Appendix B D-1.1a) | Apache-2.0 |
| GitHub CLI (`gh`) | `gh attestation verify` checks the SLSA build provenance (WP1.1) | MIT |

### Shelled out by `measure` when asked (not bundled)

| Component | Use | Licence |
|---|---|---|
| [DuckDB](https://github.com/duckdb/duckdb) CLI (`duckdb`) | Optional query engine over the SQLite snapshot (`measure --engine duckdb`, WP7.2, R7); used only when found on `PATH` | MIT |

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

### Eval fixtures (not distributed)

Four open-source codebases are kept, with their licence files, as fixtures for the `survey` skill's evals
(`evals/cases/survey/repos/`, WP5.9): two whole, two as the subsets named below. They are test data: never built,
linked, or embedded in the binary.

| Component | Version | Licence | Use | Source |
|---|---|---|---|---|
| tiny-js | commit 8214477 (2026-02-24) | MIT | C++ fixture for `survey/tinyjs-rust` and `survey/tinyjs-backlog`; line endings normalised to LF, `.gitignore` dropped | <https://github.com/gfwilliams/tiny-js> |
| Parsedown | commit 4e433a8 (2026-02-18) | MIT | PHP fixture for `survey/parsedown-modern-php`; `.github/`, `.gitignore`, and `.gitattributes` dropped | <https://github.com/erusev/parsedown> |
| MINPACK | commit 12f30bc (2022-10-06) | MIT (repository, Ondřej Čertík) and the Minpack Copyright Notice (1999) of the University of Chicago, a BSD-style licence with an acknowledgment clause (the MINPACK code), both in its `LICENSE` | Fortran fixture for `survey/minpack-traps`; a subset: `src/`, `examples/`, the seven `ex/` drivers and their data files `ex/file21`-`ex/file23`, the build files, `fpm.toml`, the readme, and the licence | <https://github.com/certik/minpack> |
| Maruku | commit 4d88d0e (2025-01-04) | MIT | Ruby fixture for `survey/maruku-compat` and `survey/maruku-backlog`; a subset: `lib/`, `bin/`, `data/entities.xml`, `spec/*.rb`, 20 of the 162 `spec/block_docs`, the readme, licence, gemspec, Gemfile, and Rakefile | <https://github.com/bhollis/maruku> |

The MINPACK fixture includes software developed by the University of Chicago, as Operator of Argonne National
Laboratory (the acknowledgment its licence asks for).

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

### Shelled out by `aayu verify`, the design verbs, and `aayu review` (not bundled)

The floors (docs/BUILD-PLAN.md WP4.3–WP4.6), the prototype capture (WP6.5), and review (WP6.6) run these as separate
programs in a scrubbed child process. Users install them; CI pins them (SHA-256 for binaries, a digest for the ZAP
image, `pnpm-lock.yaml` integrity hashes for the `@aayudha/floors` development package that holds Playwright,
axe-core, Lighthouse, and impeccable for this repository's own tests).

| Component | Version pinned in CI | Licence | Use | Source |
|---|---|---|---|---|
| semgrep | 1.180.0 | LGPL-2.1 | `sast` floor | <https://github.com/semgrep/semgrep> |
| gitleaks | 8.30.1 | MIT | `secrets` floor | <https://github.com/gitleaks/gitleaks> |
| osv-scanner | 2.6.0 | Apache-2.0 | `deps` floor | <https://github.com/google/osv-scanner> |
| Playwright | 1.64.0 | Apache-2.0 | `a11y` floors: drives Chromium through each Completion Goal state, and through the rendered `brief` page (WP7.3) | <https://github.com/microsoft/playwright> |
| axe-core | 4.14.0 | MPL-2.0 | `a11y` floor and the `brief` page check: injected into the page under test, unmodified | <https://github.com/dequelabs/axe-core> |
| Lighthouse | 13.5.0 | Apache-2.0 | `perf` floor | <https://github.com/GoogleChrome/lighthouse> |
| OWASP ZAP (`zaproxy/zap-stable` image) | 2.17.0, by digest | Apache-2.0 | `dast` floor | <https://github.com/zaproxy/zaproxy> |
| k6 | 2.3.0 | AGPL-3.0 | `api` floor; run as a program, never linked or distributed | <https://github.com/grafana/k6> |
| Guidepup (`@guidepup/guidepup`, `@guidepup/playwright`, `@guidepup/setup`) with `@playwright/test` | 0.35.0, 0.19.1, 0.29.2; 1.64.0 | MIT; Apache-2.0 | `screen-reader` floor: drives VoiceOver (macOS) and NVDA (Windows; NVDA itself is GPL-2.0, installed by `guidepup install` and run as a program) | <https://github.com/guidepup/guidepup> |
| Tesseract | user-installed, optional | Apache-2.0 | OCR in the redaction pass when present | <https://github.com/tesseract-ocr/tesseract> |
| impeccable (with its platform engine `@impeccable/cli-<os>-<arch>`) | 4.5.2 (engine 0.1.14) | Apache-2.0 | `review` lane B (WP6.6): `detect --json` against each state's URL, shelled out | <https://github.com/pbakaus/impeccable> |

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
| `modernc.org/sqlite` | `measure` snapshots, pure Go (WP7.2) | BSD-3-Clause |
| `github.com/go-git/go-git` | Read-only git access | Apache-2.0 |

Other optional tools that `doctor` reports (git-lfs, gh, node, DuckDB) are installed by the user and
run as separate programs.
Each is added to this file with its licence when the verb that calls it ships.
