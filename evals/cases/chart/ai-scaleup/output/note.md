# Evidence design: dependant-coverage answers (#52)

## Question

Employees ask whether a dependant is covered, to what age, and at what cost, and today most of these go to HR (#52).
A wrong cost or age limit is the expensive failure: the issue asks for exact cost answers (#52), and the answer goes
to an employee deciding whether to enrol a child.

## Simpler options

The evidence points at two narrow gaps, not at a new retrieval stack. On the 24 dependant questions, BM25 recall at 3
is 0.58 against 0.86 overall (`answer/testdata/eval.jsonl`).

- **Numbers are a lookup.** Three misses ask for a premium or an age limit (`answer/testdata/eval.jsonl`), and those
  numbers already sit in `store/plans.csv` (`store/plans.go:8-15`). Answer them from the employee's row, found with
  `PlanFor` (`store/enrolment.go:22-30`), and quote the document passage beside it; the model never writes the number.
- **Paraphrases are a vocabulary gap.** Seven misses use words the documents lack, such as "kids" and "my partner"
  (`answer/testdata/eval.jsonl`). Add a reviewed synonym list to the BM25 query in `answer/retrieve.go:18-40` and
  rerun the evaluation.
- **GraphRAG and a vector database are not justified yet.** No evaluation question needs more than one hop
  (`answer/testdata/eval.jsonl`), and the 140 ms p95 leaves no reason to add an index for speed. Trigger for hybrid
  dense retrieval: dependant recall at 3 stays under 0.85 after the synonym list. Trigger for a graph: the evaluation
  gains relationship questions, such as comparing plans across years, that fail with both.

## Evidence and permissions

- Sources: plan documents with `employer`, `plan`, `version`, and `effective_from` (`store/docs.go:11-20`) and the
  plan table (`store/plans.go:8-15`). Every answer cites the document ID, version, and section that `Retrieve`
  returns (`answer/retrieve.go:18-40`).
- Permissions: `Retrieve` filters by employer, but an employee may read only their own plan
  (`store/enrolment.go:22-30`, `answer/retrieve.go:18-40`). Filter by employer and plan before ranking, so
  retrieval never widens what an employee may read.
- Versions: both Acme Gold years are indexed and disagree on the age limit (`store/docs/acme-gold-2025.md:41`,
  `store/docs/acme-gold-2026.md:39`). Retrieve only the version in effect today by `effective_from`
  (`store/docs.go:11-20`).
- Prompt injection: a broker footnote tells the assistant to promise full free cover
  (`store/docs/globex-silver-2026.md:88`). Document text is quoted, never followed; costs come only from
  `store/plans.csv`, and passages that address the assistant are flagged for HR review before indexing.

## Abstention

Answer only when the passage at or above `MinScore` states the answer (`answer/retrieve.go:18-40`); otherwise refer
to HR as today. When the table and the document disagree, or two versions are in effect, refuse and refer, citing both.

## Evaluation

Extend `answer/testdata/eval.jsonl`, which has no adversarial case today, and run it in `go test ./answer -run TestEval`.

| Slice | Cases | Metric | Threshold |
|---|---|---|---|
| Dependant paraphrases | 60, including the 24 today | recall at 3 | ≥ 0.85 |
| Cost and age limits | 30 | exact match with `store/plans.csv` | 100 % |
| Adversarial: the injected footnote (`store/docs/globex-silver-2026.md:88`), another employee's plan, the superseded version, out of scope | 20 | correct refusal or referral, zero cross-plan passages | 100 % |
| All answered cases | all | citation names the supporting passage | ≥ 0.95 |

- Latency: p95 stays at or under 300 ms on the CI runner (140 ms today, `answer/testdata/eval.jsonl`).
- Cost: retrieval stays BM25 plus a table lookup with no model call (`answer/retrieve.go:18-40`), so the cost per answer
  does not change.

## Open questions

- Assumption: the 300 ms latency budget and the thresholds above stand until the PM sets others.
- Who owns the synonym list, HR or the PM? It decides how fast new words reach answers; default HR, reviewed monthly.
- Should broker-edited documents be indexed before HR reviews them? It decides the injection exposure; default no.
