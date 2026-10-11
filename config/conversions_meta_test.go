package config

import "testing"

func TestMetadataAndStatusAreKept(t *testing.T) {
	s := setup(t)
	v1 := decode(t, s, `
apiVersion: identity.crossplane.infisical.com/v1alpha1
kind: UniversalAuth
metadata:
  name: ua
  annotations:
    crossplane.io/external-name: 3b2b1c0e-0000-0000-0000-000000000001
    crossplane.io/external-create-succeeded: "2026-10-08T18:18:52Z"
spec:
  forProvider:
    identityId: i
status:
  atProvider:
    id: 3b2b1c0e-0000-0000-0000-000000000001
    identityId: i
`)
	v2 := up(t, s, v1)
	if got := v2.GetAnnotations()["crossplane.io/external-name"]; got != "3b2b1c0e-0000-0000-0000-000000000001" {
		t.Errorf("v1alpha2 external name: got %q, annotations %v", got, v2.GetAnnotations())
	}
	if got := jsonOf(t, value(t, v2, "status.atProvider.id")); got != `"3b2b1c0e-0000-0000-0000-000000000001"` {
		t.Errorf("v1alpha2 status.atProvider.id: got %s", got)
	}
	back := down(t, s, v2)
	if got := back.GetAnnotations()["crossplane.io/external-name"]; got != "3b2b1c0e-0000-0000-0000-000000000001" {
		t.Errorf("v1alpha1 external name after a round trip: got %q, annotations %v", got, back.GetAnnotations())
	}
}

// TestNullAnnotation checks that an annotation with the value "null" counts as
// no saved values, and does not break the conversion.
func TestNullAnnotation(t *testing.T) {
	s := setup(t)
	v1 := decode(t, s, `
apiVersion: project.crossplane.infisical.com/v1alpha1
kind: ProjectIdentity
metadata:
  name: pi
  annotations:
    conversion.crossplane.infisical.com/fields: "null"
spec:
  forProvider:
    projectId: p
    identityId: i
    roles: '[{"role_slug":"admin"}]'
`)
	v2 := up(t, s, v1)
	if got, want := jsonOf(t, value(t, v2, "spec.forProvider.roles")), `[{"roleSlug":"admin"}]`; got != want {
		t.Errorf("v1alpha2 roles: want %s, got %s", want, got)
	}
	if got, want := value(t, down(t, s, v2), "spec.forProvider.roles"), `[{"role_slug":"admin"}]`; got != want {
		t.Errorf("v1alpha1 roles after a round trip: want %s, got %v", want, got)
	}
}
