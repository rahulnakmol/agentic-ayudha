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
   [Generated files](#generated-files)), plus `golangci-lint run ./...` and `markdownlint-cli2` if you have
   them.
3. Commit with Conventional Commits and a DCO sign-off (below).
4. Open a PR to `dev`. CI must be green: the required checks are `build`, `generate-drift`, `lint`,
   `commits`, `security`, `docs`, `release-snapshot` ([.github/workflows/ci.yml](.github/workflows/ci.yml)) and
   `protected-paths`. Jobs that a PR's paths cannot affect report "skipped", which counts as passing. `build`
   also runs the coverage ratchet: [ratchets.toml](ratchets.toml) says what it checks and how to tighten the floor.
5. The PR is squash-merged. Then delete the branch and the worktree:

   ```sh
   git worktree remove ../wt-<branch> && git branch -D <branch>
   ```

When the build plan cannot be enforced as written, build the closest enforceable alternative and record it in the PR
description under a "Deviation from the plan:" heading: what the plan assumed, what you found, and what you built
instead. The squash commit message keeps it, so `git log` is the record. Real decisions are aayu Decision issues.
Appendix B of the plan, the decision log up to 2026-10-17, is archived privately; IDs such as D-5.5c.24 in code and
docs refer to it, and no new rows are added. Research notes live in the private repository `rahulnakmol/agentic-ayudha-research`; cite one as
"private research note `<file>`" with its section, never as a link or a path.

From PR11 onward every work package is tracked as a Story issue on this repository with a Completion Goal
(ground rule 1); before that, work is tracked in the build plan.

## Promotion (agent-run, no human approval)

Founder policy for this repository: promotions `dev` → `rolling` and `rolling` → `main` need **no human
approval**. Whoever promotes (the maintainer or an agent acting with the maintainer's credentials) does so as
soon as the gate is green; nothing waits for a person in between. The gates themselves and the commit and tag
conventions are unchanged. `dev` → `rolling` happens on the release schedule or an explicit release request;
`rolling` → `main` on an explicit stable-release request.

Only the repository admin identity can move `rolling` and `main` or create `v*` tags (rulesets
`rolling-main-promotion` and `release-tags`). The gate runs on the exact merge commit that will land, built on
a throwaway `promote/…` branch, so what was tested is what ships.

The explicit stable-release request is a comment on the Release issue whose first line is
`Stable release: vX.Y.Z`, written after the Release's rolling snapshot by a person (not a bot) with write, maintain,
or admin access. An agent never writes it; `aayu release --to main` refuses without it.

`aayu release <release-issue> --to rolling|main` (WP7.8), which an agent may run, executes exactly the steps below
for a Release issue.
It also re-runs failed gate jobs once, stamps the stories the promotion shipped `released`, publishes the release
notes, records the shipping evidence on the ledger and in the Release, and makes the Goals above the stories due
for `measure`. `--dry-run` shows what would ship. The commands below are the same procedure by hand.

### `dev` → `rolling`

The source is the newest `dev` commit that already has tier-2 results for every gating group: an
`eval-results-<sha>-<group>` artifact on a completed `evals.yml` run of that commit, for each group
`go run ./internal/evals/cmd/eval -groups -gating` prints. That is what the gate's `evals-gate` downloads. Pushes to
`dev` and the nightly run share one concurrency group, so in a busy merge window the intermediate commits get no run
and only the newest does. If dev's newest commit has no results yet, either promote the newest older commit that has
them, or dispatch once on the newest (`gh workflow run evals.yml --ref dev`), wait for it, and check again. Never
dispatch again to get a better result. `aayu release <n> --to rolling` does this check, never dispatches, and names
both choices; `--at <sha>` promotes the older commit.

```sh
git fetch origin
src="$(git rev-parse origin/dev)"                 # or the newest older dev commit with results
for id in $(gh run list --workflow evals.yml --commit "$src" --status completed --json databaseId --jq '.[].databaseId'); do
  gh api "repos/{owner}/{repo}/actions/runs/$id/artifacts" --jq '.artifacts[].name'
done                                              # every gating group's eval-results-$src-<group> must be listed
ts="$(date -u +%Y%m%dT%H%MZ)"; cand="promote/rolling-$ts"; wt="../wt-promote-rolling-$ts"
git worktree add "$wt" -b "$cand" origin/rolling && cd "$wt"
git merge --no-ff --signoff "$src" -m "Rolling snapshot: $ts"
git push origin "$cand"
gh workflow run ci.yml --ref "$cand" -f gate=rolling && sleep 15
gh run watch "$(gh run list --workflow ci.yml --branch "$cand" --event workflow_dispatch --limit 1 --json databaseId --jq '.[0].databaseId')" --exit-status
git push origin HEAD:rolling                      # fast-forward to the gated commit
git tag "rolling-$(date -u +%Y-%m-%d)" && git push origin "rolling-$(date -u +%Y-%m-%d)"
git push origin --delete "$cand" && cd - && git worktree remove "$wt" && git branch -D "$cand"
```

The rolling gate (`gate=rolling`) is the full PR suite (build matrix and size budget, unit tests, lint,
generate drift, security scans, docs, and `release-snapshot`: a signed goreleaser snapshot that
`install.sh` must accept while refusing tampered copies) plus `integration` (tests under the `integration` build
tag, race detector), `benchmarks`, and `evals-gate`: the tier-2 skill eval results of the commit being promoted
(from `evals.yml`, which runs on every push to `rolling` and `main` and nightly on `dev`) must pass the WP3.4 bar and
not regress against `main`'s results. A second promotion on the same day uses a `rolling-<date>.<n>` tag.

### `rolling` → `main`

```sh
git fetch origin
v=vX.Y.Z; cand="promote/main-$v"; wt="../wt-promote-main-$v"
git worktree add "$wt" -b "$cand" origin/main && cd "$wt"
git merge --no-ff --signoff origin/rolling -m "Release: $v"
git push origin "$cand"
gh workflow run ci.yml --ref "$cand" -f gate=stable && sleep 15
gh run watch "$(gh run list --workflow ci.yml --branch "$cand" --event workflow_dispatch --limit 1 --json databaseId --jq '.[0].databaseId')" --exit-status
git push origin HEAD:main
git tag -a "$v" -m "Release $v: <summary>" && git push origin "$v" && sleep 15   # starts release.yml
gh run watch "$(gh run list --workflow release.yml --branch "$v" --limit 1 --json databaseId --jq '.[0].databaseId')" --exit-status
git push origin --delete "$cand" && cd - && git worktree remove "$wt" && git branch -D "$cand"
```

The stable gate (`gate=stable`) is everything in the rolling gate plus `extended-integration` (live tests behind
`AAYU_LIVE=1`; the first ones land in WP1.4/WP2.3) and `smoke` (unit tests and a binary smoke test on Linux,
macOS, and Windows). Security scans run in `security` on every gate.

Every commit on `main` is tagged and released. The annotated `v*` tag starts
[.github/workflows/release.yml](.github/workflows/release.yml), which needs no human step: it refuses a tag whose
commit is not on `main` or that is not annotated, builds with goreleaser, signs `checksums.txt` with cosign
(keyless), attests SLSA provenance, attaches SBOMs, checks that `install.sh` accepts the release, installs it
from GitHub on Linux and macOS, and publishes the brew cask, Scoop manifest, and winget PR. Resume a transient
failure with `gh run rerun <run-id> --failed`. If anything fails, do not promote: fix it through `dev`, publish
a new rolling snapshot, then retry. The current release is supported, plus security patches for the previous
two minor versions; record each minor version's end-of-life date in its release notes.

### Release publishing setup (founder)

`release.yml` publishes to package managers from separate jobs, each in its own GitHub environment so its token
never reaches the build job. Until these exist, those three jobs fail with a message naming what is missing;
the GitHub release itself is unaffected.

| Environment | Secret | Needs |
|---|---|---|
| `release-homebrew` | `HOMEBREW_TAP_TOKEN` | Repository `rahulnakmol/homebrew-tap`; a fine-grained token with contents read/write on it only |
| `release-scoop` | `SCOOP_BUCKET_TOKEN` | Repository `rahulnakmol/scoop-bucket`; a fine-grained token with contents read/write on it only |
| `release-winget` | `WINGET_TOKEN` | A fork `rahulnakmol/winget-pkgs` of `microsoft/winget-pkgs`; a classic token with `public_repo` (it opens a PR on the upstream repository) |

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
- CI's `commits` job checks both on every PR commit (the sign-off must match the commit author) and checks
  the PR title, which becomes the squash subject. Squash-merge commit messages must keep the sign-off.

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

- the verbs and plumbing tables in `docs/README.md` (between the `BEGIN GENERATED` / `END GENERATED` markers);
- `LAYERS.md`;
- `docs/RACI.md`;
- the frontmatter of `skills/<verb>/SKILL.md` (the body below the frontmatter is hand-written);
- `internal/mcp/schemas/*.json`;
- `internal/config/schema.json`, the config JSON Schema, from the structs in `internal/config` (WP1.2).

`CONSTRAINTS.md` is generated the same way from `internal/config/defaults.toml`, the default configuration
embedded in the binary (WP0.5). The generator in `internal/config/gen` holds each key's meaning and plan source
and fails if a key is added or removed without updating it.

Do not edit generated output by hand. Change the registry, run `go generate ./...`, and commit both. The CI
job `generate-drift` fails on any difference.

## Protected paths

These paths need a human CODEOWNER review (see [.github/CODEOWNERS](.github/CODEOWNERS)):

`.github/`, `skills/`, `ops/`, `seats/`, `CONSTRAINTS.md`, `internal/config/defaults.toml`, `.aayudha/config.toml`,
`.agents/`, `.claude/`.

## Identities and approvals

- Humans use their own GitHub account and token.
- Agent lanes and CI use a bot identity: a GitHub App (`[identity] app_id`, `app_slug`) or a machine user
  (`[identity] machine_user`, `machine_user_secret`). Without one, agents run under the human's identity (the
  WP1.4 day-one path) and their writes are tagged `identity:human-fallback`. `aayu auth status` shows which
  token and bot identity resolve, and warns on over-scoped human tokens (`workflow`, `admin:*`, `delete_repo`).
- The App private key never lives on a laptop. A laptop gets a short-lived (≤ 1 h) installation token from
  the token broker workflow (`aayudha-token.yml`, installed by `aayu init --gates`), dispatched with the
  human's own token and returned encrypted to a one-time age key.
- CI writes as the bot with the App's installation token (`actions/create-github-app-token`, passed as
  `AAYU_BOT_TOKEN`), never `GITHUB_TOKEN`; `aayu --ci` refuses bot and `ledger` writes without it
  (Appendix B D1).
