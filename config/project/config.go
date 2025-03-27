package project

import "github.com/crossplane/upjet/pkg/config"

func Configure(p *config.Provider) {
	p.AddResourceConfigurator("infisical_project", func(r *config.Resource) {
		r.Kind = "Project"
		r.ShortGroup = "project"
	})

	p.AddResourceConfigurator("infisical_project_environment", func(r *config.Resource) {
		r.Kind = "ProjectEnvironment"
		r.ShortGroup = "project"
	})

	p.AddResourceConfigurator("infisical_project_identity", func(r *config.Resource) {
		r.Kind = "ProjectIdentity"
		r.ShortGroup = "project"
	})

}
