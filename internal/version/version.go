// Package version contains values that the build sets with -ldflags.
package version

var (
	// Version of the provider.
	Version string

	// TerraformVersion is the version of the Terraform CLI in the provider
	// image. The build sets it from TERRAFORM_VERSION in the Makefile.
	TerraformVersion string

	// TerraformProviderSource is the registry source of the Terraform
	// provider, for example "Infisical/infisical". The build sets it from
	// TERRAFORM_PROVIDER_SOURCE in the Makefile.
	TerraformProviderSource string

	// TerraformCrossplaneSpecificLegacyVersion is the version of the
	// Crossplane-specific legacy build of the Terraform provider in the
	// provider image. The v1alpha1 resources use it. The build sets it from
	// TERRAFORM_CROSSPLANE_SPECIFIC_LEGACY_VERSION in the Makefile.
	TerraformCrossplaneSpecificLegacyVersion string
)
