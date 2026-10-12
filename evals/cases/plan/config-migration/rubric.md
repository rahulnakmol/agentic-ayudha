# Rubric: plan, proof retention config migration (L07)

Score the plan from 1 to 5. The story migrates `[proof] retention_days` to a retention per artifact kind; its
footprint is L07 (operational data), so the plan has exactly one checkpoint, and it needs a rollback step. The
binary has already checked one checkpoint per footprint layer, tests on every checkpoint, the L07 rollback, and that
every criterion is covered; judge what those checks cannot.

Questions the skill lists under "open questions" are not requirements: they never lower the score.

- 5: The checkpoint's contract names the old and new config shape and how old values map; its tests would prove
  both criteria (the migrated values and the byte-for-byte rollback); the rollback step is concrete (how the
  previous file is restored or rewritten); instrumentation says what the migration records; nothing outside the
  story is planned.
- 4: As 5, with one minor gap, such as instrumentation that could be more specific.
- 3: Usable, but the rollback is vague ("revert the change") or a criterion's test is only implied.
- 2: The plan changes things the story does not ask for, or its tests cannot prove the criteria.
- 1: Not a usable plan.
