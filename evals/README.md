# evals

The skill eval harness (WP3.4). Code: `internal/evals`; configuration: [`eval.toml`](eval.toml). `evolve` (WP7.7)
adds a failing case here before any skill diff.

## Cases

`cases/<skill>/<name>/case.toml` describes one recorded run of a skill and, optionally, how to run it again on a model:

| Key | Meaning |
|---|---|
| `skill` | The skill under test: `skills/<skill>` (embedded), or a stub in `evals/skills/<skill>` |
| `input` | File with the task given to the skill |
| `artifacts` | Files the recorded run produced; each must exist |
| `schemas` | `{ file, schema }` pairs; the file must validate against the JSON Schema (path from the repository root) |
| `ready` | Artifacts holding a work item (`internal/workitems.Item` as JSON) that must pass Ready validation |
| `calls` | The recorded run's `aayu` calls; each must exit 0 against a fake GitHub serving the case's `github/` files (`github/repos/o/r/issues/12.json` answers `GET /repos/o/r/issues/12`). A call to a verb this binary does not implement yet is "n/a" (ground rule 1a). `{case}` in an argument is the case directory, for files the recorded run wrote (a slice or plan draft) |
| `scope` | `{ pattern, why }` rules for requirements the input does not state (a regular expression over the work item's title and fields). A work item that matches one fails the `scope` check in tier 1 and scores 1 in tier 2 before the judge is asked. Add a rule when a run is caught inventing a requirement, as `evolve` keeps a failing case |
| `injection` | The input carries a prompt injection: no recorded call may be a write or human-only verb, and nothing may write |
| `brief` | `{ register, purpose, draft }`: a `brief` case (WP7.3). `input` is the grounding (`internal/brief.Grounding` as JSON; the brief cases share `cases/brief/grounding.json`) and `draft` the recorded Markdown draft, which must pass `aayu brief`'s deterministic checks (structure, citations, the register's lint, visuals) and render to a page whose every image has alt text |
| `tell` | `{ kind, campaign, request, draft }`: a `tell` case (D-7.5b). `input` is the grounding (the proven and unproven Features as `aayu tell` lists them, and each Feature's packs, criteria, evidence, and Result; the tell cases share `cases/tell/grounding.json`), the case's own `design/brand.yaml` is the brand, `request` is what the human asked for, and `draft` the recorded copy, which must pass `aayu tell --from`'s checks (`tell.CheckDraft`: links only to proven Features, no sentence naming an unproven one, a named proven Feature linked, an announcing kind linking one, no avoided word, no exclamation mark when the brand avoids them, no superlative) and the case's `scope` rules. Tier 2 has the skill write the copy from the brief, the brand, and the grounding, and revise once on the findings |
| `[survey]` | `{ kind, read, survey, draft }`: a `survey` case (WP5.9). `repo` is a codebase fixture with no product docs (the real projects under `cases/survey/repos`, kept as they are, with their licences), `input` the person's request, and `draft` the recorded reply. Kind `draft`: the skill gets the index summary and every module pack as `aayu survey` prints them, plus the files in `read` (whole, or `path:start-end`) numbered by line, and writes `survey.json`; no person is in the session, so an answered question or a confirmed claim fails. Kind `backlog`: it also gets `survey`, a checked survey with a recorded interview, and writes `backlog.json`. The reply must pass `aayu survey --draft` or `--backlog` (`survey.Check`, `survey.CheckBacklog`) and the case's `scope` rules (matched over the whole reply); tier 2 revises once on their findings, and a reply that still fails scores 1 without the judge |
| `repo`, `transcript`, `cite` | A grill conversation case (WP5.3b): `repo` is a repository fixture (relative to the case; the bakery app in `cases/grill/testdata/bakery`) that the recorded calls run in and `grill --assess` reads; `transcript` is the recorded conversation; `cite` is how many criteria must cite an assessment fact. Tier 1's `concrete` check: the spec has no unresolved vague qualifier (`internal/grill.Check`), cites only facts the fixture has, every criterion quotes the human's own words from a topic they confirmed or accepted as a default, and every recorded reply is the scripted human's. `gate` (WP5.8) is the vagueness gate's expected verdict on the request: `unpack` (the first turn must ask, and every abbreviation in the request needs a `Term:` line quoting the human) or `pass` (a concrete request: no questions at all, in tier 1 and tier 2) |
| `persona` | `{ seat, stage, practice, task, draft, planted }`: a persona case (WP5.1b). `input` is the grounding (Markdown) and `draft` the seat twin's recorded answer at the team `stage`. Tier 1 runs the evidence rule (`seats.CheckGrounding`: every claim about code, architecture, risk, or progress cited, every citation found in the grounding) and the stage check (`seats.CheckStage`: the `practice`'s artefact for the stage as a heading, no other stage's, at most 400 words at a startup); `seat` must be the skill's Accountable seat. `planted`, when set, is the same answer with a planted uncited claim, which the evidence rule must catch |
| `pack` | `{ seat, stage, question, draft, incomplete }`: a decision-pack case (WP5.1b, D-5.6b). `question` must route to `seat` (`decide.OwningSeat`); `input` is the grounding (what `aayu decide --pack` printed plus what the twin read) and `draft` the recorded pack, which must pass `decide.CheckPack` (every part in order, one-sentence question, two or three options, a named door, the decision last, at most 500 words, every claim cited from the grounding). `incomplete`, when set, is a pack missing a part, which must fail. Tier 2 has the decide skill write the pack in the owning twin's persona and revise once on the findings; the rubric's first item is "a busy human can decide from this in under five minutes" |
| `feedback`, `reproduce` | An evolve case (WP7.7): the accepted Feedback issue it reproduces, and the directory holding the old skill's recorded run (the `output/` artifacts under that name). Tier 1 must fail on that run and pass on `output/` |
| `[ux]` | A UX case (`internal/evals/ux.go`) for one of the UX Researcher's skills: `output` is the recorded run's files. The design stages (`research`, `wireframe`, `mockup`, `prototype`) take `feature`, `upstream` (the earlier stages' recorded directories, research first, built and pinned as the ledger holds them), `states` (the Completion Goal states every stage must have), `team`, `practice` (research: the playbook entry whose team-stage artefact the notes carry), and `context` (more files the host reads: tokens, the design-system summary, a Figma file); `review` takes `capture` (a recorded `aayu review` capture with both screenshots) and `planted` or `clean` (the kinds lane B measured as defects or as clean); `polish` takes `page` (the page before the fix), `planted`, and `context` (the critique); `baseline` takes `sha` and `env`. Tier 1 runs each skill's floors on `output` (below) |
| `[tier2]` | `rubric` (file), `verb`: the registry verb the skill stands in for, when the skill is not itself a verb (the skill model is that verb's registry lane on the direct host, with the lane's registry alternative as the fallback), and `references`: files under the skill's `references/` that the host reads for this case, sent after the skill body; `draft` (`slice` or `plan`) and `issue`: the skill replies with that verb's draft, which runs through `aayu slice` or `aayu plan`'s checks against the case's `github/` fixture of `issue` before the judge (WP5.4); `human`: the scripted human of a grill conversation case (WP5.3b, below); `session`: the world of a session case (D-3.4f, below) |

## Tier 1: every PR

Deterministic, no model calls. `TestTier1` in `internal/evals` runs every case inside `go test`, so the CI job `build`
runs it; `go run ./internal/evals/cmd/eval -tier 1` prints the same report.

## The bars only rise

A failing eval is fixed by improving the work, never by lowering the eval (Story #400, Feedback #327). On every PR,
CI's `build` job compares this directory with the PR's base (`evals.CheckBars`, run by the coverage ratchet's command)
and fails a change that:

- lowers `pass_score`, `min_pass_k`, or `min_mean`, or raises `max_drop`, in `eval.toml`;
- moves a case into a group whose `gate` or `pass_k` is `"advisory"`, or removes a case;
- removes a case's deterministic rule (a `case.toml` scope rule, a `session.toml` require, forbid, or say_forbid
  rule), or changes its `pattern` or `allow`;
- drops a rubric item (a criterion, a score anchor, or a paragraph), or rewords one so that it loses any word.

Raising a bar and adding a case, a group (a new group starts advisory), a rule, or rubric words need nothing. When a
bar really is wrong, a human decides: `aayu decide --waiver eval-bar <story>`, accepted with
`aayu decide <n> --accept waive`, then named as `bar_waiver = <n>` in `eval.toml`. A waiver the base already names does
not count again. To run the check before a push:

```sh
go test -coverprofile=coverage.out ./... && go run ./internal/ratchet/cmd/ratchet -coverprofile coverage.out -base "$(git merge-base origin/dev HEAD)"
```

## Evolve PRs: eval-first

`aayu evolve` (WP7.7) answers accepted retro Feedback about a skill. Its PR must add a new case here that names the
Feedback (`feedback`) and reproduces it: the old skill's run in the `reproduce` directory fails tier 1, and the new
skill's run in `output/` passes. The PR must also change that skill. CI's `build` job runs the gate on every PR that is an
evolve PR (an `evolve/` head branch, or a commit with the `Aayudha-Evolve:` trailer) and rejects one without such a case.
The old run must exist and validate against its schemas, and fail another check: a missing run does not reproduce anything.

```sh
go run ./internal/evals/cmd/eval -evolve-gate -base "$BASE_SHA" -head "$HEAD_SHA" -branch "$HEAD_REF"
```

## Tier 2: nightly and on promotion

[`.github/workflows/evals.yml`](../.github/workflows/evals.yml) runs each `[tier2]` case `k` times on the skill's model
(the registry lane of its verb on the direct host) and has the pinned `judge` score every run against the rubric. The judge's model family,
inferred from its model ID, must differ from the skill model's. A skill replies with the work item plus `open_questions`
(`schemas/draft.json`): what the input leaves undecided goes there, never into the item as a requirement. A run whose output is JSON of the wrong shape, or fails
Ready, scores 1 without asking the judge (deterministic checks beat model judgement). The judge is asked for structured
output (OpenAI `json_schema`, strict) and its reply is validated by the WP1.6 structured-output helper, which retries a
judgement once. A run whose judge reply, or skill reply, still is not usable JSON is an `error`: it is not scored, and
the results file keeps a short redacted excerpt of the reply. A case's pass^k is 1 when all its completed runs score at
least `pass_score`, else 0; the suite's is the mean over cases. More than one errored run in a case fails the tier. A
reply that stops at its token cap is an `error` marked `truncated` and is not retried. Each run records every model
call: role, model, stop reason, input and output tokens, and the cap. When the group's `max_usd` is reached, the remaining runs
are recorded as errors and the results file is still written.

A brief case's skill replies with its Markdown draft. Its grounding gives each pack's criterion → evidence map (and
inspect's verdicts when the pack's head has them), and a draft cites evidence only through it (`[P1.c3]`); a case
whose map lacks an entry (`brief/unmapped-criterion`, tier 1 only) checks that no proof is claimed for it. The checks
also refuse a close or `regulatory` brief without a `## Limits of this evidence` section, a number from the Result
stated without its population ("per reviewer"), and an interface control no source names. When `aayu brief`'s
checks fail (an `eli5` reading grade above 6,
an STE lint finding, a citation that names no source, …), the skill gets the findings once and revises, as it does in a
host; a draft that still fails scores 1 without the judge (`revised` marks runs that needed the revision).

A grill conversation case (`[tier2] human`, WP5.3b) runs the skill turn by turn. The skill gets the request and
`grill --assess`'s report on the case's repository fixture, both as untrusted data, and replies with JSON turns: what
it says, up to five questions each tagged with the topic it settles (`<dimension>/<topic>`, the topics coming from
the catalogues' Topics, the skill's request topics, or `other/<key>`), and finally `done` with its readback and the
draft. The scripted human (`human.toml`) answers each question by its topic (`[topics."ux.steps/screen-count"]`), else
by the topic's dimension (`[dimensions."ux.steps"]`), else with the fallback, which parks it (`confirm`, `default`,
`park`, or `defer`), so a follow-up on another topic of a dimension gets its own answer, an unscripted idea never
becomes a requirement, and a run is reproducible whatever the wording. After `max_rounds` the human asks for the spec. Before the
judge, the draft must pass Ready and tier 1's `concrete` rules and the case's `scope` rules, else it scores 1; the judge
then scores the whole conversation (question quality, concreteness, fidelity), and the results keep each run's
exchanges and final criteria. A `gate` case scores 1 without the judge when round 1 contradicts the verdict: no question on `unpack`, any question on `pass`.

A session case (`[tier2] session`, D-3.4f) runs a host skill as it runs in a person's terminal: `build`, `verify`,
`ready`. The skill replies in JSON turns (what it says, files it writes, commands it runs, and `done`); the harness
answers each command from the case's world and sends the results back as untrusted data. The world is
`session.toml`: recorded `[[reply]]` answers (first match first; `after` makes one wait for an earlier command that
exited 0; `{checks}`, `{map}`, and `{evidence}` fill one from the checkout as it is), else the real binary in-process
for verbs that only read (`ready`, `verify --map`, `status`, `config`), against a fake GitHub serving `github/`, in a
git checkout of the case's `repo` fixture (feature maps kept under `_agents/` there become `.agents/`) with
`overlay` committed on top as the branch; else `cat` and `ls`. Nothing the skill writes is run: Go files are
parsed, and an evidence map is checked statically. A turn that runs nothing gets the next scripted `human` reply.
The floors, on the recorded session in tier 1 (`output/session.json`, replayed so every command must still exit as
recorded) and on every tier-2 run before the judge: writes only to paths matching `files`; no GitHub write; no
human-only verb; `--asked` words only as the person said them; no rerun of a command after its floor miss with nothing
changed; each `require` command run, no `forbid` command run, nothing said that matches `say_forbid`; with
`[evidence]`, every criterion mapped to a Go test that exists or a check in a feature map; `map_clean`, `aayu verify
--map` clean at the end; `keep_checks`, no behaviour check removed; and the session done within `max_turns`.

The bar: pass^k ≥ 0.8 and mean score ≥ 3.5, for the suite and for each case (so for each `brief` register). Against the baseline (the results for `main`'s commit), a mean more than
0.3 lower, or any case's pass^k below its baseline, fails.

A persona case's skill replies in its seat twin's voice (the skill body opens with the persona block) at the case's
team stage. When the evidence-rule or stage checks fail, it gets the findings once and revises; an answer that still
fails scores 1 without the judge. The judge sees the persona, the stage, and the grounding, and scores clarity,
helpful nudging, grounded citations, senior-practitioner realism for the seat, and stage fit: `retro/em-startup` and
`retro/em-enterprise` run the same scenario, so their depth must differ while both stay grounded.

A UX case's skill replies with the files it produces (JSON `{files: [{path, content}]}`), and its floors run before
the judge; a reply that fails them gets the findings once and revises, as a host does when the binary refuses, and one
that still fails scores 1. The floors: for the design stages, `aayu <stage>`'s own checks (`design.Build` against the
upstream manifests; the prototype's figma-diff gate needs a Figma file and a running app, so only its static checks run),
every Completion Goal state, research references only from the host's search results and quotations word for word from
the data plus the team stage's artefact, and token-only colours in a prototype's code; for `polish`, token-only colours,
every control kept, and every finding reported in `polish.md`; for `baseline`, the commit checked out before `aayu
baseline --sha`, the app on the case's target and never production, no GitHub write, and nothing discarded. The
mockups' and polished pages' HTML also go through lane B in Chromium (impeccable, axe, the focus-indicator and 44 px
probes): no contrast, focus, target, label, or keyboard finding may remain (tier 1 reports this floor n/a without
`AAYU_FLOOR_WEB=1`; CI `floors-web` runs it). A `review` run is lane A itself: the ui-review lane's model gets
`uireview.Prompt` on the recorded capture (both screenshots, the markup, the tree), once per eval run with no revision,
and must report every planted kind at severity 2 or more and no clean kind. The judge reads the files (or lane A's
JSON), never a rendering: it cannot see pixels, so layout and visual hierarchy are judged from the markup and styles
(D-6.5b).

Tier 2 runs in groups (`[[groups]]` in `eval.toml`), one `evals.yml` job per group, each with its own `max_usd`, so
one group cannot use up another's cap. Every call is charged at its model's list price (`models.ListPrices`, D-3.4p), and
each cap is about twice the group's latest full run at that price, rounded up to a whole dollar, so it stops a runaway
run and never cuts a normal one short (D-3.4q; the basis is on each `max_usd` line). The groups: `brief-plain` (the
`eli5` and `ste` registers, 4 USD), `brief` (the other `brief/` cases, 3 USD), `personas` (the `retro/` and `decide/`
persona cases, 14 USD), `data-ai` (the `chart/` and `measure/` cases, 8 USD), `production` (the `investigate/`, `slo/`,
`runbook/`, and `respond/` cases, 15 USD, advisory and on dispatch only, D-5.10d), `review` (the `inspect/` seeded-defect
cases, 5 USD), `conversations` (the grill conversation cases of WP5.3b and WP5.8, 37 USD, advisory until a run meets
the bar, D-5.3b.g), `tell` (the `tell/` cases, 3 USD, gating since two consecutive full runs held a mean of 5.00, D-7.5e; it runs on
dispatch and on pushes to `rolling` and `main`, never nightly or on pushes to `dev`), `host-skills` (the build, verify, and ready session cases, 12 USD, advisory and on
dispatch only, D-3.4f), `ux` (the UX Researcher's skills, 45 USD, advisory and on dispatch only, D-6.5b; its `max_tokens` raises the skill
call's output cap to 64000 for multi-file stages, D-6.5c, D-6.5e, and D-6.5g), `survey` (the `survey/` cases, 65 USD, advisory and on
dispatch only, its skill output cap 48000 for a whole survey draft, WP5.9 and D-5.9g), and `skills` (every other case, 4 USD).
Pushes to `dev` do not run the `brief` group; it runs nightly, on dispatch (input `group` picks one group), and on
pushes to `rolling` and `main`. The advisory `conversations` and `personas` groups never run nightly or on promotion
(`events` in `eval.toml`): only on dispatch and on pushes to `dev` that change their own files (`skills/grill/`, the
grill cases, `internal/grill/`; `seats/*/persona.toml`, `internal/seats/persona.go`, the `retro/` and `decide/` cases;
and each one's harness file), so a pull request that changes them gets tier 2 once it merges (D-5.3b.h). Each group's results go to `results/<sha>-<group>.json`, uploaded as the artifact
`eval-results-<sha>-<group>`; the promotion gate (`ci.yml` job `evals-gate`) fetches every group's results for the
commit being promoted and for `main`, runs `eval -compare` on each, and fails when any group is missing or fails. A group with
`pass_k = "advisory"` (`brief`, D-7.3g) passes on the mean bar and the drop rule; its pass^k findings print as
advisories. A group with `gate = "advisory"` (`brief-plain`: the eli5 and ste cases, D-7.3h) is reported but never gates:
`eval -compare` exits 0 for it, and `evals-gate` compares only `eval -groups -gating`. The `personas` group is advisory
too until a run meets the bar (D-5.1d).

Tier 2 calls models through the AI gateway in the `ai-gateway` environment (dev, rolling, and main only), so it never
runs on pull requests. Spend is capped by `[budget]` and by the group's `max_usd` in `eval.toml`.

Run it by hand with the gateway configured:

```sh
AAYU_MODELS__GATEWAY__BASE_URL=https://…/v1 AAYU_MODELS__GATEWAY__SECRET=ai-gateway AAYU_SECRET_AI_GATEWAY=… \
  go run ./internal/evals/cmd/eval -tier 2 -group brief -sha "$(git rev-parse HEAD)" -out "evals/results/$(git rev-parse HEAD)-brief.json"
```
