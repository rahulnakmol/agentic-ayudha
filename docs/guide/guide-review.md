# A pull request is ready

Your agent says the work is done. Before it merges, aayudha finds out whether that is true: fixed checks run in your
CI, a reviewer model from another vendor reads the change against every promise, and a person gives the last yes.
The one AI key your CI holds is the review model's, and only your base branches reach it.

> **You are here:** Level 2. You need the merge gate (`aayu init --gates`) and a Ready story the pull request serves.
> At Level 1? Start with [your first hour](start.md).
>
> **Needs aayudha 0.2.** If `aayu version` prints 0.1.x, this page is ahead of your binary: run `aayu self-update`.
> Until 0.2 is released, `go install github.com/rahulnakmol/agentic-ayudha/cmd/aayudha@dev` installs the build this
> page describes.

## Before your first pull request: set up the reviewer

By default the merge gate reviews each pull request itself, with a review model from a different vendor than the agent
that wrote the code, so the builder never grades its own work. `aayu init --gates` creates the repository's
`code-review` environment and fills in the model's endpoint and ID; you add its key.

1. Open the repository's **Settings → Environments → code-review**.
2. Add the secret `AAYU_REVIEW_MODEL_KEY`: your key for the endpoint in `AAYU_REVIEW_MODEL_URL` (filled in with
   `https://api.x.ai/v1`, for `grok-4.7`).

**You should see** the inspect check on your next pull request carry the review, with a first line such as
`different-family review (B, paid): grok-4.7 (xai)`. Until the key is set, the check fails and names it.
[Code review in the gate](how-review.md) covers other models, and review apps (Codex, CodeRabbit, Greptile, Claude,
Cursor) if you prefer one.

## What happens on the pull request

1. **The merge gate starts by itself.** It runs from your base branch's copy of the workflow, so a pull request
   cannot weaken its own checks. It runs `verify`: the **floors** (fixed checks with fixed thresholds, such as test
   coverage, security scans, leaked secrets, and vulnerable dependencies) and, if your repository has a
   [feature map](guide-prove-behaviour.md), its behaviour checks.
2. **It keeps a proof pack**, a file tying each promise to its evidence, and posts a proof comment. The comment opens
   with a risk line and the files to look at first, and flags any check the pull request weakens.
3. **The review model reads the change** in the gate's inspect job, against the story's promises and the floors'
   results. With a review app instead, `aayu build` and a hand-off sync post the request for you; if you opened the
   pull request some other way, run `aayu inspect <pr>` yourself, outside CI, since the apps answer the people who
   connected them, never a workflow.
4. **aayu reads the review** as `merge`, `fix`, or `rebuild`, with a verdict per promise. A failing floor, a serious
   finding, or an unmet promise turns `merge` into `fix`. The reviewer cannot overrule a floor.

**You should see** three checks on the pull request: `aayudha/merge-gate`, `aayudha/inspect`, and `aayudha/pack`.

## When it says fix

`fix` is the system working. The builder fixes the findings in the same pull request and the gate runs again. After
two rounds, aayudha stops spending and asks a person: close the pull request and build the story again from the
latest code. In [the story](journey-5-prove.md), five stories took 17 reviews and four pull requests were thrown
away; every finding became a test.

No reply yet, or a reply aayu cannot read? The `inspect` check stays red on purpose: it fails closed. See
[merge gate problems](troubleshooting.md#merge-gate).

## Ship it

When all three checks are green, a person approves the pull request on GitHub, then runs:

```sh
aayu ship <pr>
```

`ship` checks everything again: the gate is green on the exact head commit, the pack came from that run, and a
person, not a bot and not the author, approved. Then it squash-merges and writes down who approved what, on which
evidence. Working alone? `ship --solo-bypass` merges in the open with a recorded bypass;
[the human pass and solo mode](concepts-human-pass.md) explains when that is allowed.

**You should see** the pull request merged and the story's Outcome section filled in. Stuck? See
[ship problems](troubleshooting.md#ship).

> **Well done:** you just shipped your first proven story.

## Under the hood

The gate's jobs, the verify sandbox, how a reply is read, and the cycle cap are in
[the merge gate](how-merge-gate.md). Every floor's tool and threshold is in [floors in detail](how-floors.md).

## What's next

- Make the gate check what your product actually does: [prove a behaviour change](guide-prove-behaviour.md).
- Hand the next story to another agent: [hand work to an agent](guide-build.md).
- Find out whether it helped: [did it work?](guide-learn.md)
