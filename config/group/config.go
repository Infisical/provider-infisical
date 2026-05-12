package group

import "github.com/crossplane/upjet/pkg/config"

func Configure(p *config.Provider) {
	p.AddResourceConfigurator("infisical_group", func(r *config.Resource) {
		r.Kind = "Group"
		r.ShortGroup = "group"
	})
}
