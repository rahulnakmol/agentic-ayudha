# survey file formats

The three files the host's model writes for `aayu survey`. Every key below is checked; an unknown key is an error, so
a misspelt field never silently drops a claim.

## survey.json (`--draft`)

```json
{
  "product": "calc",
  "request": "the person's own words asking for the survey",
  "commit": "the commit the index was built at (optional)",
  "summary": "one paragraph: what it is, who it is for as far as known, and what is unknown",
  "engineering": [
    {"id": "E1", "topic": "architecture", "claim": "…", "basis": "observed", "cites": ["sym:src/main.cpp#main", "src/main.cpp:4-11"]},
    {"id": "E2", "topic": "risks", "claim": "…", "basis": "inferred", "cites": ["io:env:src/util.cpp:6"], "reasoning": "why the cited code supports it"}
  ],
  "capabilities": [
    {"id": "C1", "name": "Evaluate an expression", "claim": "…", "basis": "observed", "cites": ["entry:src/main.cpp:4"]}
  ],
  "product_claims": [
    {"id": "P1", "topic": "users", "claim": "Who runs it is not in the code.", "basis": "unknown", "question": "Q1"},
    {"id": "P2", "topic": "jobs", "claim": "…", "basis": "confirmed", "answer": "Q2"}
  ],
  "questions": [
    {"id": "Q1", "question": "…", "why": "what the answer changes", "default": "your default and its reason", "affects": ["P1"], "status": "open"},
    {"id": "Q2", "question": "…", "why": "…", "status": "answered", "answer": "the person's exact words"},
    {"id": "Q3", "question": "…", "why": "…", "status": "deferred", "reason": "why it can wait"}
  ],
  "traps": [
    {"id": "T1", "rule": "c-locale", "check": "Does the printed number depend on the C locale?", "claim": "…", "basis": "observed",
     "cites": ["trap:c-locale:src/util.cpp:7", "src/util.cpp:5-10"]}
  ],
  "do_not_carry": [
    {"id": "N1", "feature": "Debug tracing switched on by an environment variable", "cites": ["io:env:src/util.cpp:6"], "traps": ["T1"],
     "risk": "what copying it would cost", "replacement": "what the new version does instead", "question": "Q2", "status": "proposed"}
  ],
  "compatibility": [
    {"id": "K1", "kind": "stdout", "name": "The integer result on stdout", "cites": ["src/util.cpp:14"], "consumers": "who depends on it",
     "level": "undecided", "decided_by": "Q1", "proof": ["G1"]},
    {"id": "K2", "kind": "ui-form", "name": "The invoice form's fields and tab order", "cites": ["Invoice.frm:1"], "level": "exact",
     "decided_by": "Q4", "proof": ["check:invoicing/invoice-form.enter-invoice"]},
    {"id": "K3", "kind": "exit-code", "name": "Exit code 1 on a script error", "cites": ["src/main.cpp:9"], "level": "exact",
     "decided_by": "request", "quote": "with the same behaviour", "proof": ["G1"]}
  ],
  "modernisation": {
    "target": "Rust",
    "target_source": "request",
    "strategy": "…",
    "cites": ["test:run_tests.cpp"],
    "steps": [
      {"id": "M1", "title": "Capture golden outputs", "modules": ["mod:src"], "claims": ["E1"], "parity": ["G1"]},
      {"id": "M2", "title": "Port the engine", "modules": ["mod:src"], "claims": ["E1", "C1"], "depends_on": ["M1"], "parity": ["G1"]}
    ],
    "case_sets": [
      {"id": "G1", "title": "…", "from": ["test:tests/test001.js", "entry:src/main.cpp:4"], "compare": "exact stdout and exit code", "exclude": ["random output (E7; Q4 asks whether it matters)"],
       "reference": "commit 8214477, g++ 12.2.0, the Makefile's CFLAGS -c -g -Wall", "runners": ["entry:src/main.cpp:4"], "inputs": ["tests/test*.js"]}
    ],
    "risks": [
      {"id": "R1", "claim": "…", "basis": "observed", "cites": ["src/main.cpp:7-8"]}
    ],
    "suites": [
      {"id": "S1", "title": "The legacy project's own tests", "from": ["test:tests/test001.js"], "cases": "compat.json",
       "areas": ["expressions", "errors"], "unfixable": ["t-ld-preload: intercepts glibc internals the new build never calls"]}
    ],
    "migrations": [
      {"id": "MG1", "what": "The history file", "cites": ["src/util.cpp:20-31"], "rollback": "how a user goes back, tested",
       "proof": ["G4"]}
    ],
    "coexistence": {"mode": "shadow", "how": "both run on the same inputs; differences are reported", "compare": ["G1", "S1"],
      "cut_over": "the criteria", "rollback": "how to switch back"},
    "shims": [
      {"id": "SH1", "interface": "K1", "behaviour": "the old behaviour kept", "mechanism": "--legacy-output", "removal": "when and how it goes"}
    ]
  }
}
```

