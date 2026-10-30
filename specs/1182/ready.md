# rahulnakmol/agentic-ayudha#1182: Approve to build and mission start refuse a person in an orb Terminal instead of asking for the device code

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `4015f7940eeef9a9c81b0f67ee50bc28bef4f977e8dfac3334be4e1a18b22eee` |
| Completion Goal hash | `12370eecd9c1d4941d865dfac56591e4c3b66d1d734fa231d41199a09e9a0770` |
| Technical Owner | engineering-manager |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1182/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: AMP_THREAD_ID is set, no AAYU_AGENT and no --ci, and the person runs `aayu ready 874 --agents 10`; expected: it asks for the device code, and after the person's code it writes specs/874/approved.json as in a plain terminal
2. [api-trace] state: the same shell, `aayu mission start 874`; expected: it asks for the device code, and after it starts the mission
3. [api-trace] state: AAYU_AGENT=1 is set; expected: `aayu ready 874 --agents 10` is refused before any read, as today
4. [api-trace] state: `--ci`; expected: refused before any read, as today
5. [api-trace] state: AMP_THREAD_ID is set and the device code expires; expected: nothing is written, and it says the code expired
6. [api-trace] state: AMP_THREAD_ID is set and the code is entered by an account other than the one aayu acts as; expected: nothing is written, as presenceCheck refuses it today

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

Approve to build and mission start refuse a person in an orb's Terminal tab, while a baseline approval in the same shell goes through GitHub's device flow. `approveRefusal` and `laneRefusal` (`internal/cli/readyapprove.go:42-60`) refuse any shell with a host agent marker before any device flow. An orb's Terminal tab inherits `AMP_THREAD_ID`. `aayu ready <f> --part` refuses only `AAYU_AGENT=1` and CI, and asks for the device code otherwise (`runReadyPart`, `internal/cli/ready.go:274`). `aayu ship --solo-bypass` and `aayu inbox` do the same (`presenceCheck`, `internal/cli/presence.go:136`). AGENTS.md says a human-only command in a shell with an agent marker runs "only after the person approves them through GitHub's device flow". On 2026-10-30, @rahulnakmol approved #874's three baselines in his orb Terminal, and `aayu ready 874 --agents 10` then refused: "is refused for an agent lane (AMP_THREAD_ID)". The same refusal reaches 10 call sites: Approve to build, mission start, stop, resume, extend, gate --once-more, budget, and recover.

### Steps to reproduce

1. In an orb's Terminal tab (AMP_THREAD_ID set), run `aayu ready 874 --part intent`, and enter the device code.
2. In the same shell, run `aayu ready 874 --agents 10`.

### Expected behaviour

Step 2 asks for GitHub's device code, as step 1 did, and approves after the person enters it.

### Actual behaviour

Step 2 exits 4 with `Approve to build (aayu ready on a Feature) is a person's decision and is refused for an agent lane (AMP_THREAD_ID)`, and asks for no code.

### Completion Goal

1. [api-trace] state: AMP_THREAD_ID is set, no AAYU_AGENT and no --ci, and the person runs `aayu ready 874 --agents 10`; expected: it asks for the device code, and after the person's code it writes specs/874/approved.json as in a plain terminal
2. [api-trace] state: the same shell, `aayu mission start 874`; expected: it asks for the device code, and after it starts the mission
3. [api-trace] state: AAYU_AGENT=1 is set; expected: `aayu ready 874 --agents 10` is refused before any read, as today
4. [api-trace] state: `--ci`; expected: refused before any read, as today
5. [api-trace] state: AMP_THREAD_ID is set and the device code expires; expected: nothing is written, and it says the code expired
6. [api-trace] state: AMP_THREAD_ID is set and the code is entered by an account other than the one aayu acts as; expected: nothing is written, as presenceCheck refuses it today

### Measure

Completion Goal criteria observed

### Threshold

all 6 criteria

### Scenario

Run Approve to build and mission start with a host marker and a device code, then with AAYU_AGENT=1 and --ci, then with an expired code and a wrong account.

### Environment

go test of internal/cli with a fake GitHub, a fake ledger, and a fake device flow; never production

### Checkpoints

1. With only a host marker, Approve to build and every mission step ask for the device code and act after it (criteria 1, 2)
2. aayu's own agent lanes and CI are still refused before any read (criteria 3, 4)
3. A code that expires or comes from another account writes nothing (criteria 5, 6)

### Technical Owner

engineering-manager

### Assumptions

#### Notes

`laneRefusal` keeps refusing `AAYU_AGENT=1` and `--ci` before any read (#1032). A host marker alone goes to `presenceCheck`, whose device flow already proves the person: an agent can start the flow but never completes it, because it never enters or relays the code. This changes the path to an approval, not who may approve, so it is an authority change under Decision #1027 and waits for a person's look at the PR.

#### Review guidance

- Check: criteria 3 and 4: the refusal for aayu's own lanes and CI still comes before any read.
- Check: every call site of `laneRefusal` takes the same path; no site skips the device code.
- Ignore: the wording of the code prompt.
- Risk: a host that sets no marker and has no terminal; `presenceCheck`'s refusal covers it today.

#### Behaviours

1. says: A person in a shell with a host marker approves to build and starts a mission with the device code; rule: limit; source: criteria 1, 2
2. says: aayu's own agent lanes and CI never approve to build or start a mission; rule: never; source: criteria 3, 4
3. says: No approval is written without the acting person's device code; rule: never; source: criteria 5, 6

### Provenance

Request: "aayu ready 874 --agents 10 is refused for an agent lane (AMP_THREAD_ID)"
1. stated (request): Approve to build runs in the founder's orb Terminal with his code
2. default (scope): mission start has the same refusal and the same fix
3. default (scope): aayu's own lanes stay refused, as #1032 decided
4. default (scope): CI stays refused
5. default (scope): an expired code writes nothing, as presenceCheck does
6. default (scope): another account's code writes nothing, as presenceCheck does
````
