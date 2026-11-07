# UX design: Feature #875

Nothing here is built. The surface is planned as terminal commands, one comment per review on the Feature, and the
agent's review session. The Feature's footprint has no L01 (`specs/875/design.json`: L02, L03, L07), so it has no UX
baseline.

## Commands (planned, not built)

| Command | Who | What it does | Source |
|---|---|---|---|
| `aayu critique <f> --part <p> --seat <s> --draft findings.json` | a reviewing seat's session | Records one review of the newest approved version and posts one comment | #920 CG 1 |
| `aayu critique <f> --part <p> --seat <s> --session local` | a detached process that aayu starts | Runs the local agent CLI, then records its review | #921 CG 1; #923 CG 1 |
| `aayu ready <f> --part <p> [--cloud]` | a person | The delivered approval (#990) also starts the seat reviews; `--cloud` sends them to the tool's cloud | #921 CG 1, 4 |
| `aayu critique --sweep` | the `baseline-reviews` loop | Starts missed reviews and closes windows; no output when nothing is missed | #922 CG 1-6 |
| `aayu critique <f> [--part <p>] --status [--json]` | anyone | One row per reviewer: seat, state, attempt, thread link, findings by kind | #922 CG 5, 8 |

## Messages (planned, quoted from the Completion Goals)

| Moment | Text | Source |
|---|---|---|
| Review recorded | "Yama reviewed spec v2: 1 gap, 1 input, 1 guidance" (comment on the Feature, with a link to the record) | #920 CG 1 |
| Drafting session | "the session that drafted spec v2 may not review it" | #920 CG 2 |
| Owner seat | "Arjun owns spec and may not review it" | #920 CG 3 |
| Second review | "Yama already reviewed spec v2 at &lt;time&gt;" | #920 CG 5 |
| One seat per session | "this session already reviewed spec v2 as Yama" | #920 CG 6 |
| Too many findings | "16 findings; keep the 15 that matter most" | #920 CG 9 |
| A hand-off failed | "5 of 6 reviews started; Nala's waits for the next tick" | #921 CG 5 |
| Tool missing | "not dispatched: the &lt;tool&gt; CLI is not on this machine" | #921 CG 6 |
| Held seat | "held: waits for @alice's codex"; after attempts: "missing after 2 attempts" | #922 CG 1, 2 |
| Config | "claude is not your agent tool (amp); run setup again to change tools" | #919 CG 6 |

## The reviewing agent's experience

- One brief per seat: the twin's name and focus (for example Shesha: operability, rollback), each pinned
  artifact quoted as untrusted data with its hash, the finding schema, and the rule to write only `findings.json`
  and change no file, issue, or pull request (#921 CG 2).
- A v2 brief names the pins that changed from v1 and asks for a review of that change only (#921 CG 3).
- The mentalist's focus is the Feature defense: coverage, estimates, optimism, depth, and defend, quoted in
  [#919](https://github.com/rahulnakmol/agentic-ayudha/issues/919) CG 7.
- A cloud thread on Amp is titled "aayu #840 tech v1: Shesha reviews" (#921 CG 4).

## Person's experience

The person approves a part as today (`aayu ready <f> --part <p>`, delivered in #990). Under this Feature, the same
run says how many reviews started and why any did not (#921 CG 5-8). Approve to build then waits for the reviews
or for `[critique] review_wait_hours`. That refusal text is delivered already: "Approve to build is off: 3 of 5
reviews are not in; Approve to build opens when they are in or [critique] review_wait_hours have passed"
(`approveRun.prepare` in `internal/cli/readyapprove.go`). It cannot fire until #875 records reviews.

Accessibility: no source states a requirement beyond plain terminal text.
