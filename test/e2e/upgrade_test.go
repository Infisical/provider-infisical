//go:build e2e

/*
Copyright 2026 Infisical Inc.
*/

package e2e

import (
	"context"
	"os"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// snapshotName is the ConfigMap that keeps the external names of the objects
// from before the upgrade.
const snapshotName = "e2e-upgrade-snapshot"

// TestUpgrade checks that existing users keep working after they upgrade the
// provider in place. run.sh runs it twice:
//
//   - E2E_UPGRADE_PHASE=before, with the released provider: create every
//     kind as v1alpha1, wait until it is reconciled, and save the external
//     names.
//   - E2E_UPGRADE_PHASE=after, after run.sh upgraded to the provider under
//     test: every object must stay Ready with the same external name (so no
//     resource was recreated), read correctly as v1alpha2, accept an update
//     as v1alpha1, and delete.
func TestUpgrade(t *testing.T) {
	s := newSet(v1alpha1, "e2e-upgrade", "default")
	switch phase := os.Getenv("E2E_UPGRADE_PHASE"); phase {
	case "before":
		testBeforeUpgrade(t, s)
	case "after":
		testAfterUpgrade(t, s)
	default:
		t.Skip("set E2E_UPGRADE_PHASE to before or after; run.sh does this for the upgrade suite")
	}
}

func testBeforeUpgrade(t *testing.T, s *set) {
	if !t.Run("create", func(t *testing.T) { s.createAll(t, providerReleased) }) {
		return
	}
	t.Run("reconcile", func(t *testing.T) {
		for _, o := range objects {
			t.Run(o.testName(), func(t *testing.T) {
				t.Parallel()
				s.waitReconciled(t, o, providerReleased)
			})
		}
	})
	snapshot := map[string]string{}
	for _, o := range objects {
		u, err := s.get(context.Background(), o, v1alpha1)
		if err != nil {
			t.Fatalf("cannot read %s: %v", o.kind, err)
		}
		snapshot[o.file] = u.GetAnnotations()["crossplane.io/external-name"]
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: snapshotName, Namespace: providerNamespace, Labels: map[string]string{runLabel: settings.runID}},
		Data:       snapshot,
	}
	if err := kube.Create(context.Background(), cm); err != nil {
		t.Fatalf("cannot save the external names: %v", err)
	}
	t.Logf("saved the external names before the upgrade: %v", snapshot)
}

func testAfterUpgrade(t *testing.T, s *set) {
	cm := &corev1.ConfigMap{}
	if err := kube.Get(context.Background(), types.NamespacedName{Name: snapshotName, Namespace: providerNamespace}, cm); err != nil {
		t.Fatalf("cannot read the external names from before the upgrade: %v", err)
	}
	// The released provider could not create some objects. The provider
	// under test creates them now.
	createdBy := func(o object) provider {
		if cm.Data[o.file] != "" {
			return providerReleased
		}
		return providerUnderTest
	}
	orphan := func(o object) bool { return o.onlySynced(createdBy(o)) != "" }

	t.Run("reconcile", func(t *testing.T) {
		for _, o := range objects {
			t.Run(o.testName(), func(t *testing.T) {
				t.Parallel()
				s.waitReconciled(t, o, createdBy(o))
				before := cm.Data[o.file]
				if before == "" {
					checkExternalName(t, s, o, createdBy(o))
					return
				}
				u, err := s.get(context.Background(), o, v1alpha1)
				if err != nil {
					t.Fatal(err)
				}
				if after := u.GetAnnotations()["crossplane.io/external-name"]; after != before {
					t.Errorf("the external name changed from %q to %q: the resource was recreated", before, after)
				}
			})
		}
	})
	t.Run("read-as-v1alpha2", func(t *testing.T) {
		for _, o := range objects {
			t.Run(o.testName(), func(t *testing.T) {
				t.Parallel()
				checkOtherVersion(t, s, o)
			})
		}
	})
	// An update as v1alpha1 that reaches Infisical proves that the provider
	// under test reconciles the existing objects.
	t.Run("update-as-v1alpha1", func(t *testing.T) {
		for _, o := range objects {
			t.Run(o.testName(), func(t *testing.T) {
				t.Parallel()
				if reason := o.onlySynced(createdBy(o)); reason != "" {
					t.Skip(reason)
				}
				s.update(t, o, v1alpha1)
			})
		}
	})
	t.Run("delete", func(t *testing.T) { s.deleteAll(t, orphan) })
	if err := kube.Delete(context.Background(), cm); err != nil && !apierrors.IsNotFound(err) {
		t.Logf("cannot delete the snapshot: %v", err)
	}
}
