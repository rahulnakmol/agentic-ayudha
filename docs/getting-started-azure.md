# Getting started on Azure

aayudha reads Application Insights for `grill` baselines and `aayu measure --live`, and can keep proof media in a
private Blob Storage container. It uses plain HTTPS calls with short-lived Microsoft Entra tokens: no Azure SDK,
and no stored key when it runs in GitHub Actions. This page is part of WP7.2b (cloud integrations).

## What aayudha does with Azure

| Use | API | Identity |
|---|---|---|
| Baselines: `grill`, `aayu measure <issue> --metric azure:users` | Log Analytics query API (KQL over `AppEvents`) | read-only app registration |
| Event pulls: `aayu measure --live azure` | the same query, returning the rows of an Application Insights export | read-only app registration |
| Proof media (optional, V3) | Blob Storage, and the storage account's lifecycle policy | a separate identity or a SAS token |

Before every query aayudha asks Azure Resource Manager which actions the identity has on the resource. Any
action that is not a read (`*/read`, the two Log Analytics query actions, or `Microsoft.Support/*`) makes it refuse
the credential, and `aayu doctor` reports the write-capable action.

## Founder actions

You need an Azure subscription with a workspace-based Application Insights component, and an account that may
create app registrations and role assignments.

1. Run `aayu init --integrations` in the product repository and answer `y` to Azure. It adds
   `[integrations.azure]` to `.aayudha/config.toml` and writes `.aayudha/setup/azure/`: the steps for this
   repository (`README.md`), the GitHub federated credential, the lifecycle policy for proof media, and the
   workflow permissions.
2. Create the app registration `aayudha-read-<repo>` and its service principal (`az ad app create`,
   `az ad sp create`).
3. Add the federated credential from `.aayudha/setup/azure/federated-credential.json`
   (`az ad app federated-credential create`). It trusts only this repository's default branch.
4. Assign "Monitoring Reader" on the component and "Log Analytics Reader" on its workspace. Assign no other role.
5. Fill `org` (tenant ID), `identity` (client ID), and `project` (the component's resource ID) in
   `[integrations.azure]`, and merge the change.
6. Give each workflow job that runs `aayu` with Azure `permissions: id-token: write` (the snippet in
   `.aayudha/setup/azure/workflow.yml`).
7. For proof media on a public repository: create a storage account with public access off, a private
   container, and either a container SAS token (`sp=rcwdl`, with an expiry; store it with
   `aayu auth set-secret proof-media`) or an Entra identity with "Storage Blob Data Contributor" on the
   container. Set `[proof] media = "azblob://<account>/<container>/<prefix>"`. With a SAS token, apply
   `.aayudha/setup/azure/lifecycle-policy.json` once, because a SAS token cannot manage the lifecycle policy.

## Configuration

```toml
[integrations.azure]
scope = "read"
auth = "oidc"      # or "cli" (az login) or "key" (secret holds <client-id>:<client-secret>)
org = "<tenant ID>"
identity = "<client ID>"
project = "/subscriptions/<id>/resourceGroups/<group>/providers/Microsoft.Insights/components/<name>"
```

`auth = "oidc"` works in GitHub Actions only. On your machine, override it for one run with
`AAYU_INTEGRATIONS__AZURE__AUTH=cli`; your `az login` must then be read-only on the component, or `doctor` refuses
it. `auth = "key"` takes a client secret of the same app registration from `aayu auth set-secret azure`.

## Check it

```sh
aayu doctor
aayu measure --live azure --from 2026-09-01 --to 2026-10-01
```

`doctor` reports `azure: reachable as application <client ID> (GitHub Actions OIDC); read-only scope verified`.
A filter in a baseline (`azure:users Name == 'signed_up'`) is one KQL predicate on `AppEvents`; a `|` or `;` in
it is refused, because the filter comes from an issue and is untrusted.
