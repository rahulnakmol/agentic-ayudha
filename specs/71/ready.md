# rahulnakmol/agentic-ayudha#71: WP8.2 Docs and installers

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `d47ee92898dec0f70a19d4a4d589e940d1e3dbe8f548312892020074862ae92d` |
| Completion Goal hash | `71e05f630ccbadad58c11c75b398c7fc006460b735a346a53a7d4eea20f8090d` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/71/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: the WP8.2 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: `install.sh` on clean macOS/Linux/Windows VMs
2. [api-trace] state: the WP8.2 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: `aayu doctor` passes

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want WP8.2 Docs and installers built as docs/BUILD-PLAN.md specifies, so that M8 (0.1) delivers: Full dogfood with 100 % proof coverage, docs site, rolling snapshot, installers.

Plan text (docs/BUILD-PLAN.md, WP8.2): `tell docs` → GitHub Pages; first `rolling` snapshot; brew/Scoop/winget after the stable decision.

### Completion Goal

1. [api-trace] state: the WP8.2 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: `install.sh` on clean macOS/Linux/Windows VMs
2. [api-trace] state: the WP8.2 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: `aayu doctor` passes

### Measure

WP8.2 acceptance criteria observed (docs/BUILD-PLAN.md, the 'Accept' line of WP8.2)

### Threshold

all 2 criteria

### Scenario

Open the WP8.2 pull request to dev; CI runs build, tests, lint, security, docs, generate-drift, and release-snapshot; the acceptance tests and any live check named in the plan run and their output is the evidence.

### Environment

GitHub Actions on the pull request, and rahulnakmol/agentic-ayudha or aayu-scratch-* repositories with synthetic fixtures only
````
