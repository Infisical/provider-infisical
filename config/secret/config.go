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
}
