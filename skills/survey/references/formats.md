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
      {"id": "G1", "title": "…", "from": ["test:tests/test001.js", "entry:src/main.cpp:4"], "compare": "exact stdout and exit code", "exclude": ["random output (E7; Q4 asks whether it matters)"]}
    ],
    "risks": [
      {"id": "R1", "claim": "…", "basis": "observed", "cites": ["src/main.cpp:7-8"]}
    ]
  }
}
```

- Topics. Engineering: architecture, modules, data, interfaces, build, dependencies, tests, risks, quality,
  operations. Product: users, jobs, outcomes, value, constraints, context, competition. Capabilities have a `name`
  instead.
- Citations: a fact ID the index printed (`file:`, `sym:`, `build:`, `dep:`, `entry:`, `io:`, `test:`, `doc:`,
  `marker:`, `mod:`), or `path:line` / `path:start-end` inside the file.
- `target_source` is `"request"` only when the request names the target; otherwise ask, and use the answered
  question's ID.

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
       "completion_goal": "1. [api-trace] state: …; expected: …", "measure": "…", "threshold": "…", "scenario": "…", "environment": "…"}}
  ],
  "open_questions": ["Q1 (open): what the answer would unlock"]
}
```

- `key`: lower-case slug, unique; a re-run finds the issue by it. `parent` and `depends_on` name keys.
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
