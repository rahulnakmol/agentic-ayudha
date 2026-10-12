# Getting started on AWS

aayudha reads CloudWatch RUM for `grill` baselines and `aayu measure --live`. It signs plain HTTPS calls with
AWS Signature Version 4: no AWS SDK, and no stored key when it runs in GitHub Actions. Proof media can live in
S3 (WP4.2). This page is part of WP7.2b (cloud integrations).

## What aayudha does with AWS

| Use | API | Identity |
|---|---|---|
| Baselines: `grill`, `aayu measure <issue> --metric aws:users` | CloudWatch Logs Insights over the app monitor's log group | read-only IAM role |
| Event pulls: `aayu measure --live aws` | CloudWatch RUM `GetAppMonitorData` (the last 30 days only) | read-only IAM role |
| Proof media (optional, V3) | S3 | a separate IAM user's access key |

Before every query aayudha asks STS who the credential is and runs the IAM policy simulator for the actions it
uses and nineteen write actions. A credential that may write, or root credentials, is refused. If the role may
not run the simulator, `doctor` says the configured scope is trusted rather than verified.

## Founder actions

You need an AWS account with a CloudWatch RUM app monitor, and an account that may create IAM roles.

1. Run `aayu init --integrations` in the product repository and answer `y` to AWS. It adds `[integrations.aws]`
   and writes `.aayudha/setup/aws/`: the steps (`README.md`), `trust-policy.json`, `read-policy.json`,
   `media-policy.json`, and the workflow permissions. Replace the account ID, region, and monitor
   placeholders in the two policies.
2. Turn on the app monitor's data storage in CloudWatch Logs, so Logs Insights can count its events.
3. Register GitHub's OIDC provider once per account
   (`aws iam create-open-id-connect-provider --url https://token.actions.githubusercontent.com
   --client-id-list sts.amazonaws.com`).
4. Create the role `aayudha-read-<repo>` with `trust-policy.json` (only this repository's default branch may
   assume it) and add `read-policy.json` as its only policy.
5. Fill `project` (the app monitor name), `region`, and `identity` (the role ARN) in `[integrations.aws]`, and
   merge the change.
6. Give each workflow job that runs `aayu` with AWS `permissions: id-token: write`.
7. For proof media on a public repository: create a private S3 bucket, an IAM user with `media-policy.json`, and
   store its `ACCESS_KEY_ID:SECRET_ACCESS_KEY` with `aayu auth set-secret proof-media`; set
   `[proof] media = "s3://<bucket>/<prefix>?region=<region>"`.

## Configuration

```toml
[integrations.aws]
scope = "read"
auth = "oidc"      # or "cli" (aws configure export-credentials) or "key" (secret holds ACCESS_KEY_ID:SECRET)
identity = "arn:aws:iam::<account-id>:role/aayudha-read-<repo>"
region = "eu-west-1"
project = "<app monitor name>"
```

On your machine, use a profile that assumes the same role:
`AWS_PROFILE=aayudha-read AAYU_INTEGRATIONS__AWS__AUTH=cli aayu doctor`. The `aws` CLI runs with only its own
configuration variables, never aayudha's tokens.

## Check it

```sh
aayu doctor
aayu measure --live aws
```

A filter in a baseline (`aws:events event_type = 'signed_up'`) is one Logs Insights filter expression; a `|` or
`;` in it is refused.
