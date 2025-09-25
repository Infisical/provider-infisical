package project

import (
	"github.com/crossplane/upjet/pkg/config"
)

func Configure(p *config.Provider) {
	p.AddResourceConfigurator("infisical_project", func(r *config.Resource) {
		r.Kind = "Project"
		r.ShortGroup = "project"
	})

	p.AddResourceConfigurator("infisical_project_environment", func(r *config.Resource) {
		r.Kind = "ProjectEnvironment"
		r.ShortGroup = "project"
		r.References["project_id"] = config.Reference{
			TerraformName: "infisical_project",
		}
	})

	p.AddResourceConfigurator("infisical_project_identity", func(r *config.Resource) {
		r.Kind = "ProjectIdentity"
		r.ShortGroup = "project"
		r.References["project_id"] = config.Reference{
			TerraformName: "infisical_project",
		}
		r.References["identity_id"] = config.Reference{
			TerraformName: "infisical_identity",
		}
	})

	p.AddResourceConfigurator("infisical_project_user", func(r *config.Resource) {
		r.Kind = "ProjectUser"
		r.ShortGroup = "project"
		r.References["project_id"] = config.Reference{
			TerraformName: "infisical_project",
		}
	})

	p.AddResourceConfigurator("infisical_project_group", func(r *config.Resource) {
		r.Kind = "ProjectGroup"
		r.ShortGroup = "project"
		r.References["project_id"] = config.Reference{
			TerraformName: "infisical_project",
		}
	})

	p.AddResourceConfigurator("infisical_secret", func(r *config.Resource) {
		r.Kind = "Secret"
		r.ShortGroup = "secret"
		r.ExternalName.OmittedFields = []string{"secret_reminder"}
	})

	p.AddResourceConfigurator("infisical_project_role", func(r *config.Resource) {
		r.Kind = "ProjectRole"
		r.ShortGroup = "project"
	})
}
