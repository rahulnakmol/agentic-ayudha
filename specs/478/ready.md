# rahulnakmol/agentic-ayudha#478: aayu counsel: consult Atri, the Founder's Sage, on any topic, issue, path, or PR, with cited proof and the next step; never a review

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `63cc6c2cdb014f9fef1e46a834581ee61b5ea6964532d6def2822aaf3dd4d440` |
| Completion Goal hash | `4b0a7fd4700969d4a8ae59dbf0282d0b7d7aa8d3306f6e99706a0839273b5044` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/478/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `aayu counsel "should we split the survey into sections?"`, `aayu counsel --issue N` on a Decision, `aayu counsel internal/cli/inbox.go`, and `aayu counsel --pr N` on a pull request with a proof pack; expected: each prints Atri's grounding with every fact cited (the issue and its parent and links, the path's layer and owner seat from LAYERS.md, the pull request's changed files by layer, its proof-pack floor results, and its builder family), the open Decisions and floor waivers, the CONSTRAINTS.md floors, the counsel outline, and Atri's persona, and makes no GitHub write
2. [api-trace] state: a counsel draft with every part (the call with its one-way or two-way door, the operations, technology, and product readings with the dominant one named, one recommendation, what not to do, what would change it, the next `aayu` command, and the question to answer), each claim cited or marked unverified; and drafts that each miss one part, recommend two options, carry an uncited claim, give no `aayu` command as the next step, or claim an approval or a review ("LGTM", "I approve", "has been reviewed"); expected: `aayu counsel ... --draft <file>` exits 0 on the first and exits 4 naming the finding on each of the others
3. [api-trace] state: a passing draft with `--post` on `--pr N`, on `--issue N`, and with no pull request or issue; expected: on a pull request or issue, one comment headed as counsel from Atri, advice only and not a review, an approval, or a Decision, that names the model that wrote it and says same-family or different-family counsel against the pull request's builder family (same-family when that family is unknown), written through the actions log and replaced in place on a rerun, with no pull request review, approval, label, or check created; with no target, exit 2 and no write
4. [api-trace] state: the registry and the generated surfaces; expected: `counsel` is a verb Accountable to the Founder seat in the spec lane, exposed read-only to models over MCP, with a skill that models may invoke whose persona block is Atri's, and the host pre-tool hook lets an agent run `aayu counsel`
5. [api-trace] state: the counsel eval cases at tier 1 (a pull request and a free question); expected: each recorded counsel passes the draft checks, and a planted counsel with an uncited architectural claim and a claimed approval fails them

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, or any agent working for the founder, I want to consult Atri, the Founder's Sage, on any topic, issue, Decision, path, or pull request, the way Amp's oracle is consulted, so that I get a steer grounded in the codebase's own evidence and the next step to take, without that steer ever counting as a review or an approval. Today Atri speaks only inside `aayu decide --pack`, which shapes a decision pack with options, and no verb or MCP tool gives counsel on an arbitrary topic or a pull request. `aayu counsel` is that verb: Accountable to the Founder seat, in the spec lane at its escalation, read-only by default and callable by models. It gathers the grounding through aayu (the issue and its links, the path's layer and owner, the pull request's changed files by layer, its proof pack and builder family, the open Decisions and floor waivers, and the floors in CONSTRAINTS.md), and it checks a counsel draft for every part and for cited claims. Posting is opt-in with `--post`, labelled as counsel and never as a review.

### Completion Goal

