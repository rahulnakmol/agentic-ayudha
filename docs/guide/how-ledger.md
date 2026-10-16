# The ledger branch

The ledger is an orphan branch named `ledger` in the product's own repository. It holds records: Ready snapshots,
plans, proof packs, ship records, build state, actions segments, audit reports, and release records. It never holds
code and is never merged. This page explains why it is a branch, what is on it, who writes it, and how many writers
share it without losing a record.

![Every record lands on the ledger as one fast-forward commit, retried from a fresh tip until it wins the race.](img/how-ledger.svg)

*One write is one commit. A writer that loses the race fetches the new tip, re-applies its change, and tries again.*

## Why a branch in the same repository

- **The records travel with the repository.** Fork it, mirror it, or move it to another host and the evidence comes
  along. There is no database to run and no service to keep alive.
- **Git already gives integrity.** Every record is content-addressed by Git, every change is a commit with an author
  and time, and history cannot be rewritten without a force push, which the ruleset forbids.
- **It stays apart from the product.** An orphan branch shares no history with `dev`, `rolling`, or `main`, so a
  record never shows up in a code diff, a release, or a merge.
- **GitHub permissions apply.** A repository ruleset decides who may push it, the same way rulesets guard `main`.

`git` and `git-lfs` are required for ledger work: the Go Git library has no support for linked worktrees or LFS.
Proof media never go to the ledger; Git LFS here holds only design mockups and UI baselines on a private repository,
and the media of packs pushed before 0.2.

## Layout

The real layout today, from `git ls-tree -r --name-only origin/ledger`, grouped:

