# rahulnakmol/agentic-ayudha#1248: fix(mission): extend refuses before the time limit, so a person who checks in early cannot extend the mission

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `ca53520c079c95efa6d6e4cb1fa6925089339daa4e6c004eac8a7274b5d05985` |
| Completion Goal hash | `718bc7de04852f61ecb9ac539d90af2bef4a37950d0ab734e01a7a392f7f7264` |
| Technical Owner | engineering-manager |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1248/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: a mission is at 45 h of 48, and the extend window is 6 h; expected: `aayu mission extend <f> --reason "…"` asks for the device code, records the extend in the plan's controls, and sets the next limit to 96 h
2. [api-trace] state: a mission is at 30 h of 48; expected: extend refuses and names the time the window opens
3. [api-trace] state: a mission is at 48 h or more; expected: extend works as today
4. [screenshot] state: a mission enters the extend window and the person opens `aayu inbox`; expected: an Extend card shows the hours used, what still runs and waits, and the extend command; the card is absent outside the window
5. [api-trace] state: an agent lane runs extend; expected: it is refused, as today

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

`aayu mission extend` refuses a mission until its time limit is reached, so a person who checks in before the limit cannot extend it and must come back after the limit, when the mission already starts nothing. On 2026-11-02, @rahulnakmol ran `aayu mission extend 1170 --reason "MS2 and BUGS still building after the MS1 gate fixes"` at 45.5 h of 48. It refused with "No mission of #1170 reached its time limit: nothing to extend (mission 1 45.5 h of 48)" (internal/cli/missiontime.go:860). He checks in about every 4 hours, so the limit at 14:24 UTC fell between check-ins. Nothing warned him before it.

### Steps to reproduce

1. A mission runs with `[mission] max_hours = 48` and is at 45 h.
2. The person who started it runs `aayu mission extend <f> --reason "…"`.

### Expected behaviour

Inside a window before the limit, extend adds `[mission] max_hours` from the limit, with the reason and the person's code. aayu tells the person before the limit, in the inbox, that the extend window is open.

### Actual behaviour

Extend refuses until the limit, and nothing tells the person in advance.

### Completion Goal

1. [api-trace] state: a mission is at 45 h of 48, and the extend window is 6 h; expected: `aayu mission extend <f> --reason "…"` asks for the device code, records the extend in the plan's controls, and sets the next limit to 96 h
2. [api-trace] state: a mission is at 30 h of 48; expected: extend refuses and names the time the window opens
3. [api-trace] state: a mission is at 48 h or more; expected: extend works as today
4. [screenshot] state: a mission enters the extend window and the person opens `aayu inbox`; expected: an Extend card shows the hours used, what still runs and waits, and the extend command; the card is absent outside the window
5. [api-trace] state: an agent lane runs extend; expected: it is refused, as today

### Measure

Completion Goal criteria observed

### Threshold

all 5 criteria

### Scenario

Extend a mission inside the window, before it, and after the limit; open the inbox inside and outside the window; try from an agent lane.

### Environment

go test of internal/cli with a fake GitHub, a fake ledger, a fake clock, and a fake device flow; never production

### Checkpoints

1. Extend works inside the window and after the limit (criteria 1, 2, 3, 5)
2. The inbox shows the window in advance (criteria 4)

### Technical Owner

engineering-manager

### Assumptions

#### Notes

The window is a new `[mission] extend_window_hours` key (default 6) in `internal/config/defaults.toml`, a protected path, with a row in `internal/config/gen`. An early extend adds hours from the limit, never from now, so it gives no extra time.

#### Review guidance

- Check: criterion 1: the new limit counts from the old limit.
- Check: criterion 5: an agent lane never extends.
- Ignore: the wording of the card.
- Risk: an extend that runs a mission longer than one approved window; each extend still needs a person's code and reason.

#### Behaviours

1. says: A person can extend a mission when they check in before its limit; rule: limit; source: criteria 1, 4
2. says: Only a person extends a mission; rule: never; source: criteria 5

### Provenance

Request: "I can only check in every 4 hrs so we need to ensure we can move the journey ahead"
1. stated (request): extend at the check-in before the limit
2. default (scope): too early is refused with the time
3. default (scope): after the limit as today
4. default (scope): the inbox shows the window
5. default (scope): agent lanes refused
````
