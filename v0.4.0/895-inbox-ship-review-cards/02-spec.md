# Spec: Feature #895

## Source note

There is no spec baseline: `origin/ledger` has no `specs/895/` files. The requirements below come from the Completion
Goals of the 15 sub-issues (listed in `_data/subs.tsv`). "Cn" means criterion n of that issue's Completion Goal. The
words in quotes are the issue's own expected text. Every requirement is planned, not built, unless
[05-implementation-plan.md](05-implementation-plan.md) says a part exists on dev.

## Functional requirements

### Stale cards and results on the card (Bug #924)

- FR1: Two Story PRs shipped from the inbox a few seconds apart both merge into dev; the second is judged against dev's
  new tip, with no failed state (#924 C1).
- FR2: When the first merge changes policy (for example `[autonomy] always_person`), the second does not merge. Its card
  shows the amber block "#`<n>` changed on GitHub after you read it. Nothing was written.", a line that names the
  change, and a "What now:" line (#924 C2).
- FR3: Enter during a refresh starts the action only after that refresh ends, against the fresh read (#924 C3).
- FR4: A changed card lists the changed fields old → new (head, checks, readiness, options), says "Nothing was written.",
  and gives one "What now:" line; the block shows once per change (#924 C4, C5).
- FR5: A failed ship shows a red block on its card: what was written, what was not, GitHub's message, and the next keys;
  the bottom line shows only key hints (#924 C6).
- FR6: Every action outcome (done, refused, changed, failed, needs another step) shows on the card it acted on, and stays
  until the cursor moves or another action starts (#924 C7).
- FR7: A ship shows its stages on the card ("Re-reading #802…", "Approving…", "Merging into dev…") and the cursor stays on
  it; Tab moves on at any time (#924 C8).
- FR8: A done ship shows "✓ Shipped #802 into dev as `<sha>` · by @`<login>` · `<time>`" and moves to Done today when the cursor
  leaves (#924 C9).
- FR9: A refresh that reorders the list keeps the card under the cursor in place while its action runs (#924 C10).
- FR10: A refresh cut off while it read a changed issue's comments does not bring back the cards from before that read
  (#924 C11).

### Reads, loading, and GitHub state (Bug #925)

- FR11: With a cache and before the first read ends, the header says "From cache · `<age>` · reading GitHub…" and no empty
  state shows; the cards appear in the same frame as the read ends (#925 C1, C2).
- FR12: With no cache, a progress line with a bar ("Reading GitHub for the first time… 12 of 31 items") shows until the
  read ends (#925 C3).
- FR13: A cached card's choices show "reading…" and Enter does nothing until that card is read (#925 C4).
- FR14: After R, the header says "Refreshed `<time>` · `<n>` new · `<n>` changed"; later it says "● Live · refreshed `<age>`"
  (#925 C5, C6).
- FR15: When GitHub does not answer by the read deadline, a banner gives the data age and the retry rule, and the cards stay
  usable (#925 C7). A rate limit names the wait and when it ends (#925 C8).
- FR16: A "not done" ship card that a refresh reads as conflicting says "Not ready yet: conflicts with dev; the agent
  rebases" (#925 C9).
- FR17: Two inbox processes that write done marks at once leave whole JSON that holds both sessions' marks (#925 C10).
- FR18: `[inbox] refresh_seconds = 45`, `read_timeout_seconds = 180`, and `read_retry_seconds = 30` are defaults that a
  config can change (#925 C11).

### Ship and review cards

- FR19: A PR whose rung B failed on its head and whose reviewer app has not replied in 30 minutes is a "Needs your review"
  card that names both causes and links the inspect comment; `--list --json` has one item of kind `human-review`
  with the head SHA, why, and the comment URL; no item shows while the 30-minute wait runs (#926 C1, C2, C3).
- FR20: Choice 1 on a Ship or Needs your review card is "Approve and ship"; three such cards ship under one device code,
  each head approval is on the ledger (`inspect/approved.jsonl`), and each PR merges (#926 C4, C5).
- FR21: A PR that used its review budget (`[budget] per_pr_inspect_max`) is a Needs your review card that names the spend
  (#926 C6). Approve and ship with a red required check records the approval, merges nothing, and names the check
  (#926 C7).
- FR22: A non-Story PR that changes a CODEOWNERS path, with inspect at merge, every required check green except
  `protected-paths`, and MERGEABLE, is one bypass card for the approver; it names the CODEOWNERS paths, the
  narrow-set files, the reviewer line, and open RAID follow-ups (#927 C1). With any condition false there is no
  bypass card (#927 C2, C5).
- FR23: Ship on a bypass card asks for a one-line reason, then a second Enter; then one presence check at admin
  permission, one "Solo-mode bypass:" comment with the reason, the head, and the paths, one squash merge at the
  card's head with no `--admin`, a ledger record with the comment URL, and the head branch deleted (#927 C3, C4).
- FR24: `aayu audit` lists a bypass from a card as allowed (#927 C6). No choice on any card offers "always" or "don't ask
  again" (#927 C7).
- FR25: x selects Ship cards ("3 selected · B ships them"); B shows a hold that counts down ("Starting in 5 s · Esc
  cancels"); Esc writes nothing; after the hold, one presence check, then each PR is re-read and shipped at its own
  head with one result line each (#928 C1, C2, C3).
- FR26: Each bypass card in a batch asks its own reason before the hold (#928 C4). Enter on one Ship card ships at once,
  with no hold (#928 C5).
- FR27: `[inbox] batch_hold_seconds` (default 5) sets the hold (#928 C6). A test fails when an inbox file holds a duration
  literal for a wait, hold, deadline, or expiry; such times come from `[inbox]`, `[inbox.web]`, `[decisions]`, or
  `[time]`; only the 1 s display clock and the frame rate are exempt (#928 C7).
- FR28: Green Story PRs that wait only for a person fold into one Ship line with "1 Ship all 3 after one presence check · 2
  Open them one by one"; with a review not finished, the line says so and marks no choice (#928 C8).
- FR29: A code owner card: `[codeowners]` in `.aayudha/config.toml` writes the aayudha CODEOWNERS block, and a drift check
  names any other difference (#935 C1). Each eligible owner (user or `@org/team` member, not the author, not a bot)
  gets a card; approve posts one APPROVED GitHub review on the head after one presence check; the PR merges with no
  `--admin` and no bypass comment (#935 C2, C3, C4). An author-only owner gets the bypass card instead; a moved head
  shows the card again (#935 C5, C6).
- FR30: A Ship card lists open findings at the head (id, severity, reviewer, path:line, text). x on a finding asks for one
  line, posts it as the person, records it on the ledger, and shows inspect's new verdict (#936 C1, C2). A P1
  dismissal is a one-way door and needs a second Enter (#936 C3).
- FR31: A WAIVER card shows the floor and its source, the measured value with its CI run and age, the scope, and the expiry;
  Accept is a one-way door with one presence check; the result names the expiry (#936 C4, C5).

### Links, card layout, signals, header, leave, keys

- FR32: Each card ends with its full URL as plain text inside one OSC 8 link with an `id=`; it wraps and is never cut with
  "…" (#929 C1).
- FR33: c writes the URL with one OSC 52 write and says "Copied the link to #802." (#929 C2). o over SSH runs no opener and
  copies instead, and says why; o locally runs the opener with the URL, and copies when it fails (#929 C3, C4).
- FR34: The copy reaches the outer terminal in tmux (with `set-clipboard on` or passthrough) and in herdr locally, over
  SSH, and with `herdr --remote`; with tmux's default the card names `set -s set-clipboard on` (#929 C5, C6).
- FR35: The help says that tmux or herdr takes ctrl+b; the inbox binds neither ctrl+b nor ctrl+a; `docs/guide/` has a tmux
  and a herdr section (#929 C7, C8).
- FR36: A Ship card shows, in order: kind and number, "Why you:", the evidence row, the counsel panel, the brief, and the
  choices, each with one line on what it does; at 100x24 the brief shrinks first (#930 C1, C2).
- FR37: d shows the brief rows (what, size, pushed, checks, review, last comment) with values and times; every fact names
  its source and time or head SHA, and a fact with no source is not shown (#930 C3, C5).
- FR38: "Reply to `<twin>`" previews the text and posts it as the person on Enter; Esc posts nothing (#930 C4).
- FR39: A card marks a choice as recommended only when its source recommends it; otherwise it says "No recommendation: …"
  (#930 C6). A spend, scope, or one-way card with no recommendation shows Atri's counsel inline and marks no choice;
  a two-way card with no spend or scope shows no counsel panel (#930 C7).
- FR40: When a card enters To do and the inbox is not focused, the terminal title becomes "aayu inbox (`<n>`)" and herdr's
  pane state becomes blocked; no bell unless `[ui] bell = true`; nothing changes for a focused view (#931 C1, C2).
- FR41: The first line after a visit says "Since `<time>`: `<n>` shipped · `<n>` needs you · `<n>` changed."; `aayu inbox --brief`
  prints only that line (#931 C3, C4).
- FR42: The header names the repository, its visibility, the Initiative, the counts, and how long writes stay armed; it
  drops parts from the right on narrow terminals and shows no branch or tag; the footer is "⌘K command · ? keys"
  (#932 C1 to C4).
- FR43: q during a write says which write runs, finishes it, then quits, and prints "This session: …" (#933 C1, C2).
  `aayu inbox --today` prints one line per action with its ledger entry (#933 C3). `aayu inbox 802` opens on card
  #802 and shows another session's done mark (#933 C4).
- FR44: ? shows a grouped key overlay with this view's keys first and the same keys and words on the web and the terminal
  (#934 C1). A choice the reader may not take is hidden, with who can take it (#934 C2). A second session's write
  shows "@`<login>` is shipping this now." and pauses the choices (#934 C3).
- FR45: One keymap source generates the keys of the web, the terminal, the ? overlay, the palette, and the docs; a test fails
  when a face binds a key the source does not list; tooltips end with the key and carry `aria-keyshortcuts` (#934
  C4, C5).

### Command output wrap (Story #980)

- FR46: `ui.Printer` prose breaks only at spaces, at the last space before min(width, 100); a line that starts with
  "aayu: " continues at column 7; any other line continues at its own indent + 2 (#980 C1, C2).
- FR47: The 33 command screens of the 0.4.0 baseline at widths 60, 80, 100, and 120 cut no word, and the joined words equal
  the unwrapped output (#980 C3).
- FR48: `--plain` and `NO_COLOR` wrap the same way; `--json` and a pipe never wrap; a long URL stays whole on its own line;
  a command in backticks and an issue number are never split (#980 C4, C5).

### Permission 404 (Bug #1122)

- FR49: A Ready to ship marker or a counsel comment by a login whose permission read returns 404 is text:
  `aayu inbox --list` exits 0 and lists every other card; a 500 still exits 5 (#1122 C1 to C4).

## Non-functional requirements

| Area | Requirement | Source |
|---|---|---|
| Security | Every write is the person's, under their own token, after a presence check; Approve and ship, a batch, a bypass, an owner approval, a dismissal, and a waiver each ask once per session or per batch | #926 C5, #927 C4, #928 C3, #935 C3, #936 C5; `internal/cli/presence.go` |
| Security | No "always" or "don't ask again" choice anywhere | #927 C7 |
| Security | One-way doors (a P1 dismissal, a waiver accept, a choice that is not recommended) need a second Enter | #927 (TP13 note), #936 C3, C5 |
| Security | A marker or counsel from an account without write access is text; only a 404 is treated so, other errors fail closed | #1122 C4; `scopeTrusted` in `internal/cli/scope.go` |
| Security | The bypass merge uses the REST merge API with the head SHA and never `--admin` | #927 C4 and Assumptions |
| Reliability | The JSON cache stays whole and keeps both sessions' marks under concurrent writes (no SQLite) | #925 C10; issue #895, Scope |
| Reliability | A quit never leaves a write half done | #933 Measure and C1 |
| Performance | Defaults: refresh every 45 s, read deadline 180 s, retry every 30 s, batch hold 5 s | #925 C11, #928 C6 |
| Operability | Every inbox wait, hold, deadline, and expiry is a config key; a test enforces it | #928 C7 |
| Operability | tmux and herdr settings are documented in `docs/guide/` | #929 C8 |
| Accessibility | Choices and counsel stay in view at 100x24; keys carry `aria-keyshortcuts`; `--plain` and `NO_COLOR` keep the same wrap | #930 C2, #934 C5, #980 C4 |
| Accessibility | Every Completion Goal state is reachable by keyboard alone (floor) | `CONSTRAINTS.md`, `verify.floors.a11y.keyboard_only` |
| Quality | Changed lines need 90% coverage | `CONSTRAINTS.md`, `verify.floors.coverage.min_diff_percent` |
| Cost | A PR's review spend is capped by `[budget] per_pr_inspect_max` (15.0 in this repository) and the card shows the spend | #926 C6; `.aayudha/config.toml` |
| Test environment | Each Story is tested against `internal/github/fake` with seeded data, and captured at 120x34 | Environment section of each Story |

## Requirements not yet in the bodies

Each Story says its "Review guidance and Behaviours … are kept in the Feature's drafts" until Features #887 and #891
add the form fields (for example #924, Assumptions). Those drafts are not on GitHub or the ledger (unverified where
they live).
