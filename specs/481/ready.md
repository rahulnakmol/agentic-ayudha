# rahulnakmol/agentic-ayudha#481: Decide from the phone: every aayu user approves a human-only command through a notice on GitHub, Slack, or email, one code per batch

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `c19ed078ca9a05cca086e51a7fa0b678856508fb66e7943618abf5d624d55bff` |
| Completion Goal hash | `e5b7006836155ef92b8836465b7ce97ecd1c131bee5cc4c330285a2501add3cf` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/481/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: a checkout with no `[identity] device_client_id`, no `AAYUDHA_DEVICE_CLIENT_ID` or `AAYUDHA_APP_CLIENT_ID` repository variable, and an agent-marked shell runs `aayu decide <n> --accept <option>`; expected: the device flow starts with aayu's built-in client ID, and a configured `[identity] device_client_id` or repository variable replaces it
2. [api-trace] state: an agent runs `aayu decide <n> --accept 1` with no terminal on a Decision-ready Decision; expected: before any write to the Decision, aayu posts one comment on it that mentions the acting person and shows the code on a line of its own, https://github.com/login/device, the expiry time, one or two plain lines on what approving does (the option, the Decision, its title, a one-way or two-way door), the Decision's options and recommendation, and the exact command; a Slack or email `[notify]` channel gets the same notice; and the terminal prints the command next to the code
3. [api-trace] state: the acting person approves the code; expected: the Decision is accepted, its Decision section says the approval came through GitHub's device flow, and aayu replies on the Decision `#<n> accepted: option 1, by @<login> through the device code`
4. [api-trace] state: the code expires, the person cancels it on GitHub, a different account approves it, or the approver lacks write access; expected: exit 4 with nothing written to the Decision, and the notice comment is replaced with what happened and the one next step
5. [api-trace] state: an agent runs `aayu decide <a>,<b>,<c> --accept 1,1,2`; expected: one code covers the three; the notice and the `presence.approve` record list all three with their options; each Decision gets its own result reply; a batch with a Decision that is not Decision-ready or an option that does not match is refused before any code is issued; and a second aayu run after the approval asks for a new code
6. [api-trace] state: an agent runs `aayu release <n> --to main` with no `Stable release:` comment on the Release, and the person approves through the device code; expected: the promotion record and the shipping evidence name the approving person and the device flow, as well as the agent marker the command ran under

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

#467 lets a person approve a human-only command from any device with GitHub's device flow, but every repository must first create its own OAuth App, the code shows only in the terminal that ran the command, and four Decisions take four codes. This Story makes it work for every aayu user from the phone alone. The person asks an agent, in chat or in a GitHub comment that names the command. The agent starts the command, and aayu sends the notice to where the person already is: a comment on the Decision or issue, which reaches GitHub mobile, and Slack or email when `[notify]` has them. The notice carries the code, the link, the expiry, what approving does, and for a Decision its options and recommendation. One code can approve a batch of Decisions started together. aayu replies with the result or with what went wrong and the one next step. An agent can start a human-only command but never complete it: the approval is still the person's own GitHub sign-in on GitHub's device page.

### Completion Goal

1. [api-trace] state: a checkout with no `[identity] device_client_id`, no `AAYUDHA_DEVICE_CLIENT_ID` or `AAYUDHA_APP_CLIENT_ID` repository variable, and an agent-marked shell runs `aayu decide <n> --accept <option>`; expected: the device flow starts with aayu's built-in client ID, and a configured `[identity] device_client_id` or repository variable replaces it
2. [api-trace] state: an agent runs `aayu decide <n> --accept 1` with no terminal on a Decision-ready Decision; expected: before any write to the Decision, aayu posts one comment on it that mentions the acting person and shows the code on a line of its own, https://github.com/login/device, the expiry time, one or two plain lines on what approving does (the option, the Decision, its title, a one-way or two-way door), the Decision's options and recommendation, and the exact command; a Slack or email `[notify]` channel gets the same notice; and the terminal prints the command next to the code
3. [api-trace] state: the acting person approves the code; expected: the Decision is accepted, its Decision section says the approval came through GitHub's device flow, and aayu replies on the Decision `#<n> accepted: option 1, by @<login> through the device code`
4. [api-trace] state: the code expires, the person cancels it on GitHub, a different account approves it, or the approver lacks write access; expected: exit 4 with nothing written to the Decision, and the notice comment is replaced with what happened and the one next step
5. [api-trace] state: an agent runs `aayu decide <a>,<b>,<c> --accept 1,1,2`; expected: one code covers the three; the notice and the `presence.approve` record list all three with their options; each Decision gets its own result reply; a batch with a Decision that is not Decision-ready or an option that does not match is refused before any code is issued; and a second aayu run after the approval asks for a new code
6. [api-trace] state: an agent runs `aayu release <n> --to main` with no `Stable release:` comment on the Release, and the person approves through the device code; expected: the promotion record and the shipping evidence name the approving person and the device flow, as well as the agent marker the command ran under

