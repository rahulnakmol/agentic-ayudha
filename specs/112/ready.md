# rahulnakmol/agentic-ayudha#112: gc says how to free a build worktree it keeps

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `ee030f96d6a4977e7ed4db14549864b265ccd2a485c44f8df1d227df3236a7c4` |
| Completion Goal hash | `3b5c8a2dbc72ffdfd61d5881cac0f282eca75793c5de1cdfdf26556bd74ee506` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/112/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `aayu gc` in a checkout where a closed story's build worktree has uncommitted changes; expected: the worktree is kept and its line says "commit or discard them, then run `aayu gc` again"
2. [api-trace] state: `aayu gc` in a checkout where a closed story's build worktree has commits that are not on origin; expected: the worktree is kept and its line says "push them or delete the branch, then run `aayu gc` again"

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As a human cleaning up after builds, I want `aayu gc` to say how to free a build worktree it keeps, so that I can clear it without reading the code.

### Completion Goal

1. [api-trace] state: `aayu gc` in a checkout where a closed story's build worktree has uncommitted changes; expected: the worktree is kept and its line says "commit or discard them, then run `aayu gc` again"
2. [api-trace] state: `aayu gc` in a checkout where a closed story's build worktree has commits that are not on origin; expected: the worktree is kept and its line says "push them or delete the branch, then run `aayu gc` again"

### Measure

Completion Goal criteria observed

### Threshold

all 2 criteria

### Scenario

Close a story whose build worktree holds uncommitted changes and run `aayu gc`; then commit them without pushing and run `aayu gc` again.

### Environment

local checkout with a fixture repository and a fake GitHub; synthetic data only, never production
````
