# UX design: #900 the factory learns from its own records

The person sees asks, Feedback issues, evolve PRs, and the Build board. The agent sees an outcome it must record at the
end of each run. Nothing of this Feature is built.

## Today (delivered building blocks)

`aayu evolve --help` on [884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b):

```text
Turn accepted retro Feedback into a failing eval case, then a skill diff that passes it, opened as an agent-authored PR.

Flags:
      --feedback aayu retro   Accepted Feedback issue filed by aayu retro that the change answers; without it, list the
                              accepted retro Feedback.
      --skill string          Skill to evolve: skills/<skill>/, or a stub under evals/skills/<skill>/.
```

`aayu retro --help` (the classify flags):

```text
--classify string   Feedback issue to classify by the highest layer that can fix its correction for good (needs
                    --layer and --reason); writes no report.
--layer string      With --classify: codebase (a type, invariant, or boundary), check (a lint rule, test, or CI check),
                    prose (skill or AGENTS.md guidance), or review (what is surfaced for human review).
```

A blocked build today prints `#<n>: status:blocked: <why> (page <sent>)` and a `next:` line
(`internal/cli/build.go:1200`).

## Planned (planned, not built)

| Moment | Who | What happens (from #900 and #916) |
|---|---|---|
| A run ends | The agent | It records one outcome: success, flawed, blocked, or manual. |
| A run is blocked | The person | An ask opens (#880). In the inbox it is a card. The card text is not stated. |
| A class repeats a third time | aayu, no model | A Feedback issue is filed. |
| Feedback is filed | aayu | evolve runs within a week, eval-first, and opens an agent-authored PR for a person to review. |
| A shipped skill is named | aayu | evolve refuses it. The finding becomes Feedback for the aayu product (#916). The refusal text is not stated. |
| 4 weeks of merge records exist | The person | The Build board shows fix rounds, rework, and cost by lane and model. Each routing change cites them. |

The `--skill` help names `skills/<skill>/` as a target. After #916 that target is refused in this repository. The help
text must change with it. Inferred.

## Accessibility

No new screen is specified. The inbox has a `--plain` mode (`aayu --help`).
