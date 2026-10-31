# rahulnakmol/agentic-ayudha#1195: security fails on every PR: glob and minimatch advisories in pnpm-lock.yaml

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `e035d8e98bd7a63bb166de9935b2985c3c1d5210c21aea5707d3492d5c556449` |
| Completion Goal hash | `4f4ae0c46592f6d03d6f80f9d5791c3515971abbbbbaa6d283c17eb584724a64` |
| Technical Owner | devops-devex-tinkerer |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1195/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: pnpm-lock.yaml on the fix branch; expected: the pinned osv-scanner 2.6.0 reports 0 known vulnerabilities and exits 0
2. [api-trace] state: pnpm overrides raise glob to >=14.0.2 and minimatch to >=11.0.1 below the Storybook docgen plugin; expected: `pnpm install --frozen-lockfile` passes, and `pnpm why` shows glob 14 and minimatch 11 only in that chain
3. [api-trace] state: the design system and the floors package with the new versions; expected: tokens:check, typecheck, test, test:storybook, and build-storybook pass, and the floors package installs with a frozen lockfile

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

The `security` job fails on every pull request to dev. Its osv-scanner step finds two advisories in pnpm-lock.yaml: GHSA-3g3m-2fjr-6pw4 (glob 13.0.6, fixed in 14.0.2) and GHSA-g3hv-75wg-m249 (minimatch 10.2.6, fixed in 11.0.1). Both are Medium. They come in only through @storybook/react-vite's docgen plugin (@joshwooding/vite-plugin-react-docgen-typescript 0.7.0, which asks for glob ^13.0.6). The aayu binary contains neither package. PR #1194 failed `security` on 2026-10-31 with an unchanged lockfile.

### Steps to reproduce

1. Check out origin/dev.
2. Run `osv-scanner scan source -r .` with the pinned osv-scanner 2.6.0.

### Expected behaviour

osv-scanner finds no known vulnerability, so the `security` job passes. Storybook and the floors package still install with a frozen lockfile, and the design-system tests and the Storybook build pass.

### Actual behaviour

osv-scanner reports 2 Medium vulnerabilities and exits 1, so `security` fails on every pull request to dev.

### Completion Goal

1. [api-trace] state: pnpm-lock.yaml on the fix branch; expected: the pinned osv-scanner 2.6.0 reports 0 known vulnerabilities and exits 0
2. [api-trace] state: pnpm overrides raise glob to >=14.0.2 and minimatch to >=11.0.1 below the Storybook docgen plugin; expected: `pnpm install --frozen-lockfile` passes, and `pnpm why` shows glob 14 and minimatch 11 only in that chain
3. [api-trace] state: the design system and the floors package with the new versions; expected: tokens:check, typecheck, test, test:storybook, and build-storybook pass, and the floors package installs with a frozen lockfile

### Measure

Completion Goal criteria observed

### Threshold

all 3 criteria

### Scenario

Scan the lockfile, install with a frozen lockfile, and run the design-system checks and the Storybook build.

### Environment

local pnpm 12.0.0 and the CI jobs security, design-system, and floors; never production

### Technical Owner

devops-devex-tinkerer

### Checkpoints

1. pnpm overrides raise glob and minimatch below the docgen plugin, and the lockfile is regenerated (criteria 1, 2)
2. The design-system checks and the Storybook build pass with the new versions (criterion 3)

### Assumptions

#### Notes

- It blocks every merge to dev, so it is P1. #909 fixed a Vitest advisory the same way.

#### Review guidance

- Check: no other package in the lockfile changes version.
- Risk: glob 14 and minimatch 11 are major versions; the docgen plugin calls only globSync.

#### Behaviours

1. says: Every pull request to dev can pass the security job; rule: never; source: criteria 1, 2, 3

### Provenance

Request: "Use pnpm overrides that raise glob to >=14.0.2 and minimatch to >=11.0.1"
1. stated (request): "Use pnpm overrides that raise glob to >=14.0.2 and minimatch to >=11.0.1"
2. stated (request): "Use pnpm overrides that raise glob to >=14.0.2 and minimatch to >=11.0.1"
3. stated (request): "check that Storybook and the floors package still build and their tests pass with the major bumps"
````
