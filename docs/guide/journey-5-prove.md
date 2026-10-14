# 5. Prove it

An agent saying "done, all tests pass" is a claim. This step turns the claim into evidence. CI runs fixed checks on
the exact commit you would merge, collects evidence for every promise in the story, and a second model reviews the
change against those promises. You get a one-page verdict.

> **You are here:** step 5 of 9. [Shape](journey-1-shape.md) · [Break down](journey-2-break-down.md) ·
> [Ready](journey-3-ready.md) · [Build](journey-4-build.md) · **Prove** · [Ship](journey-6-ship.md) ·
> [Release](journey-7-release.md) · [Learn](journey-8-learn.md) · [Explain](journey-9-explain.md)

## Three layers of proof

1. **`verify`** runs the **floors**: fixed checks with fixed thresholds, such as test coverage of the changed lines,
   security scans, leaked secrets, vulnerable dependencies, accessibility, and page speed. A floor either holds or
   misses. No model can overrule a miss. See [floors](concepts-floors.md).
2. **The merge gate** runs `verify` again in CI, on the pull request's head commit, using the workflow from the base
   branch, so a pull request cannot weaken its own checks. It writes the **authoritative proof pack**: a file that
   maps each criterion of the story to its evidence, with every floor result. See [proof packs](concepts-proof.md).
3. **`inspect`** asks a model from a different vendor than the builder to review the pull request against its plan,
   its pack, and its criteria. It returns one of three verdicts: `merge`, `fix`, or `rebuild`, with a verdict and
   evidence per criterion.

## What you do, and what the agent does

**You** read the verdict on the pull request. It is short on purpose.

**The agent and CI** do the rest. The gate starts by itself when the pull request is updated.

## In the benefits-qa story

> **In the story:** pull request 148 (story 131). The merge gate took about 8 minutes, and `inspect` about 2 more.

The floors on the final commit:

| Floor | Result |
|---|---|
| Coverage of changed lines | 100 % (116 of 116 lines; the floor is 90 %) |
| Security scan (semgrep) | 0 findings |
| Leaked secrets (gitleaks) | 0 findings |
| Vulnerable dependencies (osv-scanner) | 0 critical, 0 high |
| Accessibility, page speed | Skipped: this story has no screens |

The full record is the [proof pack on the ledger branch](https://github.com/rahulnakmol/agentic-ayudha/blob/ledger/proof/131/b046d1a1d113339eb80e130c9dc0008fa223e76f/authoritative/pack.json),
with an evidence file per criterion. Then `inspect` posted its verdict:

> **inspect: merge.** The implementation satisfies all three completion criteria. Retrieval returns the full
> selected passage with its citation, policy screens the specified prohibited questions before retrieval, and
> unsupported questions return an HR referral without quoted content.

It gave each criterion its own verdict and evidence, named the reviewer model, and showed the spend: 0.37 of the
5.00 USD allowed for reviews of this pull request.

For story 134, the Ask page, the screen floors ran too: 0 serious or critical accessibility violations, every
control reachable by keyboard, nothing clipped at 200 % text size, no target smaller than 44 by 44 pixels, and a
page-speed score of 100 against a budget of 90.

## When the gate says no

It will, and that is the point. Across the five stories, `inspect` ran 17 times, against a plan of 10. On three
stories it found a new, real problem on each review: edge cases in the seed check, in request decoding (trailing
input, duplicate keys, a null user ID), and in the analytics simulation. A pull request gets two rounds of fixes.
After that, aayudha stops spending and asks for a fresh start. Four pull requests hit that limit. Each was closed
with a note, and its story was rebuilt from the latest code as a new pull request, with every finding fixed and a
test for each.

> **Tip:** treat `fix` as the system working, not failing. Each finding in benefits-qa was a real defect, and each
> one got a test so it stays fixed.

### What the gate needs

> **Needs setup:** the merge gate comes with `aayu init --gates`. `inspect` in CI needs a model key stored as a
> repository secret. Without a bot GitHub App, CI cannot write the pack to the ledger itself: the pack waits as a
> file on the CI run, and `ship` records it under your name. See [Adopt at your own pace](adopt.md).

## What you get back

- A green or red merge gate on the pull request, with every floor result.
- An authoritative proof pack: evidence for each criterion, tied to the exact commit.
- A verdict from `inspect`, per criterion, with its cost.

> **Well done:** you know the change keeps its promises, and you can show anyone why. No more "trust me".

## What's next

The proof is in. Time for your yes: [ship it](journey-6-ship.md).
