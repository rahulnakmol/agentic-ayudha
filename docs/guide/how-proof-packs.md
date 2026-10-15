# Proof packs

A proof pack is one `pack.json` file, plus the artifacts it names, that records what CI observed about one exact
commit of one story. It exists so that "done" is a checkable claim: every Completion Goal criterion points at
evidence, every evidence file has a SHA-256 hash, and every floor result is written down with its tool and
threshold. This page covers the schema, how criteria get their evidence, what makes a pack count, and how packs are
checked, stored, and purged.

![CI reads the Ready snapshot and the evidence map, runs the tests and floors, and writes a hashed pack; data goes to the ledger, media to LFS or a bucket, and ship only accepts an authoritative pack that verifies.](img/how-proof-packs.svg)

*The pack that counts is written by CI, from what CI ran.*

## How it works

A builder agent can say anything about its own work. The pack design keeps the agent from certifying it: the
evidence for a criterion is the output of a test that `verify` ran itself, the pack is tied to one head commit and
one Completion Goal hash, and only the base-branch merge gate can write a pack that `ship` accepts. The agent still
chooses which of its tests prove each promise, in the evidence map it commits; that is why `inspect` reads the pack
and judges whether the mapped tests really show each promise. In this repository's Go stories, an `api-trace`
evidence file holds the `go test` events of the mapped test.

The flow for one pull request:

1. `aayu ready` freezes the story's numbered criteria in a Ready snapshot on the `ledger` branch, at
   `specs/<story>/ready.md`, with a hash of the Completion Goal.
2. While building, the agent commits `.aayudha/evidence/<story>.toml`: which tests prove which criterion. `aayu build`
   tells the agent to write it.
3. `aayu verify` runs every floor in a sandbox. The coverage floor runs `go test -json`, and `verify` keeps that
   event stream.
4. `TestEvidence` reads the evidence map and selects the events of each mapped test. A criterion gets an artifact
   `evidence/criterion-N.jsonl` only when every named test ran and passed in this run. Otherwise the criterion gets
   nothing and the problem is reported.
5. Web floors add screenshots and recordings for screenshot and recording criteria (`screens/N.png`,
   `recordings/N.webm`), and every scanner adds its report.
6. `verify.Pack` assembles the pack and `proof.WriteLocal` writes it to `.aayudha/proof/<story>/` (git-ignored).
   In CI the merge gate uploads that directory as the `aayudha-pack` run artifact.

### Advisory and authoritative

`authority.kind` is `advisory` or `authoritative`. `proof push` decides it from the environment, not from a flag: a
pack is authoritative only when the process runs in CI and `GITHUB_WORKFLOW_REF` names
`.github/workflows/aayudha-merge-gate.yml`. Every other push, from a laptop or an agent lane, is advisory. The merge
gate runs under `pull_request_target`, so the workflow file comes from the base branch and a pull request cannot
edit it to certify itself. See [the merge gate](how-merge-gate.md).

An authoritative pack also records `workflow` and `run_id`, and `writer` when someone other than the run landed it.
`ship` reads only authoritative packs for the pull request's current head. If none is on the ledger yet, it finds
the newest successful merge-gate run for that head, downloads its `aayudha-pack` artifact, checks that it is for
the same story and head, and pushes it as authoritative with the run ID and `writer: "<login> (aayu ship)"`.

> **Honest status:** no bot GitHub App is installed on this repository, so the `aayudha/pack` job cannot write the
> ledger. Every authoritative pack here was landed by `aayu ship` under the founder's identity, which is why the
> real pack below says `"writer": "rahulnakmol (aayu ship)"`. The pack bytes are still the CI run's own.

### Validation

`aayu proof verify <story>` takes the story's best pack (newest authoritative, else newest advisory) and checks:

- every stored artifact's bytes match its `sha256` and `size`, and no path is listed twice or escapes the pack;
- every evidence path in `criteria` names an artifact of the pack;
- every criterion in the Ready snapshot has at least one evidence path, and no criterion is outside it;
- `completion_goal_hash` equals the Ready snapshot's hash, so a story edited after Ready fails;
- `pr_head_sha` equals the pull request's current head.

Floor results are reported as `floors_hold` but are not integrity problems. A pack can verify and still not ship.
`ship` runs the same `proof.Verify` and then requires `FloorsHold()`.

### Media stores

Each artifact has a `store`. Data artifacts (traces, metrics, reports) are always `git`: committed to the ledger.
Media (screenshots, recordings, screen-reader transcripts, and image, video, audio, or PDF files) go to:

- `bucket`, when `[proof] media` names a private bucket: `s3://`, `r2://`, `azblob://`, or `gs://`, with the
  credential in the `proof-media` secret. `object` holds the key. `proof push` also sets the bucket's lifecycle
  rule to `[proof] retention_days` (default 90) where it can.