- Topics. Engineering: architecture, modules, data, interfaces, build, dependencies, tests, risks, quality,
  operations. Product: users, jobs, outcomes, value, constraints, context, competition. Capabilities have a `name`
  instead.
- Citations: a fact ID the index printed (`file:`, `sym:`, `build:`, `dep:`, `entry:`, `io:`, `trap:`, `test:`,
  `doc:`, `marker:`, `mod:`, `builtin:`), or `path:line` / `path:start-end` inside the file.
- Capabilities cover the index's entry points: each `entry:` the index lists (a program's main, a script, an
  exported function or type, a header's public declarations) is cited by a capability claim, by its entry ID, its
  `path:line`, a `path:start-end` range that holds it, or the `sym:` of its definition. A file cited whole covers none.
- At most five questions are `open` at a time: one interview round. Merge questions one answer settles; a choice with
  a safe default is a risk that carries the default.
- A case set's `runners` are the programs that run it: `main` or `script` entries, or a `test:` harness that drives the
  library. They are required when the index lists more than one program; with one, it is the runner when none is
  named. `inputs` are the checkout's files the cases run on (paths or globs), when they come from the checkout; with
  none named, the `test:` files in `from` are the inputs. Each runner must register every builtin the cases call
  (`builtin:` facts; a call in an input file, or a dotted builtin such as `Math.sqrt` named in `title` or
  `compare`): a case that needs a builtin only one runner registers belongs in that runner's case set, or in
  `exclude` with the file and the claim or question that decided it.
- A question's `affects` lists the claims, do-not-carry entries, interfaces, and modernisation steps, case sets, suites,
  migrations, and shims its answer changes.
- `target_source` is `"request"` only when the request names the target; otherwise ask, and use the answered
  question's ID.
- `traps`: one finding per trap rule checked (`rule` is a heading of [traps.md](traps.md), or `"stack"` for a
  per-stack question no rule matches), with `check` (the question you asked) and a claim's basis and citations.
- `do_not_carry`: `status` is `proposed` while its `question` is open, then `confirmed` or `rejected` (the person keeps
  it) once answered. A step whose modules hold its code lists it in `avoids`.
