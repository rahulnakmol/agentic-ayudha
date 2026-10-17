# aayu research 570 (synthetic fixture)

Run the research stage for Feature #570 and write its `--from` directory.

## Feature #570: Customers choose a delivery window

A grocery web shop. Today every order is delivered "between 8:00 and 20:00"; customers cannot choose. The delivery
team can now offer two-hour windows for the next three days, each with a capacity.

Stories (Ready):

- #571 Choose a delivery window. State `default`. Completion Goal:
  1. [screenshot] state: a signed-in customer on `/preferences` with tomorrow's windows seeded (two available, one
     full); expected: the windows are listed with their availability, the full one cannot be chosen, and the chosen
     one is marked.
- #572 A window that fills up while choosing. State `slot-full`. Completion Goal:
  1. [screenshot] state: the chosen window becomes full before Save; expected: a message next to the window list
     says it is full and the nearest available windows are offered; the choice is not lost silently.
- #573 Confirmation after saving. State `saved`. Completion Goal:
  1. [screenshot] state: after Save; expected: a confirmation names the chosen window and day.

## Research data the host has (synthetic, seeded)

- Delivery records (`ops/deliveries-2026-q3.csv`): 18 % of first delivery attempts fail because nobody is home.
- Support tickets tagged `delivery-time` in Q3 (`support/2026-q3-delivery.csv`, 64 tickets): 40 ask to choose a
  time, 15 report a missed delivery, 9 ask to change a time after ordering.
- Post-delivery survey free text (`survey/2026-q3.csv`, 3 of 212 answers quoted by the host):
  - S-17: "I work shifts, so 8 to 8 is useless to me."
  - S-88: "Let me pick the evening and I will be there."
  - S-140: "I missed it twice and had to take a day off for the third try."
- No usability sessions have been run on this Feature yet. The shop has a customer panel of 300 people who agreed
  to research (`research/panel.md`), 22 of them use a screen reader or magnifier.

## References the host gathered (search results, as returned)

| Source | Kind | ID | URL |
|---|---|---|---|
| mobbin | screen | 5e4d3c2b-1a09-4f8e-8d7c-6b5a4f3e2d1c | `https://mobbin.com/screens/5e4d3c2b-1a09-4f8e-8d7c-6b5a4f3e2d1c` |
| mobbin | screen (preview image) | 5e4d3c2b-1a09-4f8e-8d7c-6b5a4f3e2d1c | `https://cdn.bytescale.com/mobbin/5e4d3c2b-preview.webp` |
| mobbin | flow | 9a8b7c6d-5e4f-4a3b-8c2d-1e0f9a8b7c6d | `https://mobbin.com/flows/9a8b7c6d-5e4f-4a3b-8c2d-1e0f9a8b7c6d` |
| web | guideline | gov-uk-radios | `https://design-system.service.gov.uk/components/radios/` |
| web | guideline | wcag-2.2-3.3.1 | `https://www.w3.org/WAI/WCAG22/Understanding/error-identification.html` |
| web | article | baymard-delivery-dates | `https://baymard.com/blog/delivery-date-selection` |
| web | image | pinterest-slot-picker | `https://i.pinimg.com/originals/slot-picker.jpg` |

The team stage and the stories' states are fixed. There is no usability evidence on choosing a window yet, so the
stage must say what to learn before the design is settled, and how.
