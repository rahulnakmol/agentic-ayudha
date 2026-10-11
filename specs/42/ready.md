# rahulnakmol/agentic-ayudha#42: WP4.2 Ledger with LFS and media store

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `94889fef4458bff22b4dba8af5d5357fb4036bf3f2dc72fe759470e43b932fd2` |
| Completion Goal hash | `5f67663ba5b37d93f3dabb99b4390e029c8e6a673968658ffb4b7e885d35ba82` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/42/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: the WP4.2 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: live scratch test
2. [api-trace] state: the WP4.2 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: public-repo fixture refuses LFS media
3. [api-trace] state: the WP4.2 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: purge removes bucket objects and rewrites `pack.json` with tombstones

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want WP4.2 Ledger with LFS and media store built as docs/BUILD-PLAN.md specifies, so that M4 (Proof) delivers: Every merge carries a CI-authoritative proof pack: pack format, ledger with LFS and media store, deterministic floors, merge gate and inspect, thin-slice verbs.

Plan text (docs/BUILD-PLAN.md, WP4.2): Extends WP2.4: LFS tracking by extension (private repos); `[proof] media` bucket upload with lifecycle rules (public repos, or by choice); retention TTL from config; `aayu proof push|get|verify|list|purge`; one replace-in-place comment on PR and issue. Agent-pushed packs are **advisory**; CI-written packs (WP4.8) are authoritative.

### Completion Goal

1. [api-trace] state: the WP4.2 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: live scratch test
2. [api-trace] state: the WP4.2 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: public-repo fixture refuses LFS media
3. [api-trace] state: the WP4.2 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: purge removes bucket objects and rewrites `pack.json` with tombstones

### Measure

WP4.2 acceptance criteria observed (docs/BUILD-PLAN.md, the 'Accept' line of WP4.2)

### Threshold

all 3 criteria

### Scenario

Open the WP4.2 pull request to dev; CI runs build, tests, lint, security, docs, generate-drift, and release-snapshot; the acceptance tests and any live check named in the plan run and their output is the evidence.

### Environment

GitHub Actions on the pull request, and rahulnakmol/agentic-ayudha or aayu-scratch-* repositories with synthetic fixtures only
````
