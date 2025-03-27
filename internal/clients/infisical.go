/*
Copyright 2021 Upbound Inc.
*/

package clients

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/upjet/pkg/terraform"

	"github.com/infisical/provider-infisical/apis/v1beta1"
)

const (
	// error messages
	errNoProviderConfig     = "no providerConfigRef provided"
	errGetProviderConfig    = "cannot get referenced ProviderConfig"
	errTrackUsage           = "cannot track ProviderConfig usage"
	errExtractCredentials   = "cannot extract credentials"
	errUnmarshalCredentials = "cannot unmarshal infisical credentials as JSON"
)

// TerraformSetupBuilder builds Terraform a terraform.SetupFn function which
// returns Terraform provider setup configuration
func TerraformSetupBuilder(version, providerSource, providerVersion string) terraform.SetupFn {
	return func(ctx context.Context, client client.Client, mg resource.Managed) (terraform.Setup, error) {
		ps := terraform.Setup{
			Version: version,
			Requirement: terraform.ProviderRequirement{
				Source:  providerSource,
				Version: providerVersion,
			},
			Configuration: map[string]any{},
		}

		configRef := mg.GetProviderConfigReference()
		if configRef == nil {
			return ps, errors.New(errNoProviderConfig)
		}
		pc := &v1beta1.ProviderConfig{}
		if err := client.Get(ctx, types.NamespacedName{Name: configRef.Name}, pc); err != nil {
			return ps, errors.Wrap(err, errGetProviderConfig)
		}

		t := resource.NewProviderConfigUsageTracker(client, &v1beta1.ProviderConfigUsage{})
		if err := t.Track(ctx, mg); err != nil {
			return ps, errors.Wrap(err, errTrackUsage)
		}

		data, err := resource.CommonCredentialExtractor(ctx, pc.Spec.Credentials.Source, client, pc.Spec.Credentials.CommonCredentialSelectors)
		if err != nil {
			return ps, errors.Wrap(err, errExtractCredentials)
		}

		if pc.Spec.Host != "" {
			ps.Configuration["host"] = pc.Spec.Host
		}

		creds := map[string]any{}
		if err := json.Unmarshal(data, &creds); err != nil {
			return ps, errors.Wrap(err, errUnmarshalCredentials)
		}

		authField, ok := creds["auth"]
		if !ok || authField == nil {
			return ps, errors.Wrap(fmt.Errorf("auth is not set"), errUnmarshalCredentials)
		}

		authMap, ok := authField.(map[string]any)
		if !ok {
			return ps, errors.Wrap(fmt.Errorf("auth is not a valid object"), errUnmarshalCredentials)
		}

		if universal, ok := authMap["universal"]; ok && universal != nil {
			universalMap, ok := universal.(map[string]any)
			if !ok {
				return ps, errors.Wrap(fmt.Errorf("universal auth is not a valid object"), errUnmarshalCredentials)
			}

			clientID, hasClientID := universalMap["client_id"]
			clientSecret, hasClientSecret := universalMap["client_secret"]

			if !hasClientID || !hasClientSecret {
				return ps, errors.Wrap(fmt.Errorf("missing client_id or client_secret in universal auth"), errUnmarshalCredentials)
			}

			ps.Configuration["auth"] = map[string]any{
				"universal": map[string]any{
					"client_id":     clientID,
					"client_secret": clientSecret,
				},
			}
		}

		return ps, nil
	}
}
