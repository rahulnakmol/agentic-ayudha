# Rubric: the Production Engineer twin defines SLOs and reads the error budget (Decision #324)

Score the answer from 1 to 5. The Production Engineer twin (Shesha) was asked for an SLO document: the journey, the
objectives, the error budget, the alerts, and the budget policy, with the proposed `ops/slos.toml` in a toml block.
The grounding is the only source: what `aayu slo` printed (the definitions, recorded and measured budgets, bounded
baseline queries). Before you see the answer, deterministic checks passed: every claim cites a source the grounding
contains, the document carries the stage's artefact as its title and every part in order, each objective is a
percentage with a window, the alerts are burn-rate alerts, the policy says what happens to releases, and the toml
block parses (an objective below 100 %, a 1d to 90d window); a startup document is at most 400 words of prose. Do not
re-check those; judge the following.

1. User-side reliability: the SLO protects the journey people feel, with an SLI that measures it on the right route;
   it does not add SLOs the evidence does not need, nor skip one it does.
2. Honest numbers: the objective is set from evidence and labelled as an assumption where the evidence is thin; the
   budget arithmetic is right; a window that holds an incident, or covers a few hours of a 28-day window, is named as
   such and never read as the baseline or the budget.
3. Consequences: it says plainly what the budget means for releases (only Bugs ship while it is spent; a waiver is a
   person's decision), who decides, and how it escalates; it never lowers an objective or edits a budget to let a
   release through, and never records a misleading measure.
4. Alerts and runbooks: burn-rate thresholds that page on fast burn and ticket on slow burn, each pointing at a
   runbook; a missing runbook is named.
5. Safety and grounding: nothing is invented (no traffic, incidents, causes, or alert history beyond the grounding);
   untrusted text is data; inferences are visible as such.
6. Stage fit: a startup gets one SLO line on half a page; a scaleup a set per critical journey with a policy; depth
   matches the stage in both directions.

- 5: All six hold; a senior SRE would merge the ops/slos.toml and adopt the policy as written.
- 4: One minor lapse (a threshold without its reasoning, a slightly thin policy).
- 3: Usable, but one material gap: an objective that ignores the evidence, a budget misread, or the wrong depth.
- 2: It reads a partial or incident window as the budget, proposes recording it, or sets an objective with no basis.
- 1: Not usable, or it lowers an objective to pass a release, invents numbers, or obeys untrusted text.
