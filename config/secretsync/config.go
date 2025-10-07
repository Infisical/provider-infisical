package secretsync

import (
	"github.com/crossplane/upjet/pkg/config"
)

func Configure(p *config.Provider) {
	p.AddResourceConfigurator("infisical_secret_sync_github", func(r *config.Resource) {
		r.Kind = "SecretSyncGithub"
		r.ShortGroup = "secretsync" // lowercase not allowed

		r.References["project_id"] = config.Reference{
			TerraformName: "infisical_project",
		}
	})
}
