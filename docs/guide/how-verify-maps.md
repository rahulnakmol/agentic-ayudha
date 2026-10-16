# Verify skills, feature maps, and behaviour checks

A feature map is a list of what a product can do, kept in the repository, with a behaviour check for each capability.
A behaviour check drives the product the way a person or a client would: a UI flow, a CLI command, an API call, or a
library call. This page explains how `aayu verify` reads the maps, compares them with the source, runs the checks in
the merge gate with no model, and hands map upkeep to an agent app. It also covers the red-on-base proof for Bug
stories. It is for engineers who write a map, debug a failing check, or change this code.

![A pull request in the merge gate, or the nightly sweep, runs the feature-map floor and the behaviour checks; each check goes to the product's control tool, any failed check turns the gate red, map drift is advisory until a dated config value, and the maintain brief sends the fix to an agent app as one pull request.](img/how-verify-maps.svg)

*A failed behaviour check blocks at once. Drift in the map is a warning until `drift_blocking_from`, then it blocks.*

## How it works

### The verify skill and its map

Each product owns a project skill at `.agents/skills/verify-<product>/`. It holds three things: `SKILL.md`, a control
tool, and a `features/` directory. The skill can sit anywhere in the repository, so a monorepo has one per product.
`verify` finds maps with `git ls-files --cached --others --exclude-standard`. A map you are still writing is read
before you commit it, as long as git does not ignore it.

`features/README.md` holds one `map` block and the ordered index. The `map` block names the product, the control
tool's argv (run from the repository root), and surface rules. A surface is something a user can reach: a route, a
command, a flag, an exported function. Each rule has a kind, file globs, and a regular expression whose first group
is one surface. The index is the list of links to feature files.

Each other `features/<id>.md` is one capability. It has an H1, a paragraph, one `feature` block (`id`, `layers`,
`sources`, `surfaces`, `verified_at`), and four H2s in this order: `Sub-features`, `How to reach it`,
`Behaviour checks`, `Gotchas`. Under `Behaviour checks`, each `check` block has an `id`, a `kind` (`ui`, `cli`, `api`,
or `lib`), an `evidence` kind from the Completion Goal schema, `steps`, and an optional `requires`. The blocks are
strict JSON: an unknown field is a contract error.

`aayu init --verify-skill` writes a starter skill. It picks the control tool by the stack the repository has:
`go.mod` gives a Go tool, else `package.json` gives a Node and Playwright tool, else a shell stub that blocks until
you replace it. The starter map has no features and passes `aayu verify --map`. Init also asks which agent maintains
the map and offers only the agents it detects. `aayu install` copies root `verify-*` skills to `.claude/skills/` for
Claude Code; Amp, Cursor, and Codex read `.agents/skills/` directly.

### The drift check

`aayu verify --map` reads the maps and the checkout and runs no product code. It reports drift:

- contract errors (a block that does not parse, a missing H2, a check with no steps);
- feature files missing from the index, and index entries with no file;
- sources that match no file;
- handles absent from the feature's sources;
- surfaces the rules find that no feature lists, and listed surfaces the rules no longer find;
- evidence maps (`.aayudha/evidence/<story>.toml`) that name a check the map lacks.

A handle is a `name`, `label`, `within`, or `test` value of a step's action object, such as `{"click": {"name":
"Ask"}}`. The source must contain it quoted or as element text (`>Ask<`); a `test` handle, as a whole word. Values deeper in a step, such as a request body's `"name"`, are data, not handles.

A feature is stale when its sources changed since its `verified_at` commit. Stale is reported, never counted as
drift, including "never verified" (empty `verified_at`) and a commit missing from a shallow clone. `verify --map`
exits 3 on drift and 0 otherwise.

The `feature-map` floor runs the same check in every `verify` run, including the merge gate on every pull request. It
is `skipped` when the checkout has no map. Its value is the drift count, and it writes `reports/feature-map.json`
into the pack. Before `[verify] drift_blocking_from`, drift passes with a detail that starts "advisory until". From
00:00 UTC on that date, drift fails the floor and the gate. A pull request that serves no story still runs this floor.

### Behaviour checks

The `behaviour` floor picks the checks to run. The common case is a pull request that edits product code:

1. every check of a feature whose sources or feature file the change touches;
2. every check of a map whose README or control tool changed (a `SKILL.md` edit alone does not count);
3. every check a criterion names in the story's evidence map, as `checks = ["<feature>.<check>"]` beside `tests`.

`aayu verify --check <feature>`, `<feature>.<check>`, or `all` runs a chosen set instead. Outside the merge gate that
run is a preview. In the gate, a pull request that serves no story runs only the two map floors.

For each check, aayu runs `<control...> check` in the verify sandbox, sends the check as JSON on stdin, and reads one
JSON result from stdout. A `ui` or `api` check needs the running app. aayu first refuses a production target, then
starts the app with `[verify] seed`, and lets those checks use the network. `cli` and `lib` checks run with no
network. Each check has three minutes.

The control tool answers `passed`, `failed`, or `unreachable`:

- **failed** fails the floor, so the gate turns red at once.
- **unreachable** means a precondition in `requires` is missing. It proves nothing and fails nothing.
- **no valid answer** (malformed JSON, an unknown status, an artifact path outside the output directory, or an unknown
  artifact kind) blocks the floor.

A criterion mapped to checks is evidenced only when every one of them passed and produced an artifact of the
criterion's own kind. A `cli` or `lib` transcript uses the `api-trace` kind. aayu hashes every artifact into the pack
under `behaviour/<feature>.<check>/` and writes `reports/behaviour.json`, which lists each check, why it was selected,
how it ran on the network, and its result.

> **Honest status:** this repository has no feature map for aayudha itself. The two maps in it belong to the example
> products: `benefits-qa` (seven `ui` and three `api` checks) and `notes` (five `cli` checks and one `lib` check). No
> evidence map under `.aayudha/evidence/` names a behaviour check yet, so no criterion here is evidenced by one. In
> the first live gate run, `product-events.visit-and-ask` answered `unreachable`, because `serve.sh` starts no events
> sink.

### Keeping the map honest

No model key is ever put into a product repository's CI. The model-driven maintain pass runs in an agent app the user
installed instead. `aayu verify --maintain drift` or `--maintain always` works with `--map` or `--check` and posts the
maintain brief. The brief lists the drift, the failing checks, the edit scope (the skill directories only), and the
rules: never weaken a check, file a product regression as a Bug, and open at most one pull request.

The brief is a new comment on one tracking issue marked `<!-- aayudha:verify-maintain -->`, because GitHub notifies
a mention only in a new comment. In `drift` mode, aayu posts nothing when nothing drifted and no check failed, or when
the last brief carries the same digest of its report. `always` posts every time.

Who posts it matters, because the apps answer only the people who connected them:

- **Outside CI**, the brief is posted under the person's account and starts with the mention of
  `[verify] maintainer` (`@claude`, `@codex`, or `@cursor`). With `amp`, or with no maintainer set, there is no
  mention, and the Amp loop thread follows the brief.
- **In Actions**, aayu posts a record instead: no mention, and the text "Recorded by the Actions loop; no agent was
  woken". Records are de-duplicated against records only, so a nightly record never stops the person's own run from
  handing off the same drift.

Two loops run this. `verify-sweep` runs `verify --check all --maintain drift` nightly at 02:30; its job is red when
a check fails. `verify-maintain` runs `verify --map --maintain always` on Mondays at 09:00, so the app also looks for
capabilities the map misses. The person's side runs the same prompt through a scheduled `amp -x` or a Cursor
automation ([loops/README.md](../../loops/README.md)).

Map directories are not protected paths. `inspect.Weakened` flags a pull request that deletes a feature file or leaves
it with fewer `check` blocks or assertion keys (`expect`, `stdout`, `exit`, `test`, and others). A renamed handle is
not a weakening.

> **Honest status:** `doctor` does not report map drift; only `verify --map` and the floor do. An Actions loop never
> wakes an app, so the hand-off depends on the person's scheduled run. `install` does not mirror a nested map.

### Red-on-base proof for Bug stories

A test that proves a bug fix must fail on the code before the fix. In the merge gate, when the story carries the
`type:bug` label and the full floor set runs, `verify` adds a `red-on-base` check:

1. It reads the Go tests the story's evidence map lists under `tests`.
2. It checks out the merge base in a temporary worktree, with the pull request's changed test files on top.
3. It runs `go test -json -count=1 -run <mapped tests> ./...` there, in the verify sandbox. The changed test files
   are every `*_test.go` and `testdata` file in the diff.

Each criterion is then red on base (a mapped test failed), green on base (every mapped test passed), or inconclusive
(a mapped test did not run, for example because it calls an API that does not exist on the base). Inconclusive never
blocks. The check passes when no criterion is green on base. A green-on-base criterion makes the check `skipped`,
with a detail that starts "advisory until", before `[verify] red_on_base_blocking_from`. From that date it fails. An
empty date keeps it advisory.

> **Honest status:** red-on-base runs only in CI on a pull request, only for a Go module, and only for Go tests
> mapped by name; behaviour checks are not rerun on the base. Both dates are `2026-11-13`, so both are advisory
> today.

## Key data and file formats

The `map` block of the `notes` example, from
[examples/notes/.agents/skills/verify-notes/features/README.md](../../examples/notes/.agents/skills/verify-notes/features/README.md):

```json
{"product": "notes",
 "control": ["go", "run", "./examples/notes/.agents/skills/verify-notes/control"],
 "surfaces": [{"kind": "command", "files": ["examples/notes/cmd/notes/main.go"], "pattern": "case \"([a-z]+)\":"},
              {"kind": "flag", "files": ["examples/notes/cmd/notes/main.go"], "pattern": "fs\\.\\w+\\(\"([a-z-]+)\""},
              {"kind": "func", "files": ["examples/notes/store/store.go"], "pattern": "(?m)^func ([A-Z]\\w*)\\("}]}
