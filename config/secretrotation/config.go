package secretrotation

import (
	"github.com/crossplane/upjet/v2/pkg/config"
)

func Configure(p *config.Provider) {
	p.AddResourceConfigurator("infisical_secret_rotation_azure_client_secret", func(r *config.Resource) {
		r.Kind = "SecretRotationAzureClientSecret"
		r.ShortGroup = "secretrotation"

		r.References["connection_id"] = config.Reference{
			TerraformName: "infisical_app_connection_azure_client_secrets",
		}

		r.References["project_id"] = config.Reference{
			TerraformName: "infisical_project",
		}
	})
}
