# rahulnakmol/agentic-ayudha#595: init writes each repository's token-exchange trust policies, and doctor checks them

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `62405d3419fc0e06ad2121b08a47d359309ee26cd0af9c10d9bb79311a1b4db8` |
| Completion Goal hash | `7b07f64faa2355a9c5e63625f13eae8f82505b3f95717063a171e68cb6505354` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/595/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `aayu init --identity` with `[identity] exchange` set on a repository whose default branch is `dev`, with a fake exchange that reports the App; expected: the checkout gains `.github/chainguard/aayudha-ci.sts.yaml` and `.github/chainguard/aayudha-agent.sts.yaml`; the CI policy's issuer is `https://token.actions.githubusercontent.com` and its claim names each aayudha workflow at `refs/heads/dev`; the agent policy names Amp's issuer with this project's ID; neither grants more than the App's six permissions
2. [api-trace] state: `aayu doctor` on a checkout whose CI policy trusts a second issuer, whose agent policy uses `subject_pattern: ".*"`, and whose permissions add `administration: write`; expected: the identity row fails, naming each of the three problems with its file
3. [api-trace] state: `aayu doctor` on the checkout written by criterion 1; expected: the identity row passes the policy check and names both files

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want `aayu init --identity` to write each repository's trust policies for the token exchange, and `aayu doctor` to check them. That way, every bot token the exchange mints is narrowed to one repository, one caller, and the aayudha App's permissions (Decision #576, option 2, story 2 of 3).

`init` writes two files under `.github/chainguard/`:
- `aayudha-ci.sts.yaml` trusts GitHub Actions' issuer. It names this repository's workflows by an exact `job_workflow_ref` on the default branch.
- `aayudha-agent.sts.yaml` trusts the agent hosts with a workload identity: Amp's issuer, with this project's `project_id`, and Cursor's issuer, with this repository in `repo_urls`.

Each grants at most the App's six permissions (contents, issues and pull requests write; actions and checks read; metadata read). `doctor`'s identity row fails a policy that:
- trusts any other issuer;
- uses a subject or claim pattern where an exact value is required;
- does not pin the workflow;
- or asks for a permission the App does not have.

### Completion Goal

1. [api-trace] state: `aayu init --identity` with `[identity] exchange` set on a repository whose default branch is `dev`, with a fake exchange that reports the App; expected: the checkout gains `.github/chainguard/aayudha-ci.sts.yaml` and `.github/chainguard/aayudha-agent.sts.yaml`; the CI policy's issuer is `https://token.actions.githubusercontent.com` and its claim names each aayudha workflow at `refs/heads/dev`; the agent policy names Amp's issuer with this project's ID; neither grants more than the App's six permissions
2. [api-trace] state: `aayu doctor` on a checkout whose CI policy trusts a second issuer, whose agent policy uses `subject_pattern: ".*"`, and whose permissions add `administration: write`; expected: the identity row fails, naming each of the three problems with its file
3. [api-trace] state: `aayu doctor` on the checkout written by criterion 1; expected: the identity row passes the policy check and names both files

### Measure

Completion Goal criteria observed

### Threshold

all 3 criteria

### Scenario

Run the `aayu init --identity` and `aayu doctor` tests in internal/cli against a fake GitHub and a fake exchange; read the written files and the doctor rows.

### Environment

local checkout and this repository's CI build job on a pull request; the policy format of Octo-STS at 082d66c (`TrustPolicy` in pkg/octosts/trust_policy.go), GitHub Actions' OIDC claims reference, and Amp's workload-identity claims observed with `amp orb id-token` on 2026-10-18

### Checkpoints

1. `init --identity` writes the two trust policies (criterion 1)
2. `doctor` checks them (criteria 2, 3)

### Links

- depends-on #594

### Assumptions

- The policy files sit under `.github/`, a protected path, so a change to them needs the founder. `.github/chainguard/` joins #572's always-a-person set when that set is built.
- Octo-STS matches subjects and claims exactly unless a pattern is given, and anchors patterns itself.

### Open questions

- deferred: "phone-safe": App creation from a phone belongs to story 1 of Decision #576 and Bug #578; this Story has no setup step

### Provenance

Request: "Then build the stories in order, with #578 (phone-safe App creation) in scope."
Vague: "phone-safe" → deferred: it is about creating the App, which is story 1 of 3; this Story has no setup step on a phone
1. stated (Decision #576): "Each repository holds only a trust policy, `.github/chainguard/<lane>.sts.yaml`, which says which issuer and subject may get which permissions on that repository."
2. stated (Decision #576, pre-mortem): "exact subjects only, Octo-STS v0.12.0 or later, and `.github/chainguard/` in the always-a-person set from #572"
3. default (correctness/doctor): a correct policy passes, so the row is not noise (ETHOS.md, deterministic checks)
Term: "trust policy" → an Octo-STS `.sts.yaml` file naming issuer, subject, claims, and permissions; "Each repository holds only a trust policy"
````
