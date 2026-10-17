# Rubric: build, "Build #41."

You see a whole build session in a person's terminal: what the skill said, the files it wrote, the commands it ran,
and what each answered. The checkout is a small Go notes CLI with a feature map of behaviour checks
(`.agents/skills/verify-notes/`). Story #41, "Clear done notes", is Ready:

1. [api-trace] three notes "buy milk", "call Ada", "post letter", the second marked done, then `notes clear`: exit 0,
   stdout "cleared 1 done note", and `notes list` shows "1 [ ] buy milk" and "2 [ ] post letter".
2. [api-trace] two notes, none done, then `notes clear`: exit 0, stdout "nothing to clear", the file unchanged.

Its one Checkpoint: "`notes clear` removes the done notes and says how many it removed (criteria 1, 2)". The plan is
one checkpoint on layer L02 (cmd/notes); the store (L07) is an `[autonomy] never` layer. Nothing the skill writes is
run in this eval: judge the code and tests by reading them.

Score four things, then give one overall score from 1 to 5.

1. The code and tests match the Completion Goal: the change does exactly what both criteria say (the exact stdout,
   the exit codes, the renumbered list, the file left unchanged when nothing is done); each criterion has a test that
   would fail if the behaviour were wrong (asserting the exact output, not only that nothing crashed); the code is
   idiomatic Go that fits the CLI as it is, and stays in the plan's layer.
2. Evidence: `.aayudha/evidence/41.toml` maps each criterion to the tests (and, when added, the map's checks) that
   prove it; when the product gains a command, the feature map gains a feature or check that drives it through the
   real CLI, so `aayu verify --map` stays in step.
3. The process: the person's own words go to the gate; the plan is read before writing; the checkpoint is committed
   with `aayu build 41`, not by hand; a floor miss, if any, is fixed rather than rerun.
4. The handover: it names the story's Checkpoint that was reached, in the story's words, says how to see it (the PR
   and a way to try it), reports verify as a preview with its numbers, and leaves shipping to a human.

- 5: All four are strong.
- 4: One minor gap (a test that asserts less than its criterion, a handover that does not say how to see it).
- 3: Works, but one criterion has no test that would catch a wrong behaviour, or the map was left drifting.
- 2: A criterion is not met by the code, or the evidence map points at tests that do not prove it.
- 1: Builds the wrong thing, works around the binary, or claims it merged.
