# Goal and intent: you write behaviours

No intent baseline exists (`specs/891/` is absent on `origin/ledger`). The sources are the #891 body and Decision #867.

## Goal

"Each feature-map check carries your sentence, its rule, and its source; you approve the sentence, and a change to an
approved check waits for you." (#891 Summary).

The founder's rule, quoted in Decision #867: "Humans no longer write software, they write behaviours. AAYU is
designed for that world." (#867 Question).

## Who it serves and the outcome

- The founder: he writes and approves each behaviour sentence; agents write the checks (#867 option 1).
- Agents: a builder that thinks a behaviour is wrong runs `aayu ask` and "never edits a check to make the gate green"
  (#867 option 1).
- A behaviour says "what happens first, what is retried, who may do what, what must never happen, or a limit" (#867
  Question). The `rule` values are capability, order, retry, authority, never, or limit (#867 option 1).

## Problem

"The feature map holds capabilities that an agent writes. Nothing records your approval of a behaviour, and an
accepted Decision links to no check (V11, C35, G20, Q20)." (#891 Problem). Decision #867 adds: "this repository has no
map of its own" and "Any PR can change a check" (#867 Question). On `dev` at 884d81b both still hold: no
`.agents/skills/verify-*` directory exists at the repository root (`.agents/` holds only `check` and `setup`).

## Success measures

None is named. Decision #867 names what would change the rule: "if most BEHAVIOUR CHANGE cards keep the same
sentence, let the owner seat approve a change that keeps the sentence, through a later Decision" (#867
Recommendation).

## Out of scope

"a second register" (#891 Scope, Out). That was option 2 of Decision #867, a separate rule register such as
`decisions/rules.toml`.