### Measure

Completion Goal criteria observed.

### Threshold

All 6 criteria.

### Scenario

1. Run `go test ./internal/cli -run 'TestPresence|TestDecide'`: the fake GitHub serves the Decisions, the repository variables, its device-flow endpoints, and the comments; the agent runs set an agent marker and have no terminal; a fake `[notify]` sink records pages.
2. Run `go test ./internal/cli -run TestReleaseLadder` for the stable release approved through the device code.

### Environment

A checkout of this repository on a branch from origin/dev, with Go and git; the fake GitHub server and scratch git repositories in the tests' temporary directories. No network, no real OAuth App, no production data.

### Checkpoints

1. Zero setup: the built-in client ID, overridable per repository (criterion 1)
2. The notice is the phone screen: the comment and pages, the result reply, and the failure states (criteria 2, 3, 4)
3. One code for a batch of Decisions, and release names the person who approved (criteria 5, 6)

### Provenance

Request: "shouldn't we do this for any user using aayudha also, so the founder can take decisions in Slack, the GitHub app and so on. The person never needs a terminal: they ask, the agent starts the human-only command, and the person only approves on the phone; an agent can start the command but never complete it. The notice is the phone screen: a GitHub comment on the Decision or issue (and Slack or email if configured) with the code in a form that's easy to copy, the github.com/login/device link, the expiry, and what approving does in one or two plain lines; for a Decision, the options and the recommendation. Fewer codes: let one approval cover a batch the person started together ("accept 324, 283, 469"), with every item listed in the notice and in the record. Confirmation back to the phone: after the command runs, reply on the same thread with the result. Clear failure states: when the code has expired, was denied, or the person lacks the permission, the notice says so and gives the one next step. Zero setup for every aayu user: device flow needs only a public client ID, so ship a default client ID for an "aayudha" OAuth app owned by the project, with AAYUDHA_DEVICE_CLIENT_ID (or config) as an override for teams that want their own app. The approval itself is still the device code, entered on any device. Show the pending command next to the code in the terminal, and let release record the approving person rather than only agent=<marker> when a device approval happened."
Vague: "and so on" → criteria 2, 3
Vague: "easy" → criterion 2
Term: "device code" → the one-time user code of GitHub's OAuth device flow, entered at https://github.com/login/device; "the device code, entered on any device"
1. stated (request): "shouldn't we do this for any user using aayudha also" / "ship a default client ID for an \"aayudha\" OAuth app owned by the project, with AAYUDHA_DEVICE_CLIENT_ID (or config) as an override"
2. stated (request): "a GitHub comment on the Decision or issue (and Slack or email if configured) with the code in a form that's easy to copy, the github.com/login/device link, the expiry, and what approving does in one or two plain lines; for a Decision, the options and the recommendation" / "Show the pending command next to the code in the terminal"
3. stated (request): "after the command runs, reply on the same thread with the result"
4. stated (request): "when the code has expired, was denied, or the person lacks the permission, the notice says so and gives the one next step"
5. stated (request): "let one approval cover a batch the person started together (\"accept 324, 283, 469\"), with every item listed in the notice and in the record"
6. stated (request): "let release record the approving person rather than only agent=<marker> when a device approval happened"

### Assumptions

- The built-in client ID is that of an OAuth App owned by the project's founder with Device Flow enabled. Its owner never sees a token, since tokens go only to the aayu process that polls for them. GitHub shows the app's name on the approval page, and accepts at most 50 code submissions an hour per app, so a team with heavy use sets its own client ID.
- A batch is `aayu decide <a>,<b>,... --accept <o1>,<o2>,...`, one option per Decision in order, at most 10 Decisions. Every Decision is checked before the code is issued. The approval lives in that one process's memory and is never written where another process could read it.
- The notice goes on the batch's first Decision (or the Release issue for a stable release), written by the bot identity when there is one. In solo mode it is written with the person's own token, and GitHub does not notify a person of their own comment: Slack, email, or a bot identity brings the push.
- A comment, a reaction, a `workflow_dispatch`, an environment approval, or a Slack message made with a token an agent may hold is not proof of a person, and aayu reads none of them as an approval. Native Slack buttons need a hosted endpoint and a Slack app; they are written down as a later option, not built.
````
