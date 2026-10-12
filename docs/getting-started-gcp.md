# Getting started on Google Cloud

aayudha reads the Google Analytics 4 BigQuery export for `grill` baselines and `aayu measure --live`, and can keep
proof media in a private Cloud Storage bucket. It uses plain HTTPS calls with short-lived OAuth tokens: no Google
SDK, and no stored key when it runs in GitHub Actions. This page is part of WP7.2b (cloud integrations).

## What aayudha does with Google Cloud

| Use | API | Identity |
|---|---|---|
| Baselines: `grill`, `aayu measure <issue> --metric gcp:users` | BigQuery `jobs.query` over `events_*` | read-only service account |
| Event pulls: `aayu measure --live gcp` | the same, returning the rows of a GA4 export | read-only service account |
| Proof media (optional, V3) | Cloud Storage JSON API, and the bucket's lifecycle | a second service account |

Read tokens carry only the `bigquery.readonly` and `cloud-platform.read-only` scopes. Before every query aayudha
also asks IAM (`testIamPermissions`) whether the identity holds a write permission on the project or on the
export's tables, and refuses it if so.

## Founder actions

You need a Google Cloud project with the GA4 BigQuery export linked (dataset `analytics_<property-id>`), and an
account that may manage IAM in that project.

1. Run `aayu init --integrations` in the product repository and answer `y` to Google Cloud. It adds
   `[integrations.gcp]` and writes `.aayudha/setup/gcp/`: the steps for this repository (`README.md`) and the
   workflow permissions.
2. Create the service account `aayudha-read-<repo>`. Grant it `roles/bigquery.dataViewer` on the export dataset
   and `roles/bigquery.jobUser` on the project. Grant nothing else.
3. Create a workload identity pool and an OIDC provider for `https://token.actions.githubusercontent.com` whose
   attribute condition admits only this repository's default branch, and grant
   `roles/iam.workloadIdentityUser` on the service account to that repository's principal set.
4. Fill `project` (`<project>.analytics_<property-id>`), `federation` (the provider's full name), `identity`
   (the service account email), and `region` when the dataset is not in US or EU, and merge the change.
5. Give each workflow job that runs `aayu` with Google Cloud `permissions: id-token: write`.
6. To run on your machine, grant yourself `roles/iam.serviceAccountTokenCreator` on the service account.
7. For proof media on a public repository: create a bucket with public access prevention, a second service
   account with `roles/storage.objectAdmin` and `roles/storage.admin` on that bucket only, and set
   `[proof] media = "gs://<bucket>/<prefix>?auth=oidc&provider=<provider>&sa=<media account>"`.

## Configuration

```toml
[integrations.gcp]
scope = "read"
auth = "oidc"      # or "cli" (gcloud, impersonating identity) or "key" (secret holds a service account JSON key)
federation = "projects/<number>/locations/global/workloadIdentityPools/github/providers/aayudha"
identity = "aayudha-read-<repo>@<project>.iam.gserviceaccount.com"
project = "<project>.analytics_<property-id>"
```

On your machine, `AAYU_INTEGRATIONS__GCP__AUTH=cli aayu doctor` impersonates the same service account through
`gcloud`, so your own broader access is never used.

## Check it

```sh
aayu doctor
aayu measure --live gcp --from 2026-09-01 --to 2026-10-01
```

A filter in a baseline (`gcp:users event_name = 'sign_up'`) is one GoogleSQL predicate on the export's columns;
a `;` or a pipe operator (`|>`) in it is refused. The daily tables are named in the property's time zone, so
aayudha reads one extra day on each side and keeps rows by `event_timestamp`.
