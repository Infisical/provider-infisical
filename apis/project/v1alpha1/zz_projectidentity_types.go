// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

// Code generated by upjet. DO NOT EDIT.

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

type ProjectIdentityInitParameters struct {

	// (String) The id of the identity.
	// The id of the identity.
	// +crossplane:generate:reference:type=github.com/infisical/provider-infisical/apis/identity/v1alpha1.Identity
	IdentityID *string `json:"identityId,omitempty" tf:"identity_id,omitempty"`

	// Reference to a Identity in identity to populate identityId.
	// +kubebuilder:validation:Optional
	IdentityIDRef *v1.Reference `json:"identityIdRef,omitempty" tf:"-"`

	// Selector for a Identity in identity to populate identityId.
	// +kubebuilder:validation:Optional
	IdentityIDSelector *v1.Selector `json:"identityIdSelector,omitempty" tf:"-"`

	// (String) The id of the project
	// The id of the project
	// +crossplane:generate:reference:type=github.com/infisical/provider-infisical/apis/project/v1alpha1.Project
	ProjectID *string `json:"projectId,omitempty" tf:"project_id,omitempty"`

	// Reference to a Project in project to populate projectId.
	// +kubebuilder:validation:Optional
	ProjectIDRef *v1.Reference `json:"projectIdRef,omitempty" tf:"-"`

	// Selector for a Project in project to populate projectId.
	// +kubebuilder:validation:Optional
	ProjectIDSelector *v1.Selector `json:"projectIdSelector,omitempty" tf:"-"`

	// (String) JSON array of role assignments for this identity. Each role object must include a role_slug field. Example: [{"role_slug":"admin"},{"role_slug":"member"}].
	// JSON array of role assignments for this identity. Each role object must include a `role_slug` field. Example: `[{"role_slug":"admin"},{"role_slug":"member"}]`.
	Roles *string `json:"roles,omitempty" tf:"roles,omitempty"`
}

type ProjectIdentityObservation struct {
	ID *string `json:"id,omitempty" tf:"id,omitempty"`

	// (String) The id of the identity.
	// The id of the identity.
	IdentityID *string `json:"identityId,omitempty" tf:"identity_id,omitempty"`

	// (String) The membership Id of the project identity
	// The membership Id of the project identity
	MembershipID *string `json:"membershipId,omitempty" tf:"membership_id,omitempty"`

	// (String) The id of the project
	// The id of the project
	ProjectID *string `json:"projectId,omitempty" tf:"project_id,omitempty"`

	// (String) JSON array of role assignments for this identity. Each role object must include a role_slug field. Example: [{"role_slug":"admin"},{"role_slug":"member"}].
	// JSON array of role assignments for this identity. Each role object must include a `role_slug` field. Example: `[{"role_slug":"admin"},{"role_slug":"member"}]`.
	Roles *string `json:"roles,omitempty" tf:"roles,omitempty"`
}

type ProjectIdentityParameters struct {

	// (String) The id of the identity.
	// The id of the identity.
	// +crossplane:generate:reference:type=github.com/infisical/provider-infisical/apis/identity/v1alpha1.Identity
	// +kubebuilder:validation:Optional
	IdentityID *string `json:"identityId,omitempty" tf:"identity_id,omitempty"`

	// Reference to a Identity in identity to populate identityId.
	// +kubebuilder:validation:Optional
	IdentityIDRef *v1.Reference `json:"identityIdRef,omitempty" tf:"-"`

	// Selector for a Identity in identity to populate identityId.
	// +kubebuilder:validation:Optional
	IdentityIDSelector *v1.Selector `json:"identityIdSelector,omitempty" tf:"-"`

	// (String) The id of the project
	// The id of the project
	// +crossplane:generate:reference:type=github.com/infisical/provider-infisical/apis/project/v1alpha1.Project
	// +kubebuilder:validation:Optional
	ProjectID *string `json:"projectId,omitempty" tf:"project_id,omitempty"`

	// Reference to a Project in project to populate projectId.
	// +kubebuilder:validation:Optional
	ProjectIDRef *v1.Reference `json:"projectIdRef,omitempty" tf:"-"`

	// Selector for a Project in project to populate projectId.
	// +kubebuilder:validation:Optional
	ProjectIDSelector *v1.Selector `json:"projectIdSelector,omitempty" tf:"-"`

	// (String) JSON array of role assignments for this identity. Each role object must include a role_slug field. Example: [{"role_slug":"admin"},{"role_slug":"member"}].
	// JSON array of role assignments for this identity. Each role object must include a `role_slug` field. Example: `[{"role_slug":"admin"},{"role_slug":"member"}]`.
	// +kubebuilder:validation:Optional
	Roles *string `json:"roles,omitempty" tf:"roles,omitempty"`
}

// ProjectIdentitySpec defines the desired state of ProjectIdentity
type ProjectIdentitySpec struct {
	v1.ResourceSpec `json:",inline"`
	ForProvider     ProjectIdentityParameters `json:"forProvider"`
	// THIS IS A BETA FIELD. It will be honored
	// unless the Management Policies feature flag is disabled.
	// InitProvider holds the same fields as ForProvider, with the exception
	// of Identifier and other resource reference fields. The fields that are
	// in InitProvider are merged into ForProvider when the resource is created.
	// The same fields are also added to the terraform ignore_changes hook, to
	// avoid updating them after creation. This is useful for fields that are
	// required on creation, but we do not desire to update them after creation,
	// for example because of an external controller is managing them, like an
	// autoscaler.
	InitProvider ProjectIdentityInitParameters `json:"initProvider,omitempty"`
}

// ProjectIdentityStatus defines the observed state of ProjectIdentity.
type ProjectIdentityStatus struct {
	v1.ResourceStatus `json:",inline"`
	AtProvider        ProjectIdentityObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion

// ProjectIdentity is the Schema for the ProjectIdentitys API. Create project identities & save to Infisical. Only Machine Identity authentication is supported for this data source
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,infisical}
type ProjectIdentity struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// +kubebuilder:validation:XValidation:rule="!('*' in self.managementPolicies || 'Create' in self.managementPolicies || 'Update' in self.managementPolicies) || has(self.forProvider.roles) || (has(self.initProvider) && has(self.initProvider.roles))",message="spec.forProvider.roles is a required parameter"
	Spec   ProjectIdentitySpec   `json:"spec"`
	Status ProjectIdentityStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ProjectIdentityList contains a list of ProjectIdentitys
type ProjectIdentityList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProjectIdentity `json:"items"`
}

// Repository type metadata.
var (
	ProjectIdentity_Kind             = "ProjectIdentity"
	ProjectIdentity_GroupKind        = schema.GroupKind{Group: CRDGroup, Kind: ProjectIdentity_Kind}.String()
	ProjectIdentity_KindAPIVersion   = ProjectIdentity_Kind + "." + CRDGroupVersion.String()
	ProjectIdentity_GroupVersionKind = CRDGroupVersion.WithKind(ProjectIdentity_Kind)
)

func init() {
	SchemeBuilder.Register(&ProjectIdentity{}, &ProjectIdentityList{})
}
