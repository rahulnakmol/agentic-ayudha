# rahulnakmol/agentic-ayudha#505: CLI developer-experience floor: --help, non-zero failures, stable --json goldens, and no flag removed without a deprecation

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `fa2f2e5ca6594dec181273119bcc9c00669572a132d45e9bc475581fe6c89be2` |
| Completion Goal hash | `4e13f66edd663081e19152a949681423a16e54e65e6758b3ff9f7a0ca77257e1` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/505/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: a stand-in CLI whose root help lists `serve` and `export` under `Commands:`, where `export --help` exits 1; then the same with an `Available Commands:` section, and an argparse-style `{serve,export}`; and `help` and `completion` listed; expected: the first fails the cli floor naming `export --help`; each listing style discovers both commands; `help` and `completion` are not walked
2. [api-trace] state: the stand-in CLI where `serve --aayu-floor-unknown` exits 0 with nothing on stderr; expected: the cli floor fails naming `serve` and the unknown-flag rule
3. [api-trace] state: a golden `{ "args": ["export", "--json"], "output": {"id": 1, "items": [{"name": "a"}]} }` and a head output that removes `items[].name`, then one that makes `id` a string, then one that adds `count`, then one that changes only values; and a command whose help lists `--json` with no golden; expected: the removed key and the retyped key fail naming the key path; the added key and the changed values pass; the command with no golden fails
4. [api-trace] state: a base help listing `--format` and a head help without it; then a base help whose `--format` line says `(deprecated)`; then a cobra CLI that hid the flag with `MarkDeprecated`; expected: the first fails naming `--format`; the second passes; the third fails, because the base help did not mark it
5. [api-trace] state: `platforms = ["cli"]` with L02 in the footprint and no `[verify.floors.cli] command`; then the same without L02; then `platforms = ["web"]` with L02; expected: the first records `skipped: cli`, which blocks ship until a floor waiver names cli; the second and third skip without blocking
6. [api-trace] state: a cli miss with the clock before 2026-11-14, then on 2026-11-14; expected: before the date the check passes with a detail that says advisory until 2026-11-14 and names the miss; on the date it fails
7. [api-trace] state: `aayu verify` on this repository with `[verify.floors.cli] command` set to the built `aayudha` and goldens under `.aayudha/cli-goldens`; expected: the cli floor walks every aayu verb and passes, and the check names the number of commands checked

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As a team that ships a command-line tool, I want a story that changes the CLI to prove it still helps, still fails loudly, and still keeps its scripts working, so that a missing `--help`, a failure that exits 0, a renamed `--json` key, or a silently removed flag cannot merge. Decision #480 (accepted, option 1, section 4) adds a `cli` floor for `[verify] platforms` with `cli` and a footprint with L02. verify runs the product's built CLI (`[verify.floors.cli] command`) in its sandbox on the base and the head, walks every command from `--help`, and checks four rules: every command answers `--help`, an unknown flag exits non-zero, each `--json` golden keeps its key paths and types, and no long flag disappears unless the base marked it deprecated. aayu's own CLI is the first one it checks. Misses are advisory until `[verify] new_floors_blocking_from` (2026-11-14), then they fail.

### Completion Goal

1. [api-trace] state: a stand-in CLI whose root help lists `serve` and `export` under `Commands:`, where `export --help` exits 1; then the same with an `Available Commands:` section, and an argparse-style `{serve,export}`; and `help` and `completion` listed; expected: the first fails the cli floor naming `export --help`; each listing style discovers both commands; `help` and `completion` are not walked
2. [api-trace] state: the stand-in CLI where `serve --aayu-floor-unknown` exits 0 with nothing on stderr; expected: the cli floor fails naming `serve` and the unknown-flag rule
3. [api-trace] state: a golden `{ "args": ["export", "--json"], "output": {"id": 1, "items": [{"name": "a"}]} }` and a head output that removes `items[].name`, then one that makes `id` a string, then one that adds `count`, then one that changes only values; and a command whose help lists `--json` with no golden; expected: the removed key and the retyped key fail naming the key path; the added key and the changed values pass; the command with no golden fails
4. [api-trace] state: a base help listing `--format` and a head help without it; then a base help whose `--format` line says `(deprecated)`; then a cobra CLI that hid the flag with `MarkDeprecated`; expected: the first fails naming `--format`; the second passes; the third fails, because the base help did not mark it
5. [api-trace] state: `platforms = ["cli"]` with L02 in the footprint and no `[verify.floors.cli] command`; then the same without L02; then `platforms = ["web"]` with L02; expected: the first records `skipped: cli`, which blocks ship until a floor waiver names cli; the second and third skip without blocking
6. [api-trace] state: a cli miss with the clock before 2026-11-14, then on 2026-11-14; expected: before the date the check passes with a detail that says advisory until 2026-11-14 and names the miss; on the date it fails
7. [api-trace] state: `aayu verify` on this repository with `[verify.floors.cli] command` set to the built `aayudha` and goldens under `.aayudha/cli-goldens`; expected: the cli floor walks every aayu verb and passes, and the check names the number of commands checked

