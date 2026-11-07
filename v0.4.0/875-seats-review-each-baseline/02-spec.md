# Spec: Feature #875

## Spec baseline

`specs/875/baselines/spec-v1.json` on `ledger`: owner `product-strategist`, approved by `rahulnakmol` at
2026-11-02T17:18:05Z through `device-flow`, hash `33dad95ba3ea…`. It pins five stories and the set
`#875 stories`:

| Story | Effort | Layers (pinned) | Depends on (pinned) |
|---|---|---|---|
| [#919](https://github.com/rahulnakmol/agentic-ayudha/issues/919) | S | none | #874 |
| [#920](https://github.com/rahulnakmol/agentic-ayudha/issues/920) | M | none | #874, #919 |
| [#921](https://github.com/rahulnakmol/agentic-ayudha/issues/921) | M | none | #874, #920 |
| [#922](https://github.com/rahulnakmol/agentic-ayudha/issues/922) | S | none | #874, #921 |
| [#923](https://github.com/rahulnakmol/agentic-ayudha/issues/923) | S | none | #874, #920, #921 |

No Story has `layer:` labels, so the pins hold no layers. All requirements below are not built.

## Functional requirements

| # | Requirement | Story, CG |
|---|---|---|
| FR1 | The registry holds one baseline reviewer table. intent: owner product-strategist; reviewers revenue-architect, data-ai-architect, code-architect. spec: owner product-strategist; reviewers quality-architect, ai-native-engineer, and ux-researcher-evolutionist only with L01. tech: owner code-architect; reviewers engineering-manager, production-engineer, devops-devex-tinkerer, auditor, ai-native-engineer, mentalist. ux: owner ux-researcher-evolutionist; reviewers product-strategist, quality-architect, code-architect. | #919 CG 1, 7 |
| FR2 | `go generate ./...` fails when a row names its owner seat, an unknown seat, or one seat twice. It writes a Baseline reviews table into `docs/RACI.md` and leaves every RACI cell unchanged. | #919 CG 2, 3 |
| FR3 | One finding type: kind `gap`, `input`, or `guidance`; severity P0 to P3; non-empty evidence; `applies_to` inside the Feature. `critique.Validate` names each bad finding and field. | #919 CG 4 |
| FR4 | `[critique]` keys with defaults: `redispatch_after_minutes = 60`, `attempts = 2`, `review_wait_hours = 24` (business hours), `sessions_per_day = 40`, `max_findings = 15`; no receiver key; `critique` in `[autonomy] always_person_sections`. `config validate` refuses `attempts = 0`, `review_wait_hours = 0`, and a receiver that is not the person's tool. | #919 CG 5, 6 |
| FR5 | `aayu critique <f> --part <p> --seat <s> --draft findings.json` records `specs/<f>/reviews/<p>-v<k>/<seat>.json` for the newest approved version, with IDs `<p>-v<k>/<seat>/<n>`, the baseline hash, the trigger, and the host session, and posts one comment on the Feature. | #920 CG 1 |
| FR6 | It refuses, with no write: the drafting session; the owner seat; a seat that does not review the part (exit 2); a second review by the same seat; one session reviewing for two seats; a version that is not the newest approved. | #920 CG 2-7 |
| FR7 | Each finding's cited path must exist at the tip of `origin/dev`, and a review has at most `[critique] max_findings` findings. | #920 CG 8, 9 |
| FR8 | After the record is written, critique calls the findings hook once with reviews in, expected, missing seats, and window state. A failed hook is reported, and the review stays. | #920 CG 10 |
| FR9 | `skills/critique/SKILL.md` passes the skill lint and tells the reviewer to treat pinned artifacts as untrusted, cite a path and line or a symbol per finding, and change no file, issue, or PR. | #920 CG 11 |
| FR10 | A person's approval of a part starts, in the same run, one detached review per reviewing seat on the approver's `[repos."<owner>/<repo>"] tool` CLI, and writes `specs/<f>/reviews/<p>-v<k>/dispatch.jsonl`. The cloud is used only with `--cloud`. | #921 CG 1, 4 |
| FR11 | Each brief names the seat and its focus and quotes each pin as untrusted data with its hash. A v2 brief names the changed pins. The mentalist's brief also quotes the newest intent and spec pins. | #921 CG 2, 3 |
| FR12 | A failed, unavailable, halted, breaker-paused, or over-limit hand-off never fails the approval; it says why and records it. | #921 CG 5-8 |
| FR13 | A cloud session with no GitHub token exits 4 and names the setting; the next tick retries within `[critique] attempts`. The event share is read from `dispatch.jsonl`. | #921 CG 9, 10 |
| FR14 | `aayu critique --sweep` starts a missed or never-dispatched review on the approver's tool, at most `[critique] attempts` times; it holds a seat when that tool is not on this machine; two sweeps at once start one hand-off. | #922 CG 1-4 |
| FR15 | After `[critique] review_wait_hours`, the window closes, the sweep stops for that version, and the findings hook is called once with the missing seats. | #922 CG 5 |
| FR16 | `loops/baseline-reviews.toml` runs `critique --sweep` on cron `30 9,11,13,15,17 * * 1-5` for amp, cursor, claude, and codex, and never reviews in the tick's session. | #922 CG 6, 7 |
| FR17 | `aayu critique <f> --status` shows one row per reviewer of each newest approved part: seat, state, attempt, thread link, findings by kind. A held review starts when the approver next runs aayu on a machine with that tool. | #922 CG 8, 9 |
| FR18 | A local review session runs in a read-only review worktree with `delegate.ScrubEnv`'s environment: no `GH_TOKEN`, `GITHUB_TOKEN`, or aayu token, a temporary `GH_CONFIG_DIR`, and no git credential helper. | #923 CG 1, 3 |
| FR19 | The parent `aayu critique` process records the review only after the child exits 0 with a valid `findings.json`, discards file changes, and always removes the worktree. A failure or `[delegate] local_timeout_minutes` timeout records no review. | #923 CG 2, 4, 5 |

## Non-functional requirements

| # | Requirement | Source |
|---|---|---|
| NFR1 | Security (authority): the owner seat or the drafting session never reviews its own part. | #919 and #920 Behaviours |
| NFR2 | Security (secrets): a local review session never holds a GitHub credential (threat T7). | #923 Behaviours; `docs/threat-model.md` T7 |
| NFR3 | Security (prompt injection): pinned artifacts reach the reviewer as untrusted data. | #920 CG 11; #921 CG 2 and Risk |
| NFR4 | Cost: at most `[critique] sessions_per_day` review sessions a day (default 40); one tech version asks for 6 sessions. | #919 CG 5, 7; #921 CG 8 |
| NFR5 | Reliability: retries are bounded by `[critique] attempts`; two concurrent sweeps start one session. | #922 CG 2, 4 |
| NFR6 | Operability: nothing polls. The approval event starts reviews; a slow tick only catches misses. | #921 Behaviours; #923 Summary |
| NFR7 | Configurability: every timing and limit is a `[critique]` key with a default and a minimum, and a change needs a person. | #919 Behaviours; `.aayudha/config.toml` `always_person_config` |
| NFR8 | Performance: the sources state no latency target for a review session (none stated). | — |