- `lfs`, on a private repository without a bucket: Git LFS on the ledger.
- `none`, when media are not pushed: only the hash and size are kept.

A public repository without a bucket refuses `proof push --media` (`ErrMediaRefused`). GitHub keeps LFS objects
until the repository is deleted, so LFS is never used for a public repository's media.

### Purge and tombstones

`aayu proof purge <story>` deletes a story's media now. Without a story it deletes media of packs older than
`retention_days`. Bucket objects are deleted and LFS files are removed from the ledger tip. Each affected
`pack.json` is rewritten with a `tombstone` (`purged_at`, `reason`) on the artifact. The hash and size stay, so the
pack still says what was captured. `proof verify` counts tombstoned and `none` artifacts as `artifacts_not_stored`
instead of failing on them.

## Key data and file formats

The Go types, from [internal/proof/pack.go](../../internal/proof/pack.go) (the JSON Schema is
`internal/proof/schema/pack.v1.json`):

```go
// SchemaVersion is the pack schema this binary writes. Readers for every
// version shipped since 0.1 stay in Decode (WP1.9 grandfathering).
const SchemaVersion = 1
…
// Pack is pack.json v1.
type Pack struct {
    Schema             int         `json:"schema_version"`
    Repo               string      `json:"repo"`
    Story              int         `json:"story"`
    PR                 int         `json:"pr,omitempty"`
    PRHeadSHA          string      `json:"pr_head_sha"`
    BaseSHA            string      `json:"base_sha,omitempty"`
    Checkpoint         string      `json:"checkpoint,omitempty"` // n/N from the Aayudha-Checkpoint trailer
    Seat               string      `json:"seat"`
    Host               string      `json:"host"`
    Model              string      `json:"model,omitempty"` // the builder model, when known
    Identity           string      `json:"identity"`        // bot, human, human-fallback, or ci
    Authority          Authority   `json:"authority"`
    CompletionGoalHash string      `json:"completion_goal_hash"`
    Footprint          []string    `json:"footprint,omitempty"` // layers the PR's changed paths fall in
    Criteria           []Criterion `json:"criteria"`
    Artifacts          []Artifact  `json:"artifacts"`
    Checks             []Check     `json:"checks"`
    Cost               Cost        `json:"cost"`
    Links              Links       `json:"links"`
    CreatedAt          string      `json:"created_at"`
}
…
// Artifact is one evidence file.
type Artifact struct {
    Path      string     `json:"path"` // relative, always "/"-separated
    Kind      string     `json:"kind"`
    SHA256    string     `json:"sha256"`
    Size      int64      `json:"size"`
    Store     string     `json:"store"`
    Object    string     `json:"object,omitempty"` // bucket: the object key
    Tombstone *Tombstone `json:"tombstone,omitempty"`
}
```

`Decode` reads `schema_version` first, validates the document against that version's schema, and refuses a newer
version with a hint to run `aayu self-update`. Artifact `kind` is one of `screenshot`, `recording`,
`sr-transcript`, `api-trace`, `metric`, or `report`. `footprint` lists the layers (L01 to L09) whose path globs in
`LAYERS.md` match a changed file. Check `result` is `pass`, `fail`, `skipped`, `waived`, or `blocked`; see
[floors in detail](how-floors.md).

The evidence map for story 131, from `.aayudha/evidence/131.toml`:

```toml
[[criterion]]
id = 1 # the dental question is answered from the dental-deductible passage, quoted with its plan and section, and grounded
tests = ["examples/benefits-qa/answer:TestAnswerCitesPassage"]

[[criterion]]
id = 2 # questions about another person's records or asking for advice are refused before retrieval (never looked up), with a reason and a message
tests = ["examples/benefits-qa/policy:TestScreenRefuses", "examples/benefits-qa/answer:TestRespondScreensFirst"]
…
```

The real authoritative pack for story 131 (pull request 148), from
`git show origin/ledger:proof/131/b046d1a1d113339eb80e130c9dc0008fa223e76f/authoritative/pack.json`, with most
artifacts and checks trimmed:

