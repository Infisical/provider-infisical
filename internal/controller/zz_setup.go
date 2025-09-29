// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/upjet/pkg/controller"

	identity "github.com/infisical/provider-infisical/internal/controller/identity/identity"
	kubernetesauth "github.com/infisical/provider-infisical/internal/controller/identity/kubernetesauth"
	universalauth "github.com/infisical/provider-infisical/internal/controller/identity/universalauth"
	project "github.com/infisical/provider-infisical/internal/controller/project/project"
	projectenvironment "github.com/infisical/provider-infisical/internal/controller/project/projectenvironment"
	projectgroup "github.com/infisical/provider-infisical/internal/controller/project/projectgroup"
	projectidentity "github.com/infisical/provider-infisical/internal/controller/project/projectidentity"
	projectrole "github.com/infisical/provider-infisical/internal/controller/project/projectrole"
	projectuser "github.com/infisical/provider-infisical/internal/controller/project/projectuser"
	providerconfig "github.com/infisical/provider-infisical/internal/controller/providerconfig"
	secret "github.com/infisical/provider-infisical/internal/controller/secret/secret"
)

// Setup creates all controllers with the supplied logger and adds them to
// the supplied manager.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	for _, setup := range []func(ctrl.Manager, controller.Options) error{
		identity.Setup,
		kubernetesauth.Setup,
		universalauth.Setup,
		project.Setup,
		projectenvironment.Setup,
		projectgroup.Setup,
		projectidentity.Setup,
		projectrole.Setup,
		projectuser.Setup,
		providerconfig.Setup,
		secret.Setup,
	} {
		if err := setup(mgr, o); err != nil {
			return err
		}
	}
	return nil
}
