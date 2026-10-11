package config

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/fieldpath"
	ujconversion "github.com/crossplane/upjet/v2/pkg/controller/conversion"
	"github.com/crossplane/upjet/v2/pkg/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
	"sigs.k8s.io/yaml"

	"github.com/infisical/provider-infisical/apis"
	conv "github.com/infisical/provider-infisical/config/conversion"
)

var registerOnce sync.Once

// setup registers the conversions of the provider configuration in upjet's global conversion registry, which the generated ConvertTo and ConvertFrom functions use
func setup(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := apis.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	registerOnce.Do(func() {
		if err := ujconversion.RegisterConversions(GetProvider(), nil, s); err != nil {
			t.Fatal(err)
		}
	})
	return s
}

// decode returns a new typed object of the scheme for the YAML manifest
func decode(t *testing.T, s *runtime.Scheme, manifest string) resource.Terraformed {
	t.Helper()
	raw, err := yaml.YAMLToJSON([]byte(manifest))
	if err != nil {
		t.Fatal(err)
	}
	var meta struct {
		APIVersion string `json:"apiVersion"`
		Kind       string `json:"kind"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatal(err)
	}
	gv := strings.SplitN(meta.APIVersion, "/", 2)
	obj, err := s.New(schema.GroupVersionKind{Group: gv[0], Version: gv[1], Kind: meta.Kind})
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, obj); err != nil {
		t.Fatal(err)
	}
	return obj.(resource.Terraformed)
}

// newObject returns an empty typed object of the same kind in the given version
func newObject(t *testing.T, s *runtime.Scheme, like resource.Terraformed, version string) resource.Terraformed {
	t.Helper()
	gvk := like.GetObjectKind().GroupVersionKind()
	gvk.Version = version
	obj, err := s.New(gvk)
	if err != nil {
		t.Fatal(err)
	}
	obj.GetObjectKind().SetGroupVersionKind(gvk)
	return obj.(resource.Terraformed)
}

// up converts a v1alpha1 object to v1alpha2, the same way the webhook does
func up(t *testing.T, s *runtime.Scheme, src resource.Terraformed) resource.Terraformed {
	t.Helper()
	dst := newObject(t, s, src, conv.VersionV1alpha2)
	if err := src.(conversion.Convertible).ConvertTo(dst.(conversion.Hub)); err != nil {
		t.Fatalf("v1alpha1 to v1alpha2: %v", err)
	}
	return dst
}

// down converts a v1alpha2 object to v1alpha1, the same way the webhook does
func down(t *testing.T, s *runtime.Scheme, src resource.Terraformed) resource.Terraformed {
	t.Helper()
	dst := newObject(t, s, src, conv.VersionV1alpha1)
	if err := dst.(conversion.Convertible).ConvertFrom(src.(conversion.Hub)); err != nil {
		t.Fatalf("v1alpha2 to v1alpha1: %v", err)
	}
	return dst
}

func value(t *testing.T, obj resource.Terraformed, path string) any {
	t.Helper()
	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		t.Fatal(err)
	}
	v, err := fieldpath.Pave(u).GetValue(path)
	if err != nil && !fieldpath.IsNotFound(err) {
		t.Fatal(err)
	}
	return v
}

func jsonOf(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// normalizedJSON returns the JSON of v. A JSON string is parsed first, so that key order and spacing do not matter
func normalizedJSON(t *testing.T, v any) string {
	t.Helper()
	if s, ok := v.(string); ok {
		var parsed any
		if err := json.Unmarshal([]byte(s), &parsed); err == nil {
			v = parsed
		}
	}
	return jsonOf(t, v)
}

func TestConversions(t *testing.T) {
	s := setup(t)
	cases := map[string]struct {
		v1alpha1 string
		// want are the expected v1alpha2 values, keyed by field path
		want map[string]string
	}{
		"SecretSyncGithub": {
			v1alpha1: `
apiVersion: secretsync.crossplane.infisical.com/v1alpha1
kind: SecretSyncGithub
metadata: {name: sync}
spec:
  forProvider:
    name: sync
    connectionId: c
    environment: dev
    secretPath: /
    destinationConfig: '{"scope":"repository","repository_owner":"o","repository_name":"r"}'
    syncOptions: '{"initial_sync_behavior":"overwrite-destination","disable_secret_deletion":false,"key_schema":"INFISICAL_{{secretKey}}"}'
`,
			want: map[string]string{
				"spec.forProvider.destinationConfig": `{"repositoryName":"r","repositoryOwner":"o","scope":"repository"}`,
				"spec.forProvider.syncOptions":       `{"disableSecretDeletion":false,"initialSyncBehavior":"overwrite-destination","keySchema":"INFISICAL_{{secretKey}}"}`,
			},
		},
		"ProjectIdentity": {
			v1alpha1: `
apiVersion: project.crossplane.infisical.com/v1alpha1
kind: ProjectIdentity
metadata: {name: pi}
spec:
  forProvider:
    projectId: p
    identityId: i
    roles: '[{"role_slug": "admin"}]'
`,
			want: map[string]string{"spec.forProvider.roles": `[{"roleSlug":"admin"}]`},
		},
		"ProjectTemplate": {
			v1alpha1: `
apiVersion: project.crossplane.infisical.com/v1alpha1
kind: ProjectTemplate
metadata: {name: pt}
spec:
  forProvider:
    name: pt
    type: secret-manager
    environments: '[{"name":"development","slug":"dev","position":1}]'
    roles: '[{"name":"Test","slug":"test","permissions":[{"action":["read","edit"],"subject":"secrets","conditions":{"environment":{"$eq":"dev"},"secretPath":{"$eq":"/"}}}]}]'
`,
			want: map[string]string{
				"spec.forProvider.environments": `[{"name":"development","position":1,"slug":"dev"}]`,
				"spec.forProvider.roles":        `[{"name":"Test","permissions":[{"action":["read","edit"],"conditions":"{\"environment\":{\"$eq\":\"dev\"},\"secretPath\":{\"$eq\":\"/\"}}","subject":"secrets"}],"slug":"test"}]`,
			},
		},
		"ProjectRole": {
			v1alpha1: `
apiVersion: project.crossplane.infisical.com/v1alpha1
kind: ProjectRole
metadata: {name: pr}
spec:
  forProvider:
    name: Tester
    slug: tester
    projectSlug: p
    permissions: '[{"action":["read","create"],"subject":"integrations"},{"action":["read"],"subject":"secrets","inverted":true,"conditions":{"environment":{"$eq":"dev"}}}]'
`,
			want: map[string]string{
				"spec.forProvider.permissionsV2": `[{"action":["read","create"],"subject":"integrations"},{"action":["read"],"conditions":"{\"environment\":{\"$eq\":\"dev\"}}","inverted":true,"subject":"secrets"}]`,
				"spec.forProvider.permissions":   `null`,
			},
		},
		"AccessApprovalPolicy": {
			v1alpha1: `
apiVersion: project.crossplane.infisical.com/v1alpha1
kind: AccessApprovalPolicy
metadata: {name: ap}
spec:
  forProvider:
    projectId: p
    secretPath: /
    environmentSlugs: [prod]
    requiredApprovals: 1
    userApprovers: [admin@infisical.com]
    groupApprovers: [g1, g2]
    userBypassers: [admin@infisical.com]
`,
			want: map[string]string{
				"spec.forProvider.approvers":        `[{"type":"user","username":"admin@infisical.com"},{"id":"g1","type":"group"},{"id":"g2","type":"group"}]`,
				"spec.forProvider.bypassers":        `[{"type":"user","username":"admin@infisical.com"}]`,
				"spec.forProvider.environmentSlugs": `["prod"]`,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			v1 := decode(t, s, tc.v1alpha1)
			v2 := up(t, s, v1)
			for path, want := range tc.want {
				if got := normalizedJSON(t, value(t, v2, path)); got != want {
					t.Errorf("v1alpha2 %s:\nwant %s\ngot  %s", path, want, got)
				}
			}
			// v1alpha1 to v1alpha2 to v1alpha1 must give exactly the same
			// v1alpha1 parameters, also the same JSON strings.
			back := down(t, s, v2)
			if want, got := jsonOf(t, value(t, v1, "spec.forProvider")), jsonOf(t, value(t, back, "spec.forProvider")); got != want {
				t.Errorf("round trip changed spec.forProvider:\nwant %s\ngot  %s", want, got)
			}
		})
	}
}

// TestNoDataLoss checks that v1alpha2 values that v1alpha1 cannot show survive a v1alpha2 to v1alpha1 to v1alpha2 round trip, for example when a v1alpha1 client reads and updates an object
func TestNoDataLoss(t *testing.T) {
	s := setup(t)
	cases := map[string]string{
		"ProjectIdentityTemporaryRole": `
apiVersion: project.crossplane.infisical.com/v1alpha2
kind: ProjectIdentity
metadata: {name: pi}
spec:
  forProvider:
    projectId: p
    identityId: i
    adoptExisting: true
    roles:
    - roleSlug: viewer
      isTemporary: true
      temporaryMode: relative
      temporaryRange: 1h
`,
		"ApproverWithUserID": `
apiVersion: project.crossplane.infisical.com/v1alpha2
kind: SecretApprovalPolicy
metadata: {name: sp}
spec:
  forProvider:
    projectId: p
    secretPath: /
    environmentSlug: prod
    requiredApprovals: 1
    approvers:
    - type: user
      id: 3b2b1c0e-0000-0000-0000-000000000001
    - type: group
      id: g1
`,
		"ProjectRoleV1Permissions": `
apiVersion: project.crossplane.infisical.com/v1alpha2
kind: ProjectRole
metadata: {name: pr}
spec:
  forProvider:
    name: Tester
    slug: tester
    projectId: p
    permissions:
    - action: read
      subject: secrets
      conditions:
        environment: dev
`,
		"IdentityMetadata": `
apiVersion: identity.crossplane.infisical.com/v1alpha2
kind: Identity
metadata: {name: id}
spec:
  forProvider:
    name: id
    orgId: o
    role: member
    metadata:
    - key: team
      value: platform
`,
	}
	for name, manifest := range cases {
		t.Run(name, func(t *testing.T) {
			v2 := decode(t, s, manifest)
			back := up(t, s, down(t, s, v2))
			if want, got := jsonOf(t, value(t, v2, "spec.forProvider")), jsonOf(t, value(t, back, "spec.forProvider")); got != want {
				t.Errorf("round trip lost data in spec.forProvider:\nwant %s\ngot  %s", want, got)
			}
			if a := back.GetAnnotations()[conv.AnnotationKey]; strings.Contains(a, conv.VersionV1alpha2+":") {
				t.Errorf("the v1alpha2 object still keeps v1alpha2 values in the %s annotation: %s", conv.AnnotationKey, a)
			}
		})
	}
}

// TestSavedValuesAreNotUsedAfterAChange checks that the values that are kept
// in the annotation are only used while they still match the object.
func TestSavedValuesAreNotUsedAfterAChange(t *testing.T) {
	s := setup(t)
	v1 := decode(t, s, `
apiVersion: project.crossplane.infisical.com/v1alpha1
kind: ProjectIdentity
metadata: {name: pi}
spec:
  forProvider:
    projectId: p
    identityId: i
    roles: '[{"role_slug": "admin"}]'
`)
	v2 := up(t, s, v1)
	// A v1alpha2 client changes the role.
	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(v2)
	if err != nil {
		t.Fatal(err)
	}
	if err := fieldpath.Pave(u).SetValue("spec.forProvider.roles", []any{map[string]any{"roleSlug": "viewer"}}); err != nil {
		t.Fatal(err)
	}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u, v2); err != nil {
		t.Fatal(err)
	}
	got := value(t, down(t, s, v2), "spec.forProvider.roles")
	if want := `[{"role_slug":"viewer"}]`; got != want {
		t.Errorf("v1alpha1 roles after a change in v1alpha2: want %s, got %v", want, got)
	}
}

// stored returns the object after it is written as JSON and read back, the
// same way the API server stores it. omitempty drops the empty values.
func stored(t *testing.T, s *runtime.Scheme, obj resource.Terraformed) resource.Terraformed {
	t.Helper()
	raw, err := json.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}
	out := newObject(t, s, obj, obj.GetObjectKind().GroupVersionKind().Version)
	if err := json.Unmarshal(raw, out); err != nil {
		t.Fatal(err)
	}
	return out
}

// TestEmptyValuesAreKept checks that a v1alpha1 client gets back the empty
// JSON values that it wrote, after the object is stored as v1alpha2.
func TestEmptyValuesAreKept(t *testing.T) {
	s := setup(t)
	cases := map[string]struct {
		manifest string
		fields   map[string]string
	}{
		"EmptyList": {
			manifest: `
apiVersion: project.crossplane.infisical.com/v1alpha1
kind: ProjectTemplate
metadata: {name: pt}
spec:
  forProvider:
    name: pt
    roles: '[]'
    environments: '[{"name":"Dev","slug":"dev","position":1}]'
`,
			fields: map[string]string{"roles": `[]`, "environments": `[{"name":"Dev","slug":"dev","position":1}]`},
		},
		"EmptyString": {
			manifest: `
apiVersion: project.crossplane.infisical.com/v1alpha1
kind: ProjectTemplate
metadata: {name: pt}
spec:
  forProvider:
    name: pt
    roles: ''
`,
			fields: map[string]string{"roles": ``},
		},
		"EmptyObject": {
			manifest: `
apiVersion: secretsync.crossplane.infisical.com/v1alpha1
kind: SecretSyncGithub
metadata: {name: sg}
spec:
  forProvider:
    name: sg
    syncOptions: '{}'
    destinationConfig: '{"scope":"repository","repository_owner":"o","repository_name":"r"}'
`,
			fields: map[string]string{"syncOptions": `{}`, "destinationConfig": `{"scope":"repository","repository_owner":"o","repository_name":"r"}`},
		},
		"EmptyListOfProjectIdentity": {
			manifest: `
apiVersion: project.crossplane.infisical.com/v1alpha1
kind: ProjectIdentity
metadata: {name: pi}
spec:
  forProvider:
    projectId: p
    identityId: i
    roles: '[]'
`,
			fields: map[string]string{"roles": `[]`},
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			v1 := decode(t, s, c.manifest)
			back := down(t, s, stored(t, s, up(t, s, v1)))
			for field, want := range c.fields {
				got := value(t, back, "spec.forProvider."+field)
				if got != want {
					t.Errorf("spec.forProvider.%s read back as v1alpha1: want %q, got %s", field, want, jsonOf(t, got))
				}
			}
		})
	}
}

// TestEmptyValueIsNotUsedAfterAChange checks that a saved empty value is not
// used after a v1alpha2 client sets a value.
func TestEmptyValueIsNotUsedAfterAChange(t *testing.T) {
	s := setup(t)
	v1 := decode(t, s, `
apiVersion: project.crossplane.infisical.com/v1alpha1
kind: ProjectIdentity
metadata: {name: pi}
spec:
  forProvider:
    projectId: p
    identityId: i
    roles: '[]'
`)
	v2 := stored(t, s, up(t, s, v1))
	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(v2)
	if err != nil {
		t.Fatal(err)
	}
	if err := fieldpath.Pave(u).SetValue("spec.forProvider.roles", []any{map[string]any{"roleSlug": "viewer"}}); err != nil {
		t.Fatal(err)
	}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u, v2); err != nil {
		t.Fatal(err)
	}
	got := value(t, down(t, s, stored(t, s, v2)), "spec.forProvider.roles")
	if want := `[{"role_slug":"viewer"}]`; got != want {
		t.Errorf("v1alpha1 roles after a change in v1alpha2: want %s, got %v", want, got)
	}
}
