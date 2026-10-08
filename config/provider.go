/*
Copyright 2021 Upbound Inc.
*/

package config

import (
	// Note(turkenh): we are importing this to embed provider schema document
	_ "embed"

	ujconfig "github.com/crossplane/upjet/v2/pkg/config"
	"github.com/infisical/provider-infisical/config/group"
	"github.com/infisical/provider-infisical/config/identity"
	"github.com/infisical/provider-infisical/config/project"
	"github.com/infisical/provider-infisical/config/secret"
	"github.com/infisical/provider-infisical/config/secretsync"
)

const (
	resourcePrefix = "infisical"
	modulePath     = "github.com/infisical/provider-infisical"
)

//go:embed schema.json
var providerSchema string

//go:embed provider-metadata.yaml
var providerMetadata string

// GetProvider returns provider configuration
func GetProvider() *ujconfig.Provider {
	pc := ujconfig.NewProvider([]byte(providerSchema), resourcePrefix, modulePath, []byte(providerMetadata),
		ujconfig.WithRootGroup("crossplane.infisical.com"),
		ujconfig.WithIncludeList(ExternalNameConfigured()),
		ujconfig.WithFeaturesPackage("internal/features"),
		ujconfig.WithDefaultResourceOptions(
			ExternalNameConfigurations(),
			APIVersions(),
		))

	for _, configure := range []func(provider *ujconfig.Provider){
		// add custom config functions
		project.Configure,
		identity.Configure,
		group.Configure,
		secretsync.Configure,
		secret.Configure,
	} {
		configure(pc)
	}

	pc.ConfigureResources()
	if err := configureConversions(pc); err != nil {
		panic(err)
	}
	return pc
}

// APIVersions configures the API versions of every resource. v1alpha2 is
// generated from the schema of the normal Terraform provider release, and it
// is the hub and storage version. v1alpha1 was generated from the
// Crossplane-specific legacy Terraform build. Its types are kept frozen in
// apis/*/v1alpha1, and it is still served through the conversion webhook.
func APIVersions() ujconfig.ResourceOption {
	return func(r *ujconfig.Resource) {
		r.Version = "v1alpha2"
		r.PreviousVersions = []string{"v1alpha1"}
		r.SetCRDStorageVersion("v1alpha2")
	}
}