1. [api-trace] state: `aayu counsel "should we split the survey into sections?"`, `aayu counsel --issue N` on a Decision, `aayu counsel internal/cli/inbox.go`, and `aayu counsel --pr N` on a pull request with a proof pack; expected: each prints Atri's grounding with every fact cited (the issue and its parent and links, the path's layer and owner seat from LAYERS.md, the pull request's changed files by layer, its proof-pack floor results, and its builder family), the open Decisions and floor waivers, the CONSTRAINTS.md floors, the counsel outline, and Atri's persona, and makes no GitHub write
2. [api-trace] state: a counsel draft with every part (the call with its one-way or two-way door, the operations, technology, and product readings with the dominant one named, one recommendation, what not to do, what would change it, the next `aayu` command, and the question to answer), each claim cited or marked unverified; and drafts that each miss one part, recommend two options, carry an uncited claim, give no `aayu` command as the next step, or claim an approval or a review ("LGTM", "I approve", "has been reviewed"); expected: `aayu counsel ... --draft <file>` exits 0 on the first and exits 4 naming the finding on each of the others
3. [api-trace] state: a passing draft with `--post` on `--pr N`, on `--issue N`, and with no pull request or issue; expected: on a pull request or issue, one comment headed as counsel from Atri, advice only and not a review, an approval, or a Decision, that names the model that wrote it and says same-family or different-family counsel against the pull request's builder family (same-family when that family is unknown), written through the actions log and replaced in place on a rerun, with no pull request review, approval, label, or check created; with no target, exit 2 and no write
4. [api-trace] state: the registry and the generated surfaces; expected: `counsel` is a verb Accountable to the Founder seat in the spec lane, exposed read-only to models over MCP, with a skill that models may invoke whose persona block is Atri's, and the host pre-tool hook lets an agent run `aayu counsel`
5. [api-trace] state: the counsel eval cases at tier 1 (a pull request and a free question); expected: each recorded counsel passes the draft checks, and a planted counsel with an uncited architectural claim and a claimed approval fails them

### Measure

Completion Goal criteria observed

### Threshold

all 5 criteria

### Scenario

Against the fake GitHub, seed a Decision with a parent and a link, a pull request with changed files in two layers, a proof pack, and a build record naming the builder's model. Run `aayu counsel` with a free question, `--issue`, a path, and `--pr`, and read the grounding and the writes the fake GitHub received. Check a complete draft and one draft per defect with `--draft`. Post a passing draft with `--post` on the pull request twice, on the issue, and with no target. Read the registry, the generated SKILL.md and MCP schema, and send `aayu counsel` to `aayu hook pre-tool`. Run the counsel cases at tier 1.

### Environment

local checkout and this repository's CI build job on a pull request; the fake GitHub in internal/github/fake, never production

### Checkpoints

1. `go test ./internal/counsel ./internal/cli -run 'TestCounsel'` shows the grounding for each kind of topic, the draft findings, and the posted comment with its label (criteria 1, 2, 3)
2. the registry, the generated skill and MCP schema, the hook, and the tier-1 counsel cases pass `go test ./internal/verbs ./internal/hosts ./internal/evals` (criteria 4, 5)

### Assumptions

- the counsel is written by the host's model (the skill), as `aayu decide --pack` and `aayu chart` are; the binary gathers the grounding and checks the draft, and makes no model call itself
- the writing model is the one AAYU_MODEL names, else the spec lane's model for the host; it is reported as given, not verified
- the builder family of a pull request is what aayu recorded for its commits (the build record on the ledger and the Aayudha-Model trailers), read with the same rule inspect uses; when it is unknown, the counsel is labelled same-family
- a draft's claims follow the evidence rule every seat follows: each cites a source the grounding or the checkout contains, or is marked unverified
- a proactive drift nudge in `aayu next` or `aayu status` is out of scope for this Story

### Provenance

Request: "atri is like amp's oracle, can be called on any topic, any PR, and so he guides, nudges and shows proof or shows direction"
Term: "oracle" → an advisor consulted on demand who gives grounded advice and never decides; Atri counsels and never rules, decides, approves, or acts
Vague: "shows proof" → criteria 1, 2
Vague: "shows direction" → criteria 2
1. stated (request): "can be called on any topic, any PR" / "shows proof"
2. stated (request): "he guides, nudges and shows proof or shows direction"
3. stated (request): "any PR" / "like amp's oracle"
4. stated (request): "can be called on any topic"
5. stated (request): "shows proof"
````
