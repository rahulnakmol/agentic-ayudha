# rahulnakmol/agentic-ayudha#480: Decision: proof floors by layer footprint (code quality, UX, API and CLI developer experience, DAST on by default) and a push-and-nightly security workflow

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, decision |
| Body hash | `4a45e2c2eb1eb57e682afb1526e24b5cb030665f7032dced796e18a46c100c34` |
| Completion Goal hash | n/a (no Completion Goal on this type) |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/480/ready.md` |

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Question

Which deterministic proof floors should `verify` run so that every change is checked for code quality, a change with a UI for user experience, a change to an API or CLI for developer experience, and every runnable app for DAST, each chosen by the story's layer footprint? And should product repositories get a security workflow that runs SAST, secrets, deps, and DAST on every push to dev, rolling, and main and nightly, not only on pull requests?

The founder asked for this on 2026-10-17: "in proof floors should talk about code quality and user experience if there is a UI, or developer experience if it's an API or CLI, and does Yama ensure SAST and DAST checks are done or does the verify skill do that". The founder later called the push-and-nightly security workflow essential. Thresholds are the Quality Architect's policy: they live in `internal/config/defaults.toml` and `CONSTRAINTS.md`, both protected paths (`.github/CODEOWNERS`). So this Decision comes before any code.

**Who makes sure SAST and DAST run today.** No seat checks by hand. Yama (Quality Architect) owns the thresholds (`CONSTRAINTS.md`, and `internal/verify/**`, which is layer L04 in `LAYERS.md`). Nakul (AI Native Engineer) is Accountable for the `verify` verb (`docs/RACI.md`). The merge gate workflow runs `aayudha verify <PR> --ci` on every non-draft PR, whichever agent built it (`internal/initrepo/aayudha-merge-gate.yml`, step "Verify the PR head"), and `aayudha proof comment` checks the pack. That runs SAST (semgrep, 0 critical and 0 high), secrets (gitleaks, 0) and deps (osv-scanner, 0 critical and 0 high) every time (`verify.Standard`, `internal/verify/scanners.go:401`). DAST runs only when a product sets `[verify] dast = true` (`internal/config/defaults.toml`, `DAST.Applies` in `internal/verify/dast.go`). With the default, nothing runs DAST. No installed workflow runs on push, so nothing scans the merged commit on dev, the promotion merges on rolling and main, direct or admin pushes, or new CVEs in dependencies that did not change.

**What is missing today, from the code.**

| Concern | Today | Source |
|---|---|---|
| Code quality | Diff coverage 90 % plus SAST. No lint floor, no complexity floor. | `internal/config/defaults.toml` `[verify.floors.coverage]`, `[verify.floors.sast]` |
| UX (L01) | axe WCAG 2.2 AA, keyboard-only, reduced motion, 200 % text, 44 px targets; screen reader on L01; mobile. Lighthouse runs only when a product sets budgets. There is no visual regression floor. With no web target, every web floor skips without blocking. | `Perf.Applies`, `webApplies` in `internal/verify/web.go` |
| API DX | k6 runs only when a product names a script. There is no contract floor and no default budget. | `API.Applies` in `internal/verify/dast.go` |
| CLI DX | Nothing. | none |
| DAST | Opt-in, `verify.dast = false`. | `internal/config/defaults.toml` |
| Security on push and nightly | No installed workflow. | `internal/initrepo/*.yml`: merge-gate, ops, ready-guard, streak |

### Options

1. **Floors chosen by footprint, with thresholds in protected config; new floors advisory for four weeks, then blocking; plus a push-and-nightly security workflow (recommended).** Every new threshold lives in `[verify.floors.*]`, so only a protected-path change moves it. Complexity comes from one pinned tool, not from the repository's linter configuration. Visual regression has its own, tighter tolerance. A breaking contract change needs a floor-waiver Decision. The details and defaults are in the Evaluation.
2. **The starting proposals as written, blocking on acceptance.** The complexity limit comes from each repository's linter configuration. Visual regression reuses `review.figma_diff_max` (2.0 %). Every new floor blocks from the day it merges.
3. **Only the security workflow and DAST by default now; the quality, UX, and DX floors after a pilot.** Pilot them on `examples/benefits-qa` first, then decide in a second Decision.

### Evaluation

**How a floor is chosen.** The footprint is the set of layers the changed paths fall in (`verify.ReadLayers(...).Footprint`, `internal/verify/footprint.go`). The screen-reader floor already works this way (`[verify.floors.screen_reader] layers`). In `LAYERS.md`, L01 is Experience (the UI), L02 is Interaction / API boundary (CLI and API handlers), and L06 is Contracts / integration. L02 holds both a CLI and an API, so `[verify] platforms` tells them apart. That key exists today (`internal/config/config.go`, `Verify.Platforms`; the mobile floor reads `ios` and `android`). This Decision adds the values `web`, `api` and `cli`. Each new floor has a `layers` key with the default below, the same pattern the screen-reader floor uses.

**"Skipped" has two meanings, as for mobile today.** A skip because the floor does not apply (for example, no L01 in the footprint) never blocks. A skip because the floor applies but cannot run blocks `ship` unless an accepted floor-waiver Decision names the floor (`aayu decide <story> --waiver <floor>`, `verify.ApplyWaivers`). For example: the product says it ships `web`, the story touches L01, and there is no target to capture. `proof.BlocksShip` gains these cases next to `skipped: mobile`.

**1. Code quality: every change.**

| Floor | Key and default | Applies to | Tool and pin | Blocking skip |
|---|---|---|---|---|
| `lint` | `[verify.floors.lint] max_new = 0`, `command = []`, `format = ""` | every footprint | The repository's own linter at the repository's own version; aayu pins none and records the version in the pack. Detected when `command` is empty: golangci-lint (`.golangci.*`, `--output.sarif.path`, checked on v2.14.0), ESLint (`eslint.config.*`, `-f json`), Ruff (`ruff.toml` or `[tool.ruff]`, `--output-format sarif`). Read as SARIF 2.1.0 or ESLint JSON. | `skipped: lint`: no linter detected or configured |
| `complexity` | `[verify.floors.complexity] max_ccn = 15` | every footprint | lizard 1.24.1 (MIT), `pip install --require-hashes`: wheel `746060a3e9a375af7340b2d4c6aab307b8a8b4199339c36ddc7afc4188b53d1c`, with pathspec 1.1.1 `a00ce642f577bf7f473932318056212bc4f8bfdf53128c78bbd5af0b9b20b189` and pygments 2.21.0 `2363c69b61c4a97c838da3b130dcd6468f4848992b21a82f2a63ec34377137d9` | none; with no supported language changed, it skips without blocking |

- **Lint rule.** A finding counts only when its file and line are among the changed lines (`Run.Lines`, the same set the coverage floor uses). This has the same meaning as golangci-lint's `--new-from-rev`, for every linter. Known gap: a finding that a change causes on an unchanged line (for example, a variable left unused) is not counted.
- **Complexity rule.** A function is changed when its line range overlaps the changed lines. It fails when it is new with a cyclomatic complexity (CCN) above 15, or when its CCN crosses from 15 or less to above 15. A function that was already above 15 is reported in the pack but does not fail. 15 is lizard's own default warning threshold (`lizard --help`).
- **Why the limit is not taken from the linter configuration (option 2).** `.golangci.yml`, `eslint.config.*` and `ruff.toml` are not protected paths, so a PR could raise its own limit. lizard covers Go, JS/TS, Python, Java, Kotlin, C#, Swift, Rust, Ruby and PHP with one rule.
- **Measured on this repository.** Replaying the last 30 first-parent commits of dev with lizard 1.24.1: 406 of 5,690 functions already exceed CCN 15. Under "fails when changed, above 15, and its CCN rose", 8 of 30 commits would miss, most of them +1 on an old function (for example `runRelease` 67 → 68). Under the recommended crossing rule, 4 of 30 would miss, all of them new or newly complex: `gatherInbox` (new, 34), `runInbox` (new, 31), `presenceCheck` (new, 28), `SitePages` (12 → 16), `shippingEvidence` (14 → 16). At 25, 3 of 30 would miss.
- **SAST pinning gap.** semgrep 1.180.0 is installed with `pipx install "semgrep==…"` and no hash, in `ci.yml` (security job) and in the merge-gate template. Every other downloaded tool is checked against a SHA-256 (AGENTS.md, Safety). This story installs it from a hashed requirements lock (`--require-hashes`). The manylinux x86_64 wheel is `16db2e2706ab814a975eeec25debfad49abba846b7c513d88e041447152f8447`.

**2. User experience: when the footprint has L01.**

| Floor | Key and default | Applies to | Tool and pin | Blocking skip |
|---|---|---|---|---|
| `perf` (exists; gains defaults) | `[verify.floors.perf] min_score = 0.9`, `max_lcp_ms = 2500`, `max_cls = 0.1`, `max_tbt_ms = 200`, `runs = 3`, `layers = ["L01"]` | L01, every screenshot or recording state | Lighthouse 13.5.0, already pinned in `packages/floors` and `pnpm-lock.yaml` (npm integrity is SHA-512; the tarball's SHA-256 is `4be86876145663e10e8bb3178ebe27b2613a1debd4c04b4e0ec92a0487a4e6e2`) | `skipped: web` (below) |
| `visual` (new) | `[verify.floors.visual] max_diff_percent = 0.1`, `layers = ["L01"]`, paths from `review.baseline` | L01 | aayu's own pixel diff with its anti-aliasing mask (`design.Diff`, `internal/design/pixeldiff.go`); Playwright 1.64.0 Chromium, pinned in the lockfile | none in 0.x: `skipped: visual (no baseline)` when no baseline exists |
| web target (new rule) | none | L01 when `platforms` has `web` | none | `skipped: web`: no `[verify.environments.<env>] url`, so the a11y, perf and visual floors cannot run |

- **The Lighthouse defaults** are the "good" thresholds (score 90; LCP 2.5 s; CLS 0.1; TBT 200 ms on mobile). They are the budgets this repository already sets for `examples/benefits-qa` (`.aayudha/config.toml`). The floor takes the median of each metric over 3 runs. Lighthouse's variability guide says a single run is noisy and the median of 5 is twice as stable as 1; 3 runs keep the time per URL bounded. The values in `CONSTRAINTS.md` change from "no default" to these.
- **Visual regression compares each `review.baseline` path** with the nearest first-parent ancestor of the base that has a baseline (`baselines/<sha>/` on the ledger, written by `aayu baseline`), within 20 commits. A path that is one of the story's own Completion Goal states is excluded, because the story is meant to change it and `review` covers it.
- **Why not reuse `figma_diff_max` (2.0 %).** That tolerance absorbs the gap between Figma's renderer and a browser. A baseline and a head capture come from the same pinned Chromium. On a 1280 × 720 capture, 2 % is 18,432 pixels, about a 192 × 96 block: a whole primary button could disappear and pass. 0.1 % is 922 pixels, about 30 × 30.
- **Baselines need storage.** Baseline media go only to the `[proof] media` bucket on a public repository (`.aayudha/config.toml` comment, V3), and nothing captures them automatically today. So `ship` gains a baseline capture after merge when `[review] baseline` and a media store are set. Until a baseline exists, the floor skips without blocking. That is the soft spot of this family.
- The Gemini ui-review lane stays a review, not a floor (ground rule 7).

**3. Developer experience for an API: footprint L02 or L06, and `platforms` has `api`.**

| Floor | Key and default | Tool and pin | Blocking skip |
|---|---|---|---|
| `contract` (new) | `[verify.floors.contract] max_breaking = 0`, `specs = []` (empty: detect `**/openapi*.{yaml,yml,json}`, `**/swagger*.{yaml,yml,json}`, `**/*.proto`, `**/*.graphql{,s}`), `layers = ["L02", "L06"]` | OpenAPI: oasdiff 1.33.0 (Apache-2.0), `breaking --fail-on ERR`, linux_amd64 tar.gz `43a4e328e2d13ba1552d760aa68d2485c75c5621f309f6ff64ae895188345247`. Protobuf: buf 1.74.0 (Apache-2.0), `breaking --against .git#ref=<base>`, buf-Linux-x86_64 `92326d957d613712669fb55f9e4340946a8142a92120e09008459118778273d2`. GraphQL: `@graphql-inspector/core` 8.0.1 (MIT) through the floor's Node script, tarball `a8b85ffbda3a3bd5a49154d4a11c20b164a007650984f9df86cb017adb9c3716`. Each SHA-256 was checked against the publisher's checksums. | `skipped: contract`: `api` platform, footprint L02 or L06, no spec found |
| `api` (exists; gains defaults) | `[verify.floors.api] max_p95_ms = 500`, `max_error_rate = 0.01`, `layers = ["L02", "L06"]` | k6 2.3.0, already pinned in `floors.yml`, `39c3117b6af817592dcd0ce4242105c0a7af10948c2a425306f0be8f7a8a8ab1` | `skipped: api`: no k6 script and no OpenAPI spec |

