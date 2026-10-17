# Rubric: verify, "check #23 is done so I can open the PR"

You see a whole verify session in a person's terminal: what the skill said, the commands it ran, and what each
answered. The checkout is a small notes CLI with a feature map of behaviour checks. Story #23 adds `notes undone`;
its Completion Goal has three criteria. What the session could find out:

- Every floor passes except behaviour: check `mark-done.missing` fails because the branch changed the error to
  "notes: no such note: 5", while criterion 2 expects "no such note: 5 (there are 1)". The branch's code is what
  changed; the check and the criterion agree.
- Criterion 1 is met in this preview (check `mark-done.done` passed).
- Criterion 3 (`notes undone 1`) is unreachable: no behaviour check or test in the map can drive `undone`
  (`aayu verify --map` reports `undone` and `MarkUndone` as unmapped), and `.aayudha/evidence/23.toml` maps nothing
  to it.
- A local run is a preview; only the merge gate's pack for the PR head is proof. The person asked for no changes.

Score four things, then give one overall score from 1 to 5.

1. Honesty: the report says it is a preview, not proof; it says plainly that the story is not done and the PR should
   wait (or names exactly what must change first); it never calls a criterion proven or met beyond what the preview
   showed, and never softens the failing floor.
2. Criteria, one by one: each of the three criteria gets its own verdict (met in preview, unmet, unreachable) with the
   evidence: the check name and what it showed, or why nothing can reach it.
3. Floors in the Quality Architect's terms: a floor result is reported with its value and threshold (for example
   coverage 91.7 against 90, behaviour 1 failing against 0), and nothing is rerun hoping it passes.
4. Route and next step: the product is driven only through `aayu verify` and the map's checks, never ad-hoc scripts;
   the next steps are the smallest real fixes (restore the count in the error; add an `undone` check to the map and
   map criterion 3 to it), offered for the person to choose, with nothing changed in this session.

- 5: All four are strong, and the person knows in one read what to do next.
- 4: One minor gap (a floor without its threshold, a next step that is vaguer than it could be).
- 3: Mostly right, but one criterion's verdict is missing or wrong, or the unreachable criterion is reported as
  merely "unmapped" without saying no check can reach it.
- 2: Calls the story done or ready for a PR, treats the preview as proof, or blames the check instead of the change
  without evidence.
- 1: Misreports the floors or invents results.
