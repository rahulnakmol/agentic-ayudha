# Customers choose a delivery window: research notes (#570)

All data is synthetic and seeded: Q3 delivery records (`ops/deliveries-2026-q3.csv`), 64 Q3 support tickets
tagged `delivery-time` (`support/2026-q3-delivery.csv`), and three quoted survey answers (`survey/2026-q3.csv`).
No usability sessions have been run on this Feature.

## What the data says

- 18 % of first delivery attempts fail because nobody is home (`ops/deliveries-2026-q3.csv`).
- Of 64 tickets, 40 ask to choose a time, 15 report a missed delivery, and 9 ask to change a time after ordering.
- Survey answers name the circumstance, not a screen:

> I work shifts, so 8 to 8 is useless to me. (S-17)

and

> I missed it twice and had to take a day off for the third try. (S-140)

What the data does not say: how people compare windows, what they expect when a window fills up, or whether
"two-hour window" reads as the arrival time or the whole delivery. These decide the `default`, `slot-full`, and
`saved` states, so they are researched before the wireframes are settled.

## Research plan

**Goals.** Learn how customers pick a window that fits their day, and whether they recover when it fills up.

**Questions.**

1. How do people choose among windows over three days: by day first, or by time of day first?
2. When the chosen window fills before Save (#572), do they notice, understand why, and pick another?
3. Does the confirmation (#573) give them what they need to plan (day, window, and how to change it)?

**Method.** Moderated remote usability test of the wireframes as a clickable prototype, 30 minutes each, think-aloud.

**Participants and recruiting.** Eight people from the customer panel (`research/panel.md`): four who reported a
missed delivery or work shifts, and at least two of the panel's 22 screen-reader or magnifier users, so the
`slot-full` message is tested with assistive technology.

**Tasks.** (1) Choose a window for tomorrow evening. (2) The window fills while you choose: finish anyway.
(3) Find which window you chose.

**How findings reach the backlog.** Each finding is tagged with its state and question, filed as Feedback on #570,
and anything that changes a Completion Goal goes back to the story owner before the mockups.

## References

Pinned in `stage.json` as URLs and IDs only: a Mobbin delivery-slot screen and checkout flow, the GOV.UK radios
pattern (one choice from a short list), WCAG 2.2 3.3.1 for the full-window message, and Baymard's delivery-date
article. The Mobbin preview image and the Pinterest image were dropped: references are pages, never pixels.
