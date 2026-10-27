# rahulnakmol/agentic-ayudha#1130: inspect drops CodeRabbit findings whose severity badge is an icon, so a Major reads as no finding

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `801988c4e637dca96e03145d0c120823901101909813a7a65e5c600cbac28909` |
| Completion Goal hash | `371f64f4ffe3dae7170461a3af8b61223ae20021474b5e9aab6e485a528bb67d` |
| Technical Owner | engineering-manager |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1130/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: the merge gate's CodeRabbit review of #1096 at b6a66dc, read from a fixture of review 5668710694 and its inline comment on `internal/cli/ship.go`, whose badge is an `<img>` icon and the word `Major`; expected: 1 finding, severity P1, on `internal/cli/ship.go`, titled `Fall back to the Outcome comment when the lock guard cannot read the locks.`, and the disposition is fix
2. [api-trace] state: the same inline comment with the emoji badge `**🟠 Major**` in place of the icon badge; expected: the same P1 finding and the disposition fix, as today
3. [api-trace] state: one icon-badge inline comment at the head for each of `Critical`, `Major`, `Minor`, `Trivial`, and `Info`; expected: the severities P0, P1, P2, P3, and P3, the same as for the emoji badges
4. [api-trace] state: the outside-diff block of CodeRabbit review 5661577947 of #1120 at 4dcc80f, as a fixture, whose summary is an icon badge with `Major · Require the named eval before shipping a listed eval fix. · missiongate.go:902-909`; expected: 1 P1 finding at `internal/cli/missiongate.go:902`
5. [api-trace] state: a CodeRabbit reply in a review thread at the head, with no severity badge in either form; expected: no finding, as today

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

inspect drops each CodeRabbit finding whose severity badge is an icon, so the merge gate's CodeRabbit review (rung A) says 0 findings and `merge` over a Major. `serviceNative` (internal/inspect/apps.go:599-607) finds a severity only with `greptileBadge` or `coderabbitBadge`, and `coderabbitBadge` (apps.go:526) needs an emoji badge such as `🟠 Major`. Since 2026-10-25, CodeRabbit writes the badge as an `<img>` icon and the word: `**<picture><img src="…/major-16.svg" …></picture> Major**`. The gate takes such a comment as "a reply in a thread, not a finding" and skips it. The outside-diff blocks in a review's body changed the same way, so `coderabbitBodyFinding` (apps.go:534) misses them too.

### Steps to reproduce

1. CodeRabbit reviews #1096 at b6a66dc (review 5668710694). Its one inline comment, on `internal/cli/ship.go`, starts `**<picture><img src="https://storage.googleapis.com/coderabbit_public_assets/review-comment-icons-v1/major-16.svg" …></picture> Major** · **Data Integrity & Integration** · **Heavy lift**`.
2. The merge gate runs inspect with the CodeRabbit review (rung A) at b6a66dc.

### Expected behaviour

The comment is a P1 finding, because `coderabbitSeverity` (apps.go:540) maps Major to P1, so the disposition is `fix`. A badge in either form, emoji or icon, gives the same severity.

### Actual behaviour

