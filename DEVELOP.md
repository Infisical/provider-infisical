# Develop

## Add a new resource from Terraform

The example adds `infisical_secret_sync_aws_parameter_store`.

1. **Find the resource name** in `config/schema.json`.
   If the resource is not there, increase `TERRAFORM_PROVIDER_VERSION` in the
   `Makefile`.

2. **Add the external name** in `config/external_name.go`:

   ```go
   "infisical_secret_sync_aws_parameter_store": config.IdentifierFromProvider,
   ```

   If the Terraform Read fails when the ID is empty, use
   `withPlaceholderID(config.IdentifierFromProvider)`.

3. **Set the kind, group and references** in `config/<group>/config.go`:

   ```go
   p.AddResourceConfigurator("infisical_secret_sync_aws_parameter_store", func(r *config.Resource) {
       r.Kind = "SecretSyncAwsParameterStore"
       r.ShortGroup = "secretsync"
       r.References["project_id"] = config.Reference{TerraformName: "infisical_project"}
   })
   ```

   For a new group, add a new `config/<group>/` package, and add its
   `Configure` to `config/provider.go`.

4. **Generate the code:**

   ```sh
   make generate
   ```

   New resources only get `v1alpha2`. Do not add them to `v1alpha1Resources`
   in `config/provider.go`.

5. **Build and test:**

   ```sh
   make reviewable
   make build
   ```

6. **Add e2e tests** (optional): add a fixture to `test/e2e/testdata/v1alpha2/`
   and an entry with `onlyV1alpha2: true` to `test/e2e/objects_test.go`.

7. **Commit** the config changes and all the generated files.

## Update the Terraform provider

1. Set `TERRAFORM_PROVIDER_VERSION` in the `Makefile`.
2. Run `make generate`.
3. Look at the changes in `package/crds/`. If a field of an existing kind is
   removed or changes its type, that is a breaking change for users.

Never change the files in `apis/*/v1alpha1/`.
