# CSV export of filtered claims: what the evidence suggests next

Story #12 shipped a CSV export that writes exactly the filtered claims and the visible columns; its authoritative pack held every floor that ran, and usage beat the target at 4.2 exports per reviewer per week [P1, R].

## Evidence

- Coverage 94 against a threshold of 90; SAST, secrets, and deps at 0 findings; a11y at 0 critical and 0 serious under WCAG 2.2 AA; keyboard pass [P1].
- The screen-reader floor was skipped: the Linux merge gate cannot run a screen reader [P1].
- Criteria 1 and 2 are proved by API traces, criterion 3 (responsive past 10,000 claims) by a recording [P1, #12].
- Weekly exports per reviewer: 4.2 against a target of at least 3, from a baseline of 0 [R].

## Ideas

- Proposal: run the screen-reader floor for this story on a runner that has one, since the merge gate skipped it [P1].
- Proposal: turn criterion 3 into a `metric` criterion with a threshold, so responsiveness on large lists is a number and not only a recording [#12].
- Proposal: with usage above target, ask reviewers whether exports of other lists matter; the scope excludes them today [#7, R].
