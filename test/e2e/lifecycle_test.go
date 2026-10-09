//go:build e2e

/*
Copyright 2026 Infisical Inc.
*/

package e2e

import (
	"context"
	"testing"
)

// TestLifecycle creates every kind in both API versions in Infisical, with the
// provider under test, and checks the full lifecycle:
//
//   - create: the object becomes Ready and gets an external name;
//   - read: the object can be read in the other API version, with the
//     converted fields, if its kind has both versions;
//   - update: a change of the object reaches Infisical;
//   - delete: the object and the Infisical resource are deleted.
func TestLifecycle(t *testing.T) {
	for _, version := range versions {
		t.Run(version, func(t *testing.T) {
			t.Parallel()
			s := newSet(version, "e2e-life-"+shortVersion(version), "default")
			orphan := func(o object) bool { return o.onlySynced(providerUnderTest) != "" }

			if t.Run("create", func(t *testing.T) { s.createAll(t, providerUnderTest) }) {
				t.Run("reconcile", func(t *testing.T) {
					for _, o := range s.objects() {
						t.Run(o.testName(), func(t *testing.T) {
							t.Parallel()
							s.waitReconciled(t, o, providerUnderTest)
							checkExternalName(t, s, o, providerUnderTest)
						})
					}
				})
				t.Run("read-as-"+otherVersion(version), func(t *testing.T) {
					for _, o := range s.objects() {
						if !o.hasVersion(otherVersion(version)) {
							continue
						}
						t.Run(o.testName(), func(t *testing.T) {
							t.Parallel()
							checkOtherVersion(t, s, o)
						})
					}
				})
				t.Run("update", func(t *testing.T) {
					for _, o := range s.objects() {
						t.Run(o.testName(), func(t *testing.T) {
							t.Parallel()
							if reason := o.onlySynced(providerUnderTest); reason != "" {
								t.Skip(reason)
							}
							s.update(t, o, version)
						})
					}
				})
			}
			t.Run("delete", func(t *testing.T) { s.deleteAll(t, orphan) })
		})
	}
}

// checkExternalName checks that a ready object has an external name, which is
// the ID of the resource in Infisical.
func checkExternalName(t *testing.T, s *set, o object, createdBy provider) {
	t.Helper()
	if o.onlySynced(createdBy) != "" {
		return
	}
	u, err := s.get(context.Background(), o, s.version)
	if err != nil {
		t.Fatal(err)
	}
	if u.GetAnnotations()["crossplane.io/external-name"] == "" {
		t.Error("the object is ready but has no external name")
	}
}

// checkOtherVersion reads the object in the other API version and checks the
// converted fields.
func checkOtherVersion(t *testing.T, s *set, o object) {
	t.Helper()
	other := otherVersion(s.version)
	u, err := s.get(context.Background(), o, other)
	if err != nil {
		t.Fatalf("cannot read as %s: %v", other, err)
	}
	checks := o.up
	if s.version == v1alpha2 {
		checks = o.down
	}
	checkValues(t, s, u, checks, "read as "+other)
}

func shortVersion(version string) string {
	return map[string]string{v1alpha1: "v1a1", v1alpha2: "v1a2"}[version]
}
