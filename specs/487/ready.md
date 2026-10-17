# rahulnakmol/agentic-ayudha#487: Every push to dev, rolling and main, and a nightly run, scan the commit with verify's security floors and file deduplicated Bugs

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `833c1fd9798c186c864c83af4dae130111e5e7727d7a6397902c2d7c6619192e` |
| Completion Goal hash | `1abf168cd9be3e0ce67ce61c57d1b4fcdcca14dd1ed89bd4fcf3e788809a3276` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/487/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `aayu init --gates` run on a repository with default branches, and on one whose `[branches]` maps dev, rolling and main to trunk, canary and prod; expected: `.github/workflows/aayudha-security.yml` is written with push triggers on exactly the three mapped branches, a nightly schedule, and workflow_dispatch, no pull_request or pull_request_target trigger, permissions limited to contents read, issues write and actions read, and no model key or AI-gateway environment
2. [api-trace] state: a checkout with a planted high semgrep finding, a planted gitleaks secret, and a high osv-scanner vulnerability, run as `aayudha verify --ci --security --since <before>` on dev against the fake GitHub; expected: the JSON and the job summary list sast, secrets and deps as fail, and one open Bug labelled `aayudha:security` per finding names the floor, the finding, the branch and the commit
3. [api-trace] state: the same findings on a second commit and on rolling, then a third run where the secret is gone; expected: no new Bug is filed, each existing Bug gets one comment per new branch and commit, and the secret's Bug gets one comment saying it was not found on that branch at that commit
4. [api-trace] state: an open `aayudha:security` Bug last seen on rolling, with and without an accepted floor-waiver Decision that waives it; a Bug seen only on dev; and a rolling tip whose latest security run is missing or failed; expected: `aayu release --to main` and `--to rolling` refuse while the unwaived rolling Bug is open, naming it; the waived Bug and the dev-only Bug do not block; `--to main` also refuses while the rolling tip has no green security run
5. [api-trace] state: repositories with the security workflow missing, installed with green runs on every tip, installed with a red run on main, and installed with no nightly run in 48 hours; expected: `aayu doctor` shows a security-workflow row that fails when it is missing, names each tip's latest conclusion, and warns on the red tip and the stale nightly run
6. [api-trace] state: the workflow's run steps executed by a test with stand-in curl, gitleaks, osv-scanner, semgrep and aayudha; expected: the installed steps pass; the test fails when a push branch is dropped from the trigger, when a downloaded tool's bytes do not match its SHA-256, or when the verify step stops calling `--security`

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As a product team using aayu, I want every push to dev, rolling and main, and a nightly run, to scan the pushed commit with verify's security floors (SAST, secrets, deps), so that a merged commit, a promotion merge, a direct or admin push, or a new CVE in an unchanged dependency is caught even though no pull request ran. Decision #480 (accepted, option 1, section 6) specifies `aayudha-security.yml`: `aayu init --gates` installs it; it runs `aayudha verify --ci --security` with the merge gate's pinned tools and holds no AI key; a failure files or updates one Bug per finding through aayu; an open security Bug seen on rolling or main blocks the next promotion through the existing promotion gate; `aayu doctor` reports it. The DAST step waits for Decision #485 and is not part of this story.

### Completion Goal

1. [api-trace] state: `aayu init --gates` run on a repository with default branches, and on one whose `[branches]` maps dev, rolling and main to trunk, canary and prod; expected: `.github/workflows/aayudha-security.yml` is written with push triggers on exactly the three mapped branches, a nightly schedule, and workflow_dispatch, no pull_request or pull_request_target trigger, permissions limited to contents read, issues write and actions read, and no model key or AI-gateway environment
2. [api-trace] state: a checkout with a planted high semgrep finding, a planted gitleaks secret, and a high osv-scanner vulnerability, run as `aayudha verify --ci --security --since <before>` on dev against the fake GitHub; expected: the JSON and the job summary list sast, secrets and deps as fail, and one open Bug labelled `aayudha:security` per finding names the floor, the finding, the branch and the commit
3. [api-trace] state: the same findings on a second commit and on rolling, then a third run where the secret is gone; expected: no new Bug is filed, each existing Bug gets one comment per new branch and commit, and the secret's Bug gets one comment saying it was not found on that branch at that commit
4. [api-trace] state: an open `aayudha:security` Bug last seen on rolling, with and without an accepted floor-waiver Decision that waives it; a Bug seen only on dev; and a rolling tip whose latest security run is missing or failed; expected: `aayu release --to main` and `--to rolling` refuse while the unwaived rolling Bug is open, naming it; the waived Bug and the dev-only Bug do not block; `--to main` also refuses while the rolling tip has no green security run
5. [api-trace] state: repositories with the security workflow missing, installed with green runs on every tip, installed with a red run on main, and installed with no nightly run in 48 hours; expected: `aayu doctor` shows a security-workflow row that fails when it is missing, names each tip's latest conclusion, and warns on the red tip and the stale nightly run
6. [api-trace] state: the workflow's run steps executed by a test with stand-in curl, gitleaks, osv-scanner, semgrep and aayudha; expected: the installed steps pass; the test fails when a push branch is dropped from the trigger, when a downloaded tool's bytes do not match its SHA-256, or when the verify step stops calling `--security`

