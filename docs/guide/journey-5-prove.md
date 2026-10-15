# 5. The gate says no

> *Monday, 07:43 UTC.* Eight minutes after the first pull request opened, a comment lands on it. Sam reads the first
> line: "inspect: fix". The agent's tests all pass. The checks all pass. And a reviewer has just found a way for a
> stranger's email address to slip into the plan documents.

## Three layers of "prove it"

An agent saying "done, all tests pass" is a claim. This chapter is where claims become evidence, or fail to.

1. **The merge gate** runs in CI on the exact commit that would merge, using the workflow from the base branch, so a
   pull request cannot weaken its own checks. It runs `verify`: the **floors**, fixed checks with fixed thresholds,
   such as test coverage of the changed lines, security scans, leaked secrets, vulnerable dependencies,
   accessibility, and page speed ([floors](concepts-floors.md)).
2. It writes the **proof pack**: a file that ties each of the story's promises to its evidence, with every check's
   result ([proof packs](concepts-proof.md)).
3. **`inspect`** hands the promises, the change, and the pack to a reviewer model from a different family, that is,
   another vendor, than the one that wrote the code. It answers `merge`, `fix`, or `rebuild`, with a verdict and evidence for each promise.

A pull request gets two rounds of fixes. If it still is not right, aayudha stops spending and asks for a fresh start:
the pull request is closed, and the story is built again from the latest code.

## The email check that would not die

Remember the routine promise from chapter 3: refuse a seed file with an email address outside example.com. Here is
what happened to it.

| Time | Pull request | Verdict | What the reviewer found |
|---|---|---|---|
| 07:43 | 135 | fix | The check skipped the plan and section fields |
| 07:56 | 135 | fix | An address in the seed's metadata, or one like `person@[192.0.2.1]`, got through |
| 08:01 | 135 | closed | Two rounds used; rebuilt from scratch |
| 08:20 | 140 | fix | Addresses with a quoted local part got through |
| 08:41 | 140 | fix | `hr@example.comé.org` passed, because the check read only the plain-letter start of the domain |
| 08:42 | 140 | closed | Two rounds used; rebuilt from scratch |
| 09:03 | 142 | merge | The new check reads everything after each "@", so only example.com passes |

Four times the agent's own tests passed and the reviewer still found a new way in. Each finding became a test, so
the final check refuses all four. That is the privacy stake from the prologue, held by a promise nobody thought was
hard.

The same thing happened elsewhere. On the API story, the reviewer sent pull request 151 back twice: once for extra
text after the JSON request, once because `{"Question": …}` with a capital Q returned 200, slipping past the
published contract. On the analytics story, pull request 156 went back twice, the second time because the simulated
HR answers could land after the end of the measured window. Both were closed and rebuilt, as 155 and 158, with every
finding fixed and tested.

In all, five stories took **17 reviews**. The team had budgeted 10: one review per pull request plus one round
of fixes. Four pull requests (135, 140, 151, 156) were thrown
away. No pull request's reviews cost more than 2 USD in total; pull request 148's cost 0.37 USD.

> **Tip:** treat `fix` as the system working, not failing. Each of these findings was a real defect, and each one now
> has a test that keeps it fixed.

## When the numbers said yes

Not every "no" was right, and aayudha has a rule for that too: a measurement beats a model's opinion.

On the Ask page, the model half of design review flagged the footer text as too faint, "about 4.2:1", below the 4.5:1
that readable text needs. The fixed checks disagreed. The measured ratio of those two colours is 6.46:1, and the
merge gate's accessibility scan on the same commit found 0 serious contrast problems. The page shipped on the
measured result. The model's note stays on the pull request, where a person can read it and decide.

## What passed

Story 131's final commit, pull request 148: 116 of 116 changed lines covered by tests (the floor is 90 %), 0 security
findings, 0 leaked secrets, 0 vulnerable dependencies. Then, at 09:31:

> **In the story:** "inspect: merge. The implementation satisfies all three completion criteria. Retrieval returns
> the full selected passage with its citation, policy screens the specified prohibited questions before retrieval,
> and unsupported questions return an HR referral without quoted content."

The Ask page, story 134, passed every screen floor: 0 serious or critical accessibility problems, every control
reachable by keyboard, nothing clipped at 200 % text size, nothing to tap smaller than 44 by 44 pixels, and a
page-speed score of 100 against a budget of 90. Both records are on the ledger branch: the
[pack for story 131](https://github.com/rahulnakmol/agentic-ayudha/blob/ledger/proof/131/b046d1a1d113339eb80e130c9dc0008fa223e76f/authoritative/pack.json)
and the
[pack for story 134](https://github.com/rahulnakmol/agentic-ayudha/blob/ledger/proof/134/8e01a42142d028066c3026ca9201bb640b0e6914/authoritative/pack.json).

## What you do, and what the agent does

**You** read the verdict on the pull request. It is short on purpose.

**The agents and CI** do the rest: the gate starts by itself, the builder fixes what the reviewer finds, and a
capped pull request is rebuilt.

## What you learn

Two reviewers that disagree, one building and one checking, find more than one reviewer that agrees with itself. And
when a model and a measurement disagree, believe the measurement.

> **Needs setup:** the merge gate comes with `aayu init --gates`. `inspect` in CI needs a model key stored as a
> repository secret. Without a bot GitHub App, CI cannot write the pack to the ledger itself: the pack waits as a
> file on the CI run, and `ship` records it under your name. See [Adopt at your own pace](adopt.md).

## The verbs in this step

<!-- BEGIN GENERATED: stage-verbs -->

| Verb | What it does |
|---|---|
| [`verify`](../../skills/verify/SKILL.md) | Runs the fixed checks (tests, security, secrets, dependencies, accessibility, speed) and records the results. |
| [`inspect`](../../skills/inspect/SKILL.md) | A second model, from another vendor, reviews the pull request against every promise and says merge, fix, or rebuild. |
| [`proof`](../../skills/proof/SKILL.md) | Stores, fetches, checks, and cleans up the proof packs that hold the evidence for each change. |

<!-- END GENERATED: stage-verbs -->

> **Under the hood:** the workflow, the sandbox `verify` runs in, and inspect's structured verdict are in
> [the merge gate](how-merge-gate.md); every floor's tool and threshold is in [floors in detail](how-floors.md); the
> pack's format is in [proof packs](how-proof-packs.md).

## What's next

Pull request 148 is green, with a pack and a "merge". One thing is missing, and only a person can give it. Sam
reaches for the approve button: [chapter 6, the yes](journey-6-ship.md).

> **You are here:** chapter 5 of 9. [Goal](journey-1-shape.md) · [Plan](journey-2-break-down.md) ·
> [Promises](journey-3-ready.md) · [Build](journey-4-build.md) · **Gate** · [Yes](journey-6-ship.md) ·
> [Release](journey-7-release.md) · [Number](journey-8-learn.md) · [Telling](journey-9-explain.md)
