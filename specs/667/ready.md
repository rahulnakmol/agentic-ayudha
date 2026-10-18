# rahulnakmol/agentic-ayudha#667: proof: the media bucket and its secret come from the run's checkout config

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `51bfd5c4b421ed50350f27edbdf9536ba94e565ba4c2e10c68992efd6427785a` |
| Completion Goal hash | `10da9cee245b9df9f803f75cf6d92f93985dd54c664c14d9a85bae49970548a1` |
| Technical Owner | quality-architect |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/667/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: a checkout `[proof] media` and `media_secret` that differ from the base branch; expected: ship refuses, or uses the base branch's values, and sends no secret to the checkout's bucket

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

`aayu ship` and other verbs build the proof store from the config of the run (`proofStore` in `internal/cli/proof.go`). That config sets `[proof] media` and `[proof] media_secret`. An agent can set both in its checkout or with `--set`. The store then sends the named local secret to a bucket that the agent chose. The deep review of PR #661 found this (finding 6). The behaviour is older than #661.

### Steps to reproduce

1. In a checkout, set `[proof] media` to a bucket that an agent controls, and `media_secret` to the name of another local secret.
2. Run `aayu ship <pr>`, which lands the gate's pack with media.

### Expected behaviour

The media bucket and its secret come only from a source that the person controls: the user config or the base branch's committed config.

### Actual behaviour

The run's checkout or `--set` selects the bucket and the secret.

### Completion Goal

1. [api-trace] state: a checkout `[proof] media` and `media_secret` that differ from the base branch; expected: ship refuses, or uses the base branch's values, and sends no secret to the checkout's bucket

### Measure

Runs that send a secret to a bucket that the checkout selected.

### Threshold

0.

### Scenario

An agent runs a verb that lands a pack with media.

### Environment

`aayu ship`, `aayu proof`; the fake in tests.

### Technical Owner

quality-architect

### Links

depends-on #661

### Checkpoints

1. The media bucket and its secret come from a trusted layer (criterion 1)

### Assumptions

The user config is a trusted layer, as it is for `[models.gateway]`.

### Open questions

deferred: refuse a checkout value, or ignore it? (the #645 rule refuses; keep the same rule)

### Parked ideas

None.

### Provenance

Request: "File fail-closed and P2/P3 findings as Bugs"
1. default (behaviour): "File fail-closed and P2/P3 findings as Bugs"
````
