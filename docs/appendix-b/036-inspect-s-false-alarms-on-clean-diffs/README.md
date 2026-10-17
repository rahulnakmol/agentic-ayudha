# Deviations recorded while fixing inspect's false alarms on clean diffs (WP4.8d, after v0.2.0)

Founder request (2026-10-17, relayed by the orchestrating thread): inspect averaged 4.19 over the review group's 36
case-variants in full run 38980397572 (7542b85) but scored 1, 1 on `inspect/clean-retry-backoff` in every lane; fix
the skill or the review plan so a clean diff gets a clean verdict without losing recall, add a clean-diff case, bring
every inspect case to at least 4.0, and decide for every failure who was wrong (the skill, the case, or the rubric),
within 40 USD of tier-2 spend. The review group scores by precision and recall against the planted defects, with no
judge, at its configured k = 2 (D-4.8b.5); it stays advisory.

| # | Plan assumption | Finding | Implemented instead |
|---|---|---|---|
