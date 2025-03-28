// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/upjet/pkg/controller"

	identity "github.com/infisical/provider-infisical/internal/controller/identity/identity"
	universalauth "github.com/infisical/provider-infisical/internal/controller/identity/universalauth"
	project "github.com/infisical/provider-infisical/internal/controller/project/project"
	projectenvironment "github.com/infisical/provider-infisical/internal/controller/project/projectenvironment"
	projectgroup "github.com/infisical/provider-infisical/internal/controller/project/projectgroup"
	projectidentity "github.com/infisical/provider-infisical/internal/controller/project/projectidentity"
	projectuser "github.com/infisical/provider-infisical/internal/controller/project/projectuser"
	providerconfig "github.com/infisical/provider-infisical/internal/controller/providerconfig"
)

// Setup creates all controllers with the supplied logger and adds them to
// the supplied manager.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	for _, setup := range []func(ctrl.Manager, controller.Options) error{
		identity.Setup,
		universalauth.Setup,
		project.Setup,
		projectenvironment.Setup,
		projectgroup.Setup,
		projectidentity.Setup,
		projectuser.Setup,
		providerconfig.Setup,
	} {
		if err := setup(mgr, o); err != nil {
			return err
		}
	}
	return nil
}
