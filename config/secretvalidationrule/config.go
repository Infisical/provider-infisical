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

		// key_constraints and value_constraints share these field names, and
		// the docs scraper keeps only the first (key) description for each, so
		// value fields would be documented as key fields. Drop them so both
		// use the description from the Terraform schema instead.
		if r.MetaResource != nil {
			for _, f := range []string{"min_length", "max_length", "regex_pattern", "required_prefix", "required_suffix"} {
				delete(r.MetaResource.ArgumentDocs, f)
			}
		}
	})
}