### Measure

Completion Goal criteria observed

### Threshold

all 6 criteria

### Scenario

Render the workflow through init on a temporary repository with default and mapped branches. Run verify --security with stand-in scanners that report the planted findings, against the fake GitHub, three times across dev and rolling. Run release against the fake GitHub with the Bugs and runs above. Run doctor against fakes of each workflow state. Execute the workflow steps with stand-in tools.

### Environment

local checkout and this repository's CI build job on a pull request; the fake GitHub in internal/github/fake, never production

### Checkpoints

1. `go test ./internal/initrepo ./internal/cli -run 'TestSecurityWorkflow|TestVerifySecurity'` shows the rendered triggers and the Bugs filed, updated and cleared (criteria 1, 2, 3)
2. `go test ./internal/cli ./internal/doctor -run 'TestReleaseSecurity|TestDoctorSecurity'` shows promotion refused and allowed, and each doctor state (criteria 4, 5)
3. `go test . -run TestSecurityWorkflowSteps` runs every step with stand-ins (criterion 6)

### Assumptions

- a finding's fingerprint is the floor, the rule or advisory ID, and the path, never a line number, so a moved line stays the same Bug
- a run files at most 10 new Bugs; the rest are listed in the summary and in the tenth Bug
- secrets on a push scan the pushed range (before..sha), or the whole history of HEAD when before is empty or all zeros, and on the nightly run
- the workflow writes Bugs with the workflow token (GITHUB_TOKEN, issues write) through aayu's actions log, as `--ci` already allows
- a Bug's body is never edited after filing; updates are comments, so the Ready guard is not tripped
- this repository installs the workflow too; its ci.yml security job stays as it is

### Open questions

- deferred: the DAST step joins the workflow once Decision #485 is accepted

### Provenance

Request: "A workflow that init --gates installs, for example aayudha-security.yml. It runs on push to dev, rolling and main, plus a nightly schedule for new CVEs. It runs verify's security floors (SAST, secrets, deps, and DAST) on the pushed commit, with the same pinned tools and SHA-256 checks as the merge gate. It holds no AI key. A failure on a branch push files or updates one Bug through aayu, with the floor, the finding and the commit, and deduplicates on later runs. On rolling and main it also blocks the next promotion until it is fixed or waived. The promotion gates already exist; wire it in, don't duplicate it. aayu doctor reports whether the security workflow is installed and passing. A test like workflows_test.go that runs each installed step with stand-ins, so a broken trigger or a dropped scanner fails CI."
Term: "SAST" → static analysis by semgrep; "verify always runs SAST (semgrep, 0 critical and 0 high)"
Term: "CVEs" → known vulnerabilities in dependencies that osv-scanner reports; "deps (osv-scanner, 0 critical and 0 high)"
Term: "DAST" → the ZAP scan of the running app; "DAST (ZAP baseline plus active XSS and injection rules, 0 high) is opt-in"
1. stated (request): "A workflow that init --gates installs, for example aayudha-security.yml. It runs on push to dev, rolling and main, plus a nightly schedule for new CVEs" / "It holds no AI key"
2. stated (request): "It runs verify's security floors (SAST, secrets, deps, and DAST) on the pushed commit" / "A failure on a branch push files or updates one Bug through aayu, with the floor, the finding and the commit"
3. stated (request): "and deduplicates on later runs"
4. stated (request): "On rolling and main it also blocks the next promotion until it is fixed or waived. The promotion gates already exist; wire it in, don't duplicate it"
5. stated (request): "aayu doctor reports whether the security workflow is installed and passing"
6. stated (request): "A test like workflows_test.go that runs each installed step with stand-ins, so a broken trigger or a dropped scanner fails CI" / "with the same pinned tools and SHA-256 checks as the merge gate"
````
