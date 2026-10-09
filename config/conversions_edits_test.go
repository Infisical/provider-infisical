package config

import (
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/fieldpath"
	"github.com/crossplane/upjet/v2/pkg/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

// setField sets the value at the path of a typed object, the same way a
// client changes it. A nil value deletes the field.
func setField(t *testing.T, obj resource.Terraformed, path string, v any) {
	t.Helper()
	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		t.Fatal(err)
	}
	p := fieldpath.Pave(u)
	if v == nil {
		err = p.DeleteField(path)
	} else {
		err = p.SetValue(path, v)
	}
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u, obj); err != nil {
		t.Fatal(err)
	}
}

const githubSyncV1alpha2 = `
apiVersion: secretsync.crossplane.infisical.com/v1alpha2
kind: SecretSyncGithub
metadata: {name: sg}
spec:
  forProvider:
    name: sg
    destinationConfig: {scope: repository, repositoryOwner: o, repositoryName: "100"}
`

// TestLegacyEditsAreKept checks that a change of a v1alpha1 client reaches
// v1alpha2, also when the old and the new value are the same number as JSON
// or differ only in an empty string.
func TestLegacyEditsAreKept(t *testing.T) {
	s := setup(t)
	cases := map[string]struct {
		v1alpha2 string
		path     string
		set      any
		check    string
		want     string
	}{
		"StringThatIsANumberInAJSONField": {
			v1alpha2: githubSyncV1alpha2,
			path:     "spec.forProvider.destinationConfig",
			set:      `{"repository_name":"1e2","repository_owner":"o","scope":"repository"}`,
			check:    "spec.forProvider.destinationConfig.repositoryName",
			want:     `"1e2"`,
		},
		"SpaceInAStringInAJSONField": {
			v1alpha2: githubSyncV1alpha2,
			path:     "spec.forProvider.destinationConfig",
			set:      `{"repository_name":" 100","repository_owner":"o","scope":"repository"}`,
			check:    "spec.forProvider.destinationConfig.repositoryName",
			want:     `" 100"`,
		},
		"StringThatIsANumberInAList": {
			v1alpha2: `
apiVersion: project.crossplane.infisical.com/v1alpha2
kind: AccessApprovalPolicy
metadata: {name: ap}
spec:
  forProvider:
    projectId: p
    secretPath: /
    environmentSlugs: [prod]
    requiredApprovals: 1
    approvers: [{type: user, username: "100"}]
`,
			path:  "spec.forProvider.userApprovers",
			set:   []any{"1e2"},
			check: "spec.forProvider.approvers",
			want:  `[{"type":"user","username":"1e2"}]`,
		},
		"RemovedEmptyString": {
			v1alpha2: `
apiVersion: secretsync.crossplane.infisical.com/v1alpha2
kind: SecretSyncGithub
metadata: {name: sg}
spec:
  forProvider:
    name: sg
    destinationConfig: {scope: repository, repositoryOwner: o, repositoryName: ""}
`,
			path:  "spec.forProvider.destinationConfig",
			set:   `{"repository_owner":"o","scope":"repository"}`,
			check: "spec.forProvider.destinationConfig.repositoryName",
			want:  `null`,
		},
		"StringThatIsANumberInOpaqueConditions": {
			v1alpha2: `
apiVersion: project.crossplane.infisical.com/v1alpha2
kind: ProjectTemplate
metadata: {name: pt}
spec:
  forProvider:
    name: pt
    roles:
    - name: r
      slug: r
      permissions:
      - action: [read]
        subject: secrets
        conditions: '{"environment":{"$eq":"100"}}'
`,
			path:  "spec.forProvider.roles",
			set:   `[{"name":"r","permissions":[{"action":["read"],"conditions":{"environment":{"$eq":"1e2"}},"subject":"secrets"}],"slug":"r"}]`,
			check: "spec.forProvider.roles[0].permissions[0].conditions",
			want:  `"{\"environment\":{\"$eq\":\"1e2\"}}"`,
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			v1 := down(t, s, stored(t, s, decode(t, s, c.v1alpha2)))
			setField(t, v1, c.path, c.set)
			if got := jsonOf(t, value(t, up(t, s, stored(t, s, v1)), c.check)); got != c.want {
				t.Errorf("v1alpha2 %s after the v1alpha1 change: want %s, got %s", c.check, c.want, got)
			}
		})
	}
}