inspect says `merge` with "coderabbit reviewed b6a66dcc7484: 0 finding(s) on that commit." (comment 6428108241 on #1096). Of the CodeRabbit inline comments on this repository, 85 use the icon form, the first on 2026-10-25; the last emoji-form comment is from 2026-10-21. So each rung A review since 2026-10-25 can hide its findings.

### Completion Goal

1. [api-trace] state: the merge gate's CodeRabbit review of #1096 at b6a66dc, read from a fixture of review 5668710694 and its inline comment on `internal/cli/ship.go`, whose badge is an `<img>` icon and the word `Major`; expected: 1 finding, severity P1, on `internal/cli/ship.go`, titled `Fall back to the Outcome comment when the lock guard cannot read the locks.`, and the disposition is fix
2. [api-trace] state: the same inline comment with the emoji badge `**🟠 Major**` in place of the icon badge; expected: the same P1 finding and the disposition fix, as today
3. [api-trace] state: one icon-badge inline comment at the head for each of `Critical`, `Major`, `Minor`, `Trivial`, and `Info`; expected: the severities P0, P1, P2, P3, and P3, the same as for the emoji badges
4. [api-trace] state: the outside-diff block of CodeRabbit review 5661577947 of #1120 at 4dcc80f, as a fixture, whose summary is an icon badge with `Major · Require the named eval before shipping a listed eval fix. · missiongate.go:902-909`; expected: 1 P1 finding at `internal/cli/missiongate.go:902`
5. [api-trace] state: a CodeRabbit reply in a review thread at the head, with no severity badge in either form; expected: no finding, as today

### Measure

Completion Goal criteria observed

### Threshold

all 5 criteria

### Scenario

Read CodeRabbit's review of a head through `serviceNative`: the real #1096 inline comment in its icon form and in the emoji form, an icon badge of each level, the real #1120 outside-diff block, and a reply with no badge.

### Environment

go test of internal/inspect with fixtures of the real CodeRabbit comments (testdata); never production

### Technical Owner

engineering-manager

### Checkpoints

1. The #1096 fixture gives a P1 finding and the disposition fix (criteria 1, 2)
2. Each icon level maps to its severity, and a reply with no badge is still no finding (criteria 3, 5)
3. The #1120 outside-diff block gives a P1 finding at its file and line (criteria 4)

### Assumptions

- CodeRabbit's level word after the icon is one of `Critical`, `Major`, `Minor`, `Trivial`, or `Info`; the icon file name is not a level (Minor uses `amber-16.svg`).
- The fix changes only how inspect reads a severity; the severity map at apps.go:540 does not change.

#### Notes

The fix is in `internal/inspect/apps.go`: `coderabbitBadge` and `coderabbitBodyFinding` also accept the icon form. inspect is the merge gate's own code, and the gate installs aayudha from the base branch.

#### Review guidance

- Check: Criterion 1: the fixture is the real comment body of #1096, not a hand-written copy.
- Check: Criterion 5: a badge-free reply still gives no finding, so replies do not become findings.
- Risk: a loose pattern can read a level word in a comment's text as a badge.
- Ignore: the finding's category; the new form has no `🔒 Security` emoji, and this Bug does not change the category.

#### Behaviours

1. says: A CodeRabbit finding counts at its severity, whatever form its badge has; rule: never; source: criteria 1, 2, 3, 4
2. says: A CodeRabbit comment with no severity badge is not a finding; rule: never; source: criteria 5

### Open questions

- answered: the inline comment of #1096 has `line` null on GitHub (position 8), so the finding has no line; the criterion names only its file.

### Parked ideas

- security category: read the security category from the new badge line, which has no `🔒 Security` emoji

### Provenance

Request: "inspect's inline-finding parser (internal/inspect/apps.go:599-607) needs an emoji badge such as "🟠 Major". CodeRabbit's inline comments now use an <img> icon and "**Major**" text, so the gate skips them as "a reply in a thread" and says 0 findings. Review 5668710694 on #1096 at b6a66dc has a valid Major at ship.go:1037 (a P1 by apps.go:540), and inspect said merge. Its criteria must cover both comment formats, with the real #1096 comment body as a fixture."
Term: "P1" → inspect's severity P1, which `coderabbitSeverity` gives a CodeRabbit Major; "a P1 by apps.go:540"
1. stated (request): "with the real #1096 comment body as a fixture" / "has a valid Major at ship.go:1037 (a P1 by apps.go:540)"
2. stated (request): "Its criteria must cover both comment formats"
3. stated (request): "Its criteria must cover both comment formats"
4. stated (request): "Its criteria must cover both comment formats"
5. stated (request): "so the gate skips them as "a reply in a thread""
````