- An approval counts only if it comes from an account with `type != Bot` that is not listed in
  `[identity] bot_logins`, and is not the PR author's.
- Secrets are named in config and stored with `aayu auth set-secret <name>` (value read from stdin) in the
  OS keychain, or the age-encrypted file under `$XDG_DATA_HOME/aayudha` on headless machines; CI passes them
  as `AAYU_SECRET_<NAME>` variables.

## Solo-mode policy

> This policy changes as the project gains people. Change it by PR, together with the matching ruleset and
> `.github/CODEOWNERS` changes.

**Today**: there is one human, `rahulnakmol`. GitHub never lets a PR author approve their own PR, and native
CODEOWNER review does not block a PR authored by the only code owner, so the CODEOWNER rule is enforced by a
gate instead (docs/BUILD-PLAN.md Appendix B, D2).

**Rule while there is one human**:

1. The `protected-paths` check ([.github/workflows/protected-paths.yml](.github/workflows/protected-paths.yml))
   fails on any PR that changes a path in `.github/CODEOWNERS`. It runs from the default branch's copy of the
   workflow, so a PR cannot weaken it.
2. The `protected-paths-codeowner-review` ruleset requires that check on `dev`. Its only bypass is the
   repository admin, and only through a pull request, never by direct push. So a protected-path PR merges
   only when the founder deliberately merges it with "bypass rules" (`gh pr merge --admin`). Agents never
   use the bypass on their own initiative.
