package secret

import (
	"github.com/crossplane/upjet/pkg/config"
)

func Configure(p *config.Provider) {
	p.AddResourceConfigurator("infisical_secret", func(r *config.Resource) {
		r.Kind = "Secret"
		r.ShortGroup = "secret"
		r.ExternalName.OmittedFields = []string{"secret_reminder"}
	})

	p.AddResourceConfigurator("infisical_secret_folder", func(r *config.Resource) {
		r.Kind = "SecretFolder"
		r.ShortGroup = "secret"

		r.References["project_id"] = config.Reference{
			TerraformName: "infisical_project",
		}
	})
}
