# Troubleshooting

Find what you see, then do what it says. Every aayu command ends with an exit code and, when it stops, a line that
starts `next:` with the command that moves you on. `aayu next` reads the repository and names the single best next
step at any time.

| Exit code | Means | What to do |
|---|---|---|
| 0 | Done | Nothing |
| 1 | A bug in aayu | Report it with the command and its output ([SECURITY.md](../../SECURITY.md) for anything sensitive) |
| 2 | A usage or configuration mistake | Read the message; it names the flag or config key |
| 3 | A floor missed | Fix the check it names; a floor is never skipped to make it pass |
| 4 | Blocked, or it needs a person | Read the message; it names what is missing and who can give it |
| 5 | Something outside aayu failed: the network, GitHub, a tool | Try again; if it repeats, check the tool or your sign-in |
| 6 | A budget limit | A person checks the spend, then `aayu resume` |

## Install

**`aayu: command not found` after the install script.** The script put the program in `~/.local/bin`, which is not on
your `PATH`. Add it, as the script's last line says, and open a new terminal. To install elsewhere, set
`AAYU_INSTALL_DIR` before running the script.

**The script says the signature or checksum did not verify.** It installed nothing, on purpose. Run it again; if it
fails again, do not work around it. Report it as a security issue ([SECURITY.md](../../SECURITY.md)).

**`aayu doctor` shows `WARN binary development build`.** You built aayu from source. That is fine for trying it; the
signed release comes from the install script.

## Tour

**`tour sets a repository up for a person; agents are refused`.** Run `aayu tour` yourself, in your own terminal, not
through your agent. The tour asks for your admin access, which is a person's decision.

**`tour asks a few questions, so it needs a terminal`.** You ran it from a script or a pipe. Run `aayu init`, then
`aayu install --host <your agent> --apply`, yourself: the tour runs exactly those.

**`this directory is not in one` (not a git repository).** `cd` into a clone of your repository, or run `git init`.

**`FAIL ... [blocking]` in step 1.** The row says what is missing, for example `git` older than 2.40 or no `git-lfs`.
Install it, then run the tour again. `WARN` and `N/A` rows are optional; leave them for later.

**`token` warns, or GitHub calls fail with 401.** Sign in with `gh auth login`, or set `AAYU_GITHUB_TOKEN` to a token
that can administer the repository.

## Ready

**`aayu ready <issue> --check` lists missing sections.** Ask your agent to fill them in with the grill skill, then
check again. A story or Bug needs a Completion Goal of numbered promises, one per line, each naming its evidence.

**A Ready story lost its `status:ready` label.** Someone edited the story after it was frozen, so the agreed version
no longer matches. Read the change. If it is right, run `aayu ready <issue>` again to agree the new version.

**`is a request in words, not an issue`.** You typed words after a verb at the terminal, such as `aayu build "make it
faster"`. At the terminal, verbs take an issue number. Run the `aayu grill … --assess --idea` command the message
prints, or type the same words to your agent: its skills and slash commands accept words and unpack them first.
[How aayudha unpacks a vague ask](unpack.md) shows how.

## Merge gate

**`aayudha/inspect` stays red with "no usable review … fails closed".** The reviewer app has not replied for this
commit, or replied in a way aayu cannot read. Run `aayu inspect <pr>` yourself, outside CI: the apps answer only the
people who connected them. Check `aayu doctor`'s `reviewer-apps` row: the app must be installed and listed in
`[inspect] apps`.

**`reviewer-apps` says it is "falling back to a human review".** No listed app comes from a different vendor than
the builder. Install one (Claude, Codex, or Cursor) and add it to `[inspect] apps`, or let a person review.

