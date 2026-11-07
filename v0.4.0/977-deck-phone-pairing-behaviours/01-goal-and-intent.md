# Goal and intent: Feature #977

## No intent baseline

The ledger has no intent baseline for #977: `git ls-tree -r --name-only origin/ledger specs/977` returns nothing.
This file uses the Feature issue body ([#977](https://github.com/rahulnakmol/agentic-ayudha/issues/977)) and its five
Stories. Nothing here is approved intent.

## Goal

"On a phone or a small screen, the founder clears an area one card at a time in a deck whose swipes never ship,
pairs a phone on his tailnet with a one-time QR code, reads every behaviour he approved in one view, asks why on any
shipped item, and asks Atri for counsel on any card. This is wave 4 of the inbox."
([#977](https://github.com/rahulnakmol/agentic-ayudha/issues/977) Summary)

## Who it serves and the outcome

- The founder, away from his desk. Every Story is written "As the founder" (#969 to #973).
- The outcome in the body: answer the inbox from a phone, safely, and see behaviours, defenses, and counsel where
  today they have no view ([#977](https://github.com/rahulnakmol/agentic-ayudha/issues/977) Problem).
- No customer metric is named. The Feature form has no Outcome field yet (Feature
  [#982](https://github.com/rahulnakmol/agentic-ayudha/issues/982), [#984](https://github.com/rahulnakmol/agentic-ayudha/issues/984)).

## Problem

From [#977](https://github.com/rahulnakmol/agentic-ayudha/issues/977) Problem:

- "The founder answers the inbox away from his desk, and v0.4.0's web face (#976) is reached only from this computer
  or a tunnel."
- "The behaviours he approves (#891) and the defense of each merged PR (#890) have no view."
- "Atri's counsel is only on Decisions." The code agrees for the source of counsel: an inbox card carries `Counsel`
  and `CounselFrom` ("posted" or "facts") (`internal/cli/inbox.go`, `InboxItem`).

## Success measures

| Story | Measure | Threshold |
|---|---|---|
| [#969](https://github.com/rahulnakmol/agentic-ayudha/issues/969) | Ships, merges, waivers, or one-way Decisions done by a swipe | 0 |
| [#970](https://github.com/rahulnakmol/agentic-ayudha/issues/970) | Ways to reach the inbox other than this computer, an SSH tunnel, the Amp portal, or the founder's tailnet | 0 |
| [#971](https://github.com/rahulnakmol/agentic-ayudha/issues/971) | Approved behaviours that the view does not show | 0 |
| [#972](https://github.com/rahulnakmol/agentic-ayudha/issues/972) | Shipped items with no Ask why | 0 |
| [#973](https://github.com/rahulnakmol/agentic-ayudha/issues/973) | Cards and issues where `?c` or Ask Atri shows nothing | 0 |

## Out of scope

From [#977](https://github.com/rahulnakmol/agentic-ayudha/issues/977) Scope and the Story bodies:

- Wi-Fi addresses (Q6: A + B + C). "aayu never listens on the Wi-Fi address"
  ([#970](https://github.com/rahulnakmol/agentic-ayudha/issues/970) c7). Wi-Fi pairing (option D) is parked.
- The open internet.
- For every wave: [#810](https://github.com/rahulnakmol/agentic-ayudha/issues/810),
  [#813](https://github.com/rahulnakmol/agentic-ayudha/issues/813), [#812](https://github.com/rahulnakmol/agentic-ayudha/issues/812),
  [#818](https://github.com/rahulnakmol/agentic-ayudha/issues/818), [#797](https://github.com/rahulnakmol/agentic-ayudha/issues/797);
  SQLite; GSAP.
- The data, approvals, and BEHAVIOUR CHANGE card of behaviours: those are #891's; #971 is only the view.
- `aayu explain` and the defense record: those are #890's; #972 is only the action.
- A model lane for Atri's counsel: "stays later (Q2)" ([#973](https://github.com/rahulnakmol/agentic-ayudha/issues/973) Assumptions).

## Estimate in the issue

"5 stories (3 M, 2 S); no story depends on another story of this wave; about 0.8 h of mission time … with 5 agents
(Decision #974), under `[mission] warn_hours = 24`" ([#977](https://github.com/rahulnakmol/agentic-ayudha/issues/977)
Scope). The labels agree: M for #969, #970, #971; S for #972, #973 (`_data/subs.tsv`).
