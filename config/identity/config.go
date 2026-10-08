package identity

import "github.com/crossplane/upjet/v2/pkg/config"

func Configure(p *config.Provider) {
	p.AddResourceConfigurator("infisical_identity", func(r *config.Resource) {
		r.Kind = "Identity"
		r.ShortGroup = "identity"
	})

	p.AddResourceConfigurator("infisical_identity_universal_auth", func(r *config.Resource) {
		r.Kind = "UniversalAuth"
		r.ShortGroup = "identity"
		r.References["identity_id"] = config.Reference{
			TerraformName: "infisical_identity",
		}
	})

	p.AddResourceConfigurator("infisical_identity_kubernetes_auth", func(r *config.Resource) {
		r.Kind = "KubernetesAuth"
		r.ShortGroup = "identity"
		r.References["identity_id"] = config.Reference{
			TerraformName: "infisical_identity",
		}
	})
}
