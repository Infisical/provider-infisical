//go:build e2e

/*
Copyright 2026 Infisical Inc.
*/

// Package e2e tests the provider in a real Kubernetes cluster with Crossplane,
// against a real Infisical instance. test/e2e/run.sh prepares the cluster and
// runs these tests. See test/e2e/README.md.
package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/crossplane/crossplane-runtime/v2/pkg/fieldpath"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/yaml"
)

const (
	v1alpha1 = "v1alpha1"
	v1alpha2 = "v1alpha2"

	// providerNamespace is the namespace of Crossplane and the provider.
	providerNamespace = "crossplane-system"
	// runLabel marks every object of a test run, for the final cleanup.
	runLabel = "e2e.infisical.com/run"

	pollInterval     = 5 * time.Second
	reconcileTimeout = 10 * time.Minute
)

var versions = []string{v1alpha1, v1alpha2}

// settings are the settings of the test run, from environment variables.
var settings struct {
	runID              string
	orgID              string
	userEmail          string
	githubConnectionID string
	githubRepoOwner    string
	githubRepoName     string
}

var kube client.Client

func TestMain(m *testing.M) {
	// run.sh also checks these, and the INFISICAL_HOST, INFISICAL_CLIENT_ID and
	// INFISICAL_CLIENT_SECRET for the ProviderConfig.
	var missing []string
	get := func(name string) string {
		v := os.Getenv(name)
		if v == "" {
			missing = append(missing, name)
		}
		return v
	}
	settings.runID = get("E2E_RUN_ID")
	settings.orgID = get("INFISICAL_ORG_ID")
	settings.userEmail = get("INFISICAL_USER_EMAIL")
	settings.githubConnectionID = get("INFISICAL_GITHUB_CONNECTION_ID")
	settings.githubRepoOwner = get("INFISICAL_GITHUB_REPO_OWNER")
	settings.githubRepoName = get("INFISICAL_GITHUB_REPO_NAME")
	if len(missing) > 0 {
		fmt.Fprintf(os.Stderr, "missing required environment variables: %s\n", strings.Join(missing, ", "))
		os.Exit(1)
	}

	cfg, err := ctrlconfig.GetConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot get the kubeconfig: %v\n", err)
		os.Exit(1)
	}
	s := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(s); err != nil {
		fmt.Fprintf(os.Stderr, "cannot build the scheme: %v\n", err)
		os.Exit(1)
	}
	if kube, err = client.New(cfg, client.Options{Scheme: s}); err != nil {
		fmt.Fprintf(os.Stderr, "cannot create the Kubernetes client: %v\n", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// set is the group of test objects of one API version that a test creates.
type set struct {
	version        string
	prefix         string
	providerConfig string
	projectID      string
}

func newSet(version, prefix, providerConfig string) *set {
	return &set{version: version, prefix: prefix, providerConfig: providerConfig}
}

// replacer returns the values of the placeholders in the test fixtures.
func (s *set) replacer() *strings.Replacer {
	projectID := s.projectID
	if projectID == "" {
		projectID = "00000000-0000-0000-0000-000000000000"
	}
	return strings.NewReplacer(
		"__PREFIX__", s.prefix,
		"__RUN_ID__", settings.runID,
		"__PROVIDER_CONFIG__", s.providerConfig,
		"__PROJECT_ID__", projectID,
		"__ORG_ID__", settings.orgID,
		"__USER_EMAIL__", settings.userEmail,
		"__GITHUB_CONNECTION_ID__", settings.githubConnectionID,
		"__GITHUB_REPO_OWNER__", settings.githubRepoOwner,
		"__GITHUB_REPO_NAME__", settings.githubRepoName,
	)
}

// render returns the test fixture of the object in the API version of the set.
func (s *set) render(t *testing.T, o object) *unstructured.Unstructured {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", s.version, o.file+".yaml"))
	if err != nil {
		t.Fatalf("cannot read the fixture: %v", err)
	}
	u := &unstructured.Unstructured{}
	if err := yaml.Unmarshal([]byte(s.replacer().Replace(string(raw))), &u.Object); err != nil {
		t.Fatalf("cannot parse the fixture %s/%s: %v", s.version, o.file, err)
	}
	return u
}

// renderJSON replaces the placeholders in a JSON value of a test case and
// parses it.
func (s *set) renderJSON(t *testing.T, raw string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(s.replacer().Replace(raw)), &v); err != nil {
		t.Fatalf("invalid JSON in a test case %q: %v", raw, err)
	}
	return v
}

// name returns the Kubernetes name of the object in this set.
func (s *set) name(o object) string {
	return s.prefix + "-" + o.name
}

// get returns the object, read in the given API version.
func (s *set) get(ctx context.Context, o object, version string) (*unstructured.Unstructured, error) {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{Group: o.group, Version: version, Kind: o.kind})
	err := kube.Get(ctx, types.NamespacedName{Name: s.name(o)}, u)
	return u, err
}

