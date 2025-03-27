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

type UniversalAuthInitParameters struct {

	// The maximum lifetime for an access token in seconds. This value will be referenced at renewal time. Default: 2592000
	AccessTokenMaxTTL *float64 `json:"accessTokenMaxTtl,omitempty" tf:"access_token_max_ttl,omitempty"`

	// The maximum number of times that an access token can be used; a value of 0 implies infinite number of uses. Default:0
	AccessTokenNumUsesLimit *float64 `json:"accessTokenNumUsesLimit,omitempty" tf:"access_token_num_uses_limit,omitempty"`

	// The lifetime for an access token in seconds. This value will be referenced at renewal time. Default: 2592000
	AccessTokenTTL *float64 `json:"accessTokenTtl,omitempty" tf:"access_token_ttl,omitempty"`

	// The ID of the identity to attach the configuration onto.
	IdentityID *string `json:"identityId,omitempty" tf:"identity_id,omitempty"`
}

type UniversalAuthObservation struct {

	// The maximum lifetime for an access token in seconds. This value will be referenced at renewal time. Default: 2592000
	AccessTokenMaxTTL *float64 `json:"accessTokenMaxTtl,omitempty" tf:"access_token_max_ttl,omitempty"`

	// The maximum number of times that an access token can be used; a value of 0 implies infinite number of uses. Default:0
	AccessTokenNumUsesLimit *float64 `json:"accessTokenNumUsesLimit,omitempty" tf:"access_token_num_uses_limit,omitempty"`

	// The lifetime for an access token in seconds. This value will be referenced at renewal time. Default: 2592000
	AccessTokenTTL *float64 `json:"accessTokenTtl,omitempty" tf:"access_token_ttl,omitempty"`

	ID *string `json:"id,omitempty" tf:"id,omitempty"`

	// The ID of the identity to attach the configuration onto.
	IdentityID *string `json:"identityId,omitempty" tf:"identity_id,omitempty"`
}

type UniversalAuthParameters struct {

	// The maximum lifetime for an access token in seconds. This value will be referenced at renewal time. Default: 2592000
	// +kubebuilder:validation:Optional
	AccessTokenMaxTTL *float64 `json:"accessTokenMaxTtl,omitempty" tf:"access_token_max_ttl,omitempty"`

	// The maximum number of times that an access token can be used; a value of 0 implies infinite number of uses. Default:0
	// +kubebuilder:validation:Optional
	AccessTokenNumUsesLimit *float64 `json:"accessTokenNumUsesLimit,omitempty" tf:"access_token_num_uses_limit,omitempty"`

	// The lifetime for an access token in seconds. This value will be referenced at renewal time. Default: 2592000
	// +kubebuilder:validation:Optional
	AccessTokenTTL *float64 `json:"accessTokenTtl,omitempty" tf:"access_token_ttl,omitempty"`

	// The ID of the identity to attach the configuration onto.
	// +kubebuilder:validation:Optional
	IdentityID *string `json:"identityId,omitempty" tf:"identity_id,omitempty"`
}

// UniversalAuthSpec defines the desired state of UniversalAuth
type UniversalAuthSpec struct {
	v1.ResourceSpec `json:",inline"`
	ForProvider     UniversalAuthParameters `json:"forProvider"`
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
	InitProvider UniversalAuthInitParameters `json:"initProvider,omitempty"`
}

// UniversalAuthStatus defines the observed state of UniversalAuth.
type UniversalAuthStatus struct {
	v1.ResourceStatus `json:",inline"`
	AtProvider        UniversalAuthObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion

// UniversalAuth is the Schema for the UniversalAuths API. <no value>
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,infisical}
type UniversalAuth struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// +kubebuilder:validation:XValidation:rule="!('*' in self.managementPolicies || 'Create' in self.managementPolicies || 'Update' in self.managementPolicies) || has(self.forProvider.identityId) || (has(self.initProvider) && has(self.initProvider.identityId))",message="spec.forProvider.identityId is a required parameter"
	Spec   UniversalAuthSpec   `json:"spec"`
	Status UniversalAuthStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// UniversalAuthList contains a list of UniversalAuths
type UniversalAuthList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []UniversalAuth `json:"items"`
}

// Repository type metadata.
var (
	UniversalAuth_Kind             = "UniversalAuth"
	UniversalAuth_GroupKind        = schema.GroupKind{Group: CRDGroup, Kind: UniversalAuth_Kind}.String()
	UniversalAuth_KindAPIVersion   = UniversalAuth_Kind + "." + CRDGroupVersion.String()
	UniversalAuth_GroupVersionKind = CRDGroupVersion.WithKind(UniversalAuth_Kind)
)

func init() {
	SchemeBuilder.Register(&UniversalAuth{}, &UniversalAuthList{})
}
