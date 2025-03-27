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
	projectidentity "github.com/infisical/provider-infisical/internal/controller/project/projectidentity"
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
		projectidentity.Setup,
		providerconfig.Setup,
	} {
		if err := setup(mgr, o); err != nil {
			return err
		}
	}
	return nil
}
