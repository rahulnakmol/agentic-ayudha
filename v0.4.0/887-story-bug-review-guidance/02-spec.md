# Spec: Review guidance

The spec baseline v1 (`specs/887/baselines/spec-v1.json` on the ledger, hash `eee927df73d9`, approved by
@rahulnakmol on 2026-11-02T17:21:28Z) pins the three Stories at their Completion Goal hashes: #1221 (`14816886…`), #1222 (`5eb0dbb4…`), and #1223 (`66a0d70c…`). The requirements below come from those Story bodies. None is built.

## Functional requirements

| # | Requirement | Story and criteria |
|---|---|---|
| FR1 | A `#### Review guidance` block under Assumptions, with `- Check:`, `- Ignore:`, and `- Risk:` lines, parses into checks, ignores, and risks, in order, through `workitems.ParseGuidance`. | [#1221](https://github.com/rahulnakmol/agentic-ayudha/issues/1221) criterion 1 |
| FR2 | `aayu ready <n> --check` fails an Ignore that names security, data, concurrency, a floor, or a P0 or P1 area, with the message `Review guidance: an Ignore may not name security, data, concurrency, a floor, or a P0 or P1 area`. | #1221 criterion 2 |
| FR3 | A Story with no block passes Ready with the warning `no Review guidance: the reviewer gets none`. | #1221 criterion 3 |
| FR4 | A block line that starts with neither Check, Ignore, nor Risk fails Ready, and Ready names the line. | #1221 criterion 4 |
| FR5 | `aayu slice <f> --draft` refuses a story whose Ignore names a limited area, with the same reason as Ready, before it creates anything. | #1221 criterion 5 |
| FR6 | A Bug's guidance parses and passes as a Story's does. | #1221 criterion 6 |
| FR7 | The brief for `aayu build <n> --delegate <receiver>` has a `## Review guidance` section with the snapshot's checks, ignores, and risks, inside the untrusted quote. | [#1222](https://github.com/rahulnakmol/agentic-ayudha/issues/1222) criterion 1 |
| FR8 | The brief uses the Ready snapshot's guidance, not an edited live body. | #1222 criterion 2 |
| FR9 | A snapshot with no guidance gives a brief with no Review guidance section. | #1222 criterion 3 |
| FR10 | A guidance line that holds an instruction stays data inside the untrusted block; the brief's rules do not change. | #1222 criterion 4 |
| FR11 | The review prompt has a `Story guidance` block with the snapshot's checks and risks inside the untrusted data, and the summary says which checks it covered. | [#1223](https://github.com/rahulnakmol/agentic-ayudha/issues/1223) criterion 1 |
| FR12 | A P3 finding inside an Ignore area is not posted and is listed in Dropped with `story guidance: ignore`. | #1223 criterion 2 |
| FR13 | A P1 finding, or a security, data, or concurrency finding, inside an Ignore area is posted, and the comment notes that the Ignore did not apply. | #1223 criterion 3 |
| FR14 | inspect uses the snapshot's guidance after a live body edit, and `DiffHash` does not change. | #1223 criterion 4 |
| FR15 | A replaced snapshot with other guidance and the same diff changes `DiffHash`, so the cached verdict is not reused. | #1223 criterion 5 |
| FR16 | A story with no guidance gives the same prompt and `DiffHash` as before the change. | #1223 criterion 6 |

## Non-functional requirements

| Area | Requirement | Source |
|---|---|---|
| Security | Guidance is untrusted text. It is quoted as data in the brief and the prompt. | #1222 criterion 4; #1223 criterion 1; ETHOS.md principle 6 |
| Security | An Ignore never drops a P0 or P1 finding, or a security, data, or concurrency finding, and never touches a floor. | #887 Scope; Decision #864 option 1 |
| Reliability | The Ignore limit check is a word list (security, auth, secret, token, data, migration, concurrency, race, lock, floor, coverage, P0, P1). It "is a guard, not a judge". | #1221 Notes |
| Cost | The guidance enters `DiffHash` only when present, so the review cache of stories with no guidance holds and no paid review reruns. | #1223 criterion 6 and Review guidance; `specs/887/design.json` contract 4 |
| Operability | Each dropped finding is listed in Dropped, so a person sees it. | #1223 Review guidance (Risk line); #864 option 1 |
| Test environment | go test of `internal/workitems`, `internal/delegate`, `internal/inspect`, and `internal/cli` with a fake GitHub, a fake ledger, and a fake review model; never production. | Environment section of #1221, #1222, #1223 |

## Differences from Decision #864 (for a person to confirm)

These are differences between the accepted Decision and the approved Stories. The evidence does not show that a
person chose them on purpose.

1. Decision #864 option 1 gives each line a fixed shape: `check:` tied to a Completion Goal number, `risk: <path>`
   (at most two), and `ignore: path <glob>`, `ignore: category <style|docs>`, or `ignore: settled #<Decision>`. The
   Stories use free text after `- Check:`, `- Ignore:`, and `- Risk:` (#1221 criterion 1; `specs/887/design.json`
   contract 1).
2. Decision #864 says an Ignore "never matches a path in `[autonomy] always_person`". No Story criterion names this
   limit (#1221, #1223).
3. Decision #864 says the field "is required at Ready". #1221 criterion 3 makes a missing block a warning
   (Provenance item 4: "default (scope)").