3. GitHub records each bypass in the ruleset rule-suite history:

   ```sh
   gh api "repos/rahulnakmol/agentic-ayudha/rulesets/rule-suites?rule_suite_result=bypass&time_period=month"
   ```

4. `aayu audit` (WP7.6; hand-run reconciliations until then) lists every such bypass as a solo-mode bypass
   when a PR comment starting `Solo-mode bypass:` came before it and only the `protected-paths` check failed,
   and writes its report to `ledger` under `audit/<date>/`. It runs with a repository admin token, since only
   an admin can read rule suites. A bypass the report marks unexplained is an incident (see
   [SECURITY.md](SECURITY.md#tool-incident-plan)).

**When a bot identity exists (WP1.4)**: agent-authored PRs are opened by the bot, so the founder can approve
them as a real review. The `protected-paths` gate passes when every protected path in the PR has an approving
review of the head commit from one of its CODEOWNERs who is a human, not a bot login, and not the author
(`internal/identity`, run from the base branch). Submitting or dismissing a review re-runs the check
(`protected-paths-review.yml` → `protected-paths-rerun.yml`). Bypasses then remain only for PRs the founder
authors.

**When a second human seat is added** (`aayu seat add <seat> --human <login>`, WP5.1): the rule switches to
true second-human review: the `protected-paths-codeowner-review` ruleset requires one approval with native
CODEOWNER review, and its `protected-paths` check and admin bypass are removed. It takes two runs, because GitHub
reads CODEOWNERS from the base branch and never lets an author approve their own PR:

1. The first run writes `[seats.<seat>]`, the `aayudha:seats` block at the end of `.github/CODEOWNERS` (the seat's
   layer paths, and with two humans every protected path names both, so each can review the other), and the RACI
   section of `AGENTS.md`, and opens the onboarding issue. The founder merges these files with a solo-mode bypass.
2. Running the same command again once that change is on `dev` switches the ruleset.

`aayu seat remove <seat>` reverses it in one run: it returns the ruleset to solo mode first (gate check required,
admin bypass back), so the founder can merge the CODEOWNERS change as before. Seat changes are recorded on
`ledger` in `seats/changes.jsonl`.

## Third-party code

Every new dependency must have its licence recorded in
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) in the same PR. CI licence scanning (`go-licenses` and an npm scan in the
`security` job) gates it. This project is BSD-3-Clause ([LICENSE](LICENSE)); by contributing you agree your contribution is
licensed under it.