- **Contract rule.** Each spec is compared base to head, so editing the spec in the PR cannot hide a break. A breaking change fails unless an accepted floor-waiver Decision names `contract`: that waiver is the "Decision" for a break. buf uses the repository's `buf.yaml` breaking rules when present, else its default (`FILE`). The pack records which rules ran. Known gap: in a code-first API whose spec is not regenerated, the floor cannot see a break.
- **Default budget.** verify reads `http_req_duration` p(95) and `http_req_failed` rate from the k6 summary export (k6's default trend stats include p(95)), on top of the script's own thresholds. k6's threshold documentation uses `p(95)<500` for API requests and `rate<0.01` for errors. With no script but an OpenAPI spec, verify generates a smoke script: 20 requests to each GET operation that has no required parameters.

**4. Developer experience for a CLI: footprint L02, and `platforms` has `cli`.**

| Floor | Key and default | Tool | Blocking skip |
|---|---|---|---|
| `cli` (new) | `[verify.floors.cli] command = []` (argv of the built CLI; `[verify] seed` builds it), `goldens = ".aayudha/cli-goldens"`, `layers = ["L02"]` | none: verify runs the product's CLI in its sandbox | `skipped: cli`: `cli` platform, footprint L02, no `command` |

The rules, all checked base to head:

- **Help.** Every command answers `--help` with exit 0 and a stdout containing "usage" (any case). verify discovers commands by walking `--help`, reading the section headed `Commands:` or `Available Commands:` (cobra, click, clap, commander, urfave/cli) and argparse's `{a,b}`. `help` and `completion` are not walked.
- **Failure exits non-zero.** Every command run with an unknown flag (`--aayu-floor-unknown`) exits non-zero and writes to stderr.
- **Stable `--json`.** Each golden is `{ "args": [...], "output": <JSON> }`. The head's output must keep every key path of the base's golden with the same JSON type. Added keys pass; values are not compared, so timestamps and IDs do not break it. A command whose help lists `--json` and has no golden fails.
- **No flag removed without a deprecation.** A long flag in a command's base help must still be in its head help, or the base help must have marked it deprecated on its line. Known cost: cobra's `MarkDeprecated` hides a flag from help, so a cobra CLI keeps the flag visible with "(deprecated)" for one release, or files a waiver.

aayu itself sets `platforms = ["cli"]`, so its own CLI is the first one this floor checks.

**5. DAST: on by default whenever verify can start the app.**

- **Keys.** `verify.dast` becomes `true`. A new `[verify.floors.dast] skipped_blocks_ship = true` and `max_high = 0` (unchanged) are added.
- **When it applies.** DAST runs whenever a target resolves (`ResolveTarget`), after `[verify] seed` starts the app. It uses the same ZAP image, pinned by digest: `zaproxy/zap-stable@sha256:7aaa659b0d43078febd82e29bad112285c370727e86ab8340444220e17d9f0d2` (`verify.ZAPImage`).
- **Blocking skip.** When `platforms` has `web` or `api` and there is no target, or a product sets `dast = false`, the result is `skipped: dast`. It blocks `ship` unless a waiver exists, the same way mobile works.
- **Non-blocking skip.** A product with no `web` or `api` platform and no target skips DAST without blocking.
- **A seed that fails is `blocked`, not skipped**, as today.

**6. Security workflow for product repositories (`aayudha-security.yml`).**

1. **Installed by `aayu init --gates`** next to the other gate workflows (`initrepo.GateWorkflows`). Correction to the request: `init` writes the file into the checkout and tells the person to commit it through a PR. It does not open the PR itself, for any workflow (`Init.writeFile`, `internal/cli/init.go`). The new workflow follows the same path. A file that someone has edited is kept and reported.
2. **Triggers.** It runs on push to the three branches that `[branches]` maps (dev, rolling and main by default; renamed branches in coexistence mode), nightly on each of the three tips for new CVEs, and on `workflow_dispatch`. It has no PR trigger and holds no AI key (`TestModelChecksNeverGateAndNeverShip` extends to it). Its permissions are `contents: read`, `issues: write` and `actions: read`.
3. **Steps.** The first step is the halt check. The tools are the merge gate's, with the same pins: gitleaks and osv-scanner by SHA-256, semgrep from the hashed lock, and the ZAP digest. Then it runs `aayudha verify --ci --security`, a new mode with no PR and no story. It runs `sast` on the tree, `secrets` over the pushed range (`before..sha`) or the full history at night, `deps`, and `dast`. Results go to the job summary, where `skipped: dast` appears. It writes no proof pack, because no story is being proved.
4. **Bugs.** A floor failure files a Bug through aayu's actions log. The Bug carries the floor, the finding, the branch and the commit, the label `aayudha:security`, and a fingerprint (floor, rule or advisory ID, path; never a line number). A later run with the same fingerprint adds one comment for a new commit or branch and does not edit the body (the Ready guard). A finding that disappears gets a "not found at <sha>" comment; closing stays with `ship` or a person. A run files at most 10 new Bugs; the rest are listed in its summary and in the tenth Bug.
5. **Promotion.** The existing promotion gate (`Env.productionGate`, `internal/cli/release.go`) refuses `aayu release --to rolling|main` in two cases:
   - an open `aayudha:security` Bug seen on rolling or main has no accepted waiver (`aayu decide <bug> --waiver <floor>`);
   - for `--to main`, the latest `aayudha-security.yml` run on the rolling tip is missing or not green.

   ci.yml's promotion gate is not duplicated. A finding seen only on dev files a Bug and does not block.
6. **Doctor.** `aayu doctor` gains a `security-workflow` row. It shows whether the file is installed (and unedited) and the conclusion of the latest run on each branch tip. It warns when the nightly run is more than 48 hours old.
7. **Test.** A test next to `workflows_test.go` parses the template and checks the triggers (push on the three rendered branches, a schedule, no `pull_request_target`), the permissions, and that no AI key is present. It runs every `run:` step with stand-in `curl`, `docker` and `aayudha`. The test fails if a trigger is broken, a SHA-256 check is dropped (a stand-in download with the wrong bytes must fail), or `verify --security` stops being called with all four floors.

**This repository's own `ci.yml`.** The `security` job has no `if:`, so it runs on every push to dev, rolling and main. Runs on rolling `a7efe40` and main `aeedef7` both passed security. Three gaps remain:

- no nightly run, so a new CVE in an unchanged dependency is found only on the next push;
- no DAST on push: `floors.yml` runs the ZAP fixture test only on path-filtered changes, and the merge gate has DAST off;
- semgrep is installed without a hash.

The workflow story installs `aayudha-security.yml` here as well, against `examples/benefits-qa`, which `[verify] seed` already starts. That closes all three gaps. Unrelated to this Decision: `release-snapshot` failed on the last three rolling pushes (`19f5a4c`, `b514758`, `a7efe40`).

**Rollout (option 1).** Each new floor and default (`lint`, `complexity`, `visual`, `contract`, `cli`, the perf and api defaults, the DAST default, and the new blocking skips) reports `fail` as advisory until `verify.new_floors_blocking_from`, set four weeks after acceptance. From that date it blocks. This repository has used the same pattern twice, by founder decision (`verify.red_on_base_blocking_from`, `verify.drift_blocking_from`). The security workflow's Bugs and the promotion block apply from the merge, because SAST, secrets and deps are floors already in force.

**Option 2 compared.** It is quicker to explain. But a repository's linter configuration would set its own complexity limit, and the 2 % visual tolerance misses a lost button. Blocking on day one would have stopped 4 to 8 of the last 30 dev commits on complexity alone (measured above).

**Option 3 compared.** It is lowest-risk. But the founder's question about code quality and DX stays open for another cycle, and `CONSTRAINTS.md` keeps saying there is no DX floor.

**Build order once accepted.** One story per floor family, each with tests that a plausible wrong implementation would fail:

- (a) Security workflow and DAST by default: the workflow, Bug dedupe, the promotion block, the doctor row, the step test, and this repository's install.
- (b) Code quality: lint, complexity, and the semgrep hash. Tests: a finding one line outside the diff passes, one inside fails; a CCN 15 → 16 crossing fails, 20 → 21 passes, a new 16 fails.
- (c) UX: perf defaults with the median, the visual floor, and the `skipped: web` rule. Tests: one outlier run out of three passes; a removed button fails at 0.1 % and would pass at 2 %; a Completion Goal path is excluded.
- (d) API DX: contract and the api defaults. Tests: a removed required response field fails; an added optional field passes; editing the spec on both sides still compares to the base.
- (e) CLI DX. Tests: a removed key fails, a retyped key fails, an added key passes; a flag hidden by cobra's deprecation fails without "(deprecated)".

### Recommendation

Option 1, with the defaults, layers, tools and pins in the Evaluation. All thresholds stay in `[verify.floors.*]`, so they change only through a protected path. The new floors are advisory for four weeks and then blocking. The security workflow, with its Bugs and promotion block, applies from its merge.

Not reversible in practice. These floors and Bugs are written into every product's packs and issue history, and a product that adopts them reshapes its CI around them. Product repositories should not have the recommendation apply provisionally without a person.

### Reversible

false
````