| Path | Contents | Written by |
|---|---|---|
| `specs/<n>/ready.md` | Ready snapshot: the body and Completion Goal hashes at Ready | `aayu ready` |
| `specs/<n>/plan.json` | The story's plan: footprint, checkpoints, plan-time review or bypass | `aayu plan` |
| `proof/<story>/<sha>/<authority>/pack.json` | A proof pack for one head commit; `<authority>` is `authoritative` (the merge gate's, landed by `aayu ship`) or `advisory` | `aayu ship`, or `aayu proof push` |
| `proof/<story>/<sha>/<authority>/artifacts/…` | Evidence, coverage, and floor reports the pack hashes (text; never media) | same as the pack |
| `proof/<story>/<sha>/authoritative/ship.json` | The ship record: merge commit, human pass, streak | `aayu ship` |
| `build/<n>/state.json` | Where `aayu build` is in a story | `aayu build` |
| `actions/<date>/<host>-<run>.jsonl` | Actions segments: the hash-chained write log | every verb that writes |
| `audit/<date>/` | Audit reports and their GitHub input snapshots | `aayu audit` |
| `retro/` | Retro reports and `retro/incidents.jsonl` | `aayu retro`, `aayu respond` |
| `releases/<n>/<target>.json` | One promotion: commits, gate runs, artifacts, stories released | `aayu release` |
| `measure/due/<n>/<tag>.json` | When a Goal or Initiative is due to be measured | `aayu release` |
| `tell/release/<tag>.json` and `.md` | Release notes and their sources | `aayu tell` |

Helpers such as `ready.LedgerPath`, `proof.Dir`, and `actions.Log.SegmentPath` build these paths. This repository's
ledger has no `retro/` directory yet, because no retro has run here.

## How a write works

Every write goes through `ledger.Writer.Write` in [internal/ledger/ledger.go](../../internal/ledger/ledger.go). A
`Commit` carries whole files to replace, bytes to append, and paths to delete. The writer:

1. Checks every path is clean and relative.
2. Takes a lock file, `aayudha-ledger.lock`, in the clone's shared Git directory, so two processes on one machine,
   even in different worktrees, never race. A lock older than 10 minutes is taken over, since its holder died.
3. Fetches the remote ledger tip into a private ref, `refs/aayudha/ledger-tip/<random>`.
4. Checks out that tip in a temporary linked worktree, outside the user's checkout, and applies the change.
5. If nothing changed, returns the tip and pushes nothing. Otherwise it commits, adding an `Aayudha-Identity:` trailer
   when the writer carries a tag such as `identity:human-fallback`.
6. Pushes the new commit to `refs/heads/ledger` as a plain push. With LFS enabled, it pushes LFS objects first.

The push is the compare-and-swap. It succeeds only as a fast-forward of the tip it was built on. If another writer got
there first, Git rejects it, and the writer fetches the new tip, resets the worktree, re-applies the same change, and
pushes again. It tries up to 10 times, waiting 100 ms doubling each time, with jitter, capped at 5 seconds, then fails
with `ErrContended`. A ruleset refusal is a different error and is not retried.

Two design choices make retries safe:

- **Appends, not rewrites, for logs.** Actions segments and `retro/incidents.jsonl` use `Commit.Append`, so a retry
  adds its lines to whatever is at the new tip.
- **One path per fact.** Packs live under their story, head commit, and authority. Two writers almost never touch the
  same file, and `ledger.ContentPath` gives content-addressed names where two writers might.

Two builders on one story are kept apart by a **lease** on GitHub, not on the ledger: `aayu build` sets the story's
assignee and a lock comment with an expiry four hours out (`LeaseTTL` in `internal/cli/lease.go`), `aayu gc` reclaims
expired leases, and `aayu ship` releases it.

Every ledger push is itself an action (`ledger.push`) in the actions log, with the paths and message, except the push
of an actions segment, which carries the log itself.

## Key data

The ledger's own README, from `git show origin/ledger:README.md`, trimmed:

```text
# `ledger` branch

This orphan branch holds aayudha's own proof record for this repository. It
shares no history with `dev`, `rolling` or `main` and is never merged into them.
…
## Who writes here

Only CI and the bot identity write to this branch (see BUILD-PLAN.md §0.1,
Trust model). The bot identity does not exist until WP1.4. Until then, under the
WP0.1 day-one rule, the founder's identity may also push here. The `ledger-writers`
ruleset enforces this. `init --identity` tightens it once the bot exists.

## No media in LFS

This repository is public, so proof media never goes into Git LFS on this
branch (decision V3). LFS objects cannot be purged from GitHub, and recordings
and transcripts can contain PII. Git holds `pack.json` and hashes only. Media
lives in the private bucket configured as `[proof] media`.
```

A Ready snapshot, from `git show origin/ledger:specs/131/ready.md`, trimmed:

```text
# rahulnakmol/agentic-ayudha#131: Answers quote the best plan passage; policy refuses or refers the rest

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `8cb726034148c5b09e52e62608a4e051745005556e9fd7c7ab7fea94bd1a02dc` |
| Completion Goal hash | `e6e99c0c0e6aa4c1d6044a1ae26d91d9df3530e0a18d9d0b5883bd0d796e28cb` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/131/ready.md` |
…
Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.
```

A release record, from `git show origin/ledger:releases/144/rolling.json`, trimmed:

```json
{
  "repo": "rahulnakmol/agentic-ayudha",
  "release": 144,
  "source": "dev",
  "target": "rolling",
  "source_sha": "a631247bd29c6e3252bd82a9d4a07f796f594448",
  "commit": "9991cd392d282692aa9f5309d67ecc5c3362627a",
  "message": "Rolling snapshot: 20261012T1447Z",
  "tag": "rolling-2026-10-12.4",
  "candidate": "promote/rolling-20261012T1447Z",
  …
  "stories_released": [
    132,
    133,
    134
  ],
  "already_released": [
    130,
    131
  ],
  "release_notes": "tell/release/rolling-2026-10-12.4.md",
  …
  "promoted_by": "rahulnakmol",
  "promoted_at": "2026-10-12T14:47:06Z"
}
```

## Who may write

`aayu init` makes sure the ledger is protected by two rulesets. It adopts an existing ruleset that already blocks
`deletion` and `non_fast_forward` on `refs/heads/ledger`, or creates:

- **`aayudha-ledger`**, with the `update` rule. Its bypass actor is the repository's write role, which also covers
  maintain and admin, so anyone with write access can run `aayu ship` and land the merge gate's pack and the ship
  record with their own token.
- **`aayudha-ledger-history`**, with `deletion` and `non_fast_forward` and no bypass actor. A bypass actor bypasses
  every rule of its ruleset, so these rules live apart, and nobody, an admin included, force-pushes or deletes the
  ledger without first changing a ruleset (an audited event).

`aayu init --identity` creates the bot GitHub App and adds it as a second bypass actor of `aayudha-ledger`, for agent
lanes with broker tokens. Rerunning `aayu init` on a repository set up before 0.2, whose single `aayudha-ledger`
ruleset let only admins write, creates `aayudha-ledger-history` first and then narrows `aayudha-ledger` to updates with
the write role added. Shipping merges into `dev` only; promotion to `rolling` and `main` stays a separate release step
behind its own admin-only rulesets.

A private personal repository on GitHub Free has no rulesets: GitHub answers 403 "Upgrade to GitHub Pro or make this
repository public". `aayu init` then says the rulesets were not applied and why, records it in `.aayudha/state.json`,
and finishes the other steps. The ledger branch stays unprotected, so anyone with write access can force-push or
delete it, and the `ledger-rulesets` row of `aayu doctor` warns until you make the repository public or upgrade and run
`aayu init` again.

CI writes nothing to the ledger. On a personal repository the GitHub Actions app cannot be a ruleset bypass actor, so
`GITHUB_TOKEN` can never write the ledger, and the proof design does not need it to: the merge gate keeps its pack as
a run artifact and comments on the pull request, and a human's `ship` lands it. See
[trust model and identities](how-trust.md).

## Retention

Pack JSON, hashes, and data artifacts stay on the ledger. Media (screenshots, recordings, screen-reader transcripts)
never come here:

- **Every repository:** media stay in the merge gate run's `aayudha-pack` artifact. GitHub deletes it after
  `[proof] retention_days` (default 90), at most the repository's artifact retention limit: 90 days on a public
  repository, up to 400 on a private one. The ledger copy of `pack.json` keeps each media file's SHA-256 and size,
  with `"store": "none"`.
- **Opt-in, longer:** with `[proof] media` naming a private bucket, `aayu ship` also copies the media there, and the
  bucket's own lifecycle rule (`EnsureLifecycle` in `internal/proof/bucket.go`) expires them after `retention_days`.

There is no purge job and no loop: nothing on the ledger expires, and nothing outside it needs a person. Packs from
before 0.2 may still hold media in Git LFS on a private ledger, or a `tombstone` from the 0.1 `proof purge`; `aayu
proof verify` reads both.

## Invariants

1. A ledger write never overwrites another writer's commit: pushes are fast-forward only, and a lost race re-applies on
   the new tip. Enforced in `internal/ledger/ledger.go` (`TestConcurrentWritersAllLand`, `TestLiveConcurrentWriters`)
   and by the ruleset's `non_fast_forward` rule.
2. A write that changes nothing pushes nothing, and `--dry-run` writes nothing. Enforced in `internal/ledger/ledger.go`
   (`TestNoopDryRunAndErrors`).
3. Actions segments are appended, never replaced. Enforced in `ledger.Sink` (`TestSink`).
4. The ledger cannot be deleted or force-pushed by anyone, and only a human with write access or the bot App can
   update it. Enforced by the two rulesets that `internal/initrepo/init.go` creates (`TestInitTwiceDryRunPublicCoexistence`,
   `TestLiveLedgerWriters`) or the one it adopts (`TestInitAdoptsLedgerAndUninit`).
5. Proof media never go to the ledger or Git LFS, and CI never writes the ledger. Enforced in
   `internal/proof/ledger.go` (`TestMediaNeedABucket`) and `internal/cli/proof.go` (`TestProofPushVerifyCLI`,
   `TestProofCommentInMergeGate`).
6. Tokens never appear on a Git command line: the writer passes them as an HTTP header through `GIT_CONFIG_*`
   environment variables, in a scrubbed environment. Enforced in `Writer.git` in `internal/ledger/ledger.go`.

## Where it lives in the code

- [internal/ledger/ledger.go](../../internal/ledger/ledger.go): the writer, reads of the tip, and the actions sink.
- [internal/proof/ledger.go](../../internal/proof/ledger.go): pack push and best pack.
- [internal/cli/ship.go](../../internal/cli/ship.go): landing the merge gate's pack and the ship record.
- [internal/cli/lease.go](../../internal/cli/lease.go): story leases.
- [internal/cli/release.go](../../internal/cli/release.go): release records.
- [internal/initrepo/init.go](../../internal/initrepo/init.go) and
  [internal/initrepo/extras.go](../../internal/initrepo/extras.go): the ledger branch, its rulesets, and the App bypass.

> **Honest status:** no bot GitHub App is installed on this repository yet, and agents write under the founder's
> account. Its own rulesets predate `aayu init`: `ledger-writers` restricts updates, with the write role as its bypass
> actor, and `ledger-history` blocks force pushes and deletion for everyone (Appendix B D-4.2b.h). CI does not write
> the ledger: the merge gate leaves the authoritative pack as a run artifact, and `aayu ship` lands it under the
> founder's identity, saying so in `pack_landed`. 91 ledger commits so far carry the
> `identity:human-fallback` trailer. The ledger README predates the current layout: its table still names `packs/`,
> where the code writes `proof/`, and it still says CI and the bot write here and that media live in a bucket. No verb
> writes that file (it was written by hand when the branch was created), so it is left as it is and this page
> describes the real layout. This repository is public and `[proof] media` is empty, so no media has been pushed here.

## Read next

- [Proof packs](how-proof-packs.md): what is inside `pack.json`.
- [Configuration, secrets, and the audit trail](how-config-and-audit.md): how actions segments are reconciled.
