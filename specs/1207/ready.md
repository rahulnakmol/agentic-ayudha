# rahulnakmol/agentic-ayudha#1207: The mission starts a story that already has an open Refs PR built outside it

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `72ceb3a398101e0227852331c25b378556d6955fc433800f026873bed1903006` |
| Completion Goal hash | `9b6bb24339920613b21c44e2b0a5812d8905b69e1b1349db2210c0bbd0fa510b` |
| Technical Owner | engineering-manager |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1207/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: a Ready story of a started mission has an open PR whose body says `Refs #n`, built outside the mission, and no delegation; expected: `aayu mission next <f>` lists it as kept on that PR, not as a start, and writes no claim line
2. [api-trace] state: the same story, `aayu mission recover <f>`; expected: it writes `delegate/<n>/state.json` once, with that PR, status pr-open, and the mission's tool as the receiver; a second run writes nothing
3. [api-trace] state: the adopted story, `aayu build <n> --delegate <tool>`; expected: it syncs that PR (its head and inspect's disposition) and starts no new hand-off, branch, or subthread

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

The mission starts a story that already has an open Refs PR built outside it. `aayu mission next` holds a story only for a status label (`status:in-progress` or `status:validation`), a live lease, a claim line, a stop, recover's record, or a locked body that changed (`internal/cli/missionnext.go`, `listNext`). It never reads open PRs. `aayu mission recover` writes an open PR (`Refs #n`) into `delegate/<n>/state.json`, but that state has no receiver. So `aayu build <n> --delegate <tool>` sees a fresh hand-off and starts a new subthread (`internal/cli/delegate.go`, `d.fresh`). Seen on Bug #1191: its PR #1199 was open when the Kanha of #1170 claimed #1191 at 14:25Z on 2026-10-31. The start failed only because Amp had no `AMP_API_KEY`.

### Steps to reproduce

1. Bug #1191 is Ready and in the mission of #1170, and PR #1199 (`Refs #1191`) is open from a branch built outside the mission.
2. @rahulnakmol starts the mission of #1170.
3. Its Kanha runs `aayu mission next 1170`, then `aayu build 1191 --delegate amp`.

### Expected behaviour

`aayu mission next` keeps #1191 on PR #1199 and starts nothing for it. Recover and build adopt PR #1199 as #1191's delegation, and a later build syncs it.

### Actual behaviour

`aayu mission next` lists #1191 as a start and claims it. `aayu build 1191 --delegate amp` starts a fresh hand-off, so a second agent builds #1191 while PR #1199 is open.

### Completion Goal

1. [api-trace] state: a Ready story of a started mission has an open PR whose body says `Refs #n`, built outside the mission, and no delegation; expected: `aayu mission next <f>` lists it as kept on that PR, not as a start, and writes no claim line
2. [api-trace] state: the same story, `aayu mission recover <f>`; expected: it writes `delegate/<n>/state.json` once, with that PR, status pr-open, and the mission's tool as the receiver; a second run writes nothing
3. [api-trace] state: the adopted story, `aayu build <n> --delegate <tool>`; expected: it syncs that PR (its head and inspect's disposition) and starts no new hand-off, branch, or subthread

### Measure

Completion Goal criteria observed

### Threshold

all 3 criteria

### Scenario

Start a mission whose story has an open Refs PR built outside it; run next, recover, and build --delegate.

### Environment

go test of internal/cli with a fake GitHub, a fake ledger, and a fake cloud agent; never production

### Checkpoints

1. next and recover keep and adopt the outside PR (criteria 1, 2)
2. build syncs the adopted PR (criteria 3)

### Technical Owner

engineering-manager

### Links

### Assumptions

#### Notes

A PR is the story's when its body's `Refs #n` names it, or its branch names it, as `StoryOf` reads it. Until this is fixed, a person holds such a story with `aayu mission stop <f> --story <n> --reason "…"` and resumes it before ship.

#### Review guidance

- Check: criterion 1: next reads open PRs once per run, not once per story.
- Check: criterion 3: the adopted delegation names the mission's tool, so build syncs instead of a fresh hand-off.
- Risk: a PR that names the story by mistake holds it; the list shows the PR, so a person sees it.

#### Behaviours

1. says: A story with an open Refs PR is never started a second time; rule: never; source: criteria 1, 3
2. says: Recover records that PR as the story's delegation, once; rule: always; source: criteria 2

### Provenance

Request: "the mission starts a story that already has an open Refs PR built outside it"
1. stated (request): next and recover adopt that PR as the story's delegation
2. stated (request): next and recover adopt that PR as the story's delegation
3. stated (request): next and recover adopt that PR as the story's delegation
````