// TestV1alpha2EditsAreShown checks that a v1alpha1 client sees a change of a
// v1alpha2 client, and not the JSON that it wrote before.
func TestV1alpha2EditsAreShown(t *testing.T) {
	s := setup(t)
	cases := map[string]struct {
		destinationConfig string
		repositoryName    string
		want              string
	}{
		"StringThatIsANumber": {
			destinationConfig: `{"scope":"repository","repository_owner":"o","repository_name":"100"}`,
			repositoryName:    "1e2",
			want:              `{"repository_name":"1e2","repository_owner":"o","scope":"repository"}`,
		},
		"AddedEmptyString": {
			destinationConfig: `{"scope":"repository","repository_owner":"o"}`,
			repositoryName:    "",
			want:              `{"repository_name":"","repository_owner":"o","scope":"repository"}`,
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			v2 := stored(t, s, up(t, s, decode(t, s, `
apiVersion: secretsync.crossplane.infisical.com/v1alpha1
kind: SecretSyncGithub
metadata: {name: sg}
spec:
  forProvider:
    name: sg
    destinationConfig: '`+c.destinationConfig+`'
`)))
			setField(t, v2, "spec.forProvider.destinationConfig.repositoryName", c.repositoryName)
			if got := value(t, down(t, s, stored(t, s, v2)), "spec.forProvider.destinationConfig"); got != c.want {
				t.Errorf("v1alpha1 destinationConfig after the v1alpha2 change: want %s, got %v", c.want, got)
			}
		})
	}
}

// TestSameDataKeepsSavedValues checks that the saved values are still used
// when a client writes the same data again, maybe as other JSON text.
func TestSameDataKeepsSavedValues(t *testing.T) {
	s := setup(t)
	role := `
apiVersion: project.crossplane.infisical.com/v1alpha2
kind: ProjectRole
metadata: {name: pr}
spec:
  forProvider:
    name: r
    slug: r
    projectId: p
    permissions: [{action: read, subject: secrets}]
    permissionsV2: [{action: [read], subject: secrets, conditions: '{"environment":{"$eq":"dev"}}'}]
`
	t.Run("V1PermissionsAfterTheSameDataAsOtherText", func(t *testing.T) {
		v1 := down(t, s, stored(t, s, decode(t, s, role)))
		setField(t, v1, "spec.forProvider.permissions", `[ { "subject": "secrets", "conditions": {"environment": {"$eq": "dev"}}, "action": ["read"] } ]`)
		got := jsonOf(t, value(t, up(t, s, stored(t, s, v1)), "spec.forProvider.permissions"))
		if want := `[{"action":"read","subject":"secrets"}]`; got != want {
			t.Errorf("v1alpha2 permissions: want %s, got %s", want, got)
		}
	})
	t.Run("NoV1PermissionsAfterAChange", func(t *testing.T) {
		v1 := down(t, s, stored(t, s, decode(t, s, role)))
		setField(t, v1, "spec.forProvider.permissions", `[{"action":["read","edit"],"subject":"secrets","conditions":{"environment":{"$eq":"dev"}}}]`)
		back := up(t, s, stored(t, s, v1))
		if got, want := jsonOf(t, value(t, back, "spec.forProvider.permissionsV2[0].action")), `["read","edit"]`; got != want {
			t.Errorf("v1alpha2 permissionsV2 action: want %s, got %s", want, got)
		}
		if got := value(t, back, "spec.forProvider.permissions"); got != nil {
			t.Errorf("v1alpha2 permissions: want no value after a change in v1alpha1, got %s", jsonOf(t, got))
		}
	})
	t.Run("ApproverOrderAfterALabelUpdate", func(t *testing.T) {
		v1 := down(t, s, stored(t, s, decode(t, s, `
apiVersion: project.crossplane.infisical.com/v1alpha2
kind: AccessApprovalPolicy
metadata: {name: ap}
spec:
  forProvider:
    projectId: p
    secretPath: /
    environmentSlugs: [prod]
    requiredApprovals: 1
    approvers: [{type: group, id: g1}, {type: user, username: a@b.c}]
`)))
		v1.SetLabels(map[string]string{"changed": "label"})
		got := jsonOf(t, value(t, up(t, s, stored(t, s, v1)), "spec.forProvider.approvers"))
		if want := `[{"id":"g1","type":"group"},{"type":"user","username":"a@b.c"}]`; got != want {
			t.Errorf("v1alpha2 approvers: want %s, got %s", want, got)
		}
	})
	t.Run("LegacyJSONTextAfterALabelUpdate", func(t *testing.T) {
		text := `{ "scope":"repository", "repository_name":"r","repository_owner":"o" }`
		v2 := stored(t, s, up(t, s, decode(t, s, `
apiVersion: secretsync.crossplane.infisical.com/v1alpha1
kind: SecretSyncGithub
metadata: {name: sg}
spec:
  forProvider:
    name: sg
    destinationConfig: '`+text+`'
`)))
		v2.SetLabels(map[string]string{"changed": "label"})
		if got := value(t, down(t, s, stored(t, s, v2)), "spec.forProvider.destinationConfig"); got != text {
			t.Errorf("v1alpha1 destinationConfig: want %s, got %v", text, got)
		}
	})
}
