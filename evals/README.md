# evals

The skill eval harness: tier 1 deterministic cases run per PR, tier 2 rubric runs nightly or on promotion, with
baselines in `evals/results/<main-sha>.json`. Lands in WP3.4; `evolve` (WP7.7) adds a failing case before any skill diff.
