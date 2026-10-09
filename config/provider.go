/*
Copyright 2021 Upbound Inc.
*/

package config

import (
	// Note(turkenh): we are importing this to embed provider schema document
	_ "embed"

	ujconfig "github.com/crossplane/upjet/v2/pkg/config"
	"github.com/infisical/provider-infisical/config/appconnection"
	"github.com/infisical/provider-infisical/config/group"
	"github.com/infisical/provider-infisical/config/identity"
	"github.com/infisical/provider-infisical/config/project"
	"github.com/infisical/provider-infisical/config/secret"
	"github.com/infisical/provider-infisical/config/secretrotation"
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
		secretrotation.Configure,
		appconnection.Configure,
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
// Resources that are not in v1alpha1Resources only have v1alpha2.
func APIVersions() ujconfig.ResourceOption {
	return func(r *ujconfig.Resource) {
		r.Version = "v1alpha2"
		if v1alpha1Resources[r.Name] {
			r.PreviousVersions = []string{"v1alpha1"}
		}
		r.SetCRDStorageVersion("v1alpha2")
	}
}

// v1alpha1Resources are the resources that have a frozen v1alpha1 API. Do not
// add new resources here.
var v1alpha1Resources = map[string]bool{
	"infisical_access_approval_policy":   true,
	"infisical_group":                    true,
	"infisical_identity":                 true,
	"infisical_identity_kubernetes_auth": true,
	"infisical_identity_universal_auth":  true,
	"infisical_project":                  true,
	"infisical_project_environment":      true,
	"infisical_project_group":            true,
	"infisical_project_identity":         true,
	"infisical_project_role":             true,
	"infisical_project_template":         true,
	"infisical_project_user":             true,
	"infisical_secret":                   true,
	"infisical_secret_approval_policy":   true,
	"infisical_secret_folder":            true,
	"infisical_secret_sync_github":       true,
}
