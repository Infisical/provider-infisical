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

type ProjectUserInitParameters struct {

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

	// (String) JSON array of role assignments for this user. Each role object must include a role_slug field. Example: [{"role_slug":"admin"},{"role_slug":"member"}].
	// JSON array of role assignments for this user. Each role object must include a `role_slug` field. Example: `[{"role_slug":"admin"},{"role_slug":"member"}]`.
	Roles *string `json:"roles,omitempty" tf:"roles,omitempty"`

	// (String) The usename of the user. By default its the email
	// The usename of the user. By default its the email
	Username *string `json:"username,omitempty" tf:"username,omitempty"`
}

type ProjectUserObservation struct {
	ID *string `json:"id,omitempty" tf:"id,omitempty"`

	// (String) The membershipId of the project user
	// The membershipId of the project user
	MembershipID *string `json:"membershipId,omitempty" tf:"membership_id,omitempty"`

	// (String) The id of the project
	// The id of the project
	ProjectID *string `json:"projectId,omitempty" tf:"project_id,omitempty"`

	// (String) JSON array of role assignments for this user. Each role object must include a role_slug field. Example: [{"role_slug":"admin"},{"role_slug":"member"}].
	// JSON array of role assignments for this user. Each role object must include a `role_slug` field. Example: `[{"role_slug":"admin"},{"role_slug":"member"}]`.
	Roles *string `json:"roles,omitempty" tf:"roles,omitempty"`

	// (String) The usename of the user. By default its the email
	// The usename of the user. By default its the email
	Username *string `json:"username,omitempty" tf:"username,omitempty"`
}

type ProjectUserParameters struct {

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

	// (String) JSON array of role assignments for this user. Each role object must include a role_slug field. Example: [{"role_slug":"admin"},{"role_slug":"member"}].
	// JSON array of role assignments for this user. Each role object must include a `role_slug` field. Example: `[{"role_slug":"admin"},{"role_slug":"member"}]`.
	// +kubebuilder:validation:Optional
	Roles *string `json:"roles,omitempty" tf:"roles,omitempty"`

	// (String) The usename of the user. By default its the email
	// The usename of the user. By default its the email
	// +kubebuilder:validation:Optional
	Username *string `json:"username,omitempty" tf:"username,omitempty"`
}

// ProjectUserSpec defines the desired state of ProjectUser
type ProjectUserSpec struct {
	v1.ResourceSpec `json:",inline"`
	ForProvider     ProjectUserParameters `json:"forProvider"`
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
	InitProvider ProjectUserInitParameters `json:"initProvider,omitempty"`
}

// ProjectUserStatus defines the observed state of ProjectUser.
type ProjectUserStatus struct {
	v1.ResourceStatus `json:",inline"`
	AtProvider        ProjectUserObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion

// ProjectUser is the Schema for the ProjectUsers API. Create project users & save to Infisical. Only Machine Identity authentication is supported for this resource
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,infisical}
type ProjectUser struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// +kubebuilder:validation:XValidation:rule="!('*' in self.managementPolicies || 'Create' in self.managementPolicies || 'Update' in self.managementPolicies) || has(self.forProvider.roles) || (has(self.initProvider) && has(self.initProvider.roles))",message="spec.forProvider.roles is a required parameter"
	// +kubebuilder:validation:XValidation:rule="!('*' in self.managementPolicies || 'Create' in self.managementPolicies || 'Update' in self.managementPolicies) || has(self.forProvider.username) || (has(self.initProvider) && has(self.initProvider.username))",message="spec.forProvider.username is a required parameter"
	Spec   ProjectUserSpec   `json:"spec"`
	Status ProjectUserStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ProjectUserList contains a list of ProjectUsers
type ProjectUserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProjectUser `json:"items"`
}

// Repository type metadata.
var (
	ProjectUser_Kind             = "ProjectUser"
	ProjectUser_GroupKind        = schema.GroupKind{Group: CRDGroup, Kind: ProjectUser_Kind}.String()
	ProjectUser_KindAPIVersion   = ProjectUser_Kind + "." + CRDGroupVersion.String()
	ProjectUser_GroupVersionKind = CRDGroupVersion.WithKind(ProjectUser_Kind)
)

func init() {
	SchemeBuilder.Register(&ProjectUser{}, &ProjectUserList{})
}
