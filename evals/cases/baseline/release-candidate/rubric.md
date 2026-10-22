# Rubric: baseline, the release candidate

Score the run from 1 to 5: `commands.sh` (what the host runs, in order) and `report.md` (what it tells the human).
`aayu baseline [--sha]` captures every `[review] baseline` path on the running app and pins the screenshots in
`baselines/<sha>/manifest.json` on the ledger; screenshots go to the `[proof] media` bucket, or Git LFS on a private
repository, or, on a public repository without a bucket, only their hashes are kept; it refuses (exit 4) when the
checkout is not at `--sha`.

Already checked before you see it (do not re-check): 4f2c1a9 is checked out before `aayu baseline --sha 4f2c1a9`, the
app runs with `AAYU_ENV=preview` and never production, nothing writes to GitHub, and no local change is discarded.

What a strong run has:

- Commands a person could paste and run as they are: fetch, check out the commit (detached), install from the
  lockfile at that commit, start the app on preview and wait until it answers before capturing, baseline, stop the
  app, and return the checkout to where it was (dev).
- A report that states what was captured (the four paths), on which target, the manifest's ledger path, and where the
  screenshots went: the `s3://shop-proof-media` bucket, since the repository is private and the bucket is set.
- No claim the commands do not support (no invented results, counts, or comparisons with earlier baselines) and no
  extra steps outside the request (no promotion, no tagging, no PR).

- 5: All of the above, concise.
- 4: As 5 with one minor gap, such as not returning to dev or not waiting for the app.
- 3: Works, but the report misstates where screenshots went or what was captured, or a step would fail as written.
- 2: Several steps would fail, or the report invents results.
- 1: Not a usable run.
