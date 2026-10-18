# Run it alone: the solo founder

You are one person with a product to build. aayudha puts an agent twin in every one of its fourteen seats, so the
interviewing, planning, building, checking, and reporting happen without you. What stays with you are the steps only
a person may take, and GitHub will not even let you approve your own pull request. This page shows what you still do
by hand, how to keep most of it in one list, and how the agents earn the right to need you less.

![The twins fill every seat. You keep nine kinds of step: answer the interview, accept Decisions, comment a solo-mode bypass on a risky plan's Story and record it, post a solo-mode bypass and ship, ask for a stable release, approve production actions, accept Feedback and review skill changes, publish, and halt or resume. aayu inbox gathers most of what waits for you into one list.](media/solo-founder.svg)

*The agents do the work. You keep the decisions, and one inbox holds most of them.*

## What you still do by hand

Each step below is one that only a person may take: a choice, a yes, or a brake.

| Step | How |
|---|---|
| Answer the interview and confirm what done looks like | In your agent: [`grill`](skills/grill.md) |
| Accept a Decision or a floor waiver | `aayu decide <n> --accept <option>` ([`decide`](skills/decide.md)) |
| Review a plan that touches a risky layer | A comment on the Story starting `Solo-mode bypass:`, then `aayu plan <story> --solo-bypass`, recorded as no review ([`plan`](skills/plan.md)) |
| Merge a proven pull request | A comment starting `Solo-mode bypass:`, then `aayu ship <pr> --solo-bypass` ([`ship`](skills/ship.md)) |
| Ask for a stable release | `aayu release <n> --to main` in your terminal, or a comment on the Release issue: `Stable release: v1.2.0` ([`release`](skills/release.md)) |
| Approve a production action | Approve the waiting run's GitHub Environment ([`respond`](skills/respond.md)) |
| Accept Feedback and review skill changes | Label the Feedback `needs:grill`; review evolve's pull request ([`retro`](skills/retro.md), [`evolve`](skills/evolve.md)) |
| Publish docs or launch copy | `aayu tell <kind> --from <draft> --publish` ([`tell`](skills/tell.md)) |
| Stop or restart everything | `aayu halt --reason "<why>"`, then `aayu resume` |

With a bot App installed (`aayu init --identity`), the agents' pull requests are the App's, not yours, so you can
review them as a person and the two bypass rows change: approve a risky plan with `aayu plan <story> --approve`, and
approve the pull request on GitHub, then run `aayu ship <pr>` without `--solo-bypass`. The bypass is refused once an
App is installed.

`decide --accept`, `plan --solo-bypass`, `ship --solo-bypass`, `tell --publish`, `resume`, and `inbox` are a
person's to run. Run them in your own terminal and they just work. aayu's own agent lanes and CI are always refused,
and in Claude Code, Codex, and Cursor the hook aayu installs blocks them before they run. In a shell that carries an
agent's marker, such as an orb's Terminal tab, the command cannot tell you from an agent, so it asks you to approve
through GitHub's device flow: open the link it prints on any device, enter the code, and approve
([approving from any device](how-trust.md#approving-from-any-device-presence-check)).

Two commands are different on purpose. Once you have asked for a stable release, an agent may run `aayu release` to
carry it out; your request is the gate. And anyone, agent included, may pull the brake with `aayu halt`; only a person
releases it with `aayu resume`.

## The solo-mode bypass, in the open

GitHub does not let you approve your own pull request, so a true human pass is impossible alone. aayudha does not
fake one. You say so in a comment that starts with `Solo-mode bypass:` and gives the reason, then run the command:

```sh
aayu ship <pr> --solo-bypass
```

**You should see** the pull request merged, with "Human pass: none possible" and a link to your comment in the ship
record. [`audit`](skills/audit.md) lists every bypass in its own section. The bypass is refused when an agent runs it,
when your comment is missing, when a bot App is installed, or when a second person holds a paired seat: then someone
can review, so someone must. Stuck? See [ship problems](troubleshooting.md#ship).

## One list: the inbox

Working alone, you should not be the reason work waits. There is no switch that turns the human-only gates off,
because an agent could flip it. Instead, one command gathers most of the human-only steps that wait for you, each with
the twin's recommendation:

```sh
aayu inbox
```

It lists open Decisions (floor waivers and provisional ones included), pull requests where the review stopped after
its two fix rounds, stories whose pull request merged without a ship record, Bugs whose fix merged, and stories that
another story replaced. It does not list a pull request waiting for your approval or a plan waiting for
`plan --approve`; with a bot App installed, find those in GitHub's review requests. Press `a` to accept the recommendation, `c` to choose another option, `s` to skip, `o` to
open it, or `q` to quit. Every write is yours, under your own token, and logged.

**You should see** the steps one at a time, each with its recommendation. When it has steps, `aayu` with no command
shows "Waiting in your inbox" with the count, and `aayu next` names `aayu inbox`. Add `--dry-run` to see what each key
would write. The inbox runs only in your own terminal; agents are refused.

## Decisions that do not wait forever

A Decision records a real choice with options and a recommendation, and only you accept it. When you are busy, the
`decisions` loop counts business hours in your `[time]` zone and pages you after 24. Working alone, a reversible
Decision's recommendation also applies **provisionally** at that point: labelled `decision:provisional`, with a date
by which you ratify or revert it. The inbox lists provisional Decisions with "ratify" as the recommendation.

The window grows to 48 hours once aayu counts two people: the people in paired seats plus `[autonomy] approver`. When
you pair your first teammate, set the approver to your own login, or the pod still counts as one person.

Some Decisions always wait for you, however long it takes: a one-way door, a waiver of any kind, a seat change, and
anything only a person may do, such as a change to the floors, the skills, or the protected config. The solo window
is `[decisions] solo_provisional_after_hours`, read only from the protected `.aayudha/config.toml` and never below 12
hours ([configuration](how-config-and-audit.md)).

## Autonomy the agents earn

A layer can earn the right to merge without your review. After 20 merges in a row on that layer with no correction (no
requested changes, no commit by a person, no revert, no linked Bug), [`ship`](skills/ship.md) records a change there as
`autonomous` instead of waiting for a review. A protected workflow computes the streak from the merge history, so no
agent can change it. One correction resets it.

The risky layers never earn it: L04 domain policy, L07 stored data, and L09 the platform always need a person who owns
those paths, whatever the streak.

Without a bot App, the agents write as you, and their pull requests carry the `identity:human-fallback` label; the
streak counts them as agent work all the same. With an App (`aayu init --identity`), the agents write as their own
identity, and the streak workflow also records each correction on the ledger ([adopt at your own pace](adopt.md)).

## When a second person joins

Solo mode ends seat by seat. Pair the person with the seats they care about, one command each, and the twin keeps
doing the work while the person reviews it.

![A solo seat becomes paired with aayu seat add, which writes the config, names the person in CODEOWNERS for the seat's layers, and opens an onboarding issue. Paired, the twin drafts and the person reviews and approves; with two people, protected paths need one CODEOWNER approval and the solo-mode bypass is refused. aayu seat remove returns the seat to solo.](media/solo-to-paired.svg)

*Pairing is one command, and it changes who must review.*

```sh
aayu seat add code-architect --human <github-login>
aayu seat list
```

**You should see** `policy: two-human pending` from `aayu seat add`, an onboarding issue addressed to the person, and
the seat as `paired` with their login and layers in `aayu seat list`. The command changes protected files, so commit
them through a pull request. Until that merges, trust the `seat add` line: `seat list` reads your local CODEOWNERS and
already says `two-human`. Once it merges, run the same `aayu seat add` again: it now reports `policy: two-human`, one
CODEOWNER's approval is enough, and the solo-mode bypass is refused for protected paths. The second person reviews
instead.

`aayu seat remove <seat>` returns a seat to solo and hands its person's open stories back to the twin.

> **Honest status:** a repository adopted with `aayu init` has no `protected-paths-codeowner-review` ruleset and no
> solo-mode gate, so with one human GitHub itself enforces no review on protected paths (`ship` still does), and
> `seat add` reports `policy: n/a`. Once the second person's CODEOWNERS change has merged, the second `seat add`
> creates the ruleset: one CODEOWNER approval on your dev branch, with no bypass. It needs a token with
> Administration: write, and a private repository needs a GitHub plan with rulesets. `seat remove` deletes it again.

## What can go wrong

- **A command says it is for a person.** You ran it from your agent, or with no terminal. Run it in your own
  terminal, or approve the device-flow prompt it prints when your shell carries an agent's marker.
- **The bypass is refused with a second person paired.** Good: ask them to review. See
  [when a second person joins](#when-a-second-person-joins).
- **Work stops everywhere.** Someone ran `aayu halt`, which stops every agent, loop, and workflow until
  `aayu resume`. Or a spend limit tripped: agent lanes pause and a person is paged (exit 6).

## Under the hood

How a person's approval is told apart from a bot's, what the bypass checks, and how the inbox reads the record are in
[trust and identities](how-trust.md). The default streak and risky layers are in
[CONSTRAINTS.md](../../CONSTRAINTS.md), and the Decision hours in `[decisions]` of your config.

## What's next

- Meet the twins doing the work: [your agentic team](concepts-team.md).
- See solo mode on a real day: [the yes](journey-6-ship.md).
