# Contributing

Thanks for helping. Scope and sequencing live in [docs/BUILD-PLAN.md](docs/BUILD-PLAN.md); read the work
package (WP) your change belongs to before you start. Report security issues privately as described in
[SECURITY.md](SECURITY.md), not here.

Features marked "n/a until WPx.y" do not exist yet (ground rule 1a).

## Requirements

- Go 1.25 or newer.
- `git` (and `git-lfs` once ledger operations exist, WP2.4/WP4.2).
- `gh` is optional but makes the PR flow below easier.

## Branches

| Branch | Role |
|---|---|
| `dev` | Default branch and nightly integration. Every feature, bugfix, and hotfix PR targets it. |
| `rolling` | Canary snapshots, promoted from `dev`. |
| `main` | Stable production. Every commit is tagged `v*` and released. |
| `ledger` | Orphan branch for this repository's own proof packs. Not for code. |

Never open a PR against `rolling` or `main`. Promotions are done by the maintainer (below).

## Pull request flow

1. Work in a dedicated worktree and branch cut from the latest `dev`:

   ```sh
   git fetch origin && git worktree add ../wt-<branch> -b <branch> origin/dev
   ```

2. Make the change. Run `go build ./... && go vet ./... && go test ./...` and `go generate ./...` (see
   [Generated files](#generated-files)).
3. Commit with Conventional Commits and a DCO sign-off (below).
4. Open a PR to `dev`. CI must be green.
5. The PR is squash-merged. Then delete the branch and the worktree:

   ```sh
   git worktree remove ../wt-<branch> && git branch -D <branch>
   ```

From PR11 onward every work package is tracked as a Story issue on this repository with a Completion Goal
(ground rule 1); before that, work is tracked in the build plan.

## Promotion (maintainer only)

- **`dev` → `rolling`**: after the full rolling suite passes, a non-fast-forward merge commit
  `Rolling snapshot: <timestamp-or-version>` and a lightweight tag `rolling-<date>`.
- **`rolling` → `main`**: only on an explicit stable-release decision, as a merge commit
  `Release: v<major>.<minor>.<patch>` with an annotated `v*` tag. Every commit on `main` is tagged and
  released. The `v*` tag ruleset allows those tags from `main` only.

### Emergency path

For a critical security incident or outage only:

1. Branch from `rolling` or `main` (wherever the fix must land).
2. Open the fix as a PR to `dev` and merge it there first.
3. Cherry-pick that exact commit to `rolling` and/or `main`.
4. Run the release gate, tag, and release.
5. Record why the emergency path was used in the PR and in the release notes (and in the security advisory
   if there is one).

## Commits

- **Conventional Commits**: `type(scope): summary`, e.g. `feat(cli): add version command`,
  `fix(ledger): retry on non-fast-forward`. Types: `feat`, `fix`, `docs`, `refactor`, `test`, `build`, `ci`,
  `chore`, `perf`, `revert`. Mark breaking changes with `!` or a `BREAKING CHANGE:` footer.
- **DCO sign-off on every commit**: use `git commit -s`. This adds a trailer with your real name and email:

  ```text
  Signed-off-by: Your Name <you@example.com>
  ```

  To fix commits already on your branch: `git rebase --signoff origin/dev`.
- CI checks both (n/a until WP0.4). Squash-merge commit messages must keep the sign-off.

The DCO is required from the first commit so that every contribution's origin is recorded; it keeps a future
relicense possible (plan risk R10). There is no separate CLA.

### Developer Certificate of Origin

By adding `Signed-off-by` you certify the following (from <https://developercertificate.org>):

```text
Developer Certificate of Origin
Version 1.1

Copyright (C) 2004, 2006 The Linux Foundation and its contributors.

Everyone is permitted to copy and distribute verbatim copies of this
license document, but changing it is not allowed.


Developer's Certificate of Origin 1.1

By making a contribution to this project, I certify that:

(a) The contribution was created in whole or in part by me and I
    have the right to submit it under the open source license
    indicated in the file; or

(b) The contribution is based upon previous work that, to the best
    of my knowledge, is covered under an appropriate open source
    license and I have the right under that license to submit that
    work with modifications, whether created in whole or in part
    by me, under the same open source license (unless I am
    permitted to submit under a different license), as indicated
    in the file; or

(c) The contribution was provided directly to me by some other
    person who certified (a), (b) or (c) and I have not modified
    it.

(d) I understand and agree that this project and the contribution
    are public and that a record of the contribution (including all
    personal information I submit with it, including my sign-off) is
    maintained indefinitely and may be redistributed consistent with
    this project or the open source license(s) involved.
```

## Generated files

`internal/verbs/registry.go` is the single source for verbs and plumbing commands (ground rule 4). These are
generated from it by `go generate ./...`:

- the verbs and plumbing tables in `README.md` (between the `BEGIN GENERATED` / `END GENERATED` markers);
- `LAYERS.md`;
- `docs/RACI.md`;
- the frontmatter of `skills/<verb>/SKILL.md` (the body below the frontmatter is hand-written);
- `internal/mcp/schemas/*.json`.

Do not edit generated output by hand. Change the registry, run `go generate ./...`, and commit both. The CI
job `generate-drift` fails on any difference (n/a until WP0.4).

## Protected paths

These paths need a human CODEOWNER review (see [.github/CODEOWNERS](.github/CODEOWNERS)):

`.github/`, `skills/`, `ops/`, `seats/`, `CONSTRAINTS.md`, `.aayudha/config.toml`, `.agents/`, `.claude/`.

## Identities and approvals

- Humans use their own GitHub account and token.
- Agent lanes and CI use a bot identity: a GitHub App or a machine user (n/a until WP1.4). Until then agents
  run under the human's identity (the WP1.4 day-one path) and their writes are tagged
  `identity:human-fallback`.
- An approval counts only if it comes from an account with `type != Bot` that is not listed in
  `[identity] bot_logins` (enforced by verbs from WP1.4).

## Solo-mode policy

> This policy changes as the project gains people. Change it by PR, together with the matching ruleset and
> `.github/CODEOWNERS` changes.

**Today**: there is one human, `rahulnakmol`. GitHub never lets a PR author approve their own PR, so a
protected-path PR authored under the founder's identity cannot get the required CODEOWNER review.

**Rule while there is one human**:

1. Such a PR is merged by the repository admin through an explicit ruleset bypass. The bypass is allowed only
   through a pull request, never by direct push.
2. GitHub records each bypass in the ruleset rule-suite history:

   ```sh
   gh api repos/rahulnakmol/agentic-ayudha/rulesets/rule-suites
   ```

3. Every such bypass is listed as a solo-mode bypass by a hand-run reconciliation of that history against the
   merged PRs, until `aayu audit` automates it (WP7.6). A bypass that is not on the list is an incident (see
   [SECURITY.md](SECURITY.md#tool-incident-plan)).

**When a bot identity exists (WP1.4)**: agent-authored PRs are opened by the bot, so the founder approves
them as a real review and no bypass is needed for them. Bypasses remain only for PRs the founder authors.

**When a second human seat is added** (`aayu seat add <seat> --human <login>`, WP5.1): the rule switches to
true second-human review. Bypasses for protected paths stop, and the second human is added to CODEOWNERS
for their seat's layers.

## Third-party code

Every new dependency must have its licence recorded in
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) in the same PR. CI licence scanning gates it (n/a until
WP0.4). This project is BSD-3-Clause ([LICENSE](LICENSE)); by contributing you agree your contribution is
licensed under it.
