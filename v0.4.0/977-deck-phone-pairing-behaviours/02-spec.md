# Specification: Feature #977

## Source and status

There is no spec baseline on the ledger for #977 (`specs/977/` does not exist on `origin/ledger`). The requirements
come from the Completion Goals of the five Story issues. They are planned, not approved, and not built. "cN" means
Completion Goal criterion N of that Story.

## Functional requirements

### The deck ([#969](https://github.com/rahulnakmol/agentic-ayudha/issues/969))

1. FR-1. Deck mode shows one card at a time with a header that names the area, the position, and the swipes:
   "Answer · 1 of 4 · swipe right: take Shuka's recommendation · left: snooze · up: open the thread" (c1).
2. FR-2. A swipe right on a two-way card tilts the card and names the action before release, then shows "✓ Answered:
   “Acts on a card”. Undo · 5 s". Nothing is posted until the 5 s end (c2).
3. FR-3. A swipe right on a Ship card does nothing: the card springs back and opens its confirm, "This is a one-way
   door. Read it, then press Ship #811. A swipe never ships." (c3).
4. FR-4. Every swipe has a button and a key that do the same action (WCAG 2.2, 2.5.7). In the terminal `D` shows one
   card and → ← ↑ act (c4).
5. FR-5. With reduced motion on there is no fly-out; the next card replaces the last (c5).
6. FR-6. `[inbox] undo_seconds` sets the undo time; default 5 s (c7).
7. FR-7. Deck is the default on a phone and a toggle on larger screens (Assumptions).

### Pair a phone ([#970](https://github.com/rahulnakmol/agentic-ayudha/issues/970))

1. FR-8. `Q` in the terminal and Pair a phone on the web, with Tailscale Serve set up, show a QR code and "Scan with a
   phone on your tailnet. One phone, one use. Expires in 4:59." (c1).
2. FR-9. The pairing code is in the link's `#` part, works once, for one phone, and expires after 5 minutes. A second
   scan is refused (c2).
3. FR-10. A tailnet user who is not the founder is refused before the sign-in page (c3).
4. FR-11. Without Tailscale Serve: "Tailscale Serve is not set up here, so a phone cannot reach the inbox. Run:
   tailscale serve --bg 7431" (c4).
5. FR-12. The paired phone has its own device-flow check and its own session: 1 hour after its last action, 24 hours
   at most (c5).
6. FR-13. `[inbox.web] pair_code_minutes` sets the code life; default 5 (c6).

### The Behaviours view ([#971](https://github.com/rahulnakmol/agentic-ayudha/issues/971))

1. FR-14. A view by Feature with one row per behaviour: the says sentence, its rule, its source, who approved it and
   when, its last result with its age, and its status: enforced, proposed, failing, stale, or never verified (c1).
2. FR-15. A failing proposed behaviour shows status failing in the same place as an enforced one (c2).
3. FR-16. A behaviour whose last result is 9 days old shows status stale with "last run 9 d ago" (c3). The age at which
   a result becomes stale is not stated in the issue; unverified.

### Ask why ([#972](https://github.com/rahulnakmol/agentic-ayudha/issues/972))

1. FR-17. An Ask why action on every Done card in the terminal and in its web peek (c1).
2. FR-18. Ask why with a question runs `aayu explain 802 “why X instead of Y?”` to the Accountable seat. The answer
   cites the defense record and the code (c2).
3. FR-19. For a seat with a paired human, the question goes to that person as an ask (c3).
4. FR-20. A seat that cannot answer says "I do not know" and shows the Bug it opened on the defense (c4).

### Ask Atri ([#973](https://github.com/rahulnakmol/agentic-ayudha/issues/973))

1. FR-21. `?c` on a card opens a panel "Atri · counsel, not approval · #802 Ship into dev" with the decision and its
   door, a Reading line whose facts each name their source and time, "Recommended: …", "What would change this: …",
   and "The question for you: …", above the same choices (c1).
2. FR-22. Where there is no counsel from facts: "Atri has no counsel from facts here. To discuss it, run aayu counsel
   1230 in your agent tool (copied)." with a Copy button (c2).
3. FR-23. The panel has no choice and writes nothing; Esc leaves the card's choices and the cursor as before (c3).
4. FR-24. The same panel on an issue in Browse that has counsel from facts (c5).

## Non-functional requirements

| Area | Requirement | Source |
|---|---|---|
| Safety | No ship, merge, waiver, or one-way Decision by a swipe; Undo before any post | [#969](https://github.com/rahulnakmol/agentic-ayudha/issues/969) c2, c3, Measure |
| Security | Listen on 127.0.0.1 only; never on the Wi-Fi address; the phone reaches it only through Tailscale Serve | [#970](https://github.com/rahulnakmol/agentic-ayudha/issues/970) c4, c7 |
| Security | One-time pairing code in the URL fragment, one phone, 5 minutes; non-founder tailnet users refused before sign-in | [#970](https://github.com/rahulnakmol/agentic-ayudha/issues/970) c2, c3 |
| Security | Threat row T22 covers a leaked QR code. T22 is to be added by [#957](https://github.com/rahulnakmol/agentic-ayudha/issues/957); `docs/threat-model.md` ends at T21 today | [#970](https://github.com/rahulnakmol/agentic-ayudha/issues/970) Assumptions |
| Safety | Atri never acts: the counsel panel writes nothing | [#973](https://github.com/rahulnakmol/agentic-ayudha/issues/973) c3; `AGENTS.md` (Atri "never rules, decides, approves, or acts") |
| Accessibility | axe-core 4.14, WCAG 2.2 AA: 0 serious and 0 critical findings on every Deck, pair, Behaviours, and counsel state | #969 c6, #970 c8, #971 c4, #973 c4; `CONSTRAINTS.md` `verify.floors.a11y.*` |
| Accessibility | A non-drag alternative for every swipe (WCAG 2.2, 2.5.7); reduced motion honoured | [#969](https://github.com/rahulnakmol/agentic-ayudha/issues/969) c4, c5 |
| Accessibility | Touch targets at least 44×44 CSS px (inferred: the floor applies to every state; the issues do not restate it) | `CONSTRAINTS.md` `verify.floors.a11y.min_target_px` |
| Operability | `[inbox] undo_seconds` (5) and `[inbox.web] pair_code_minutes` (5) | [#969](https://github.com/rahulnakmol/agentic-ayudha/issues/969) c7; [#970](https://github.com/rahulnakmol/agentic-ayudha/issues/970) c6 |
| Supply chain | `go.mod` and `THIRD_PARTY_NOTICES.md` are expected paths, so a new dependency is expected (inferred: a QR code encoder) | [#977](https://github.com/rahulnakmol/agentic-ayudha/issues/977) Expected paths |
