# Goal and intent: Feature #976

## No intent baseline

The ledger has no intent baseline for #976: `git ls-tree -r --name-only origin/ledger specs/976` returns nothing.
This file uses the Feature issue body ([#976](https://github.com/rahulnakmol/agentic-ayudha/issues/976)) and its
Stories. Nothing here is approved intent.

## Goal

`aayu inbox --web` opens the same inbox in a browser, on this computer, over an SSH tunnel, or through an Amp portal.
Every card and state has the same words as the terminal and is always live
([#976](https://github.com/rahulnakmol/agentic-ayudha/issues/976) Summary).

It is wave 3 of four inbox Features under Initiative [#870](https://github.com/rahulnakmol/agentic-ayudha/issues/870):
[#895](https://github.com/rahulnakmol/agentic-ayudha/issues/895) (wave 1, terminal ship and review steps),
[#975](https://github.com/rahulnakmol/agentic-ayudha/issues/975) (wave 2, the day in areas, asks, missions, decisions), this
Feature (wave 3), and [#977](https://github.com/rahulnakmol/agentic-ayudha/issues/977) (wave 4, the deck and phone).
Each wave starts when the Features it depends on have merged ([#976](https://github.com/rahulnakmol/agentic-ayudha/issues/976) Summary).

## Who it serves and the outcome

- The founder, who "works across a terminal, a browser, and a phone"
  ([#976](https://github.com/rahulnakmol/agentic-ayudha/issues/976) Problem). Every Story is written "As the founder",
  except [#968](https://github.com/rahulnakmol/agentic-ayudha/issues/968): "a person who uses a keyboard or a screen
  reader".
- The outcome the body names: one window onto the factory. "The founder sees the inbox as his one window onto the
  factory (UX proposals v27)" ([#976](https://github.com/rahulnakmol/agentic-ayudha/issues/976) Problem).
- The Feature form has no Outcome field yet (that is Feature [#982](https://github.com/rahulnakmol/agentic-ayudha/issues/982)),
  so #976 names no customer metric. [#961](https://github.com/rahulnakmol/agentic-ayudha/issues/961) criterion 11
  plans to show "founder minutes per merged PR: not measured yet" in the now bar.

## Problem

From [#976](https://github.com/rahulnakmol/agentic-ayudha/issues/976) Problem:

- The inbox exists only in the terminal.
- A PR has to be read on github.com.
- Lineage and the release scope are separate commands.
- A mission's Shape and Deliver have no page.

The code agrees: the `inbox` verb is terminal only, "Refused for agents and needs a terminal, except --list"
(`internal/verbs/registry.go`, `plumb("inbox", …)`).

## Success measures

Each Story names a Measure and a Threshold (from the Story bodies):

| Story | Measure | Threshold |
|---|---|---|
| [#957](https://github.com/rahulnakmol/agentic-ayudha/issues/957) | Requests from outside 127.0.0.1, or without the session cookie and CSRF header, that the server accepts | 0 |
| [#958](https://github.com/rahulnakmol/agentic-ayudha/issues/958) | States whose words differ between the terminal and the web | 0 |
| [#959](https://github.com/rahulnakmol/agentic-ayudha/issues/959) | Background reads per hour with no change that cost more than a 304 | 0 |
| [#960](https://github.com/rahulnakmol/agentic-ayudha/issues/960) | Mission stories in a state the Deliver tab does not show | 0 |
| [#961](https://github.com/rahulnakmol/agentic-ayudha/issues/961) | Viewport classes (compact, medium, expanded, extra-large) where every view works | 4 of 4 |
| [#962](https://github.com/rahulnakmol/agentic-ayudha/issues/962) | PR tabs that render GitHub text unescaped | 0 |
| [#963](https://github.com/rahulnakmol/agentic-ayudha/issues/963) | Cards whose lineage has a gap that is not shown | 0 |
| [#964](https://github.com/rahulnakmol/agentic-ayudha/issues/964) | Views where the release filter applies | 100% of views |
| [#965](https://github.com/rahulnakmol/agentic-ayudha/issues/965) | Theme variants with a role pair under WCAG 2.2 AA | 0 |
| [#966](https://github.com/rahulnakmol/agentic-ayudha/issues/966) | Writes that run from typed text without a card's confirm | 0 |
| [#967](https://github.com/rahulnakmol/agentic-ayudha/issues/967) | Views with no designed empty state | 0 |
| [#968](https://github.com/rahulnakmol/agentic-ayudha/issues/968) | Serious or critical axe findings on any web state | 0 |
| [#983](https://github.com/rahulnakmol/agentic-ayudha/issues/983) | Work items of a release that its views do not show, or show with the wrong count | 0 |

## Out of scope

From [#976](https://github.com/rahulnakmol/agentic-ayudha/issues/976) Scope:

- The deck, phone pairing, the Behaviours and Ask why views: Feature [#977](https://github.com/rahulnakmol/agentic-ayudha/issues/977).
- Wi-Fi addresses (Q6).
- For every wave: why reviews fail ([#810](https://github.com/rahulnakmol/agentic-ayudha/issues/810),
  [#813](https://github.com/rahulnakmol/agentic-ayudha/issues/813)), a gate that runs an old head
  ([#812](https://github.com/rahulnakmol/agentic-ayudha/issues/812)), the superseded cancelled run
  ([#818](https://github.com/rahulnakmol/agentic-ayudha/issues/818)), and failure wording in general
  ([#797](https://github.com/rahulnakmol/agentic-ayudha/issues/797)).
- SQLite: the JSON cache stays, with writes safe for two sessions.
- GSAP: motion is CSS only.
- The open internet: the web face is reached only from this computer, an SSH or Amp tunnel, or a phone on the
  founder's tailnet.
- Inline PR review comments ([#962](https://github.com/rahulnakmol/agentic-ayudha/issues/962) Assumptions).
- Remappable keys: the keys are fixed in 0.4.0 (K4) ([#968](https://github.com/rahulnakmol/agentic-ayudha/issues/968) Assumptions).
- A model behind the command bar: a fixed grammar in v0.4.0, a model later (Q2 A)
  ([#966](https://github.com/rahulnakmol/agentic-ayudha/issues/966) Assumptions).

## Estimate in the issue

"12 stories (5 L, 6 M, 1 S); longest chain #957 → #958 → #961 → #968; about 15.5 h of mission time … with 5 agents
(Decision #974), under `[mission] warn_hours = 24`" ([#976](https://github.com/rahulnakmol/agentic-ayudha/issues/976) Scope).
The Feature now has 13 Stories: [#983](https://github.com/rahulnakmol/agentic-ayudha/issues/983) (effort L) was added
after the estimate (`_data/subs.tsv`). The estimate does not include it.

The effort labels do not match the estimate. In `_data/subs.tsv`, #957 to #968 have 6 L
(#957, #958, #960, #961, #964, #965), 5 M (#959, #962, #963, #966, #968), and 1 S (#967). With #983, the Feature
has 7 L, 5 M, and 1 S.
