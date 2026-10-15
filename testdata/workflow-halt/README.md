# Recorded halt runs (Bug #173)

On 2026-10-15, `floors.yml`, `inspect-live.yml`, and `screen-reader.yml` from branch
`fix/173-workflows-halt` ran on the scratch repository `rahulnakmol/aayu-scratch-halt-1792045632` (public,
deleted after the runs):

- **halted**: with the repository variable `AAYUDHA_HALT` set, `floors.yml` on the push of the branch's tree
  (commit `2222cf6`) and the other two on `workflow_dispatch`;
- **resumed**: after `AAYUDHA_HALT` was deleted, `floors.yml` on a push touching `packages/floors/` (commit
  `54dc11e`) and the other two on `workflow_dispatch`.

`<phase>-<workflow>-jobs.json` is each run's jobs and steps from
`GET /repos/{owner}/{repo}/actions/runs/{id}/jobs` (name and conclusion only); `runs.json` lists the runs.
`TestHaltedWorkflowRuns` (`workflows_test.go`) checks them against the current workflows.
