# End-to-end tests

These tests run the provider in a kind cluster with Crossplane, against a real
Infisical instance. They cover every kind of the provider in both API
versions: `v1alpha1`, which was generated from the Crossplane-specific legacy
Terraform build, and `v1alpha2`, which is generated from the normal Terraform
provider.

## Suites

`test/e2e/run.sh <suite>` creates the cluster, installs Crossplane and the
provider, runs the Go tests in this folder, and deletes everything it created,
also in Infisical.

| Suite | Go tests | What it checks |
|---|---|---|
| `install` | `TestConversion` | Every CRD serves both versions, stores `v1alpha2` and uses the conversion webhook. Every kind converts from `v1alpha1` to `v1alpha2` and back with exactly the fields that the client wrote (also the same JSON strings). A `v1alpha1` client can update a `v1alpha2` object without losing `v1alpha2`-only data. The webhook works again after the provider pod restarts. These tests do not call Infisical. |
| `install` | `TestLifecycle` | Every kind, in both versions, with the provider of this commit: it becomes Ready and gets an external name, reads correctly in the other version, a change reaches Infisical, and delete works. |
| `upgrade` | `TestUpgrade` | Existing users keep working. The released provider (`OLD_PROVIDER_PACKAGE`, default `v0.1.15`) creates every kind as `v1alpha1`. Then the provider is upgraded in place. Every object must stay Ready with the same external name (so no resource is recreated), read correctly as `v1alpha2`, accept an update as `v1alpha1`, and delete. |

CI runs both suites with Crossplane 1.16.5 (the oldest supported version) and
2.4.2. See `.github/workflows/e2e-tests.yml`.

## Environment variables

The tests fail when one of these is not set.

| Variable | Value |
|---|---|
| `INFISICAL_HOST` | URL of the Infisical instance. The kind cluster must reach it. For a local instance use `http://host.docker.internal:8080`. |
| `INFISICAL_CLIENT_ID`, `INFISICAL_CLIENT_SECRET` | Universal auth credentials of a machine identity that is an org admin. |
| `INFISICAL_ORG_ID` | ID of the organization of that identity. |
| `INFISICAL_USER_EMAIL` | Email of an existing user in the organization (for ProjectUser and the approval policies). |
| `INFISICAL_GITHUB_CONNECTION_ID` | ID of a GitHub App connection in the organization (for SecretSyncGithub). |
| `INFISICAL_GITHUB_REPO_OWNER`, `INFISICAL_GITHUB_REPO_NAME` | The repository that the test secret sync uses. The sync never deletes secrets in the repository, and the synced environment has no secrets. |

In CI, they come from the repository secrets with the prefix `E2E_`, for
example `E2E_INFISICAL_HOST`.

Every run has its own run ID, which is part of the names of the Infisical
resources. If a run cannot clean up, it prints the run ID: delete the
Infisical resources with that ID in their name by hand.

## Run the tests locally

```sh
make build
cat > .work/e2e.env <<EOF
INFISICAL_HOST=http://host.docker.internal:8080
INFISICAL_CLIENT_ID=...
INFISICAL_CLIENT_SECRET=...
INFISICAL_ORG_ID=...
INFISICAL_USER_EMAIL=...
INFISICAL_GITHUB_CONNECTION_ID=...
INFISICAL_GITHUB_REPO_OWNER=...
INFISICAL_GITHUB_REPO_NAME=...
EOF
E2E_ENV_FILE=$PWD/.work/e2e.env make test-e2e E2E_SUITE=install CROSSPLANE_VERSION=2.4.2
E2E_ENV_FILE=$PWD/.work/e2e.env make test-e2e E2E_SUITE=upgrade CROSSPLANE_VERSION=1.16.5
```

Set `KEEP_CLUSTER=true` to keep the kind cluster after the run. The test
results (JSON and JUnit) and, after a failure, the diagnostics (provider logs,
managed resources, events) are in `_output/e2e/<suite>-<Crossplane version>/`.

## Known gaps

The tests document these as reasons in their output:

- **KubernetesAuth** cannot become ready: Infisical calls the Kubernetes API
  of `kubernetesHost` when the auth method is created, and the test has no
  Kubernetes API that Infisical can reach. The tests only check that the
  provider reconciles it.
- **ProjectRole** cannot be created with the normal Terraform provider: its
  Read returns an error instead of removing the resource when the role is not
  found. Existing ProjectRoles keep working, which the upgrade suite checks.
- **Secret**: the released provider cannot create a new Secret (its first
  refresh reads the secret with an empty ID). The provider of this commit can,
  which the upgrade suite checks.
