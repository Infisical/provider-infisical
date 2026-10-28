package secretvalidationrule

import (
	"github.com/crossplane/upjet/v2/pkg/config"
)

func Configure(p *config.Provider) {
	p.AddResourceConfigurator("infisical_secret_validation_rule_static_secrets", func(r *config.Resource) {
		r.Kind = "SecretValidationRuleStaticSecrets"
		r.ShortGroup = "secretvalidationrule"

		r.References["project_id"] = config.Reference{
			TerraformName: "infisical_project",
		}
	})
}