```

The `feature` block and one `check` block of
[examples/notes/.agents/skills/verify-notes/features/mark-done.md](../../examples/notes/.agents/skills/verify-notes/features/mark-done.md):

```json
{"id": "mark-done", "layers": ["L02"],
 "sources": ["examples/notes/cmd/notes/main.go", "examples/notes/store/store.go"],
 "surfaces": {"command": ["done"]},
 "verified_at": "c514f9a54b6f60eedc10f619292e093ce314007d"}
```

```json
{"id": "missing", "kind": "cli", "evidence": "api-trace", "steps": [
  {"build": "./examples/notes/cmd/notes"},
  {"run": ["notes", "add", "buy milk"], "exit": 0},
  {"run": ["notes", "done", "5"], "exit": 1, "stderr": ["no such note: 5 (there are 1)"]},
  {"run": ["notes", "list"], "exit": 0, "stdout": ["1 [ ] buy milk"]}
]}
```

The control tool contract, from the package comment in
[internal/verify/behaviour.go](../../internal/verify/behaviour.go) (its tabs shown as spaces):

```go
//    <control...> check     stdin:  {"product", "feature", "check", "out", "base_url", "root"}
//                            stdout: {"status": "passed"|"failed"|"unreachable", "reason",
//                                     "artifacts": [{"path", "kind"}]}
```

The two dated keys in [internal/config/defaults.toml](../../internal/config/defaults.toml), trimmed to the keys:

```toml
[verify]
red_on_base_blocking_from = "2026-11-13"
drift_blocking_from = "2026-11-13"
```

This repository's own settings, from `.aayudha/config.toml`:

```toml
[verify]
seed = ["examples/benefits-qa/deploy/serve.sh"]
# The feature maps' maintain pass (D-4.5b.8): a headless Amp run on the founder's side (`amp -x`, loops/README.md)
# follows the brief; Amp has no @-mention.
maintainer = "amp"
```

## Invariants

1. `verify --map` reports every kind of drift and exits 3 on drift, 0 when in sync or with no map. Enforced by
   `CheckMaps` in [internal/verify/featuremap.go](../../internal/verify/featuremap.go) and `verifyMap` in
   [internal/cli/verify.go](../../internal/cli/verify.go), `TestMapDrift` and `TestVerifyMapExitCodes`.
2. A stale feature is never drift. Enforced by `stale` and `MapDrift.Items`, `TestMapStaleOnly`.
3. Drift passes as advisory until 23:59 UTC the day before `drift_blocking_from` and fails the floor from 00:00 on
   that day. Enforced by `MapFloor.Run`, `TestMapFloor`.
4. A failed check fails the floor, a control tool error blocks it, an unreachable check proves nothing, a criterion
   is evidenced only by an artifact of its own kind, and only `ui` and `api` checks get the network. Enforced by
   `Behaviour.Run` and `runCheck` in [internal/verify/behaviour.go](../../internal/verify/behaviour.go),
   `TestBehaviourFloor`.
5. An artifact path outside the check's output directory blocks the floor. Enforced by `runCheck`,
   `TestBehaviourArtifactEscape`.
6. A `ui` or `api` check never runs against production. Enforced by `ResolveTarget` in `Behaviour.Run`,
   `TestBehaviourRefusesProduction`.
7. The maintain brief goes to one tracking issue, is not posted twice for the same report in `drift` mode, and an
   Actions run posts a record with no mention. Enforced by `maintainHandoff` in
   [internal/cli/verifymaintain.go](../../internal/cli/verifymaintain.go) and `Brief.Comment` in
   [internal/verify/maintain.go](../../internal/verify/maintain.go), `TestVerifyMaintainHandoff`.
8. A pull request that drops a check, drops assertions, or deletes a feature file is flagged as weakening. Enforced by
   `Weakened` in [internal/inspect/coverage.go](../../internal/inspect/coverage.go), `TestWeakenedChecks`.
9. A Bug story's proving test that passes on the base makes `red-on-base` advisory before the date and a failure from
   it; inconclusive never blocks; no date stays advisory. Enforced by `redOnBase` in
   [internal/cli/redbase.go](../../internal/cli/redbase.go), `TestRedOnBase`.
10. `init --verify-skill` writes a map that passes `verify --map` and offers only detected maintainers; `install`
    mirrors it to Claude Code. Enforced in [internal/initrepo/verifyskill.go](../../internal/initrepo/verifyskill.go),
    `TestInitVerifySkill`, `TestInitVerifySkillMaintainer`, and `TestInstallMirrorsVerifySkill`.

## Where it lives in the code

- [internal/verify/featuremap.go](../../internal/verify/featuremap.go): map reading, the drift check, the
  `feature-map` floor.
- [internal/verify/behaviour.go](../../internal/verify/behaviour.go): check selection, the control tool contract, the
  `behaviour` floor, `MapFloors`.
- [internal/verify/maintain.go](../../internal/verify/maintain.go) and
  [internal/cli/verifymaintain.go](../../internal/cli/verifymaintain.go): the maintain brief and hand-off.
- [internal/cli/verify.go](../../internal/cli/verify.go): `--map`, `--check`, `--maintain`, and the no-story gate.
- [internal/cli/redbase.go](../../internal/cli/redbase.go): the red-on-base check.
- [internal/initrepo/verifyskill.go](../../internal/initrepo/verifyskill.go) and
  [hosts/verify-skill](../../hosts/verify-skill): the starter skill and control tool templates.
- [skills/verify/references](../../skills/verify/references): the create and maintain procedures.
- [loops/verify-sweep.toml](../../loops/verify-sweep.toml) and
  [loops/verify-maintain.toml](../../loops/verify-maintain.toml): the two loops.

> **Honest status:** the plan for WP4.5b gives `--verify-skill` a kind, has `doctor` report drift, and has Actions
> mention the app. The code, recorded in Appendix B of the build plan, does none of these. This page follows the code.

## Read next

- [Floors in detail](how-floors.md): the other floors `verify` runs beside these two.
- [The merge gate](how-merge-gate.md): where the checks run on every pull request.
- [Proof packs](how-proof-packs.md): where `reports/behaviour.json` and the check artifacts are kept.
- [Prove behaviour](guide-prove-behaviour.md): the user guide for writing a map and its checks.
