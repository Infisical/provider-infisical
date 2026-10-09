package appconnection

import (
	"github.com/crossplane/upjet/v2/pkg/config"
)

func Configure(p *config.Provider) {
	p.AddResourceConfigurator("infisical_app_connection_azure_client_secrets", func(r *config.Resource) {
		r.Kind = "AppConnectionAzureClientSecrets"
		r.ShortGroup = "appconnection" // lowercase not allowed

		r.References["project_id"] = config.Reference{
			TerraformName: "infisical_project",
		}
	})
}
