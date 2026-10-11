# `ledger` branch

This orphan branch holds aayudha's own proof record for this repository. It
shares no history with `dev`, `rolling` or `main` and is never merged into them.

## Layout

| Path | Contents |
|---|---|
| `packs/` | Proof packs (`pack.json` plus content hashes) |
| `actions/<date>/` | Action segments: the append-only log of writes made through `aayu` |
| `specs/<id>/` | Ready snapshots of work-item specs, frozen when an item becomes ready |
| `audit/<date>/` | Audit reports |

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
