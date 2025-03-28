/*
Copyright 2022 Upbound Inc.
*/

package config

import (
	"github.com/crossplane/upjet/pkg/config"
	"github.com/pkg/errors"
)

// ExternalNameConfigs contains all external name configurations for this
// provider.
var ExternalNameConfigs = map[string]config.ExternalName{
	"infisical_project":                 config.IdentifierFromProvider,
	"infisical_identity":                config.IdentifierFromProvider,
	"infisical_project_environment":     config.IdentifierFromProvider,
	"infisical_identity_universal_auth": config.IdentifierFromProvider,
	"infisical_project_identity": func() config.ExternalName {
		e := config.IdentifierFromProvider
		e.GetExternalNameFn = func(tfstate map[string]any) (string, error) {
			membershipID, ok := tfstate["membership_id"]
			if !ok {
				return "", errors.Errorf("no membership_id attribute found in tfstate")
			}

			if membershipID == nil {
				return "", nil
			}

			membershipIDStr, ok := membershipID.(string)
			if !ok {
				return "", errors.Errorf("membership_id attribute is not a string")
			}
			return membershipIDStr, nil
		}
		return e
	}(),
	"infisical_project_user": func() config.ExternalName {
		e := config.IdentifierFromProvider
		e.GetExternalNameFn = func(tfstate map[string]any) (string, error) {
			membershipID, ok := tfstate["membership_id"]
			if !ok {
				return "", errors.Errorf("no membership_id attribute found in tfstate")
			}

			if membershipID == nil {
				return "", nil
			}

			membershipIDStr, ok := membershipID.(string)
			if !ok {
				return "", errors.Errorf("membership_id attribute is not a string")
			}
			return membershipIDStr, nil
		}
		return e
	}(),
	"infisical_project_group": func() config.ExternalName {
		e := config.IdentifierFromProvider
		e.GetExternalNameFn = func(tfstate map[string]any) (string, error) {
			membershipID, ok := tfstate["membership_id"]
			if !ok {
				return "", errors.Errorf("no membership_id attribute found in tfstate")
			}

			if membershipID == nil {
				return "", nil
			}

			membershipIDStr, ok := membershipID.(string)
			if !ok {
				return "", errors.Errorf("membership_id attribute is not a string")
			}
			return membershipIDStr, nil
		}
		return e
	}(),
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
