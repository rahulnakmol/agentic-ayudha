# #879 UX design

The surfaces are `aayu triage`, `aayu ready`, the Bug issue form, and a TRIAGE card in `aayu inbox`.

## Today on dev

`aayu triage --help` at [884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b):

```text
Classify intake issues as Feedback, Bug, or Incident by label only; never edits bodies or touches non-intake issues.

Usage:
  aayu triage [issue] [flags]

Flags:
      --as string   Class chosen by the host's model; without it the binary asks the triage lane's model. One of: feedback, bug, incident.
  -h, --help        help for triage
```

A Story with no parent Feature gets: "#`<n>` has no parent Feature. A Story is Ready only under a Feature approved to
build (Decision #861)." (`internal/cli/readycover.go:26`). A Bug gets no such refusal today.

The Bug form's "Environment" field reads "Where the evidence is captured, with seeded or synthetic data only (V3)."
(`work-items/forms/aayudha-bug.yml`).

## Planned, not built

- `aayu triage <bug> --decide hotfix|feature|repro|not-a-bug` (#879) and `--reason <reason>` for not-a-bug (#954
  criterion 4). Help text not in any source.
- The Ready refusal for an untriaged Bug. Its wording is not in any source.
- A Bug form field "Where it happens" (#879, #861).
- Story [#954](https://github.com/rahulnakmol/agentic-ayudha/issues/954) (under #975) gives the TRIAGE card at
  120x34:
  - "TRIAGE · Bug #1240 · …", the repro fields each ✓, Shesha's counsel, and "1 Hotfix · 2 Carve out a Feature · 3
    Need a better repro · 4 Not a bug" (criterion 1).
  - With "Where it happens" empty: "The repro report misses: where it happens. Hotfix and Carve out are off. 3 asks
    the reporter." (criterion 2).
  - After 1: "✓ Triaged #1240 as hotfix by @rahulnakmol · 15:20 · Make Ready comes next." (criterion 3).

## Agent-facing surface

An agent can still classify intake with `aayu triage <issue> --as bug`. It cannot run `--decide`, which is
human-only (#879). An agent can run `aayu ready <bug> --check` to see the refusal before asking a person.

## Accessibility

The sources state no accessibility requirement beyond the global `--plain` flag.
