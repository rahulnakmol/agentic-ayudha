# UX design: #899 a finding blocks a merge only with proof

The person reads inspect's comment on the PR and the cards in `aayu inbox`. The builder agent reads inspect's result
and the PR body template. Nothing of this Feature is built.

## Today (delivered building blocks)

`aayu inspect --help` on [884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b):

```text
Adversarially review a PR against its plan, pack, and criteria; return a structured disposition: merge, fix, or rebuild.

Flags:
      --approve aayu inbox   As the [autonomy] approver, record your review of the PR's head on the ledger and post it
                             as the human review the merge gate reads; a person-verified step, and the only way a human
                             review counts.
      --dismiss string       Record the finding with this ID as dismissed in this repository, so inspect stops raising
                             findings like it.
      --mode string          general: one reviewer; peer: two reviewers of different families cross-check each other
                             (default from [inspect]). One of: general, peer.
      --reason string        Why the finding is dismissed (with --dismiss), or what you checked (with --approve).
```

Today a builder answers every finding in the PR body's RAID log: "fixed (with the test), declined (with the reason),
or filed (with the issue)" (`AGENTS.md`, Code review). After three judged rounds inspect says `step-back`, and only a
person reopens with `/aayu review again` (`internal/inspect/inspect.go`, `MaxRounds`, `ReopenCommand`).

## Planned (planned, not built)

| Moment | Who | What they see (from #899 and #910) |
|---|---|---|
| A P0 or P1 is raised | Builder and person | The finding comes with a probe. The comment says whether the probe fails on the head. The text is not stated. |
| The probe fails | Builder | The finding blocks until it is fixed. |
| No probe, or the probe passes | Person | The finding is filed as its own Bug at its severity; the PR can merge. |
| A P0 on a CODEOWNERS path | Person | It blocks, proven or not. |
| A new P2 or P3 after `merge` | Builder | It is under a "Follow-up" heading; one Bug is filed for the PR; the disposition stays `merge`. |
| A declined finding | Builder | No RAID row is needed (#899 Scope). |
| Merge record | Person, later | Finding precision per PR (#899, with #897). |

## Agent-facing surface

- The builder skill (`skills/build/**`) and the delegation brief (`internal/delegate/brief.go`) tell a builder how to
  act on findings. Both are Expected paths of #899, so both change with the RAID rule. Inferred.
- The reviewer prompt asks for a probe or patch per finding. The prompt file is not named in the issue. Inferred.

## Accessibility

No new screen. The inspect comment is Markdown on GitHub.
