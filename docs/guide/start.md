# Your first hour

In the next hour you will install aayudha, set up one GitHub repository with a single guided command, and file your
first story with promises a person can check. Nothing here changes your CI or needs a model key. You can stop at the
end and keep what you have: clear, checked work items on GitHub.

> **You are here:** setting up. After each step, "You should see" tells you what success looks like. If you see
> something else, the [troubleshooting](troubleshooting.md) link at that step says what to do.

## What you need

- A GitHub repository you administer, cloned on your machine, and the `gh` command line signed in
  (`gh auth login`). A token in `AAYU_GITHUB_TOKEN` works too.
- `git` 2.40 or newer, and `git-lfs`.
- One of the agents aayudha works inside: Claude Code, Codex, Cursor, or Amp.

## 1. Install aayudha

```sh
curl -fsSL https://raw.githubusercontent.com/rahulnakmol/agentic-ayudha/main/install.sh | sh
```

The script downloads the latest signed release and checks its signature and checksum before it copies anything. It
installs one program under two names: `aayudha`, and the short `aayu` this guide uses.

**You should see** `signature and checksum verified`, then `installed aayu 0.1.0`. If it also says
`add ~/.local/bin to your PATH`, do that, and open a new terminal.

Check it:

```sh
aayu version
```

**You should see** `aayu 0.1.0` or newer. Other ways to install, such as `go install` or a pinned version, are in
[Install and connect](../getting-started.md). Stuck? See [install problems](troubleshooting.md#install).

## 2. Take the tour

In your repository's folder, run:

```sh
aayu tour
```

Run it in your own terminal, not through your agent: the tour is a guided first run for a person, and it refuses
to run for an agent. It takes three steps, and it asks before it changes anything:

1. **This machine.** It checks git, git-lfs, your GitHub sign-in, and the config, the same checks as `aayu doctor`.
2. **The repository.** It runs `aayu init`, which lists every label, issue form, and file it will add, then asks
   before it uses your admin access. `init` also creates `ledger`, a separate branch where aayudha keeps its records,
   such as exactly what was agreed for each story.
3. **Your agent.** It asks which agent you use, then runs `aayu install` for it: one skill per aayudha verb (a short
   instruction file your agent reads when you ask for that verb), the MCP server, and hooks.

**You should see** `PASS` on the machine check, the plan `init` prints, and at the end:

```text
★ Level 1! This repository has clear work items. Commit what changed in a pull request.
Next → aayu grill story
```

The picture on the [README](../../README.md) shows a whole run. Stuck? See [tour problems](troubleshooting.md#tour).

> **It fits around what you have.** `init` adds one marked section to an existing `AGENTS.md` and leaves the rest
> alone. If you have no `dev` branch, it maps your existing branches, such as `main`, and renames nothing. And
> `aayu uninit` removes what it created.

Commit what the tour wrote, `.aayudha/`, the issue forms, and your agent's skill folder, through a pull request,
like any other change.

## 3. Ask for your first story

A story is the smallest piece of work that delivers something a person can check. On a bigger project, stories sit
under a Feature, Features under an Initiative (a bet worth making), and Initiatives under a Goal with a number.
You can start with one story; [the story](journey.md) shows the whole ladder at work.

Open your agent in the repository and ask, in plain words:

> Use the grill skill to write a story: people should be able to export their report as CSV.

In Claude Code you can also type `/grill story` (`/aayudha:grill story` with the plugin). The agent interviews you, one question at a time: who needs this,
what "done" looks like, and what must not happen. If your ask is vague ("make it nicer"), it stops and unpacks it
with you first; [how aayudha unpacks a vague ask](unpack.md) shows a real conversation. When every promise is
something a person could check, it files the story as a GitHub issue.

**You should see** a new issue labelled `type:story`, with a **Completion Goal**: numbered promises, each naming the
evidence that will prove it, such as `1. [api-trace] state: …; expected: …`.

## 4. Mark it Ready

Ready means complete, agreed, and frozen. aayudha records exactly what was agreed, so a later edit is visible
instead of silent.

```sh
aayu ready <issue> --check   # read-only: says what is missing, if anything
aayu ready <issue>           # labels it Ready and posts a snapshot of exactly what was agreed
```

**You should see** `--check` exit cleanly, then a `status:ready` label on the issue and a comment that holds the
agreed text's fingerprint. Stuck? See [Ready problems](troubleshooting.md#ready).

> **Well done:** you have a story on GitHub that anyone on your team can read, with promises a machine can check.
> That alone beats most backlogs.

## 5. Ask what to do next

```sh
aayu next
```

**You should see** one command and why, for example `aayu build <issue>` because that story is Ready and nobody is
building it. Whenever you are unsure, `aayu next` reads the repository and names the single best next step.

## Optional: your first proven pull request (another half hour)

To have every pull request checked before it merges, move to Level 2. The review in your own CI arrives in aayudha
0.3.

If `aayu version` prints 0.2.x or older, run `aayu self-update` first. If you installed aayu with Homebrew, Scoop, or
winget, self-update only prints that tool's upgrade command, so run that command.

Until 0.3 is released, install the integration build with
`go install github.com/rahulnakmol/agentic-ayudha/cmd/aayudha@dev`. Go leaves your installed `aayu` as it was, so
point `aayu` at the new build for this shell:

- macOS or Linux: `alias aayu="$(go env GOPATH)/bin/aayudha"`, or `alias aayu="$(go env GOBIN)/aayudha"` if
  `go env GOBIN` prints a folder.
- Windows PowerShell: `Set-Alias aayu "$(go env GOPATH)\bin\aayudha.exe"`, or
  `Set-Alias aayu "$(go env GOBIN)\aayudha.exe"` if `go env GOBIN` prints a folder.

1. `aayu init --gates` adds the merge gate workflow and creates the `code-review` environment, with Grok 4.7 at
   `https://api.x.ai/v1` filled in. Commit the workflow through a pull request and merge it. The gate runs from your
   base branch's copy, so it starts checking from the next pull request. If a review app or a person will review
   instead of the CI model, put that choice from step 2 in this same pull request.

   **You should see** an `environment code-review` line in init's report, and the workflow file in the merged pull
   request. If the line is missing, read the note below step 2. If init stops with an error before its report, such
   as a failed GitHub call, see [Tour problems](troubleshooting.md#tour), then run `aayu doctor`.
2. In **Settings → Environments → code-review**, add the secret `AAYU_REVIEW_MODEL_KEY`: your key for that endpoint.
   The review model must come from a vendor other than the one that builds; change `AAYU_REVIEW_MODEL_URL` and
   `AAYU_REVIEW_MODEL` to use another. [Code review in the merge gate](how-review.md) explains each setting.

   A review app, such as Codex, CodeRabbit, or Greptile, can review instead: set `[inspect] reviewer` to its name and
   list it in `[inspect] apps`. CodeRabbit and Greptile review every pull request on their own.

   Codex, Claude, and Cursor review only when asked. `aayu build` asks after each push; for a pull request it did not
   open, run `aayu inspect <pr>` in your own terminal.

   The gate reads `.aayudha/config.toml` from your base branch. A later pull request is checked with the default
   `ci`, which fails without the key, so make this change in step 1's pull request.

   > **Needs setup:** a private repository on GitHub Free has no environments. There, `aayu init --gates` says it could
   > not create `code-review`. Choose a review app as above, or set `[inspect] reviewer = "human"` with `[autonomy] approver`
   > set to the person who reviews, or make the repository public or upgrade. Put these settings in step 1's pull request, as above.

   **You should see** `AAYU_REVIEW_MODEL_KEY` under **Environment secrets** on the `code-review` page. `aayu doctor`
   cannot read the environment, so the gate's first run is where a wrong key shows: if it fails, see
   [the merge gate in troubleshooting](troubleshooting.md#merge-gate). With a review app instead, the
   `reviewer-apps` row of `aayu doctor` passes, and the app's installation page lists this repository: doctor cannot
   tell whether the app is installed. With `human`, `aayu config show` lists your `[inspect] reviewer` and
   `[autonomy] approver`.
3. Ask your agent to build the story (`aayu build <issue>`), or hand it to another agent's cloud
   ([hand work to an agent](guide-build.md)).

**You should see** the merge gate run on the pull request, a proof comment, and the reviewer's verdict.
[A pull request is ready](guide-review.md) walks you through it.

The review runs only after the checks and proof pass. If they fail, the inspect check is skipped and the gate names
the failure.

The inspect comment's **Review:** field, under its heading, names the review model, for example
`different-family review (B, paid): grok-4.7 (xai)`. The model reviews only when its family differs from the
builder's and the author has write access.

Otherwise the comment names the app or person that reviews instead. It also names any listed Codex, Claude, or Cursor
it skipped, since those review only when asked.

With `human`, a GitHub review alone does not count, so the gate stays at `no-reply`. The `[autonomy] approver`
reviews the pull request, then runs `aayu inspect <pr> --approve --reason "<what you checked>"`, or approves its card
in `aayu inbox`.

If a setting is missing, the gate fails and names it: see [Merge gate problems](troubleshooting.md#merge-gate).

## What's next

- See the whole loop on a real project: [the story](journey.md).
- Find the command for your situation: [when to use what](when-to-use-what.md).
- Decide how far to go: [adopt at your own pace](adopt.md).