**`inspect` says `fix` twice, then pages a person.** The pull request used its two rounds. Close it, and build the
story again from the latest code ([a pull request is ready](guide-review.md#when-it-says-fix)).

**A floor fails with "tool not installed".** The merge gate installs every tool; on your own machine a floor whose tool
is missing blocks, it never passes. `aayu doctor`'s `optional-tools` row lists what to install for local checks.

**The pack says `skipped: dast: no runnable target`.** Your product is a web app or an API (`[verify] platforms`), but
verify has nothing to scan, and that blocks `ship`. Give verify a command that starts the app (`[verify] seed`) and
its URL (`[verify.environments.local] url`, for example `http://127.0.0.1:8080`), or ask a person to waive it with
`aayu decide <story> --waiver dast`. With no Docker or Podman on your machine, verify leaves the scan to CI and does
not block locally; `aayu doctor`'s `dast` row shows the target, the scan mode, and the runtime it found.

**The proof comment flags "checks this PR weakens".** The pull request removes or loosens a check. That is allowed but
never silent: a person decides whether it is right.

## Ship

`ship` refuses, with exit 4, until everything holds. The message names the missing piece:

| Message | Fix |
|---|---|
| `the merge gate must be green` | Wait for the gate, or fix what it found |
| `no merge gate pack for the head` | Push again, or re-run the gate: the pack must come from the gate's run on this exact commit |
| `floors do not hold in the merge gate's pack` | Fix the floor, or have a person accept a floor-waiver Decision |
| `no human pass` | A person, not a bot and not the author, approves the pull request on GitHub |
| `plan-time review is required` | A person reviews the plan of a story that touches a risky layer |
| `no story` | Add `Refs #<story>` to the pull request body |

Working alone, a human pass is impossible: [the human pass and solo mode](concepts-human-pass.md) explains
`ship --solo-bypass`.

## Hand-offs

**`build --delegate` exits 4 and lists commands.** The receiver's CLI, sign-in, or cloud connection is missing. Run the
install or sign-in command it prints, or pick one of the agents it lists as available. `aayu doctor`'s `agents` row
shows the same.

**`the agent environment is not on the default branch yet`.** The cloud environment files are verified but not merged. Commit
them through a pull request first: a cloud agent sees only your default branch.

**`agent-env` shows a host as stale.** Your code now asks for something the setup script does not install, or someone
edited a generated file. Run `aayu install --env --verify`.

## Loops

**doctor's `loops` row shows `missing`.** Run `aayu install --loops`, follow its steps for the tool you picked, then
record the loop with `--record <loop>=<ID>`.

**It shows `stale`.** The loop file changed after you set it up. Set it up again in the tool and record it again.

**It shows `double`.** The loop runs twice: it is recorded in two tools, or it has `agent = false` (so GitHub Actions
runs it) and is still recorded in a tool. Delete it in the tool, then forget it there:
`aayu install --loops --host <that tool> --record <loop>=`.

**A loop run stops with exit 4 or 6.** The repository is halted, or a budget paused the agents. A person reads why,
then runs `aayu resume`.

## Integrations

**`aayu doctor` says `the posthog key is write-capable` or `the sentry key is write-capable`.** The key has a scope
that does not end in `:read`. Create a new key with only the scopes in
[bring in analytics and errors](guide-integrations.md), then store it with `aayu auth set-secret posthog` or
`aayu auth set-secret sentry` in your own terminal.

**`[integrations.posthog] project is not set`, or a Sentry `org` or `project` error.** Add the field by hand to
`.aayudha/config.toml`. PostHog and Sentry want the numeric project ID; Sentry also wants the organization slug.

**`secret "posthog" has no value`, or the same for `sentry`.** The key was never stored on this machine. Run
`aayu auth set-secret posthog` or `aayu auth set-secret sentry` in your own terminal.

**A 403 from Sentry's sessions or events.** The token lacks `org:read`. Create a token with `org:read`, `event:read`,
and `project:read`, and store it again.

**A 401, or no data, from a key that works in the provider's own UI.** Your account is in another region or is
self-hosted. Set `url` in the section, for example `url = "https://eu.posthog.com"`. A self-hosted `url` goes in
your user config (`~/.config/aayudha/config.toml`), an `AAYU_INTEGRATIONS__<NAME>__URL` variable, or `--set`: the
repository's config may name only the provider's cloud hosts, so `aayu` refuses any other there.

## Everything else

Run `aayu doctor`. Every row that is not `PASS` says what is wrong and how to fix it. `aayu doctor --fix` re-syncs your
agent's skill copies after an upgrade.

## What's next

- Back to [your first hour](start.md).
- Find the command for your situation: [when to use what](when-to-use-what.md).
