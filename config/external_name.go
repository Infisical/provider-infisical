/*
Copyright 2022 Upbound Inc.
*/

package config

import (
	"github.com/crossplane/upjet/pkg/config"
)

// ExternalNameConfigs contains all external name configurations for this
// provider.
var ExternalNameConfigs = map[string]config.ExternalName{
	"infisical_project":                 config.IdentifierFromProvider,
	"infisical_identity":                config.IdentifierFromProvider,
	"infisical_project_environment":     config.IdentifierFromProvider,
	"infisical_identity_universal_auth": config.IdentifierFromProvider,
	"infisical_project_identity":        config.ParameterAsIdentifier("membershipId"),
}

// ExternalNameConfigurations applies all external name configs listed in the
// table ExternalNameConfigs and sets the version of those resources to v1beta1
// assuming they will be tested.
func ExternalNameConfigurations() config.ResourceOption {
	return func(r *config.Resource) {
		if e, ok := ExternalNameConfigs[r.Name]; ok {
			r.ExternalName = e
		}
	}
}

// ExternalNameConfigured returns the list of all resources whose external name
// is configured manually.
func ExternalNameConfigured() []string {
	l := make([]string, len(ExternalNameConfigs))
	i := 0
	for name := range ExternalNameConfigs {
		// $ is added to match the exact string since the format is regex.
		l[i] = name + "$"
		i++
	}
	return l
}
