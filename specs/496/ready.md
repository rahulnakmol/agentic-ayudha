# rahulnakmol/agentic-ayudha#496: DAST runs by default for web and API apps: active scans only of apps verify starts, Docker or Podman, advisory for four weeks

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `4b74f47ca0dd44d5b7796e96457316a57aa3aa7e772dce6de08e8cdaf658b566` |
| Completion Goal hash | `a5c33d42fff077ceb753bfbddde276a769c3d6c8dde2ee30673b18c5806235a8` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/496/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `[verify] platforms = ["web"]`, a `[verify] seed` that starts the app, and the target `http://127.0.0.1:<port>`; then the same app at a non-loopback URL with no seed; then that URL with `[verify.environments.<env>] active_scan = true`; expected: the first run's ZAP plan contains the active scan job with rules 40012, 40014, 40018 and 90020 and the check detail says active; the second plan has no active scan job and the detail says passive and that a passive scan cannot see a reflected XSS; the third plan has the active scan job; `active_scan = true` set in the user config, in `.aayudha/config.local.toml`, in the environment, or with --set fails the config load, naming `.aayudha/config.toml`, a protected path
2. [api-trace] state: `[verify] platforms` set to `["cli"]`, and unset, with a target URL configured; expected: the dast check is skipped with `n/a: dast (no web or api in [verify] platforms)`, the floors hold, and ship is not blocked
3. [api-trace] state: `[verify] platforms = ["api"]`, no target URL, with and without an accepted floor-waiver Decision that names dast; expected: the check reads `skipped: dast: no runnable target`, names `[verify] seed`, `[verify.environments.local] url` and `aayu decide <story> --waiver dast`, and blocks the floors; the waiver releases it; `aayu doctor` shows the same fix
4. [api-trace] state: stand-in `docker` and `podman` on PATH: both, only podman, only docker, and neither, run locally and with `--ci`; expected: both runs docker; only podman runs podman; each runs `docker.io/zaproxy/zap-stable@sha256:7aaa659b0d43078febd82e29bad112285c370727e86ab8340444220e17d9f0d2` with `--network host` and the work directory mounted; neither locally is a non-blocking skip that says to install Docker or Podman and that CI runs the scan; neither with `--ci` is blocked; `aayu doctor` names the runtime found, or none
5. [api-trace] state: the macOS runtime layout (GOOS darwin) with the target on 127.0.0.1, under docker and under podman; expected: the plan's URL host becomes `host.docker.internal` under docker and `host.containers.internal` under podman, and no `--network host` is passed
6. [api-trace] state: a stand-in ZAP report with one high alert, at a date before `verify.new_floors_blocking_from` and at that date; expected: before the date the check passes with an advisory detail naming the date and the alert; on the date it fails
7. [api-trace] state: `aayudha-security.yml` as `aayu init --gates` renders it, run with stand-ins; expected: `verify --security` runs sast, secrets, deps and dast, and a dast skip appears in the job summary

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As a product team using aayu, I want DAST to run by default against my web or API app with guardrails that keep it safe and runnable, so that a reflected XSS or an injection is caught before ship without ever attacking a shared environment. Decision #485 (accepted, option 1, section 5) sets `verify.dast = true` with guardrails G1 to G5: the active XSS and injection rules run only against an app verify starts itself on a loopback host, and any other URL gets a passive-only scan unless its environment opts in with `active_scan = true`; DAST applies only to the `web` and `api` platforms; a web or api story with no runnable target records a blocking `skipped: dast` that says how to fix it; ZAP runs under Docker or Podman, rootless included, by the fully qualified pinned digest; and high-risk alerts are advisory for four weeks before they block. The security workflow (#487) gains the dast floor.

### Completion Goal

1. [api-trace] state: `[verify] platforms = ["web"]`, a `[verify] seed` that starts the app, and the target `http://127.0.0.1:<port>`; then the same app at a non-loopback URL with no seed; then that URL with `[verify.environments.<env>] active_scan = true`; expected: the first run's ZAP plan contains the active scan job with rules 40012, 40014, 40018 and 90020 and the check detail says active; the second plan has no active scan job and the detail says passive and that a passive scan cannot see a reflected XSS; the third plan has the active scan job; `active_scan = true` set in the user config, in `.aayudha/config.local.toml`, in the environment, or with --set fails the config load, naming `.aayudha/config.toml`, a protected path
2. [api-trace] state: `[verify] platforms` set to `["cli"]`, and unset, with a target URL configured; expected: the dast check is skipped with `n/a: dast (no web or api in [verify] platforms)`, the floors hold, and ship is not blocked
3. [api-trace] state: `[verify] platforms = ["api"]`, no target URL, with and without an accepted floor-waiver Decision that names dast; expected: the check reads `skipped: dast: no runnable target`, names `[verify] seed`, `[verify.environments.local] url` and `aayu decide <story> --waiver dast`, and blocks the floors; the waiver releases it; `aayu doctor` shows the same fix
4. [api-trace] state: stand-in `docker` and `podman` on PATH: both, only podman, only docker, and neither, run locally and with `--ci`; expected: both runs docker; only podman runs podman; each runs `docker.io/zaproxy/zap-stable@sha256:7aaa659b0d43078febd82e29bad112285c370727e86ab8340444220e17d9f0d2` with `--network host` and the work directory mounted; neither locally is a non-blocking skip that says to install Docker or Podman and that CI runs the scan; neither with `--ci` is blocked; `aayu doctor` names the runtime found, or none
5. [api-trace] state: the macOS runtime layout (GOOS darwin) with the target on 127.0.0.1, under docker and under podman; expected: the plan's URL host becomes `host.docker.internal` under docker and `host.containers.internal` under podman, and no `--network host` is passed
6. [api-trace] state: a stand-in ZAP report with one high alert, at a date before `verify.new_floors_blocking_from` and at that date; expected: before the date the check passes with an advisory detail naming the date and the alert; on the date it fails
7. [api-trace] state: `aayudha-security.yml` as `aayu init --gates` renders it, run with stand-ins; expected: `verify --security` runs sast, secrets, deps and dast, and a dast skip appears in the job summary

