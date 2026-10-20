# 6. The yes

> *Monday, 09:03 UTC.* Pull request 142, the plan documents story, is finally green: a pack, and a "merge" from the
> reviewer. Five reviews and two thrown-away pull requests are behind it. Now it needs a person's yes from someone other than its author. On a team of two, your colleague would give it.
> You have no colleague to ask. You open the pull request, find the approve button, and GitHub will not let you use
> it.

## Why a person says the last word

Floors decide whether a change may merge. A person decides whether it should. In aayudha that decision is the
**human pass**: an approving review from a person's GitHub account, never a bot's. `ship`, the verb that merges, runs
only under a person's identity, and it refuses to merge unless everything lines up: the merge gate is green on the
exact head commit, the proof pack is the one the gate built on that commit (`ship` fetches it from the CI run and
checks every fingerprint, so a pack the agent made itself never counts), the human pass is there, and for risky
layers the plan was reviewed.

## The one thing you cannot get

With one person, a real human pass is impossible: GitHub does not let you approve your own pull request. That
leaves two bad options and one honest one. You could skip the check and say nothing. You could approve from a second
account and pretend. Or you can merge in the open and leave a record that says exactly what was missing.

aayudha only allows the third, and only when it is true. `ship --solo-bypass` refuses if a second person is paired
with a seat or a bot App exists, because then a real review is possible. Everything else still has to hold: the merge
gate green on the exact commit, the gate's own pack, inspect's "merge". At 09:03 you post a comment on pull request
142 that begins "Solo-mode bypass:". It says the gate is green with inspect's "merge", that no second person can
review, and that the pull request merges through `aayu ship 142 --solo-bypass`. It ends: "No review was given or
faked."

A minute later you run it. `ship` merges the pull request and writes story 130's Outcome, here slightly shortened:

- Merged in pull request 142 as commit `b5359bd` on 2026-10-12.
- Authoritative pack: built by the merge gate in CI, and landed on the ledger by `ship` under your name as the
  founder. CI never writes the ledger; `ship` takes the pack from the gate's own run on that commit.
- Human pass: none possible. You, the founder, merged under a recorded solo-mode bypass.
- Plan-time review: none. You recorded a solo-mode plan-review bypass for layer L07.
- The ship record, and the log of every write `ship` made, both on the ledger.

All five of your stories ship this way, between 09:04 and 14:28. Every bypass is a line that `audit` will list in
its own section, so you, or whoever audits you, can review each one after the fact. The moment a second person joins a seat, the
bypass stops being allowed and the real review is required again.

> **Tip:** run `aayu status <goal>` after shipping. It rolls up progress and proof for the whole Goal. For Goal 125 it
> reached 5 of 5 stories shipped with authoritative proof.

## What you do, and what the agent does

**You** give the yes. On a team of two or more, a teammate approves the pull request on GitHub and you run
`aayu ship <pr>`; alone, you run it with `--solo-bypass`, as you did today. If any check is missing, `ship` refuses
and tells you which one.

**`ship`** checks everything, copies the gate's pack (without screenshots and recordings) to the ledger, squash-merges
into your integration branch (`dev`, or `main` if that is all you have), writes the Outcome, releases the agent's
lease, and cleans up the branch.

## What you learn

A safeguard you cannot meet is not a reason to fake it. Record that it was skipped, say why, and make it easy to
review later. That is what makes the rest of the record trustworthy.

## The verbs in this step

<!-- BEGIN GENERATED: stage-verbs -->

| Verb | What it does |
|---|---|
| ![ship](../../design/icons/verbs/ship.svg) [`ship`](skills/ship.md) | Merges a proven pull request after your yes, and writes down who approved what, on which evidence. |

<!-- END GENERATED: stage-verbs -->

> **Under the hood:** how a human pass is recognised, what solo mode checks, and why agents write as a bot are in
> [trust and identities](how-trust.md). Where the ship record and the bypasses are kept is in
> [the ledger branch](how-ledger.md).

## What's next

Merged is not the same as in people's hands. Everything on `dev` still has to climb one more gate:
[chapter 7, the release](journey-7-release.md).

> **You are here:** chapter 6 of 9. [Goal](journey-1-shape.md) · [Plan](journey-2-break-down.md) ·
> [Promises](journey-3-ready.md) · [Build](journey-4-build.md) · [Gate](journey-5-prove.md) · **Yes** ·
> [Release](journey-7-release.md) · [Number](journey-8-learn.md) · [Telling](journey-9-explain.md)
