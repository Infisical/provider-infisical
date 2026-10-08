//go:build e2e

/*
Copyright 2026 Infisical Inc.
*/

package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// conversionProviderConfig does not exist, so the provider never calls
// Infisical for the objects of the conversion tests. These tests only use the
// Kubernetes API and the conversion webhook.
const conversionProviderConfig = "e2e-no-provider-config"

// TestConversion tests the API conversion webhook between v1alpha1 and
// v1alpha2 for every kind:
//
//   - every CRD serves both versions, stores v1alpha2 and uses the webhook;
//   - a v1alpha1 object reads as v1alpha2 with the converted fields, and back
//     as v1alpha1 with exactly the fields that the client wrote;
//   - a v1alpha2 object reads as v1alpha1 with the converted fields, and a
//     v1alpha1 client can update it without losing v1alpha2-only data;
//   - the webhook works again after the provider pod restarts.
func TestConversion(t *testing.T) {
	t.Run("CRDs", testCRDs)

	sets := map[string]*set{
		v1alpha1: newSet(v1alpha1, "e2e-conv-v1a1", conversionProviderConfig),
		v1alpha2: newSet(v1alpha2, "e2e-conv-v1a2", conversionProviderConfig),
	}
	fixtures := map[string]map[string]*unstructured.Unstructured{}
	for _, version := range versions {
		s := sets[version]
		fixtures[version] = map[string]*unstructured.Unstructured{}
		for _, o := range objects {
			u := s.render(t, o)
			if err := unstructured.SetNestedField(u.Object, "Orphan", "spec", "deletionPolicy"); err != nil {
				t.Fatal(err)
			}
			if err := kube.Create(context.Background(), u.DeepCopy()); err != nil {
				t.Fatalf("cannot create %s %s as %s: %v", o.kind, s.name(o), version, err)
			}
			fixtures[version][o.file] = u
		}
	}
	t.Cleanup(func() {
		for _, version := range versions {
			for _, o := range objects {
				u, err := sets[version].get(context.Background(), o, version)
				if err == nil {
					_ = kube.Delete(context.Background(), u)
				}
			}
		}
	})

	t.Run("created-as-v1alpha1", func(t *testing.T) {
		s := sets[v1alpha1]
		for _, o := range objects {
			t.Run(o.testName(), func(t *testing.T) {
				t.Parallel()
				checkCreatedAsV1alpha1(t, s, o, fixtures[v1alpha1][o.file])
			})
		}
	})
	t.Run("created-as-v1alpha2", func(t *testing.T) {
		s := sets[v1alpha2]
		for _, o := range objects {
			t.Run(o.testName(), func(t *testing.T) {
				t.Parallel()
				checkCreatedAsV1alpha2(t, s, o, fixtures[v1alpha2][o.file])
			})
		}
	})
	t.Run("after-provider-restart", func(t *testing.T) {
		restartProvider(t)
		for _, version := range versions {
			s := sets[version]
			for _, o := range objects {
				t.Run(version+"/"+o.testName(), func(t *testing.T) {
					for _, read := range versions {
						if _, err := s.get(context.Background(), o, read); err != nil {
							t.Errorf("cannot read the %s object as %s: %v", version, read, err)
						}
					}
				})
			}
		}
	})
}

// testCRDs checks the versions and the conversion settings of every CRD.
func testCRDs(t *testing.T) {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{Group: "apiextensions.k8s.io", Version: "v1", Kind: "CustomResourceDefinitionList"})
	if err := kube.List(context.Background(), list); err != nil {
		t.Fatalf("cannot list the CRDs: %v", err)
	}
	crds := map[string]map[string]any{}
	for _, crd := range list.Items {
		group, _, _ := unstructured.NestedString(crd.Object, "spec", "group")
		kind, _, _ := unstructured.NestedString(crd.Object, "spec", "names", "kind")
		crds[group+"/"+kind] = crd.Object
	}
	for _, o := range kinds() {
		t.Run(o.kind, func(t *testing.T) {
			crd, ok := crds[o.group+"/"+o.kind]
			if !ok {
				t.Fatalf("no CRD for %s/%s", o.group, o.kind)
			}
			served := map[string]bool{}
			storage := ""
			list, _, _ := unstructured.NestedSlice(crd, "spec", "versions")
			for _, item := range list {
				v, _ := item.(map[string]any)
				name, _ := v["name"].(string)
				served[name], _ = v["served"].(bool)
				if s, _ := v["storage"].(bool); s {
					storage = name
				}
			}
			for _, version := range versions {
				if !served[version] {
					t.Errorf("%s is not served", version)
				}
			}
			if storage != v1alpha2 {
				t.Errorf("the storage version is %q, want %s", storage, v1alpha2)
			}
			if strategy, _, _ := unstructured.NestedString(crd, "spec", "conversion", "strategy"); strategy != "Webhook" {
				t.Errorf("the conversion strategy is %q, want Webhook", strategy)
			}
			if ca, _, _ := unstructured.NestedString(crd, "spec", "conversion", "webhook", "clientConfig", "caBundle"); ca == "" {
				t.Error("the conversion webhook has no CA bundle: Crossplane did not set up the webhook")
			}
			if svc, _, _ := unstructured.NestedString(crd, "spec", "conversion", "webhook", "clientConfig", "service", "name"); svc == "" {
				t.Error("the conversion webhook has no service: Crossplane did not set up the webhook")
			}
		})
	}
}