```json
{
  "schema_version": 1,
  "repo": "rahulnakmol/agentic-ayudha",
  "story": 131,
  "pr": 148,
  "pr_head_sha": "b046d1a1d113339eb80e130c9dc0008fa223e76f",
  "base_sha": "b5359bd5c041820193b6dcadc0f0b10d5aff1836",
  "checkpoint": "3/3",
  "seat": "ai-native-engineer",
  "host": "cli",
  "model": "claude-opus-5-5",
  "identity": "ci",
  "authority": {
    "kind": "authoritative",
    "workflow": ".github/workflows/aayudha-merge-gate.yml",
    "run_id": 38231401215,
    "writer": "rahulnakmol (aayu ship)"
  },
  "completion_goal_hash": "e6e99c0c0e6aa4c1d6044a1ae26d91d9df3530e0a18d9d0b5883bd0d796e28cb",
  "footprint": [
    "L04",
    "L05"
  ],
  "criteria": [
    {
      "id": 1,
      "evidence": [
        "evidence/criterion-1.jsonl"
      ]
    },
    …
  ],
  "artifacts": [
    …
    {
      "path": "evidence/criterion-1.jsonl",
      "kind": "api-trace",
      "sha256": "43d40ade5147918903107d06ccfef7876cb920602f8dc2755a3ea947ee89ce62",
      "size": 615,
      "store": "git"
    },
    …
  ],
  "checks": [
    {
      "floor": "coverage",
      "result": "pass",
      "value": 100,
      "threshold": 90,
      "tool": "aayudha diff coverage (go test -coverprofile)",
      "version": "go1.25.14",
      "detail": "116 of 116 changed executable lines covered (go coverprofile)",
      "network": "isolated"
    },
    …
  ],
  "cost": {
    "tokens_in": 0,
    "tokens_out": 0,
    "usd": 0
  },
  "links": {
    "issue": "rahulnakmol/agentic-ayudha#131",
    "pr": "rahulnakmol/agentic-ayudha#148",
    "ready_snapshot": "specs/131/ready.md"
  },
  "created_at": "2026-10-12T09:48:42Z"
}
```

The first line of its evidence for criterion 1, from
`git show origin/ledger:proof/131/b046d1a1d113339eb80e130c9dc0008fa223e76f/authoritative/artifacts/evidence/criterion-1.jsonl`.
`Time` and `Elapsed` are dropped so identical runs give identical bytes and hashes:

```json
{"Action":"run","Package":"github.com/rahulnakmol/agentic-ayudha/examples/benefits-qa/answer","Test":"TestAnswerCitesPassage"}
```

> **Honest status:** `verify` does not fill `cost` yet, so packs record zero spend. Model spend is tracked by the
> model gateway and shown by `inspect`. This repository is public and `[proof] media` is empty, so the screenshots in
> story 134's pack have `"store": "none"`: their hashes are kept, their bytes are not.

## Invariants

1. Evidence comes only from this run: a criterion gets a test artifact only if every mapped test ran and passed
   in the same `go test -json` run. Enforced in [internal/verify/evidence.go](../../internal/verify/evidence.go),
   `TestTestEvidence`.
2. A pack is authoritative only when written from the merge-gate workflow in CI. Every other push is advisory.
   Enforced by `authority()` in [internal/cli/proof.go](../../internal/cli/proof.go); `TestProofPushVerifyPurgeCLI`
   checks that a push from outside the gate lands as advisory.
3. Every stored artifact matches its hash and size, every Ready criterion has evidence, and the pack matches the
   pull request head and the Ready Completion Goal hash. Enforced by `Verify` in
   [internal/proof/verify.go](../../internal/proof/verify.go), `TestVerifyDetects`.
4. `ship` accepts only an authoritative pack for the current head that verifies and whose floors hold. Enforced in
   [internal/cli/ship.go](../../internal/cli/ship.go), `TestShipRefusals`.
5. Media never reach a public repository's ledger or Git LFS. Enforced by `Push` in
   [internal/proof/ledger.go](../../internal/proof/ledger.go), `TestPublicRepoRefusesMedia` and
   `TestPrivateRepoMediaInLFS`.
6. Purging removes bytes but never hashes: each purged artifact gets a tombstone. Enforced by `Purge` in
   [internal/proof/ledger.go](../../internal/proof/ledger.go), `TestPushVerifyPurgeBucket`.
7. Every pack validates against the JSON Schema of its `schema_version`. Enforced by `Decode` in
   [internal/proof/pack.go](../../internal/proof/pack.go), `TestSchemaValidatesSamples`.

## Where it lives in the code

- [internal/proof/pack.go](../../internal/proof/pack.go): types, schema decode, `FloorsHold`, media rules.
- [internal/proof/verify.go](../../internal/proof/verify.go): `proof verify` integrity checks.
- [internal/proof/ledger.go](../../internal/proof/ledger.go): push, list, best pack, purge.
- [internal/proof/bucket.go](../../internal/proof/bucket.go) and
  [internal/proof/cloudmedia.go](../../internal/proof/cloudmedia.go): S3, R2, Azure Blob, and GCS media stores.
- [internal/proof/schema](../../internal/proof/schema): the JSON Schema.
- [internal/verify/evidence.go](../../internal/verify/evidence.go): the evidence map and test evidence.
- [internal/verify/verify.go](../../internal/verify/verify.go): pack assembly.
- [internal/cli/proof.go](../../internal/cli/proof.go) and [internal/cli/ship.go](../../internal/cli/ship.go):
  the CLI and the landing of the gate's pack.

## Read next

- [The merge gate](how-merge-gate.md): the workflow that writes authoritative packs.
- [Floors in detail](how-floors.md): every check in the `checks` array.
