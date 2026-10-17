# rahulnakmol/agentic-ayudha#481: Approve agents' human-only steps as one batch: by a comment from a separate approver identity, with one device code as the fallback and for one-way doors

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `b09ae4e906c0534bbe9b18e8c275b11a5ed1edda1f3882400ae106b36eac13a4` |
| Completion Goal hash | `de269f76734c0edba7d6fe50228c808ea83f89ae692b96c706ee904e90ba6dca` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/481/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: no `[identity] device_client_id`, no `AAYUDHA_DEVICE_CLIENT_ID` or `AAYUDHA_APP_CLIENT_ID` repository variable, and an agent-marked shell runs a human-only command; expected: the device flow starts with aayu's built-in client ID, a configured client ID replaces it, and the terminal prints the exact command and what approving does next to the code
2. [api-trace] state: comment approvals are off (no bot identity), and an agent runs `aayu decide <a>,<b>,<c> --accept 1,1,2`; expected: before any Decision is written, aayu posts one batch comment on #<a> that numbers the three Decisions, gives each its effect in plain words, its door, and its options and recommendation, shows the batch hash, says why comment approval is off, and gives one device code with https://github.com/login/device and the expiry; one approval accepts all three; the `presence.approve` record lists all three; each Decision gets the reply `#<n> accepted: option <o>, by @<login> through the device code`; and a second aayu run asks for a new code
3. [api-trace] state: the bot identity and `[approvals] approvers` are set, the session reaches no approver credential, and every Decision in the batch is reversible; expected: aayu posts the batch as the bot with the `/aayu approve all`, `/aayu approve 1 3`, and `/aayu skip 2` replies explained, and exits 4 with nothing accepted; `aayu inbox --ci` on the approver's `/aayu approve 1 3` reply then accepts exactly items 1 and 3 as the bot, records `accepted by @<login> in <comment URL>`, and replies with each result
4. [api-trace] state: the reply comes from a bot, from a login that is neither an approver nor the Decision's authority, from an account without write access, or is edited, older than the batch, aimed at an edited batch or one the bot did not write, or names a Decision whose body changed since the batch; expected: nothing is accepted, and aayu replies with what happened and the one next step
5. [api-trace] state: a batch holds an irreversible Decision or a floor waiver under the default `[approvals] device_for = ["one-way"]`, or the agent session can reach an approver's credential; expected: the batch asks for the device code instead of a comment and says why, and `aayu doctor` in that session fails its approvals row, naming the credential's source and login but never its value
6. [api-trace] state: the batch's device code expires, the person cancels it, another account approves it, or the approver lacks write access; expected: exit 4, no Decision written, and the batch comment replaced with what happened and the one next step
7. [api-trace] state: an agent runs `aayu release <n> --to main` with no `Stable release:` comment, and the person approves through the device code; expected: the promotion record and the shipping evidence name the approving person and the device flow, as well as the agent marker the command ran under

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

Decision #488 (option 1, accepted by @rahulnakmol on 2026-10-17) replaces this Story's first plan. One device code per command is cumbersome, and a person's comment should count as an approval once no agent can write as that person. This Story builds the first two steps of option 1.