- `compatibility`: `kind` is one of cli, exit-code, stdout, stderr, env, locale, file-format, config, plugin, protocol,
  api, data-store, ui-form, ui-shortcut, ui-print, ui-clipboard, ui-file-association, os-integration, com. `level` is
  `exact`, `differ` (with `difference`), `drop` (with `deprecation`: the warning, the release, the removal), or
  `undecided`. Only the person decides a level: `decided_by` is `"request"` when their request says it (then `quote`
  holds the request's own words that decide it, and the check finds them in `request`), an answered question, or,
  while `undecided`, the open question that will decide. `proof` names case sets, suites, or user
  behaviour checks that replay a user journey (`check:<product>/<feature>.<check>`, from the product's feature map, `aayu verify --map`); an `exact` interface needs exact evidence,
  and a user-visible one (`ui-…`, `os-integration`) a check. A case set that proves an `exact` interface names its
  `reference`: the pinned legacy build its goldens come from (the commit, the toolchain and its version, the build
  flags), since another build of the same source can print other results.
- `suites`, `migrations` (each with a tested `rollback`), `coexistence` (`mode`: shadow, side-by-side, or switch), and
  `shims` (each with its `removal` plan) are described in [compatibility.md](compatibility.md).

## backlog.json (`--backlog`)

```json
{
  "product": "calc",
  "items": [
    {"key": "goal", "type": "goal", "title": "…", "basis": ["P2"], "fields": {"summary": "…"}},
    {"key": "rust-engine", "type": "initiative", "parent": "goal", "title": "…", "basis": ["M1", "E1"], "fields": {"summary": "…"}},
    {"key": "evaluate", "type": "feature", "parent": "rust-engine", "title": "…", "basis": ["C1", "E1"],
     "fields": {"summary": "…", "problem": "…", "scope": "In: … Out: …"}},
    {"key": "port-eval", "type": "story", "parent": "evaluate", "title": "…", "labels": ["effort:M"], "basis": ["E1", "C1"],
     "step": "M2", "parity": ["G1"], "depends_on": ["capture-golden"],
     "fields": {"summary": "As …, I want …, so that …",
       "completion_goal": "1. [api-trace] state: the Rust build and case set G1; expected: aayu survey --parity reports no difference",
       "measure": "…", "threshold": "…", "scenario": "…", "environment": "…"}},
    {"key": "div-zero", "type": "bug", "parent": "evaluate", "title": "…", "labels": ["effort:S"], "basis": ["marker:src/engine.cpp:5"],
     "fields": {"summary": "…", "reproduce": "…", "expected": "…", "actual": "…",
       "completion_goal": "1. [api-trace] state: …; expected: …", "measure": "…", "threshold": "…", "scenario": "…", "environment": "…"}},
    {"key": "no-env-debug", "type": "decision", "parent": "evaluate", "title": "Do not carry over …", "basis": ["N1"],
     "constrains": ["port-eval"],
     "fields": {"summary": "the question (Q2)", "options": "1. Keep it. 2. Replace it with …", "evaluation": "the risk, cited",
       "recommendation": "…", "reversible": "true"}}
  ],
  "open_questions": ["Q1 (open): what the answer would unlock"]
}
```

- `key`: lower-case slug, unique; a re-run finds the issue by it. `parent`, `depends_on`, and `constrains` name keys.
- `contract` (Story or Bug): the interfaces (K1) the item keeps. An `exact` one needs every case set and suite its
  `proof` names in `parity`, and every feature-map check in the Completion Goal; when the backlog ports anything, each `exact`
  interface is kept by some Story or Bug. An `undecided` one cannot be kept until the person decides.
- `avoids` (Story or Bug): the do-not-carry entries (N1) its step lists. Each needs a `decision` item with that entry
  as its basis that `constrains` the item; it is filed after the items it constrains, with `constrains #N` links.
- Evidence kinds in a Completion Goal: `screenshot`, `recording`, `sr-transcript`, `api-trace`, `metric`.
- A Story or Bug passes the Ready stage of the work-item schema before it can be filed.

## cases.json (`--golden`, `--parity`)

```json
{"cases": [
  {"id": "test001", "set": "G1", "argv": ["{bin}", "tests/test001.js"], "compare": ["exit"]},
  {"id": "readme-text", "set": "G2", "argv": ["php", "{bin}"], "stdin": "characterisation/readme.md"},
  {"id": "syntax-error", "set": "G3", "argv": ["{bin}", "characterisation/err.js"], "timeout_s": 10}
]}
```

- `id`: file-safe and unique; outputs are `golden/<cases file name>/<id>.stdout` and `.stderr`.
- `argv` must run `{bin}`, the `--bin` program; an interpreter may come first.
- `stdin` and `cwd` are paths inside the surveyed checkout; commit the inputs with the cases file.
- `compare`: any of `stdout`, `stderr`, `exit`; default all three. Compare less only with a reason in the case set.
- A case runs hermetically: `PATH`, a scratch `HOME`, `LC_ALL=C`, and `TZ=UTC`, nothing else from the caller.
  `env` adds what it needs as `NAME=value` (`GEM_PATH`, `JAVA_HOME`, a variable the program reads); each one is
  an environment dependency the survey records. A JVM or Python under `LC_ALL=C` writes non-ASCII output as `?`;
  when the locale probe shows that, capture with `"env": ["LC_ALL=C.UTF-8"]` so the goldens are not lossy.
