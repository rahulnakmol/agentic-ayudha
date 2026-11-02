# rahulnakmol/agentic-ayudha#1255: Every agent PR into dev merges on proof, at any risk and any hour

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, decision |
| Body hash | `7c8bd75a5e9f84475c49ddacb53cb60ed64f2acce27ef97a8c1cae1a0668605a` |
| Completion Goal hash | n/a (this type has no Completion Goal) |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1255/ready.md` |

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Question

Which agent PRs into dev merge on proof with no person, and when do they merge? On 2026-11-02, @rahulnakmol wrote: "i am giving you the permission in aayu that low, mediuma nd high risk pr's can auto merge and the we need to catup to the momentum, build lop waits for none and it ships proof proof is validated and automerges all agentic."

Today:
- ship merges on proof at every risk level. Its proof rules do not read the `risk:` label (internal/cli/shipproof.go, internal/autonomy/).
- A PR that touches a path in `[autonomy] always_person` waits for a person's ship (.aayudha/config.toml:75). These paths are ship.go, shippass.go, shipproof.go, presence.go, internal/identity/, internal/autonomy/, internal/config/, the merge-gate, protected-paths, release, approve and token workflows, CODEOWNERS, .goreleaser.yaml, install.sh, go.mod, go.sum, and loops/build-ready.toml.
- A change to those keys themselves also waits for a person (`always_person_config`, .aayudha/config.toml:102).
- On 2026-11-02, PRs #1214, #1220 and #1236 wait for a person, because each changes internal/cli/ship.go.
- Decision #1168 keeps overnight for authoring only, so no code merges between the evening review and the morning.

### Options

1. **Keep today's narrow set.** Every PR outside the narrow set merges on proof at any risk. A narrow-set PR waits for the person's next window (Story #1254). Merges also run overnight.
2. **Every agent PR merges on proof, around the clock (recommended).** A PR merges on proof at any risk and at any hour, ship.go and the other gate files included. Proof means the required checks, the pack and its floors, and inspect at merge with a cross-family review (rung A or B). A merge that touched a former narrow-set path gets an audit card in the next window, and the person can revert it. Only `always_person_config` stays with a person: the keys that say who may approve and what merges without a person. A workflow that reads a secret or names an environment also stays with a person (AGENTS.md).
3. **Empty the narrow set entirely.** As option 2, but a change to `always_person_config` and to the secret and environment workflows also merges on proof, with no audit card.

### Evaluation

- **Founder's words (2026-11-02).** "auto merges all agentic"; "build lop waits for none"; "there should be no blocker to continute work ... user has only two windows".
- **What waits today.** All 3 PRs that wait for him change internal/cli/ship.go. Option 1 frees none of them, and options 2 and 3 free all three.
- **Throughput.** Item merges into dev were 63 in 8 days, from 3 to 16 a day. Overnight merges add about 10 hours a day to the build.
- **Option 2 risk.** An agent PR can change the code that judges its own merge, such as ship.go or the merge gate. The proof still runs, from the code on dev before the merge, with a review from another model family. The person sees each such merge in the next window and can revert it. The keys that set this policy cannot change without a person.
- **Option 3 risk.** An agent PR can also widen what merges without a person, or change a workflow that holds a secret, and nobody sees it until something breaks.

### Recommendation

Option 2. No agent PR waits for a person, at any risk or hour, which is the founder's ask. Only the keys that set this policy, and workflows with secrets or environments, stay with a person. Each gate-file merge gets an audit card in the next window.

The door: two-way. The policy is one config change that a later PR can revert. It is marked not reversible, so it never applies provisionally.

What would change this: if a gate-file merge that merged on proof is reverted in a window, go back to option 1 for those paths for one week.

### Reversible

false

### Links

constrains #1170
````