- **One batch.** aayu groups the human-only Decision acceptances an agent starts together into one numbered comment. Each item says in plain words what accepting does and whether it is a two-way or a one-way door, and the batch shows its hash.
- **Approval by comment.** Where the bot identity and the recorded approvers are in place and every item is a two-way door, the approver replies `/aayu approve all`, `/aayu approve 1 3`, or `/aayu skip 2`. A default-branch workflow verifies that reply and applies it as the bot.
- **Device code as the fallback.** Everywhere else, including one-way doors by default (`[approvals] device_for = ["one-way"]`), one device code covers the whole batch.
- **Results and failures.** aayu replies with each result or with what went wrong and the one next step.
- **Exposure check.** `doctor` fails when an agent session can reach an approver's GitHub credential, and such a session never offers comment approval.
- **Carried over from the first plan.** The device code needs no setup, through a built-in client ID. The terminal shows the command next to the code. A stable release names the person who approved it.
- **Later steps.** The approver App for one-way doors (#488, step 3) and comment approvals for steps other than Decisions are later Stories.

### Completion Goal

1. [api-trace] state: no `[identity] device_client_id`, no `AAYUDHA_DEVICE_CLIENT_ID` or `AAYUDHA_APP_CLIENT_ID` repository variable, and an agent-marked shell runs a human-only command; expected: the device flow starts with aayu's built-in client ID, a configured client ID replaces it, and the terminal prints the exact command and what approving does next to the code
2. [api-trace] state: comment approvals are off (no bot identity), and an agent runs `aayu decide <a>,<b>,<c> --accept 1,1,2`; expected: before any Decision is written, aayu posts one batch comment on #<a> that numbers the three Decisions, gives each its effect in plain words, its door, and its options and recommendation, shows the batch hash, says why comment approval is off, and gives one device code with https://github.com/login/device and the expiry; one approval accepts all three; the `presence.approve` record lists all three; each Decision gets the reply `#<n> accepted: option <o>, by @<login> through the device code`; and a second aayu run asks for a new code
3. [api-trace] state: the bot identity and `[approvals] approvers` are set, the session reaches no approver credential, and every Decision in the batch is reversible; expected: aayu posts the batch as the bot with the `/aayu approve all`, `/aayu approve 1 3`, and `/aayu skip 2` replies explained, and exits 4 with nothing accepted; `aayu inbox --ci` on the approver's `/aayu approve 1 3` reply then accepts exactly items 1 and 3 as the bot, records `accepted by @<login> in <comment URL>`, and replies with each result
4. [api-trace] state: the reply comes from a bot, from a login that is neither an approver nor the Decision's authority, from an account without write access, or is edited, older than the batch, aimed at an edited batch or one the bot did not write, or names a Decision whose body changed since the batch; expected: nothing is accepted, and aayu replies with what happened and the one next step
5. [api-trace] state: a batch holds an irreversible Decision or a floor waiver under the default `[approvals] device_for = ["one-way"]`, or the agent session can reach an approver's credential; expected: the batch asks for the device code instead of a comment and says why, and `aayu doctor` in that session fails its approvals row, naming the credential's source and login but never its value
6. [api-trace] state: the batch's device code expires, the person cancels it, another account approves it, or the approver lacks write access; expected: exit 4, no Decision written, and the batch comment replaced with what happened and the one next step
7. [api-trace] state: an agent runs `aayu release <n> --to main` with no `Stable release:` comment, and the person approves through the device code; expected: the promotion record and the shipping evidence name the approving person and the device flow, as well as the agent marker the command ran under

### Measure

Completion Goal criteria observed.

### Threshold

All 7 criteria.

### Scenario

1. Run `go test ./internal/cli -run 'TestPresence|TestApprov|TestDecide'`. The fake GitHub serves the Decisions, the repository variables, the device-flow endpoints, the comments with their edit times, and the collaborator permissions. Agent runs set an agent marker and have no terminal; the `inbox --ci` run reads a recorded `issue_comment` event.
2. Run `go test ./internal/doctor -run TestApprovals` for the exposure check against stand-in credentials.
3. Run `go test ./internal/cli -run TestReleaseLadder` for the stable release approved through the device code.

### Environment

A checkout of this repository on a branch from origin/dev, with Go and git; the fake GitHub server and scratch git repositories in the tests' temporary directories. No network, no real GitHub App or OAuth App, no production data.

### Checkpoints

1. The built-in client ID and the command next to the code, and the release record (criteria 1, 7)
2. One batch with one device code, the replies, and the failure states (criteria 2, 6)
3. Comment approvals through `inbox --ci` and the approval workflow, and every refusal (criteria 3, 4)
4. One-way doors and the exposure check in `doctor` (criterion 5)

### Provenance

Request: "entering a device code for every decision is cumbersome. Approvals must work flawlessly for agents running in the cloud. They must surface as a group. And a formatted PR or issue comment from the founder or user should count as an approval. Group the approvals: one comment on an issue or PR, numbered, each in plain words with what it does and whether it is easy to reverse, plus a hash of the batch. Approve with a formatted comment such as "/aayu approve all", "/aayu approve 1 3", "/aayu skip 2"; a workflow verifies the author is a recorded human approver and not a bot, the comment is unedited, it came after the batch, and the hash matches, then applies the steps as the bot and records "approved by @x in <comment URL>". Fall back where separation is not in place: comment approvals are refused, with a plain reason, and aayu uses device flow, one code per batch. One-way doors (stable release and the like) always need the device code, even when comment approvals are on. Make [approvals] device_for = ["one-way"] the default. doctor fails, or at least warns, when an agent session holds a human token. The reply-back confirmation and the clear failure states still apply. Ship a default client ID with AAYUDHA_DEVICE_CLIENT_ID (or config) as an override. Show the pending command next to the code in the terminal, and let release record the approving person rather than only agent=<marker> when a device approval happened."
Vague: "flawlessly" → criteria 2, 3, 4, 6
Vague: "easy" → criteria 2, 5
Vague: "stable" → criteria 5, 7
Term: "one-way door" → an irreversible step: a Decision with Reversible false or a floor waiver, a stable release, a solo bypass; "One-way doors (stable release and the like)"
1. stated (request): "Ship a default client ID with AAYUDHA_DEVICE_CLIENT_ID (or config) as an override" / "Show the pending command next to the code in the terminal"
2. stated (request): "Group the approvals: one comment on an issue or PR, numbered, each in plain words with what it does and whether it is easy to reverse, plus a hash of the batch" / "aayu uses device flow, one code per batch" / "The reply-back confirmation and the clear failure states still apply"
3. stated (request): "Approve with a formatted comment such as \"/aayu approve all\", \"/aayu approve 1 3\", \"/aayu skip 2\"" / "then applies the steps as the bot and records \"approved by @x in <comment URL>\""
4. stated (request): "a workflow verifies the author is a recorded human approver and not a bot, the comment is unedited, it came after the batch, and the hash matches"
5. stated (request): "One-way doors (stable release and the like) always need the device code, even when comment approvals are on. Make [approvals] device_for = [\"one-way\"] the default. doctor fails, or at least warns, when an agent session holds a human token"
6. stated (request): "The reply-back confirmation and the clear failure states still apply"
7. stated (request): "let release record the approving person rather than only agent=<marker> when a device approval happened"

### Assumptions

- Comment approvals here cover accepting Decisions, the human-only step that took four codes on 2026-10-17. Other two-way steps and the approver App for one-way doors (#488, step 3) are later Stories.
- A batch is `aayu decide <a>,<b>,... --accept <o1>,<o2>,...`: one option per Decision, in order, at most 10 Decisions. One batch can be open per thread, so an approver never types the hash.
- Comment approval is on only when the bot identity is configured and `[approvals] approvers` names at least one login. The approval workflow is `aayudha-approve.yml`, triggered by `issue_comment` and installed by `aayu init --gates`. It never runs on `pull_request_review`, because that event runs the PR's own workflow file.
- The built-in client ID belongs to the founder's OAuth App "aayudha presence check", which has Device Flow enabled. Its owner never sees a token. GitHub accepts at most 50 code submissions an hour for each app.
- In solo mode, without a bot identity, the batch comment is written with the person's own token, and GitHub does not notify a person of their own comment.
````