// create creates the object and, for a Secret, the Kubernetes Secret with
// its value.
func (s *set) create(t *testing.T, o object) {
	t.Helper()
	ctx := context.Background()
	if o.kind == "Secret" {
		value := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: s.prefix + "-secret-value", Namespace: providerNamespace, Labels: map[string]string{runLabel: settings.runID}},
			StringData: map[string]string{"value": "e2e-value"},
		}
		if err := kube.Create(ctx, value); err != nil && !apierrors.IsAlreadyExists(err) {
			t.Fatalf("cannot create the value of the Secret: %v", err)
		}
	}
	u := s.render(t, o)
	if err := kube.Create(ctx, u); err != nil {
		t.Fatalf("cannot create %s %s as %s: %v", o.kind, s.name(o), s.version, err)
	}
	t.Logf("created %s %s as %s", o.kind, s.name(o), s.version)
}

// createAll creates all objects: first the parents, then the others. The
// Secret has no reference to the Project, so it is created when the Project
// has an ID.
func (s *set) createAll(t *testing.T, createdBy provider) {
	t.Helper()
	for _, o := range objects {
		if o.parent {
			s.create(t, o)
		}
	}
	for _, o := range objects {
		if o.parent {
			s.waitReconciled(t, o, createdBy)
		}
	}
	project, err := s.get(context.Background(), objectByFile("project"), s.version)
	if err != nil {
		t.Fatalf("cannot get the Project: %v", err)
	}
	s.projectID, _ = fieldString(project, "status.atProvider.id")
	for _, o := range objects {
		if !o.parent {
			s.create(t, o)
		}
	}
}

// waitReconciled waits until the provider reconciled the object: Ready and
// Synced, or only a Synced condition for objects that cannot become ready in
// the test.
func (s *set) waitReconciled(t *testing.T, o object, createdBy provider) {
	t.Helper()
	reason := o.onlySynced(createdBy)
	if reason != "" {
		t.Logf("%s only needs a Synced condition: %s", o.kind, reason)
	}
	waitFor(t, reconcileTimeout, func() (bool, string) {
		u, err := s.get(context.Background(), o, s.version)
		if err != nil {
			return false, err.Error()
		}
		ready, readyMsg := condition(u, "Ready")
		synced, syncedMsg := condition(u, "Synced")
		if reason != "" {
			return synced != "", "no Synced condition yet"
		}
		return ready == "True" && synced == "True", fmt.Sprintf("Ready=%s %s / Synced=%s %s", ready, readyMsg, synced, syncedMsg)
	})
}

// update applies the update of the test case and waits until Infisical
// reports the new value.
func (s *set) update(t *testing.T, o object, version string) {
	t.Helper()
	up := o.update[version]
	if up == nil {
		t.Skip("no update test for this kind in this version")
	}
	ctx := context.Background()
	patch, err := json.Marshal(map[string]any{"spec": map[string]any{"forProvider": s.renderJSON(t, up.set)}})
	if err != nil {
		t.Fatal(err)
	}
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{Group: o.group, Version: version, Kind: o.kind})
	u.SetName(s.name(o))
	if err := kube.Patch(ctx, u, client.RawPatch(types.MergePatchType, patch)); err != nil {
		t.Fatalf("cannot update %s: %v", o.kind, err)
	}
	want := s.renderJSON(t, up.want)
	waitFor(t, reconcileTimeout, func() (bool, string) {
		got, err := s.get(ctx, o, version)
		if err != nil {
			return false, err.Error()
		}
		v, _ := fieldpath.Pave(got.Object).GetValue(up.check)
		if !contains(v, want) {
			return false, fmt.Sprintf("%s is %s, want %s", up.check, jsonString(v), jsonString(want))
		}
		synced, msg := condition(got, "Synced")
		return synced == "True", "Synced=" + synced + " " + msg
	})
}

