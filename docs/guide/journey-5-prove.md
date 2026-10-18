# 5. The gate says no

> *Monday, 07:43 UTC.* Eight minutes after the first pull request opened, a comment lands on it. You read the first
> line: "inspect: fix". The agent's tests all pass. The checks all pass. And a reviewer has just found a way for a
> stranger's email address to slip into the plan documents.

## Three layers of "prove it"

An agent saying "done, all tests pass" is a claim. This chapter is where claims become evidence, or fail to.

1. **The merge gate** runs in CI on the exact commit that would merge, using the workflow from the base branch, so a
   pull request cannot weaken its own checks. It runs `verify`: the **floors**, fixed checks with fixed thresholds,
   such as test coverage of the changed lines, security scans, leaked secrets, vulnerable dependencies,
   accessibility, and page speed ([floors](concepts-floors.md)).
2. It writes the **proof pack**: a file that ties each of the story's promises to its evidence, with every check's
   result ([proof packs](concepts-proof.md)). The pack, screenshots and recordings included, stays on the CI run, and
   a comment on the pull request shows each promise's verdict, the checks, and the pack's fingerprint. It opens with
   a risk line, high, medium, or low, that tells the person who merges where to look first. For a Bug, the gate also
   runs the proving tests on the code before the fix: a test that passes there does not show the fix.
3. **`inspect`** asks a reviewer from a different family, that is, another vendor, than the agent that wrote the
   code, to review the change. aayu reads its reply as `merge`, `fix`, or `rebuild`, with a verdict for each promise.
   No reply means no merge.

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

In all, five stories took **17 reviews**. Your plan for the day allowed 10: one review per story plus one round
of fixes. The gate itself allows two rounds before it closes a pull request, and it needed them. Four pull requests (135, 140, 151, 156) were thrown
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

> **Since then:** on 12 October the reviewer was a model the merge gate called with a key held in CI. Now the gate
> calls a review model of another vendor itself, with one key held in the `code-review` environment, which only the
> base branches and the default branch reach ([code review in the gate](how-review.md)). An agent app from another
> vendor, such as Claude or Codex, can review instead, and aayu reads its reply. The gate can also run behaviour checks from a feature map, which drive
> your product the way a person would, and for a Bug it shows the proving tests failing on the base commit first.
> See [a pull request is ready](guide-review.md) and [prove a behaviour change](guide-prove-behaviour.md).

## What you do, and what the agent does

**You** read the verdict and the proof comment on the pull request. They are short on purpose. Then you go back to
your coffee: the fixing is not your job.

**The agents and CI** do the rest: the gate starts by itself, the builder fixes what the reviewer finds, and a
capped pull request is rebuilt.

## What you learn

Two reviewers that disagree, one building and one checking, find more than one reviewer that agrees with itself. And
when a model and a measurement disagree, believe the measurement.

> **Needs setup:** the merge gate comes with `aayu init --gates`, and the review needs a reviewer app installed on
> your account and named in `[inspect] apps`. The proof needs nothing else: no bot App, bucket, or secret. The pack waits as a file on the CI
> run for `[proof] retention_days` (90 by default), and `ship` checks it came from that run and records it under your
> name. An agent's own preview, even with screenshots and video in your thread, never counts. See
> [Adopt at your own pace](adopt.md).

## The verbs in this step

<!-- BEGIN GENERATED: stage-verbs -->

| Verb | What it does |
|---|---|
| [`verify`](skills/verify.md) | Runs the fixed checks (tests, security, secrets, dependencies, accessibility, speed) and records the results. |
| [`inspect`](skills/inspect.md) | A model from another vendor, or an agent app you choose, reviews the pull request against every promise; aayu reads its reply as merge, fix, or rebuild. |
| [`proof`](skills/proof.md) | Stores, fetches, and checks the proof packs that hold the evidence for each change. |

<!-- END GENERATED: stage-verbs -->

> **Under the hood:** the workflow, the sandbox `verify` runs in, and inspect's structured verdict are in
> [the merge gate](how-merge-gate.md); every floor's tool and threshold is in [floors in detail](how-floors.md); the
> pack's format is in [proof packs](how-proof-packs.md).

## What's next

At 09:03, pull request 142, the plan documents story, is the first to turn green, with a pack and a "merge". One
thing is missing, and only a person can give it. You reach for the approve button: [chapter 6, the yes](journey-6-ship.md).

> **You are here:** chapter 5 of 9. [Goal](journey-1-shape.md) · [Plan](journey-2-break-down.md) ·
> [Promises](journey-3-ready.md) · [Build](journey-4-build.md) · **Gate** · [Yes](journey-6-ship.md) ·
> [Release](journey-7-release.md) · [Number](journey-8-learn.md) · [Telling](journey-9-explain.md)
