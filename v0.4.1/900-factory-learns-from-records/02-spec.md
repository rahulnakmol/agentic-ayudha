# Specification: #900 the factory learns from its own records

No spec baseline exists (no `specs/900/` on `origin/ledger`), and no Story is filed, so no Completion Goal criteria
exist. Requirements trace to [#900](https://github.com/rahulnakmol/agentic-ayudha/issues/900) Scope and Decision
[#916](https://github.com/rahulnakmol/agentic-ayudha/issues/916), and to a proposed slice in
[05-implementation-plan.md](05-implementation-plan.md) (proposed, not filed). #916 names "ST4, ST5, ST10" of #900; no
such Stories are filed, so their content is unverified.

## Functional requirements

| ID | Requirement | Source | Proposed slice |
|---|---|---|---|
| FR1 | Each run ends with one outcome: success, flawed, blocked, or manual. | #900 Scope | 900-A |
| FR2 | A blocked outcome opens an ask. | #900 Scope; needs `aayu ask` of #880 | 900-B |
| FR3 | A finding or failure class seen three times files a Feedback issue, with no model. | #900 Scope | 900-C |
| FR4 | evolve runs on that trigger, within a week, and keeps its failing-eval-first rule. | #900 Summary and Scope | 900-D |
| FR5 | evolve refuses a change to a shipped skill: a skill under `skills/` in this repository, or one that `.aayudha/lock.toml` lists as an aayu skill in a product repository. | #916 Option 1 | 900-E |
| FR6 | A repository's own skills, such as `.agents/skills/verify-*/`, stay evolvable, eval-first. `[evolve] cooldown_days` (default 7) applies to them only. | #916 Option 1 | 900-E |
| FR7 | A finding about a shipped skill becomes Feedback for the aayu product. | #916 Option 1 | 900-E |
| FR8 | After 4 weeks of merge records, fix rounds, rework, and cost by lane and model show on the Build board. | #900 Scope; needs #897's merge record | 900-F |
| FR9 | A routing change cites that evidence. Routing evidence is built last. | #900 Scope | 900-F |

Decision #916 also names a sizing predictor ("The effort per size recalibrates on every merge. A milestone or mission retro
writes a predictor report that the slice skill reads"). #900's Scope does not list it. It is recorded here as an open
point, not a requirement.

## Non-functional requirements

| ID | Kind | Requirement | Source |
|---|---|---|---|
| NFR1 | Cost | Feedback filing on a repeated class uses no model. | #900 Scope |
| NFR2 | Quality | evolve keeps failing-eval-first: a case that reproduces the problem on the old skill comes before the change. | #900 Scope; `internal/cli/evolve.go`, `runEvolve` comment |
| NFR3 | Quality | evolve never merges; a person reviews its PR. | `internal/cli/evolve.go`, `runEvolve` comment |
| NFR4 | Quality | No eval bar is lowered. | `evals/README.md`, "The bars only rise" |
| NFR5 | Data | A routing number cites its source records. A missing field stays a gap (rule of #897's merge record). Inferred from #897. | #897 Scope |

## Points the sources do not settle

- "The Build board" (#900 Scope): the evidence does not show a Build board on 884d81b (search of `internal/cli/` and
  `internal/status/`). Which surface it means is unverified. #895 or #975 (inbox waves) may hold it. Unverified.
- What counts as one "failure class" is not defined. `inspect.Finding.Class` exists for review findings
  (`internal/inspect/inspect.go:63-80`). A class for CI or build failures does not exist. Inferred.
