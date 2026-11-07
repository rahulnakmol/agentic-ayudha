# UX design: Feature #977

## Read this first

**None of these screens is built.** There is no web face on dev at bc78349 (884d81b plus #1238; no `internal/cli/inboxweb/`), no phone
pairing, no Deck, no Behaviours view, no Ask why action, and no `?c` key. There is no UX design baseline on the ledger
(`specs/977/` does not exist on `origin/ledger`).

The repository holds no design files for these screens: `design/` has only `design/brand.yaml`, the docs brand. Open
PR [#817](https://github.com/rahulnakmol/agentic-ayudha/issues/817) (not merged) would add seat emblems and verb icons
under `design/icons/`; the counsel panel's seat emblem could use them (inferred). The Stories cite touchpoints TP48,
TP68 and TP77 of "interaction specs v4"; that document is not in the repository, so I could not check it. Everything
below is from the Completion Goals, which quote the exact words. Viewports: 390x844 (phone), 1280x800 (laptop),
120x34 (terminal).

## Deck on a phone ([#969](https://github.com/rahulnakmol/agentic-ayudha/issues/969))

Deck is the default on a phone and a toggle on larger screens (Assumptions). It sits inside the wave 3 frame; the
phone now bar "never covers the deck's buttons" ([#961](https://github.com/rahulnakmol/agentic-ayudha/issues/961) c8).

| State | What the person sees |
|---|---|
| Open on the Answer area | "Answer · 1 of 4 · swipe right: take Shuka's recommendation · left: snooze · up: open the thread" (c1) |
| Swipe right on a two-way card | The card tilts and names the action before release; then "✓ Answered: “Acts on a card”. Undo · 5 s"; nothing posts until the 5 s end (c2) |
| Swipe right on a Ship card | Nothing happens; the card springs back and opens its confirm: "This is a one-way door. Read it, then press Ship #811. A swipe never ships." (c3) |
| Reduced motion | No fly-out; the next card replaces the last (c5) |
| `[inbox] undo_seconds = 10` | "Undo · 10 s" (c7) |

Every swipe also has a button and a key (WCAG 2.2, 2.5.7). In the terminal, `D` shows one card and → ← ↑ act (c4).

## Pair a phone ([#970](https://github.com/rahulnakmol/agentic-ayudha/issues/970))

| State | What the person sees |
|---|---|
| `Q` in the terminal, or Pair a phone on the web, with Tailscale Serve set up | A QR code and "Scan with a phone on your tailnet. One phone, one use. Expires in 4:59." (c1) |
| `[inbox.web] pair_code_minutes = 2` | "Expires in 1:59." (c6) |
| Tailscale Serve not set up (terminal 120x34) | "Tailscale Serve is not set up here, so a phone cannot reach the inbox. Run: tailscale serve --bg 7431" (c4) |
| A second scan, or another tailnet user | Refused; the other user is refused before the sign-in page (c2, c3) |
| After pairing, on the phone | The phone's own device-flow sign-in (the wave 3 screen "Prove it is you: open github.com/login/device and enter `<code>`." from [#957](https://github.com/rahulnakmol/agentic-ayudha/issues/957) c4), then its own session: 1 hour after its last action, 24 hours at most (c5) |

## The Behaviours view ([#971](https://github.com/rahulnakmol/agentic-ayudha/issues/971))

A laptop view by Feature, for example Feature #895. One row per behaviour with these columns:

| Column | Content |
|---|---|
| Says | The behaviour sentence |
| Rule | Its rule |
| Source | Where it came from |
| Approved | Who approved it and when |
| Last result | The result with its age, such as "last run 9 d ago" |
| Status | enforced, proposed, failing, stale, or never verified |

A failing proposed behaviour shows in the same place as an enforced one (c2). An old result shows "stale" (c3).

## Ask why ([#972](https://github.com/rahulnakmol/agentic-ayudha/issues/972))

- An Ask why action on a Done card for #802 in the terminal (120x34) and in its web peek (1280x800) (c1).
- The person types a question, such as "why X instead of Y?". The planned command is `aayu explain 802 “why X instead
  of Y?”`. The answer cites the defense record and the code (c2).
- For a seat with a paired human, the question goes to that person as an ask (c3).
- When the seat cannot answer: "I do not know" and the Bug it opened on the defense (c4).

## Ask Atri ([#973](https://github.com/rahulnakmol/agentic-ayudha/issues/973))

On a Ship card for #802, `?c` opens a panel above the card's choices (c1):

```text
Atri · counsel, not approval · #802 Ship into dev
The decision: ship #802 into dev. A one-way door.
Reading: … (each fact names its source and time)
Recommended: …
What would change this: …
The question for you: …
```

- The choices under it are the same as without the panel (c1). Esc closes it; the choices and the cursor are as before;
  it writes nothing (c3).
- On a Feedback issue in Browse with no counsel from facts: "Atri has no counsel from facts here. To discuss it, run
  aayu counsel 1230 in your agent tool (copied)." with a Copy button (c2).
- On an issue in Browse that has counsel from facts: the same panel in the issue's peek (c5).

Note (inferred from the code): `?` alone opens help in the terminal inbox today (`internal/cli/inboxtui.go`, `case
"?": m.help = true`). A `?c` sequence must not break that key.

## Accessibility

Each Story has an axe-core 4.14 criterion with 0 serious and 0 critical findings: Deck (#969 c6), pair states (#970
c8), Behaviours (#971 c4), and the counsel panel in Aruna Light and Aruna Dark (#973 c4). The wave 3 keyboard walk
names "the deck" ([#968](https://github.com/rahulnakmol/agentic-ayudha/issues/968) c7), but "Wave 4 states carry their
own axe criterion" ([#968](https://github.com/rahulnakmol/agentic-ayudha/issues/968) Assumptions).

## Agent-facing surface

Planned: `aayu explain <n> "<question>"` answers from the Accountable seat (#972, owned by #890). Atri's panel has no
agent action: Atri counsels and never acts (`AGENTS.md`, Atri). An agent can already read the cards with `aayu inbox
--list` (`internal/verbs/registry.go`).
