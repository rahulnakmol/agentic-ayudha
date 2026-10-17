# rahulnakmol/agentic-ayudha#432: Solo mode: a reversible Decision applies provisionally after 24 business hours, never a waiver, seat change, or human-only step (V8)

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `de82d44c91cbb908867d9b4e89dd02d701dfc915a332655e8f271fc9e23b4afa` |
| Completion Goal hash | `cc06bbbbf229ce58b4eac0c5bbe134107552bf935058ab9855b0d34cb867a3ce` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/432/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: one human in config (no second paired seat human or approver) and an open, reversible, Decision-ready Decision created 30 business hours ago; expected: `aayu decide` applies its recommendation provisionally (decision:provisional, with a ratify-or-revert deadline); with a second human paired on a seat, the same Decision is not applied until 48 business hours
2. [api-trace] state: open, Decision-ready Decisions marked `reversible: true` and 100 business hours old: one labelled decision:waiver, one with a `waives #N` link, one titled as a waiver without the label, one about a seat, one that changes a protected path (`[autonomy]` in config), and one with `reversible: false`; expected: `aayu decide` applies none of them, in solo mode or not, and names the reason for each
3. [api-trace] state: `[decisions] solo_provisional_after_hours` set to 11 in `.aayudha/config.toml`, and set to 24 in the user config, in `.aayudha/config.local.toml`, in AAYU_DECISIONS__SOLO_PROVISIONAL_AFTER_HOURS, or with --set; expected: each config load fails, naming the minimum of 12 or that the key is read only from `.aayudha/config.toml`
4. [api-trace] state: a repository with an open provisional Decision; expected: `aayu status` lists it with its ratify-or-revert deadline and the commands that reverse it (`aayu inbox`, `aayu decide <n> --accept <option>`)

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder working alone, I want a reversible Decision's recommendation to apply provisionally after 24 business hours instead of 48 when I am the only human, so that a two-way door does not wait on me, while nothing that must stay human ever applies on its own. Rule V8 today applies a reversible, Decision-ready Decision's recommendation after `[decisions] provisional_after_hours` (48 business hours) and then asks for it to be ratified or reverted. In solo mode the new key `[decisions] solo_provisional_after_hours` (default 24) shortens that window. It has a hard minimum of 12 business hours, and it is read only from `.aayudha/config.toml`, a protected path like `[autonomy]`. The provisional rule never applies to a one-way door, to a waiver of any kind (eval-bar, floor, coverage, or another), to a seat change, or to anything the trust model lists as human-only, so an agent cannot wait out a ratchet or a gate. Provisional Decisions are shown in `aayu status` and in `aayu inbox`, so the founder can reverse them.

### Completion Goal

1. [api-trace] state: one human in config (no second paired seat human or approver) and an open, reversible, Decision-ready Decision created 30 business hours ago; expected: `aayu decide` applies its recommendation provisionally (decision:provisional, with a ratify-or-revert deadline); with a second human paired on a seat, the same Decision is not applied until 48 business hours
2. [api-trace] state: open, Decision-ready Decisions marked `reversible: true` and 100 business hours old: one labelled decision:waiver, one with a `waives #N` link, one titled as a waiver without the label, one about a seat, one that changes a protected path (`[autonomy]` in config), and one with `reversible: false`; expected: `aayu decide` applies none of them, in solo mode or not, and names the reason for each
3. [api-trace] state: `[decisions] solo_provisional_after_hours` set to 11 in `.aayudha/config.toml`, and set to 24 in the user config, in `.aayudha/config.local.toml`, in AAYU_DECISIONS__SOLO_PROVISIONAL_AFTER_HOURS, or with --set; expected: each config load fails, naming the minimum of 12 or that the key is read only from `.aayudha/config.toml`
4. [api-trace] state: a repository with an open provisional Decision; expected: `aayu status` lists it with its ratify-or-revert deadline and the commands that reverse it (`aayu inbox`, `aayu decide <n> --accept <option>`)

### Measure

Completion Goal criteria observed

### Threshold

all 4 criteria

### Scenario

Against the fake GitHub with a fixed clock, run the decide sweep over Decisions of each kind in criteria 1 and 2 at 30 and 100 business hours, once with one human in config and once with a second paired human. Load config with the solo key at 11 and in each non-repository layer. Run `aayu status` with a provisional Decision open.

### Environment

local checkout and this repository's CI build job on a pull request; the fake GitHub in internal/github/fake, never production

### Checkpoints

1. `go test ./internal/decide ./internal/cli -run 'TestSoloProvisional|TestProvisionalNever'` shows the 24-hour window in solo mode, 48 with two humans, and every excluded kind refused with its reason (criteria 1, 2)
2. `go test ./internal/config -run TestSoloProvisionalKey` shows the minimum and the repository-only rule, and `aayu status` lists the provisional Decision (criteria 3, 4)

### Assumptions

- hours are business hours on the `[time]` clock, as `provisional_after_hours` already is; the minimum of 12 is a constant in code, not config
- solo mode for this rule is at most one distinct human login across paired seats and `[autonomy] approver`
- in solo mode the window is the shorter of the two keys, so `provisional_after_hours = 0` (V8 off) still turns provisional application off
- a Decision is a waiver when it is labelled decision:waiver, links `waives #N`, or its title or question says waive or waiver; it is a seat change when its title or question names a seat; it is trust-model human-only when its title or question names approving a PR, publishing, `aayu init`, or a protected path (`.github/`, `skills/`, `ops/`, `seats/`, `CONSTRAINTS.md`, `CODEOWNERS`, `.aayudha/config.toml`, `[autonomy]`, `[models]`, `[decisions]`), a floor, or a ratchet
- these exclusions apply in every mode, so V8 also stops applying them for teams of more than one human

### Provenance

Request: "In solo mode (one human), shorten this to 24 hours, configurable, with a hard minimum the config cannot go below. Apply it only to Decisions their author marked reversible (two-way doors) that have a recommendation. It never applies to one-way doors, waivers of any kind (eval-bar, floor, coverage, or otherwise; this is essential, or an agent could wait out the ratchet), seat changes, or anything the trust model lists as human-only. Make the config key a protected path like [autonomy]. Show provisional Decisions in status and in inbox so the founder can reverse them."
Vague: "protected" → criteria 3
1. stated (request): "In solo mode (one human), shorten this to 24 hours" / "Apply it only to Decisions their author marked reversible (two-way doors) that have a recommendation"
2. stated (request): "It never applies to one-way doors, waivers of any kind (eval-bar, floor, coverage, or otherwise" / "seat changes, or anything the trust model lists as human-only"
3. stated (request): "configurable, with a hard minimum the config cannot go below" / "Make the config key a protected path like [autonomy]"
4. stated (request): "Show provisional Decisions in status and in inbox so the founder can reverse them"
````
