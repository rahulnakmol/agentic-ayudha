# Google Cloud for o/r (aayudha)

Written by `aayu init --integrations` (WP7.2b). aayudha queries the Google Analytics 4 BigQuery export with a
read-only service account. GitHub Actions impersonates it through workload identity federation; no key is
stored.

## Founder actions (read-only analytics)

Run these with an account that may manage IAM in the project. `<project>` holds the GA4 export dataset
`analytics_<property-id>`; `<number>` is the project number (`gcloud projects describe <project>
--format='value(projectNumber)'`).

1. Create the service account and grant read only: data on the export dataset, and running query jobs:

   ```sh
   gcloud iam service-accounts create aayudha-read-r --project <project>
   bq add-iam-policy-binding --member=serviceAccount:aayudha-read-r@<project>.iam.gserviceaccount.com --role=roles/bigquery.dataViewer <project>:analytics_<property-id>
   gcloud projects add-iam-policy-binding <project> --member=serviceAccount:aayudha-read-r@<project>.iam.gserviceaccount.com --role=roles/bigquery.jobUser
   ```

   Nothing else. aayudha asks IAM (`testIamPermissions`) before every query and refuses an identity that can
   change datasets or tables, and its tokens carry only the `bigquery.readonly` and `cloud-platform.read-only`
   scopes.

2. Let this repository's `dev` branch on GitHub Actions impersonate it:

   ```sh
   gcloud iam workload-identity-pools create github --project <project> --location global
   gcloud iam workload-identity-pools providers create-oidc aayudha --project <project> --location global \
     --workload-identity-pool github --issuer-uri https://token.actions.githubusercontent.com \
     --attribute-mapping 'google.subject=assertion.sub,attribute.repository=assertion.repository,attribute.ref=assertion.ref' \
     --attribute-condition "assertion.repository == 'o/r' && assertion.ref == 'refs/heads/dev'"
   gcloud iam service-accounts add-iam-policy-binding aayudha-read-r@<project>.iam.gserviceaccount.com \
     --role roles/iam.workloadIdentityUser \
     --member 'principalSet://iam.googleapis.com/projects/<number>/locations/global/workloadIdentityPools/github/attribute.repository/o/r'
   ```

3. Fill `[integrations.gcp]` in `.aayudha/config.toml`: `project` = `<project>.analytics_<property-id>`,
   `federation` = `projects/<number>/locations/global/workloadIdentityPools/github/providers/aayudha`,
   `identity` = the service account email, and `region` when the dataset is not in US or EU. Commit it through a
   pull request.
4. In each workflow job that runs `aayu` with it, add the snippet in `workflow.yml` (`id-token: write`).
5. Check: `aayu doctor` reports `gcp: reachable as …; read-only scope verified`.

Locally, impersonate the same service account through `gcloud` (grant yourself
`roles/iam.serviceAccountTokenCreator` on it):

```sh
AAYU_INTEGRATIONS__GCP__AUTH=cli aayu doctor
```

## Founder actions (proof media, optional)

A public repository needs a private store for proof media (V3). Use a second service account,
`aayudha-media-r`, never the read-only one.

1. Create a bucket with public access prevention on (`gcloud storage buckets create gs://<bucket>
   --public-access-prevention --uniform-bucket-level-access`).
2. Grant the media account `roles/storage.objectAdmin` and, so aayudha can keep the retention rule,
   `roles/storage.admin` on that bucket only (`gcloud storage buckets add-iam-policy-binding gs://<bucket> …`).
3. Set `[proof] media = "gs://<bucket>/r?auth=oidc&provider=projects/<number>/locations/global/workloadIdentityPools/github/providers/aayudha&sa=aayudha-media-r@<project>.iam.gserviceaccount.com"`
   and bind `roles/iam.workloadIdentityUser` on it as in step 2 above.