// delete deletes the object and waits until it is gone. Objects that were
// never created in Infisical are orphaned, because their Terraform resource
// cannot be read.
func (s *set) delete(t *testing.T, o object, orphan bool) {
	t.Helper()
	ctx := context.Background()
	u, err := s.get(ctx, o, s.version)
	if apierrors.IsNotFound(err) {
		return
	}
	if err != nil {
		t.Fatalf("cannot get %s: %v", o.kind, err)
	}
	if orphan {
		if err := kube.Patch(ctx, u, client.RawPatch(types.MergePatchType, []byte(`{"spec":{"deletionPolicy":"Orphan"}}`))); err != nil {
			t.Fatalf("cannot set the Orphan deletion policy: %v", err)
		}
	}
	if err := kube.Delete(ctx, u); err != nil && !apierrors.IsNotFound(err) {
		t.Fatalf("cannot delete %s: %v", o.kind, err)
	}
	waitFor(t, reconcileTimeout, func() (bool, string) {
		got, err := s.get(ctx, o, s.version)
		if apierrors.IsNotFound(err) {
			return true, ""
		}
		if err != nil {
			return false, err.Error()
		}
		_, msg := condition(got, "Synced")
		return false, "still exists: " + msg
	})
}

// deleteAll deletes the objects of the set: first the objects that reference
// others, then the parents. When a parent is deleted first, Infisical deletes
// the children with it, and some children then cannot be read any more.
func (s *set) deleteAll(t *testing.T, orphan func(object) bool) {
	t.Helper()
	for _, parents := range []bool{false, true} {
		t.Run(map[bool]string{false: "children", true: "parents"}[parents], func(t *testing.T) {
			for _, o := range objects {
				if o.parent != parents {
					continue
				}
				t.Run(o.testName(), func(t *testing.T) {
					t.Parallel()
					s.delete(t, o, orphan(o))
				})
			}
		})
	}
}

// waitFor polls cond until it returns true. It fails the test with the last
// message of cond after the timeout.
func waitFor(t *testing.T, timeout time.Duration, cond func() (bool, string)) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var msg string
	for {
		var ok bool
		if ok, msg = cond(); ok {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out after %s: %s", timeout, msg)
		}
		time.Sleep(pollInterval)
	}
}

// condition returns the status and the message of a condition.
func condition(u *unstructured.Unstructured, conditionType string) (string, string) {
	conditions, _, _ := unstructured.NestedSlice(u.Object, "status", "conditions")
	for _, c := range conditions {
		m, ok := c.(map[string]any)
		if !ok || m["type"] != conditionType {
			continue
		}
		status, _ := m["status"].(string)
		message, _ := m["message"].(string)
		return status, message
	}
	return "", ""
}

func pave(u *unstructured.Unstructured) *fieldpath.Paved {
	return fieldpath.Pave(u.Object)
}

func fieldString(u *unstructured.Unstructured, path string) (string, error) {
	return pave(u).GetString(path)
}

// contains reports whether got holds want: objects must have all keys of want
// with matching values, lists must have the same length and matching items. A
// JSON string in got is parsed when want is not a string, so that the JSON
// strings of v1alpha1 can be checked.
func contains(got, want any) bool {
	got, want = normalize(got), normalize(want)
	if s, ok := got.(string); ok {
		if _, wantString := want.(string); !wantString {
			var parsed any
			if json.Unmarshal([]byte(s), &parsed) != nil {
				return false
			}
			got = parsed
		}
	}
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			return false
		}
		for k, v := range w {
			if !contains(g[k], v) {
				return false
			}
		}
		return true
	case []any:
		g, ok := got.([]any)
		if !ok || len(g) != len(w) {
			return false
		}
		for i := range w {
			if !contains(g[i], w[i]) {
				return false
			}
		}
		return true
	case string:
		g, ok := got.(string)
		if !ok {
			return false
		}
		// Two JSON strings match when they hold the same data.
		var gv, wv any
		if json.Unmarshal([]byte(g), &gv) == nil && json.Unmarshal([]byte(w), &wv) == nil {
			if _, isObject := wv.(map[string]any); isObject {
				return jsonString(gv) == jsonString(wv)
			}
		}
		return g == w
	default:
		return jsonString(got) == jsonString(want)
	}
}

// normalize returns v as it would be after a JSON round trip, so that numbers
// of different Go types compare equal.
func normalize(v any) any {
	raw, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out any
	if json.Unmarshal(raw, &out) != nil {
		return v
	}
	return out
}

func jsonString(v any) string {
	raw, err := json.Marshal(normalize(v))
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(raw)
}

func otherVersion(version string) string {
	if version == v1alpha1 {
		return v1alpha2
	}
	return v1alpha1
}

// kinds returns the distinct kinds of the test objects, sorted.
func kinds() []object {
	seen := map[string]bool{}
	var out []object
	for _, o := range objects {
		if !seen[o.kind] {
			seen[o.kind] = true
			out = append(out, o)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].kind < out[j].kind })
	return out
}