### Measure

Completion Goal criteria observed

### Threshold

all 7 criteria

### Scenario

Run the dast floor with stand-in container runtimes that record their arguments and write a recorded ZAP report, across the platform, target, seed, active_scan, GOOS, CI and date combinations above. Run verify against the fake GitHub for the waiver. Run doctor against probes with each runtime. Run the security workflow steps with stand-ins.

### Environment

local checkout and this repository's CI build job on a pull request; the fake GitHub in internal/github/fake, never production; the real ZAP image only in the floors-dast job

### Links

depends-on #487

### Checkpoints

1. `go test ./internal/verify -run 'TestDASTGuardrails|TestContainerRuntime'` shows active or passive by target, n/a by platform, the blocking skip, the runtime choice, and the advisory date (criteria 1, 2, 3, 4, 5, 6)
2. `go test ./internal/doctor ./internal/cli -run 'TestContainerRuntimeRow|TestVerifyDASTWaiver'` and `go test . -run TestSecurityWorkflowSteps` (criteria 3, 4, 7)

### Assumptions

- "verify starts the app itself" means `[verify] seed` ran in this verify run and the target's host is 127.0.0.1, ::1, or localhost
- `verify.new_floors_blocking_from` is 2026-11-14, four weeks after the founder accepted #480 and #485 on 2026-10-17; it is shared by every new floor of Decision #480
- the macOS and Windows host rewrite is checked with stand-ins only; a real run on those hosts is not part of this story
- a rootless Podman run needs the work directory mode 0777, which verify already sets

### Open questions

- deferred: a real macOS run of ZAP under Docker Desktop and podman machine needs a macOS runner

### Provenance

Request: "Build the DAST story too now, with guardrails G1-G5 as written. Active scanning (the XSS and injection rules) runs only against an app that verify starts itself, locally or in CI, in its sandbox. A deployed or shared URL (staging or production) is never actively scanned unless the repo config names it and opts in to active scanning, which is a protected path. A passive-only scan of a configured URL is fine. DAST applies only to web or api platforms. For a CLI or library it is n/a and never blocks. No runnable target for a web or api story means skipped: dast, which blocks ship unless waived. aayu doctor and the skipped message should say how to give verify a start command or target. verify should run the pinned ZAP image (by digest) with whichever container runtime is present: docker first, then podman, including rootless podman. With neither runtime present locally, verify says so and leaves the scan to CI. In CI, a missing runtime is a failure. doctor reports which runtime it found. Keep the 4-week advisory period before high-risk alerts block."
Term: "DAST" → the ZAP scan of the running app; "DAST (ZAP baseline plus active XSS and injection rules, 0 high) is opt-in"
Vague: "protected" → criteria 1
Term: "ZAP" → the OWASP ZAP scanner verify runs from the pinned image; "it uses ZAP (internal/verify/dast.go, the pinned image zaproxy/zap-stable@sha256:7aaa…)"
Term: "CI" → the GitHub Actions workflows that run verify; "the merge gate"
Term: "XSS" → cross-site scripting; "the active cross-site scripting and injection rules"
1. stated (request): "Active scanning (the XSS and injection rules) runs only against an app that verify starts itself, locally or in CI, in its sandbox" / "A deployed or shared URL (staging or production) is never actively scanned unless the repo config names it and opts in to active scanning, which is a protected path. A passive-only scan of a configured URL is fine"
2. stated (request): "DAST applies only to web or api platforms. For a CLI or library it is n/a and never blocks"
3. stated (request): "No runnable target for a web or api story means skipped: dast, which blocks ship unless waived. aayu doctor and the skipped message should say how to give verify a start command or target"
4. stated (request): "verify should run the pinned ZAP image (by digest) with whichever container runtime is present: docker first, then podman, including rootless podman" / "With neither runtime present locally, verify says so and leaves the scan to CI. In CI, a missing runtime is a failure. doctor reports which runtime it found"
5. stated (request): "verify should run the pinned ZAP image (by digest) with whichever container runtime is present" / "including rootless podman"
6. stated (request): "Keep the 4-week advisory period before high-risk alerts block"
7. stated (request): "Build the DAST story too now, with guardrails G1-G5 as written"
````