### Measure

Completion Goal criteria observed

### Threshold

all 7 criteria

### Scenario

Run the cli floor against git fixtures of a base and a head commit with stand-in CLIs (shell scripts that print recorded help and JSON), across the listing styles, golden, deprecation, platform, footprint and date combinations above. Run `aayu verify` on this repository with the built aayudha binary.

### Environment

local checkout and this repository's CI build job on a pull request; stand-in CLIs in a temporary directory; never production

### Links

depends-on #496

### Checkpoints

1. `go test ./internal/verify -run 'TestCLIFloor'` shows discovery, the four rules, the blocking skip, and the advisory date (criteria 1, 2, 3, 4, 5, 6)
2. a local `aayu verify` on this repository with the built binary (criterion 7)

### Assumptions

- `[verify] seed` builds the CLI that `[verify.floors.cli] command` runs, on the base and on the head
- values in a `--json` golden are not compared, so timestamps and IDs do not break it
- `verify.new_floors_blocking_from` is 2026-11-14, shared by every floor Decision #480 added, and lands with #496

### Open questions

- deferred: cobra's `MarkDeprecated` hides a flag from help, so a cobra CLI keeps the flag visible with "(deprecated)" for one release, or files a waiver; Decision #480 records this cost

### Provenance

Request: "in proof floors should talk about code quality and user experience if there is a UI, or developer experience if it's an API or CLI. Goal: proof floors that, chosen by the story's layer footprint, cover: Developer experience, when it changes an API or a CLI (L02, or however LAYERS.md maps them). CLI DX: every command has --help, a failure exits non-zero, a --json output stays stable against goldens, and no flag is removed without a deprecation. Ground rule 7 still holds: a floor is deterministic, and no model opinion can be a floor. It should list each floor, its default, the layers it applies to, the tool pinned by SHA-256, and what skipped means. Build only after it is accepted, one story per floor family, each with tests that a plausible wrong implementation would fail. Start the stories outside DAST now under #480's acceptance (code quality, UX, API and CLI developer experience)."
Term: "CLI" → a product's command-line interface; "Developer experience, when it changes an API or a CLI"
Term: "API" → a product's HTTP or RPC interface; "Developer experience, when it changes an API or a CLI"
Term: "DX" → developer experience; "CLI DX: every command has --help"
Term: "UI" → the product's user interface, layer L01; "user experience if there is a UI"
Term: "LAYERS" → LAYERS.md, the map of the nine layers and their owners; "however LAYERS.md maps them"
Term: "L02" → the Interaction / API boundary layer in LAYERS.md; "(L02, or however LAYERS.md maps them)"
Term: "SHA-256" → the hash each downloaded tool is checked against; "the tool pinned by SHA-256"
Term: "DAST" → the ZAP scan of the running app, built in #496; "Start the stories outside DAST now"
Vague: "stable" → criteria 3
Term: "JSON" → the machine-readable output a `--json` flag prints; "a --json output stays stable against goldens"
1. stated (request): "every command has --help"
2. stated (request): "a failure exits non-zero"
3. stated (request): "a --json output stays stable against goldens"
4. stated (request): "no flag is removed without a deprecation"
5. stated (request): "Developer experience, when it changes an API or a CLI (L02, or however LAYERS.md maps them)" / "what skipped means"
6. stated (request): "Start the stories outside DAST now under #480's acceptance"
7. stated (request): "Developer experience, when it changes an API or a CLI"
````
