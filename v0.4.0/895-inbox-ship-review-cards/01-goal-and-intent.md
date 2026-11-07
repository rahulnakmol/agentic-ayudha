# Goal and intent: Feature #895

## Source note

There is no intent baseline. `git ls-tree -r --name-only origin/ledger specs/895` lists no files, so there is no
`intent-v1.json` and no `.draft` to quote. Everything below comes from the Feature body
([#895](https://github.com/rahulnakmol/agentic-ayudha/issues/895)), its sub-issues, and the parent Initiative
([#870](https://github.com/rahulnakmol/agentic-ayudha/issues/870)). Issue #895 has no comments.

## Goal

"`aayu inbox` in the terminal shows every ship and review step that needs a person as a card with its reason and
evidence. A batch ships through one presence check, a PR whose review cannot finish becomes a card instead of a
silent no-reply, and every result shows on the card it acted on." (issue #895, Summary)

## Who it serves and the outcome

- The founder (the `[autonomy] approver`, `rahulnakmol` in `.aayudha/config.toml`). Most Stories are written "As the
  founder" (for example #926, #927, #928).
- A code owner of a protected path ([#935](https://github.com/rahulnakmol/agentic-ayudha/issues/935): "As a code owner
  of a protected path").
- Every user of aayu commands, for the wrap rule ([#980](https://github.com/rahulnakmol/agentic-ayudha/issues/980):
  "every aayu command's prose").

Outcome: the person clears every stalled or protected PR, every false finding, and every floor waiver from one place,
with one presence check, and never pastes `gh pr merge --admin` (#926, #927, #936 Summaries).

The parent Initiative #870 states the business hypothesis: if every step that needs a person is one record with one
verb, then PRs merged into dev reach 1,000 in 28 days "without more of his time". Its baseline is 527 PRs merged into
dev from 2026-09-23 to 2026-10-21 (issue #870, Hypothesis, Baseline, Target).

## Problem

From issue #895, Problem:

- In the v0.3.0 release on 2026-10-20, `aayu inbox` said nothing waited while #802 and #676 each needed a person's
  review. Rung B had failed on their heads (Bug #813), CodeRabbit does not review a commit twice, and a rerun never
  retries rung B (Bug #810). The release stalled overnight until @rahulnakmol ran `aayu inspect 676 --approve` and
  `aayu inspect 802 --approve` by hand.
- Other steps ran outside the inbox, each with its own presence check or typed command: dismissing a false finding
  (`aayu inspect 681 --dismiss`; Bug #816), `/aayu review again` past the round cap (#672, #676), and admin merges
  after a "Solo-mode bypass:" comment (#811, #676).
- The inbox showed unfinished reads, acted on stale cards, and put every result on its bottom line.

## Success measures

Each Story and Bug states a measure and a threshold (from each issue body):

| Issue | Measure | Threshold |
|---|---|---|
| [#924](https://github.com/rahulnakmol/agentic-ayudha/issues/924) | Inbox actions that end red or in the bottom line when the person changed nothing, per release | 0 in v0.4.0 (v0.3.0: 2 on #802) |
| [#925](https://github.com/rahulnakmol/agentic-ayudha/issues/925) | Sessions that show an empty list before the first read ends, per release | 0 in v0.4.0 (v0.3.0: 1) |
| [#926](https://github.com/rahulnakmol/agentic-ayudha/issues/926) | Release-blocking PRs whose review could not finish and that had no card, per release | 0 in v0.4.0 (v0.3.0: #802 and #676) |
| [#927](https://github.com/rahulnakmol/agentic-ayudha/issues/927) | Protected-path merges into dev made outside `aayu inbox`, per release | 0 (v0.3.0: #811 and #676 by hand) |
| [#928](https://github.com/rahulnakmol/agentic-ayudha/issues/928) | Presence checks per batch of ships | 1 per batch |
| [#929](https://github.com/rahulnakmol/agentic-ayudha/issues/929) | Terminal setups where o or c gets the link to the person | 6 of 6 |
| [#930](https://github.com/rahulnakmol/agentic-ayudha/issues/930) | Card kinds that show why you, the evidence row, and counsel above the choices | 10 of 10 kinds |
| [#931](https://github.com/rahulnakmol/agentic-ayudha/issues/931) | Signals that fire for a view already open | 0 |
| [#932](https://github.com/rahulnakmol/agentic-ayudha/issues/932) | Header and footer facts that show their source and age | 12 of 12 |
| [#933](https://github.com/rahulnakmol/agentic-ayudha/issues/933) | Writes left half done by a quit | 0 |
| [#934](https://github.com/rahulnakmol/agentic-ayudha/issues/934) | Choices shown to a login that cannot take them | 0 |
| [#935](https://github.com/rahulnakmol/agentic-ayudha/issues/935) | Protected-path PRs merged by bypass when another eligible code owner existed | 0 |
| [#936](https://github.com/rahulnakmol/agentic-ayudha/issues/936) | Dismissals and waiver accepts typed as commands, per release | 0 |
| [#980](https://github.com/rahulnakmol/agentic-ayudha/issues/980) | Command screens at 80 columns with a word cut at the edge | 0 |
| [#1122](https://github.com/rahulnakmol/agentic-ayudha/issues/1122) | Completion Goal criteria observed | all 4 |

## Out of scope

From issue #895, Scope, "Out":

- Everything in waves 2 to 4 (#975, #976, #977).
- For every wave: why reviews fail (#810, #813), a gate that runs an old head (#812), the superseded cancelled run
  (#818), and failure wording in general (#797).
- SQLite: the JSON cache stays, "with writes safe for two sessions".
- GSAP: motion is CSS only.
- The open internet: the web face is reached only from this computer, an SSH or Amp tunnel, or a phone on the
  founder's tailnet.
- Bugs #846 and #847, fixed in #850 for v0.3.0.

## Entry conditions stated by the Feature

"Its stories are made Ready only after this Feature's intent, spec, and UX design baselines are approved, its web and
terminal prototype states pass Hanu's review (D4), and it passes Approve to build (#877)." (issue #895, Scope). None
of these baselines exists on the ledger (see Source note). The evidence does not show a Hanu review record for this
Feature.

## UX research sources

The Stories cite "UX proposals v27", "interaction specs v4" (touchpoints TP1 to TP60), a "design study" (Q1, Q2b, Q4b,
terminal matrix T1 to T9), and "web44" (for example #924, #925, #928, #929 Provenance). These documents are not in
this repository and not on the ledger branch: a search of the checkout and of `origin/ledger` file names found no
match. Their content here is only what the issue bodies quote. A search of GitHub issues and PRs for "Superhuman" and
"HEY inbox" found no match, so this record makes no claim about those apps.
