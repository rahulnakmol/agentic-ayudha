# rahulnakmol/agentic-ayudha#1256: The build brief has no Goldfish review: no fresh session of another model family reviews the draft PR before it is ready

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `ee28dc739b0daad2a2bae49042d2be2eb46bfeecfe4e0ee809510a5e1a7b9e17` |
| Completion Goal hash | `91abdc9a0519d2a63aebd07ef245c70ddb1ff8a96cc6dafc4dcc41f90bec4917` |
| Technical Owner | ai-native-engineer |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1256/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `aayu build <n> --delegate amp` for a Ready story; expected: the brief has a Goldfish review step that names the reviewer model, the four inputs, the mean-review instruction, the stop rule (only P3 left, at most 3 rounds), and the RAID log
2. [api-trace] state: the builder's model family equals the family of `[inspect] peer_model`, or peer_model is empty; expected: the brief names the first review candidate whose family differs from the builder's, as `inspect` picks it (internal/cli/inspect.go:662)
3. [api-trace] state: the brief's Goldfish step; expected: it says the review is a preview and never a verdict, and that a P0 or P1 the builder declines goes to a person in the next window

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

The build brief has no Goldfish review: a fresh review session on a model of another family, before the PR is ready. Its "Review loop and stop conditions" section only says that `inspect` reviews the PR in the merge gate (internal/delegate/brief.go:189). So every finding costs a CI round of about 18 minutes plus a paid review of up to 32 minutes. Merged PRs took 2.8 heads on average (25 PRs merged since 2026-10-30). @rahulnakmol asked for the Elephant-Goldfish Model (https://drensin.medium.com/elephants-goldfish-and-the-new-golden-age-of-software-engineering-c33641a48874). Under it, a fresh session that sees only the design and the diff reviews the code until only nits remain. Today the protocol is only a message to builders (.amp/in/artifacts/d3-pastes/goldfish-review.md), so a builder that aayu starts later does not get it. This Bug depends-on #1250, the draft-first brief.

### Steps to reproduce

1. Run `aayu build <n> --delegate amp` for a Ready story.
2. Read `.aayudha/state/delegate/<n>/brief-1.md`.

### Expected behaviour

The brief has a Goldfish review step. On the draft PR, the builder opens a fresh review session on the model that `[inspect] peer_model` picks, of another family than the builder's (internal/config/config.go:142). The session gets only the Story's criteria and Review guidance, the Feature's technical design, and the PR diff at its head. Its instruction is a "mean review", with file:line, a severity, and the smallest fix for each finding. The builder fixes and repeats until only P3 findings remain, for at most 3 rounds, and records each finding in the RAID log. Then the builder marks the PR ready once. The merge gate's review still decides.

### Actual behaviour

The brief has no review before ready, so the merge gate's review finds what a fresh session would have found, one paid round at a time.

### Completion Goal

1. [api-trace] state: `aayu build <n> --delegate amp` for a Ready story; expected: the brief has a Goldfish review step that names the reviewer model, the four inputs, the mean-review instruction, the stop rule (only P3 left, at most 3 rounds), and the RAID log
2. [api-trace] state: the builder's model family equals the family of `[inspect] peer_model`, or peer_model is empty; expected: the brief names the first review candidate whose family differs from the builder's, as `inspect` picks it (internal/cli/inspect.go:662)
3. [api-trace] state: the brief's Goldfish step; expected: it says the review is a preview and never a verdict, and that a P0 or P1 the builder declines goes to a person in the next window

### Measure

Completion Goal criteria observed

### Threshold

all 3 criteria

### Scenario

Write a brief for a story built by an Anthropic model and for one built by an OpenAI model, with and without `[inspect] peer_model`.

### Environment

go test of internal/delegate and internal/cli with a fake GitHub; never production

### Checkpoints

1. The brief carries the Goldfish step with the right model (criteria 1, 2)
2. The step keeps the merge gate's review as the verdict (criteria 3)

### Technical Owner

ai-native-engineer

### Assumptions

#### Notes

This Bug changes the brief only. Recording the Goldfish thread as attested review evidence, so it could stand in for the CI review, is a separate spike: in an orb, aayu and gh write as the person, so a review a builder reports cannot yet be told apart from one it made up.

#### Review guidance

- Check: criterion 2: the reviewer's family is never the builder's.
- Ignore: the wording of the mean-review instruction.
- Risk: a brief that tells the builder the Goldfish review is enough to merge; criterion 3 catches it.

#### Behaviours

1. says: A fresh session of another family reviews each PR before it is ready; rule: limit; source: criteria 1, 2
2. says: The merge gate's review stays the verdict; rule: never; source: criteria 3

### Links

depends-on #1250

### Provenance

Request: "Also instead of running code review in gitub action what we use subagent with a named model not of same family or a thread to review the code like how Elephants, Goldfish and the New Golden Age of Software Engineering this been followed"
1. stated (request): a review by a named model of another family, in a thread
2. default (scope): the model comes from [inspect] peer_model, as inspect picks it
3. default (scope): the merge gate's review stays the verdict
````
