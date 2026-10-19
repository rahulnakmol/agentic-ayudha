# rahulnakmol/agentic-ayudha#709: install --loops --apply leaves an event loop unrecorded and an existing Amp subscription paused

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `4217fbf6a8e782bb0bae9628cd9240dbaa296b547fc69dab6c1282864e254f23` |
| Completion Goal hash | `69770e1f081bca3a048fab92110d4ac8d841b295566ec3cf1b7b5baf0bd09665` |
| Technical Owner | DevOps & DevEx Tinkerer |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/709/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `install --loops --host amp --apply` for an event loop, where `amp -ox` prints only the thread URL and the thread later replies `subscription <ID>` followed by a sentence; expected: aayu waits for the reply, records the loop as `subscription <ID>`, and the loop shows as active
2. [api-trace] state: the same command; expected: the thread's prompt asks it to update and enable an existing equivalent subscription with the rendered input (update_github_subscription, enabled: true) and to reply with exactly one line, `subscription <ID>`
3. [api-trace] state: the same command, and the thread never replies with a subscription ID; expected: after the wait the loop stays unrecorded, shows as missing, and the step to record it by hand is printed

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

`aayu install --loops --host amp --apply` leaves an event loop unrecorded, and can leave its Amp subscription paused with old instructions. On 2026-10-19 at 06:02 UTC @rahulnakmol ran it from dev 7c214cc and triage-opened was reported "missing". Two causes, both in `ampLoops` (`internal/cli/installloops.go:363`):

1. `amp -ox` returns the thread's URL when the thread starts, before its agent replies (every scheduled loop in that run was recorded with its thread URL as the ID). `ampLoops` reads the subscription ID only from the last line of that output (`subscriptionID` on `lastLine(out)`, line 392), so it never sees the reply.
2. `create_github_subscription` returned an existing equivalent subscription, 5d10ad31-52c8-42ef-9860-3d874655784f, titled "aayudha triage-new", paused, with the old triage-new instructions. Amp keeps an equivalent subscription's paused state and saved instructions. The prompt said "Do nothing else", so the setup thread T-01a152c0-d5d9-76b2-a75c-9f165feca52c left it paused and replied `subscription <ID> (this matched an existing subscription …; I left it unchanged)`, a sentence that the last-line pattern would not accept either.

### Steps to reproduce

1. In a checkout of rahulnakmol/agentic-ayudha where Amp already holds a paused GitHub subscription equivalent to loops/amp/triage-opened.json, run `aayu install --loops --host amp --apply`.
2. Run `aayu install --loops --host amp`.

### Expected behaviour

The loop's subscription has the rendered title and instructions and is enabled, and loops/active.json records triage-opened with `subscription <ID>`, so it shows as active.

### Actual behaviour

triage-opened is not recorded and shows as missing; the subscription stays paused with the old triage-new instructions.

### Completion Goal

1. [api-trace] state: `install --loops --host amp --apply` for an event loop, where `amp -ox` prints only the thread URL and the thread later replies `subscription <ID>` followed by a sentence; expected: aayu waits for the reply, records the loop as `subscription <ID>`, and the loop shows as active
2. [api-trace] state: the same command; expected: the thread's prompt asks it to update and enable an existing equivalent subscription with the rendered input (update_github_subscription, enabled: true) and to reply with exactly one line, `subscription <ID>`
3. [api-trace] state: the same command, and the thread never replies with a subscription ID; expected: after the wait the loop stays unrecorded, shows as missing, and the step to record it by hand is printed

### Measure

Event loops that install --loops --apply records as active after one run.

### Threshold

1 of 1 event loop (triage-opened) recorded as active, with its subscription enabled.

### Scenario

A person runs `aayu install --loops --host amp --apply` in a repository whose Amp account already holds an equivalent, paused triage subscription.

### Environment

aayu built from `dev` at 7c214cc, run in an Amp orb's Terminal tab with Amp CLI 0.0.1792382450.

### Technical Owner

DevOps & DevEx Tinkerer

### Checkpoints

1. The event loop's prompt asks the thread to update and enable an existing equivalent subscription and to reply with one line (criteria 2)
2. ampLoops finds the ID on any line, and waits on the thread's reply with `amp threads markdown` when `amp -ox` printed none (criteria 1, 3)

### Assumptions

Amp subscription IDs are UUIDs; `amp threads markdown <thread>` prints a thread's messages, its last assistant message last.

### Provenance

Request: "When the spawner returns an existing subscription, install --loops --apply must still end with the loop's subscription set to the rendered input and enabled, and recorded. Ask the setup thread to update and enable a matching existing subscription, and to reply with exactly one line, `subscription <id>`. Parse the ID robustly from the output, not only from the last line. Add a test that fails on dev."
1. default (behaviour): "must still end with the loop's subscription set to the rendered input and enabled, and recorded"
2. default (behaviour): "Ask the setup thread to update and enable a matching existing subscription, and to reply with exactly one line"
3. default (behaviour): "Parse the ID robustly from the output, not only from the last line"
````
