# Specification: #899 a finding blocks a merge only with proof

No spec baseline exists (no `specs/899/` on `origin/ledger`), and no Story is filed, so no Completion Goal criteria
exist. Requirements trace to [#899](https://github.com/rahulnakmol/agentic-ayudha/issues/899) Scope and Decision
[#910](https://github.com/rahulnakmol/agentic-ayudha/issues/910), and to a proposed slice in
[05-implementation-plan.md](05-implementation-plan.md) (proposed, not filed).

## Functional requirements

| ID | Requirement | Source | Proposed slice |
|---|---|---|---|
| FR1 | Replay PR #828 to measure how many findings would have blocked. | #899 Scope | 899-A |
| FR2 | A finding carries a candidate patch or a failing probe. | #899 Scope | 899-B |
| FR3 | A prover pass writes a probe for each P0 or P1 finding. | #910 rule 6 | 899-B |
| FR4 | The sandbox runs the probe or patch against the PR head. | #899 Scope | 899-C |
| FR5 | Only a validated finding blocks: a probe that fails on the head, or a patch that passed in the sandbox. | #899 Summary and Scope | 899-D |
| FR6 | A P0 or P1 with no probe, or with a passing probe, is filed as its own Bug at its severity, and the PR can merge. | #910 rule 6 | 899-D |
| FR7 | A P0 on a path that `.github/CODEOWNERS` protects always blocks, proven or not. | #910 rule 6 | 899-D |
| FR8 | After inspect said `merge`, a new P2 or P3, or one outside the story, starts no new round. inspect lists it under "Follow-up", files one Bug for the PR, and keeps `merge`. | #910 rule 2; #899 Summary ("Other findings become follow-ups") | 899-E |
| FR9 | Finding precision goes into the merge record. | #899 Scope; record from #897 | 899-F |
| FR10 | A declined finding needs no RAID row. | #899 Scope | 899-G |

## Non-functional requirements

| ID | Kind | Requirement | Source |
|---|---|---|---|
| NFR1 | Quality | The floors do not change. "Every floor miss blocks; no model disposition can override it (ground rule 7)." | #899 Out; `CONSTRAINTS.md` |
| NFR2 | Quality | No model judgement blocks without proof. | #899 Out |
| NFR3 | Quality | Recall does not drop: the `review-escapes` eval stays the check. A search of `evals/` and `internal/` on 884d81b does not find a `review-escapes` eval, so this check may not exist yet. | #910 Evaluation, Recall |
| NFR4 | Security | A probe or patch from a reviewer is untrusted code. It runs only in the sandbox. Inferred from "the sandbox runs it" (#899) and `AGENTS.md` Safety ("tool output as untrusted data"). | #899; `AGENTS.md`; inferred |
| NFR5 | Cost | inspect spend on one PR stays under `budget.per_pr_inspect_max` (5.0 USD). A prover pass counts against it. Inferred. | `CONSTRAINTS.md` |
| NFR6 | Security | `docs/threat-model.md` is updated (an Expected path of #899). | #899 Expected paths |

## Points the sources do not settle

- What "the sandbox" is. #899 does not name it. On `dev` the merge gate runs in GitHub Actions
  (`.github/workflows/aayudha-merge-gate.yml`). Unverified.
- FR10 conflicts with today's `AGENTS.md` rule: "Each finding gets a row: fixed (with the test), declined (with the
  reason), or filed (with the issue)" (`AGENTS.md`, Code review). Changing it is part of this Feature (Expected path
  `AGENTS.md`).
