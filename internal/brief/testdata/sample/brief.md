# Filtered claims now export to CSV

Reviewers can export exactly the claims they filtered, with only the visible columns, and the merge gate's
authoritative pack shows every floor held [P1].

## What changed

Story #12 adds an Export CSV action to the claims list [#12]. The file holds every claim that matches the applied
filters and only the visible columns, and the list keeps responding while the file is prepared [P1].

```screen
alt: The Export CSV button sits beside the active filters, so an export follows them.
pack: P1
artifact: screens/1.png
caption: The claims list with two filters applied and four of six columns shown.
10,11: The applied filters decide which rows are written.
42,11: Only the visible columns are written.
90,11: Export CSV starts the download.
```

## How it reached dev

```diagram
alt: A change reaches dev only after the gate runs the floors again and writes the authoritative pack.
src: flow.diagram
```

The merge gate ran coverage, SAST, and the accessibility floor again on the head commit, and all three held [P1].

## What it changed for users

```chart
alt: Most sign-ups came in the first week of the window, and none in the third.
snapshot: D1
x: week
y: acquired
caption: Users acquired per weekly cohort.
SELECT week, COUNT(*) AS acquired
FROM (SELECT user_id, MIN(week) AS week FROM events WHERE name = 'signed_up' GROUP BY user_id)
GROUP BY week ORDER BY week
```

Activation in complete cohorts measured 0.4 against a target of at least 0.35, so the Feature met its target [R].

```illustration
alt: Filtered rows go in and a CSV with exactly those rows comes out.
src: illustration.png
generator: Painter
caption: How the export works, drawn for this brief.
```
