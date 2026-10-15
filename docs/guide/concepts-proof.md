# Proof packs

When an agent says "done", how do you know? A **proof pack** is the answer. It is a small file, written by CI for
one exact commit, that maps every promise in the story to the evidence that it was kept, and records every check
that ran.

![A story's numbered criteria and the pull request's head commit both feed the merge gate in CI, which writes a proof pack with evidence for each criterion; ship reads the pack before it merges.](img/proof.svg)

*CI writes the pack, not the agent that did the work.*

## What is in a pack

- The story, the pull request, and the exact commit it describes.
- The fingerprint of the story's Completion Goal at Ready, so the pack is checked against what was agreed.
- For each criterion, the evidence: API traces, screenshots, recordings, screen-reader transcripts, or metrics.
- Every floor result, with the tool, its version, the value, and the threshold.
- Who wrote it, and from which CI run.

## Two kinds of pack

| Kind | Written by | Counts for shipping |
|---|---|---|
| Authoritative | The merge gate, in CI, using the base branch's workflow | Yes |
| Advisory | An agent, on its own machine | No: it helps the agent, but it is a claim, not proof |

This split is the whole point. The agent that did the work never gets to certify it.

## Why proof packs exist

"It works on my machine" and "the model says it is fine" are not evidence. A pack is. It lets you:

- **Ship with confidence.** `ship` refuses to merge without an authoritative pack for the head commit.
- **Answer questions later.** Months on, anyone can open the pack and see what was checked, and how.
- **Claim only what is true.** The docs, release notes, and briefs link a Feature only when every story under it
  has an authoritative pack.

> **In the story:** here is the
> [real pack for story 131](https://github.com/rahulnakmol/agentic-ayudha/blob/ledger/proof/131/b046d1a1d113339eb80e130c9dc0008fa223e76f/authoritative/pack.json).
> It maps three criteria to three API-trace files and records the floors: 116 of 116 changed lines covered, no
> security findings, no secrets, no vulnerable dependencies.

## Where packs live

The merge gate keeps each pack, screenshots and recordings included, as a file on its CI run, and posts a comment
on the pull request with each promise's verdict, the check results, and the pack's fingerprint. GitHub deletes that
file after 90 days by default (`[proof] retention_days`). When you run `ship`, it checks that the file came from the
gate's own run on the exact commit and copies the pack, without its screenshots and recordings, to the `ledger`
branch: a separate branch in your own repository that holds records, never code. The pack and its fingerprints stay
there for good. None of this needs a bot App, a storage bucket, or a secret.

Terms are defined in the [glossary](../../GLOSSARY.md).
> **Under the hood:** the `pack.json` format, field by field, is in [proof packs](how-proof-packs.md); where packs are
> stored is in [the ledger branch](how-ledger.md).

## What's next

The checks inside a pack are called floors. See [floors](concepts-floors.md).
