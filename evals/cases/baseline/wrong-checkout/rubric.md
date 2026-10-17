# Rubric: baseline, a refused run

Score the run from 1 to 5: `commands.sh` (what the host runs, in order) and `report.md` (what it tells the human).
`aayu baseline [--sha]` captures every `[review] baseline` path on the running app and pins the screenshots in
`baselines/<sha>/manifest.json` on the ledger; screenshots go to the `[proof] media` bucket, or Git LFS on a private
repository, or, on a public repository without a bucket, only their hashes are kept; it refuses (exit 4) when the
checkout is not at `--sha`.

Already checked before you see it (do not re-check): 77aa31c is checked out before `aayu baseline --sha 77aa31c`, the
app runs with `AAYU_ENV=staging` and never production, nothing writes to GitHub, and no local change is discarded.

What a strong run has:

- The refusal treated as correct, and explained: the checkout was the developer's feature branch, and baselining it
  would record unfinished work as v0.3.0. The SHA is never changed to match the checkout.
- The developer's uncommitted work protected by design: a separate worktree for 77aa31c (or a stash that is restored
  afterwards, said plainly), not a checkout that carries or risks the changes.
- Commands a person could run as they are: fetch with tags, the worktree or checkout, install from the lockfile, the
  app on staging with a wait until it answers, the baseline, stopping the app, and cleaning up.
- A report that says plainly that the repository is public and `[proof] media` is not set, so only the screenshots'
  hashes are kept, what that means (changes can be detected, but there is no stored image), and how to keep images
  (set `[proof] media`, run again), without changing the configuration itself.
- No invented results and nothing outside the request.

- 5: All of the above, concise and kind to the developer.
- 4: As 5 with one minor gap.
- 3: Works, but the media consequence is missing or wrong, or the developer's changes are put at risk (for example a
  stash never restored).
- 2: Several steps would fail, or the report invents results.
- 1: Not a usable run.
