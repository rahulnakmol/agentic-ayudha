# Hand work to an agent

A story is Ready. You can let the agent you are talking to build it, or hand it to another agent's cloud: Codex,
Cursor, Claude Code, or Amp. Either way the work comes back as a pull request through the same merge gate, and
nothing merges without a person.

> **You are here:** a Ready story, at Level 2. New to Ready? See [step 4 of your first hour](start.md#4-mark-it-ready).
>
> **Needs aayudha 0.2.** If `aayu version` prints 0.1.x, this page is ahead of your binary: run `aayu self-update`.
> Until 0.2 is released, `go install github.com/rahulnakmol/agentic-ayudha/cmd/aayudha@dev` installs the build this
> page describes.

## Build it here

```sh
aayu build <story>
```

The agent works in its own git worktree, so your checkout is untouched. It follows the plan one layer at a time,
commits a checkpoint after each, checks it, then opens a pull request. A budget keeps watch: by default 8 hours,
3 failed attempts, and 60 USD of metered spend, which applies only when aayudha's own model calls are pay-as-you-go
(a gateway endpoint or a provider key); subscription use is not counted. When it runs out, the agent stops and asks
a person. A pause at a subscription usage limit is not a failed attempt, and the 8 hours keep running. When an agent
building on your machine (`--delegate <agent> --local`) hits its limit and says when it resets, the build waits and
then continues by itself. When it gives no reset time, the build stops as paused: run
`aayu build <story> --delegate <agent> --local` again once the limit resets. When the reset comes after the 8 hours
end, the budget runs out during the wait, so the story is blocked as for any wall-time breach, with no attempt
counted: once the limit resets, a person runs `aayu ready <story>` for a fresh budget, then the same `aayu build`
command.

**You should see** a pull request, and a comment on the story that reads like a flight log: checkpoint, layer, result,
and budget used.

### The session review loop

Before each push, `build` has a reviewer model of a different family from the builder review the branch's change
(Decision #656). It uses inspect's own review: the same prompt, schema, and rules as the merge gate's model review.
When the reviewer says `fix`, the checkpoint is committed but not pushed. `build` lists each open item: a P0 to P2
finding with its file, line, and fix, or an unmet criterion (`C<n>`). The agent then does one of two things for each:

- fixes it, and runs `aayu build <story>` again: the fix is the same checkpoint's next attempt, not a failed one;
- declines it with a reason: `aayu build <story> --decline "<id>: <why>"`, one item a line.

The loop ends when nothing is left open, or after 3 rounds since the last push. Then `build` pushes. The pull
request's description gets a **Session review (self-attested)** section, replaced in place on each push. It lists the
rounds, the reviewer, and which items were fixed, declined (with the reason), or left open at the cap. This is the
builder's own record, not a review that counts: in solo mode a review written in the build session cannot be told
from the builder's word. The merge gate's inspect reviews the pull request again and decides.

The reviewer is the one the merge gate uses when it can be called from your machine:

- `AAYU_REVIEW_MODEL`, `AAYU_REVIEW_MODEL_URL`, and `AAYU_REVIEW_MODEL_KEY`, the same names as the `code-review`
  environment's settings in CI, so you set one thing in both places (or store the key with
  `aayu auth set-secret review-model-key`);
- else `[inspect] paid_reviewer` through `[models.gateway]` or its provider's key;
- else the review lane's model, when its key is here.

It must be of a family the builder cannot be (`AAYU_MODEL`, or the host's build lane). When none can be called, the
push goes out without a session review, and the section says why. A build delegated to another agent is reviewed by
the merge gate only. Each round is metered spend, within `[budget] per_pr_inspect_max`.

## Hand it to another agent

From inside your agent, type the short form, `/codex`, `/cursor`, `/claude`, or `/amp`, with the story number. Or run:

```sh
aayu build <story> --delegate codex     # or cursor, claude, amp, or auto
```

aayudha sends the receiver one brief: the story's promises, the plan, the branch and pull request rules, and a
request to end with a `## What I proved` section. `auto` picks the cheapest connected cloud agent that fits the
story's size. Gave it words instead of a story number? A vague ask is unpacked with you first, and the story is made
Ready before anything leaves ([how aayudha unpacks a vague ask](unpack.md)).

**You should see** the hand-off recorded on the story, then a pull request from the receiver.

### Cloud by default

A hand-off goes to the receiver's cloud agent, not your laptop. aayudha checks first that the agent is connected and
that its cloud environment can build and test your repository. `--local` runs the receiver's command line on your
machine instead, in a fresh worktree with only its own key, so it cannot push.

If the receiver is not ready, `build` says exactly what is missing (the CLI, the sign-in, or the cloud connection),
gives the commands to fix it, and lists the agents you can use instead. It never installs or signs in for you. See
[hand-off problems](troubleshooting.md#hand-offs).

### Set up the cloud environment once

```sh
aayu install --env              # shows what it found in your code, and every host's files
aayu install --env --verify     # writes them, then proves each host in a clean container
```

`install --env` reads your code for toolchains, the package manager, services, and the names of the variables you
need. It asks only what the code cannot say, then writes one setup script every host calls. Commit the files it
writes: a cloud agent sees only what is on your default branch.

**You should see** each host marked verified in `aayu doctor`'s `agent-env` row.

## Follow it to the end

Run the same `build --delegate` command again to sync. aayudha reads the receiver's report as a preview, then the
merge gate's verdict:

- a required check failed: it goes back to the builder, once per new commit;
- `fix`: the reviewer's findings go back;
- `merge`: it names `aayu ship <pr>` for you;
- `rebuild`, or two rounds used: a person is asked.

The [`ci-failed` loop](guide-loops.md) can run that sync for you whenever a check fails.

## Under the hood

The brief, the receiver adapters, the environment spec, and the cycle cap are in
[hand-offs, agent environments, and loops](how-agents.md). How each host loads aayudha is in
[your agent](hosts/README.md).

## What's next

- The pull request is open: [a pull request is ready](guide-review.md).
- Make recurring work run itself: [keep things moving with loops](guide-loops.md).
- See a build on a real day: [chapter 4 of the story](journey-4-build.md).