// checkCreatedAsV1alpha1 checks an object that a client created as v1alpha1.
func checkCreatedAsV1alpha1(t *testing.T, s *set, o object, fixture *unstructured.Unstructured) {
	ctx := context.Background()
	asV2, err := s.get(ctx, o, v1alpha2)
	if err != nil {
		t.Fatalf("cannot read as v1alpha2: %v", err)
	}
	want := forProvider(fixture)
	if o.changedShape() {
		checkValues(t, s, asV2, o.up, "read as v1alpha2")
	} else if got := forProvider(asV2); !contains(got, want) {
		t.Errorf("read as v1alpha2, spec.forProvider is %s, want %s", jsonString(got), jsonString(want))
	}
	asV1, err := s.get(ctx, o, v1alpha1)
	if err != nil {
		t.Fatalf("cannot read as v1alpha1: %v", err)
	}
	// A v1alpha1 client gets back exactly what it wrote, also the same JSON
	// strings. Otherwise GitOps tools would see a change.
	if got := forProvider(asV1); jsonString(got) != jsonString(want) {
		t.Errorf("read back as v1alpha1, spec.forProvider changed:\n got %s\nwant %s", jsonString(got), jsonString(want))
	}
}

// checkCreatedAsV1alpha2 checks an object that a client created as v1alpha2,
// and then a v1alpha1 client updated.
func checkCreatedAsV1alpha2(t *testing.T, s *set, o object, fixture *unstructured.Unstructured) {
	ctx := context.Background()
	asV1, err := s.get(ctx, o, v1alpha1)
	if err != nil {
		t.Fatalf("cannot read as v1alpha1: %v", err)
	}
	if o.changedShape() {
		checkValues(t, s, asV1, o.down, "read as v1alpha1")
	}
	// A v1alpha1 client changes a label and writes the whole object back.
	labels := asV1.GetLabels()
	labels["e2e.infisical.com/updated-as"] = v1alpha1
	asV1.SetLabels(labels)
	if err := kube.Update(ctx, asV1); err != nil {
		t.Fatalf("cannot update as v1alpha1: %v", err)
	}
	asV2, err := s.get(ctx, o, v1alpha2)
	if err != nil {
		t.Fatalf("cannot read as v1alpha2: %v", err)
	}
	if got, want := forProvider(asV2), forProvider(fixture); jsonString(got) != jsonString(want) {
		t.Errorf("after an update as v1alpha1, the v1alpha2 spec.forProvider changed:\n got %s\nwant %s", jsonString(got), jsonString(want))
	}
}

// checkValues checks the values of the test case in an object.
func checkValues(t *testing.T, s *set, u *unstructured.Unstructured, checks []check, context string) {
	t.Helper()
	p := pave(u)
	for _, c := range checks {
		got, err := p.GetValue(c.path)
		if err != nil {
			t.Errorf("%s, %s: %v", context, c.path, err)
			continue
		}
		if want := s.renderJSON(t, c.want); !contains(got, want) {
			t.Errorf("%s, %s is %s, want %s", context, c.path, jsonString(got), jsonString(want))
		}
	}
}

func forProvider(u *unstructured.Unstructured) any {
	v, _, _ := unstructured.NestedFieldNoCopy(u.Object, "spec", "forProvider")
	return v
}

// restartProvider deletes the provider pod and waits until a new pod is
// ready. The provider is only ready when its conversion webhook has started.
func restartProvider(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	selector := client.MatchingLabels{"pkg.crossplane.io/provider": "provider-infisical"}
	pods := &corev1.PodList{}
	if err := kube.List(ctx, pods, client.InNamespace(providerNamespace), selector); err != nil || len(pods.Items) == 0 {
		t.Fatalf("cannot find the provider pod: %v", err)
	}
	old := map[string]bool{}
	for i := range pods.Items {
		old[pods.Items[i].Name] = true
		if err := kube.Delete(ctx, &pods.Items[i]); err != nil && !apierrors.IsNotFound(err) {
			t.Fatalf("cannot delete the provider pod: %v", err)
		}
	}
	t.Log("deleted the provider pod, waiting for a new ready pod")
	waitFor(t, 5*time.Minute, func() (bool, string) {
		if err := kube.List(ctx, pods, client.InNamespace(providerNamespace), selector); err != nil {
			return false, err.Error()
		}
		for _, p := range pods.Items {
			if old[p.Name] || p.DeletionTimestamp != nil {
				continue
			}
			for _, c := range p.Status.Conditions {
				if c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue {
					return true, ""
				}
			}
			return false, fmt.Sprintf("pod %s is not ready", p.Name)
		}
		return false, "no new provider pod yet"
	})
}
