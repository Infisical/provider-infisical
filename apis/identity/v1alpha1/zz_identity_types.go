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

type IdentityInitParameters struct {

	// (String) The name for the identity
	// The name for the identity
	Name *string `json:"name,omitempty" tf:"name,omitempty"`

	// (String) The ID of the organization for the identity
	// The ID of the organization for the identity
	OrgID *string `json:"orgId,omitempty" tf:"org_id,omitempty"`

	// access'. If you've created custom roles, you can use their slugs as well.
	// The role for the identity. Available default role options are 'admin', 'member', and 'no-access'. If you've created custom roles, you can use their slugs as well.
	Role *string `json:"role,omitempty" tf:"role,omitempty"`
}

type IdentityObservation struct {

	// (List of String) The authentication types of the identity
	// The authentication types of the identity
	AuthModes []*string `json:"authModes,omitempty" tf:"auth_modes,omitempty"`

	// (String) The ID of the identity
	ID *string `json:"id,omitempty" tf:"id,omitempty"`

	// (String) The name for the identity
	// The name for the identity
	Name *string `json:"name,omitempty" tf:"name,omitempty"`

	// (String) The ID of the organization for the identity
	// The ID of the organization for the identity
	OrgID *string `json:"orgId,omitempty" tf:"org_id,omitempty"`

	// access'. If you've created custom roles, you can use their slugs as well.
	// The role for the identity. Available default role options are 'admin', 'member', and 'no-access'. If you've created custom roles, you can use their slugs as well.
	Role *string `json:"role,omitempty" tf:"role,omitempty"`
}

type IdentityParameters struct {

	// (String) The name for the identity
	// The name for the identity
	// +kubebuilder:validation:Optional
	Name *string `json:"name,omitempty" tf:"name,omitempty"`

	// (String) The ID of the organization for the identity
	// The ID of the organization for the identity
	// +kubebuilder:validation:Optional
	OrgID *string `json:"orgId,omitempty" tf:"org_id,omitempty"`

	// access'. If you've created custom roles, you can use their slugs as well.
	// The role for the identity. Available default role options are 'admin', 'member', and 'no-access'. If you've created custom roles, you can use their slugs as well.
	// +kubebuilder:validation:Optional
	Role *string `json:"role,omitempty" tf:"role,omitempty"`
}

// IdentitySpec defines the desired state of Identity
type IdentitySpec struct {
	v1.ResourceSpec `json:",inline"`
	ForProvider     IdentityParameters `json:"forProvider"`
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
	InitProvider IdentityInitParameters `json:"initProvider,omitempty"`
}

// IdentityStatus defines the observed state of Identity.
type IdentityStatus struct {
	v1.ResourceStatus `json:",inline"`
	AtProvider        IdentityObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion

// Identity is the Schema for the Identitys API. Create and manage identity in Infisical.
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,infisical}
type Identity struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// +kubebuilder:validation:XValidation:rule="!('*' in self.managementPolicies || 'Create' in self.managementPolicies || 'Update' in self.managementPolicies) || has(self.forProvider.name) || (has(self.initProvider) && has(self.initProvider.name))",message="spec.forProvider.name is a required parameter"
	// +kubebuilder:validation:XValidation:rule="!('*' in self.managementPolicies || 'Create' in self.managementPolicies || 'Update' in self.managementPolicies) || has(self.forProvider.orgId) || (has(self.initProvider) && has(self.initProvider.orgId))",message="spec.forProvider.orgId is a required parameter"
	// +kubebuilder:validation:XValidation:rule="!('*' in self.managementPolicies || 'Create' in self.managementPolicies || 'Update' in self.managementPolicies) || has(self.forProvider.role) || (has(self.initProvider) && has(self.initProvider.role))",message="spec.forProvider.role is a required parameter"
	Spec   IdentitySpec   `json:"spec"`
	Status IdentityStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// IdentityList contains a list of Identitys
type IdentityList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Identity `json:"items"`
}

// Repository type metadata.
var (
	Identity_Kind             = "Identity"
	Identity_GroupKind        = schema.GroupKind{Group: CRDGroup, Kind: Identity_Kind}.String()
	Identity_KindAPIVersion   = Identity_Kind + "." + CRDGroupVersion.String()
	Identity_GroupVersionKind = CRDGroupVersion.WithKind(Identity_Kind)
)

func init() {
	SchemeBuilder.Register(&Identity{}, &IdentityList{})
}
