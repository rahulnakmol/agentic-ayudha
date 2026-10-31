# rahulnakmol/agentic-ayudha#1208: fix(delegate): an Amp mission needs the amp CLI signed in to start a story

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `f8714f60638606522d0168c284631a31f934dfc96a975bc6c6e747f771552365` |
| Completion Goal hash | `b2b293a821d4c9f5c05086bd3ed88bb5ecf1891a77f8e2b9222d551f188a6126` |
| Technical Owner | engineering-manager |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1208/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: AMP_THREAD_ID is set, the amp CLI is not signed in, and #1172 is marked start; expected: `aayu build 1172 --delegate amp` claims #1172, writes its brief, and prints a host hand-off: the brief path, the thread title, and the command that records the thread, and starts no CLI
2. [api-trace] state: the Kanha created the thread T-1 with its own thread tool and runs `aayu build 1172 --delegate amp --thread T-1`; expected: the delegation state records T-1 and its URL as the CLI path does, and `aayu mission show 1170` lists #1172 running on that thread
3. [api-trace] state: the claim is older than the claim lease and no thread was recorded; expected: the claim is released and #1172 is listed start again
4. [api-trace] state: the amp CLI is signed in; expected: `aayu build 1172 --delegate amp` starts the thread through the CLI, as today
5. [api-trace] state: `--thread` names a thread ID that is not an Amp thread ID; expected: it is refused and nothing is recorded
6. [api-trace] state: a follow-up for #1172 and the CLI is not signed in; expected: the follow-up is printed as a host hand-off to send with the thread tool, not dropped

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

A mission on Amp cannot start a story from its own Kanha thread unless the Amp CLI is signed in on the machine. `aayu build <n> --delegate amp` starts the story's thread only through the CLI (`AmpCLI.Handoff` runs `amp -ox … --project`, internal/delegate/receivers.go:261), which needs a saved account or AMP_API_KEY. The Kanha of #1170 runs in an Amp orb thread (T-01a18d24) that can create threads itself, with no CLI login. On 2026-10-31, after @rahulnakmol started #1170 at 14:24 UTC, every start failed with "Amp's cloud agent is not connected here … not signed in": #1172, #1191, #1192 and #1201 each went back to status:ready (#1201 stayed status:in-progress after a start was stopped). The founder said: "This is exactly this we need to fix, doesn't need a cli dependency".

### Steps to reproduce

1. Start a mission on amp with `aayu mission start 1170`; its Kanha runs in an Amp orb thread whose `amp` CLI is not signed in.
2. The Kanha runs `aayu mission next 1170`, which marks #1172 start.
3. The Kanha runs `aayu build 1172 --delegate amp`.

### Expected behaviour

The story's own Amp thread starts, with no CLI sign-in, and the delegation is recorded as with the CLI.

### Actual behaviour

The start fails with "Sign in: amp login", and the story goes back to status:ready.

### Completion Goal

1. [api-trace] state: AMP_THREAD_ID is set, the amp CLI is not signed in, and #1172 is marked start; expected: `aayu build 1172 --delegate amp` claims #1172, writes its brief, and prints a host hand-off: the brief path, the thread title, and the command that records the thread, and starts no CLI
2. [api-trace] state: the Kanha created the thread T-1 with its own thread tool and runs `aayu build 1172 --delegate amp --thread T-1`; expected: the delegation state records T-1 and its URL as the CLI path does, and `aayu mission show 1170` lists #1172 running on that thread
3. [api-trace] state: the claim is older than the claim lease and no thread was recorded; expected: the claim is released and #1172 is listed start again
4. [api-trace] state: the amp CLI is signed in; expected: `aayu build 1172 --delegate amp` starts the thread through the CLI, as today
5. [api-trace] state: `--thread` names a thread ID that is not an Amp thread ID; expected: it is refused and nothing is recorded
6. [api-trace] state: a follow-up for #1172 and the CLI is not signed in; expected: the follow-up is printed as a host hand-off to send with the thread tool, not dropped

### Measure

Completion Goal criteria observed

### Threshold

all 6 criteria

### Scenario

Start a story from an Amp Kanha thread with no CLI login, record the thread, let a claim lapse, repeat with the CLI signed in, give a bad thread ID, and send a follow-up.

### Environment

go test of internal/cli and internal/delegate with a fake GitHub, a fake ledger, and a fake amp CLI; never production

### Checkpoints

1. With no CLI login, build hands the start to the host and records the thread the host made (criteria 1, 2, 5)
2. A hand-off with no thread releases its claim on time (criteria 3)
3. A signed-in CLI and follow-ups work as before (criteria 4, 6)

### Technical Owner

engineering-manager

### Assumptions

#### Notes

The Kanha brief (`KanhaBrief`, internal/delegate/brief.go:337) gains one step for Amp: on a host hand-off, create the story's thread with the thread tool, with the brief as its prompt, then record it. aayu never calls an Amp API with the person's credentials; the Amp thread does the creating. The PR found by `Refs #n` stays the signal of progress, as with the CLI.

#### Review guidance

- Check: criterion 3: an unrecorded hand-off never holds its claim past the lease.
- Check: criterion 5: only an Amp thread ID is recorded.
- Ignore: the wording of the hand-off lines.
- Risk: two Kanha events creating two threads for one story; the claim is taken before the hand-off, and the second build sees it.

#### Behaviours

1. says: A mission on Amp starts its stories from its Kanha thread with no CLI sign-in; rule: limit; source: criteria 1, 2
2. says: A hand-off nobody completes never holds a story; rule: never; source: criteria 3

### Provenance

Request: "This is exactly this g we need to fix doesn’t need a cli dependency"
1. stated (request): no CLI dependency to start a story
2. stated (request): the thread the host creates is the story's thread
3. default (scope): a lapsed hand-off releases its claim
4. default (scope): the CLI path stays
5. default (scope): only an Amp thread ID is recorded
6. default (scope): follow-ups have the same path
````
