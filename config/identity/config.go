package identity

import "github.com/crossplane/upjet/pkg/config"

func Configure(p *config.Provider) {
	p.AddResourceConfigurator("infisical_identity", func(r *config.Resource) {
		r.Kind = "Identity"
		r.ShortGroup = "identity"
	})

	p.AddResourceConfigurator("infisical_identity_universal_auth", func(r *config.Resource) {
		r.Kind = "UniversalAuth"
		r.ShortGroup = "identity"
		r.ExternalName.OmittedFields = []string{"access_token_trusted_ips", "client_secret_trusted_ips"}
	})
}
