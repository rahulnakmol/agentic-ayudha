# UX design: Feature #876

Nothing here is built. The surface is planned as two terminal commands and a section of the grill skill. The
inbox screens are out of scope ([#876](https://github.com/rahulnakmol/agentic-ayudha/issues/876) Scope). The
footprint has no L01 (`specs/876/design.json`), so there is no UX baseline.

## Commands (planned, not built)

| Command | Who | What it does | Source |
|---|---|---|---|
| `aayu grill feature <f> --findings [--json]` | anyone, usually the agent in the grill session | Lists each finding with its state; writes nothing | #1114 CG 7-9 |
| `aayu critique <f> --resolve <id> --as dismissed --reason "<line>"` | a person, in their own terminal | Dismisses one gap after the device code | #1115 CG 1 |
| `aayu ready <f> --part <p> --request --fixes <id>` | the agent | Drafts the fix as a new version (delivered in #993) | #1116 CG 1 |
| `aayu ready <f>` | a person | Approve to build (delivered in #995); shows each dismissal to ratify | #1115 CG 6 |

## Terminal output (planned, quoted from #1114 CG 7)

```text
#840 technical design v1 · 1 review in · 1 gap · 1 input · 1 guidance
<one line per finding: ID, kind, severity, seat, applies_to, state>   (gaps first)
Next: fix it in a new version (aayu ready 840 --part tech --request --fixes <id>), or dismiss it yourself (aayu critique 840 --resolve <id> --as dismissed --reason "…")
```

## Messages (planned)

| Moment | Text | Source |
|---|---|---|
| Dismissed | "Dismissed tech-v1/production-engineer/1 as @rahulnakmol. Approve to build lists it for you to ratify." | #1115 CG 1 |
| No reason | "a dismissal needs --reason" | #1115 CG 2 |
| Unknown or wrong kind | "no such gap: tech-v1/production-engineer/9"; "tech-v1/production-engineer/2 is an input; only a gap keeps Approve to build off" | #1115 CG 4 |
| Already resolved | "tech-v1/production-engineer/1 was dismissed at &lt;time&gt;"; "… is fixed in technical design v2" | #1115 CG 5 |
| Too long | "the reason has 612 characters; keep it to 500 ([critique] reason_max_chars)" | #1115 CG 7 |
| Unreadable record | "Approve to build is off: specs/840/reviews/tech-v1/auditor.json cannot be read" | #1114 CG 6 |
| No reviews | "#840 has no seat reviews yet" | #1114 CG 9 |

Delivered and waiting for data: the open-gap refusal ("Approve to build is off: … has 1 open gap (…). Next: aayu
grill feature 840 --findings") and the ratified line ("1 dismissal ratified: …") are already in
`approveRun.prepare` and `ready.Ratified`. They cannot show until a critique state reports gaps.

## The agent's experience in the grill session (planned)

From [#1116](https://github.com/rahulnakmol/agentic-ayudha/issues/1116) CG 1-3: the agent runs `--findings`, takes
each open gap one at a time with a recommendation (fix or dismiss), drafts the version request on "fix", and on
"dismiss" gives the person the exact `--resolve` command with the person's own reason. When the person does not
answer on a gap, the agent leaves it open and says that Approve to build stays off.

Accessibility: no source states a requirement beyond plain terminal text.
