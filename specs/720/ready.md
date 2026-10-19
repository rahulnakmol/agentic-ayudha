# rahulnakmol/agentic-ayudha#720: aayu inbox: approve the session on the inbox's first screen, with one key to open GitHub and one to copy the code

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `105ca55ec2cbed775a088b8621947fd685637a67cc397398ed550d2697078388` |
| Completion Goal hash | `f6d16605aa4fd1669fde2404f65d4f76e8ca2d5ec40d1b6a66455ba240a07bc6` |
| Technical Owner | ux-researcher-evolutionist |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/720/ready.md` |

Completion Goal at Ready:

1. [screenshot] state: `aayu inbox` at 120x34 and at 72x34 in a shell with an agent marker and no approval yet this session; expected: the inbox's first screen shows the code large in a box, https://github.com/login/device as an OSC 8 hyperlink, "Waiting for GitHub…", the expiry as a countdown with its UTC time, and the key bar "Enter open GitHub · c copy the code · q quit"; every line is exactly the terminal's width
2. [api-trace] state: the full-screen inbox while a code is pending, then the run's stdout, stderr, the inbox log, the actions log, the ledger, and the inbox cache after it; expected: the code appears only on the screen; it is absent from stdout, stderr, the log, the status line, the ledger, and the cache
3. [api-trace] state: Enter or o with a browser on the machine, Enter with none, then c; expected: Enter or o opens https://github.com/login/device; with no browser the status line says to open the link on any device and that c copies the code; c sends the code through OSC 52 and the body says "Copied" once and the status line does not repeat it
4. [screenshot] state: the screen inside tmux with set-clipboard set to external; expected: it says the copy reaches the clipboard only with set-clipboard on, names `tmux set -s set-clipboard on`, and offers selecting the code with the mouse; with set-clipboard on, no such line shows
5. [api-trace] state: the code expires, then Enter, then the person approves on GitHub; expected: the screen says the code expired and that nothing was done, without telling the person to run the command again; Enter asks GitHub for a new code; on approval the screen says "Approved by @login" and the inbox loads; q at any point quits with "inbox was not approved; nothing was done"
6. [api-trace] state: `aayu inbox` in a terminal with no agent marker, and the line-by-line inbox (--plain) in a marked shell; expected: the full-screen inbox shows no login screen and opens from its last read as before; --plain prints the code as before
7. [api-trace] state: the presence check's approval for a code approved by a bot, by another account than the token's, and by an account without write access; expected: each is refused exactly as before the change; the approval is recorded in the actions log as before; presence.go's only change is the hook that replaces the printed code

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want the device-flow approval of an `aayu inbox` session to be the inbox's own first screen, with a way to open GitHub's page and copy the code in one keypress each, so that approving from the orb's Terminal tab takes seconds and I never retype a code read off plain text (@rahulnakmol, 2026-10-19).

Today the code is printed as plain text on stderr before the full-screen inbox starts (`presenceCheck` in `internal/cli/presence.go`). The new screen shows the same code in the inbox's frame. presence.go gains one hook, a callback called in place of the print, and its approval logic stays as it is. GitHub returns no `verification_uri_complete` for aayu's app (checked with a live request that printed only the response keys), so the shortest path is open plus copy.

### Completion Goal

1. [screenshot] state: `aayu inbox` at 120x34 and at 72x34 in a shell with an agent marker and no approval yet this session; expected: the inbox's first screen shows the code large in a box, https://github.com/login/device as an OSC 8 hyperlink, "Waiting for GitHub…", the expiry as a countdown with its UTC time, and the key bar "Enter open GitHub · c copy the code · q quit"; every line is exactly the terminal's width
2. [api-trace] state: the full-screen inbox while a code is pending, then the run's stdout, stderr, the inbox log, the actions log, the ledger, and the inbox cache after it; expected: the code appears only on the screen; it is absent from stdout, stderr, the log, the status line, the ledger, and the cache
3. [api-trace] state: Enter or o with a browser on the machine, Enter with none, then c; expected: Enter or o opens https://github.com/login/device; with no browser the status line says to open the link on any device and that c copies the code; c sends the code through OSC 52 and the body says "Copied" once and the status line does not repeat it
4. [screenshot] state: the screen inside tmux with set-clipboard set to external; expected: it says the copy reaches the clipboard only with set-clipboard on, names `tmux set -s set-clipboard on`, and offers selecting the code with the mouse; with set-clipboard on, no such line shows
5. [api-trace] state: the code expires, then Enter, then the person approves on GitHub; expected: the screen says the code expired and that nothing was done, without telling the person to run the command again; Enter asks GitHub for a new code; on approval the screen says "Approved by @login" and the inbox loads; q at any point quits with "inbox was not approved; nothing was done"
6. [api-trace] state: `aayu inbox` in a terminal with no agent marker, and the line-by-line inbox (--plain) in a marked shell; expected: the full-screen inbox shows no login screen and opens from its last read as before; --plain prints the code as before
7. [api-trace] state: the presence check's approval for a code approved by a bot, by another account than the token's, and by an account without write access; expected: each is refused exactly as before the change; the approval is recorded in the actions log as before; presence.go's only change is the hook that replaces the printed code

### Measure

Places outside the login screen where the device code appears, across stdout, stderr, the inbox log, the status line, the actions log, the ledger, and the inbox cache, in a run that approves a session.

### Threshold

0.

### Scenario

@rahulnakmol runs `aayu inbox` in the orb's Terminal tab, presses c and pastes the code into github.com/login/device on his phone, approves, sees "Approved by @rahulnakmol", and the inbox loads.

### Environment

`aayu inbox` in a shell with an agent marker (the orb's Terminal tab), at 120 and 72 columns, inside and outside tmux.

### Technical Owner

ux-researcher-evolutionist

### Links

### Checkpoints

1. `go test -race ./internal/cli -run 'TestInboxLoginScreen|TestInboxApprovesHumanReviews|TestInboxTUI|TestPresence'` passes (criteria 1, 2, 3, 4, 5, 6, 7)
2. Renders of the real login screen at 120x34 and 72x34 (waiting, copied, approved, expired) are in the Amp thread (criteria 1, 4, 5)

### Assumptions

The person's terminal accepts OSC 52. aayu cannot confirm the clipboard received it, so the screen offers selecting the code with the mouse.

### Open questions

### Parked ideas

A QR code of the link for a phone (the code still has to be typed, so it saves little).

### Provenance

Request: "the presence check, where we need to log in to GitHub and enter a code: the device login should be part of the aayu inbox UI, with a way to copy it with a single click, or it opens in the user's browser"
1. stated (behaviour): "the device login should be part of the aayu inbox UI"
2. confirmed (behaviour): "the code is shown only to the person at the terminal, never logged, never in the ledger"
3. stated (behaviour): "with a way to copy it with a single click, or it opens in the user's browser" / "Copied shows both in the body and in the status line. Keep it in the body only"
4. confirmed (behaviour): "Check that tmux passes OSC 52 through (set-clipboard), and say in the UI what to do if the copy does not reach the clipboard"
5. confirmed (behaviour): "Show the expiry as a countdown, and a Waiting for GitHub… line that turns into Approved by @login before the inbox loads"
6. default (behaviour): "the device login should be part of the aayu inbox UI"
7. confirmed (behaviour): "Keep its approval logic unchanged, and add only a hook so a caller can render the pending code itself"
````
