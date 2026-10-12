# Incident: solo-mode bypass without a prior comment (PR #86) — 2026-10-12

Found by `aayu audit` (report audit-20261012T052742Z.md) as the only unexplained bypass: rule suite 4473766681,
`refs/heads/dev` 110cf35, pushed 2026-10-12T02:20:04Z by rahulnakmol, bypassing `required_status_checks`
(protected-paths-codeowner-review). PR #86 (WP4.8 merge gate and inspect) had no "Solo-mode bypass:" comment before
the merge; the M4 thread's report said it did, which was wrong.

Facts checked 2026-10-12T06:15Z:
- Only protected path changed: `.github/workflows/inspect-live.yml` (new; the WP4.8 live inspect acceptance workflow).
- Check runs on the merged head 3b74840: build, generate-drift, lint, commits, security, docs, release-snapshot,
  floors-web, floors-dast all success; `protected-paths` failure (by design). No required check other than
  protected-paths was bypassed.
- Merged by the founder account (agent acting under the founder's standing authorisation for solo-mode bypasses).

Classification: procedural lapse (missing record), not an unauthorised change. Remediation: a late
"Solo-mode bypass (recorded late …)" comment was posted on #86. Future audits should list this suite as
"solo-mode bypass, recorded late" by referencing this file.
