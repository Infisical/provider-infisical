//go:build e2e

/*
Copyright 2026 Infisical Inc.
*/

package e2e

import (
	"fmt"
	"os"
	"path/filepath"
)

// provider is the provider that creates an object in Infisical.
type provider int

const (
	// providerUnderTest is the provider that this repository builds.
	providerUnderTest provider = iota
	// providerReleased is the released provider that the upgrade test starts
	// from.
	providerReleased
)

// object is one managed resource of the test fixtures in testdata/<version>.
type object struct {
	// file is the name of the fixture file, without ".yaml".
	file string
	// name is the suffix of the Kubernetes name, after the set prefix.
	name  string
	group string
	kind  string

	// parent objects are created first and deleted last, because other
	// objects reference them.
	parent bool

	// onlyV1alpha2 is true for kinds that were added after v1alpha1 was
	// frozen. They only have a v1alpha2 API and fixture, and the conversion
	// and upgrade tests skip them. All other kinds have both API versions.
	onlyV1alpha2 bool

	// notReady returns why the object cannot become ready in the test when
	// the given provider creates it. nil means it always becomes ready.
	notReady func(createdBy provider) string

	// update is the update test of each API version: a merge patch of
	// spec.forProvider, and the value that status.atProvider must then
	// have. Values are JSON with the placeholders of the fixtures.
	update map[string]*update

	// up are the values that an object created as v1alpha1 has when it is
	// read as v1alpha2. down are the values that an object created as
	// v1alpha2 has when it is read as v1alpha1. Only the kinds whose fields
	// changed shape between the versions have them. For all other kinds,
	// spec.forProvider is the same in both versions.
	up, down []check
}

type update struct {
	set   string
	check string
	want  string
}

// check is a value at a field path. want is JSON with the placeholders of the
// fixtures. A v1alpha1 field that holds a JSON string matches when the string
// holds the wanted data.
type check struct {
	path string
	want string
}

// onlySynced returns why the object cannot become ready in the test when the
// given provider creates it, or "" when it must become ready. Such objects
// only need a Synced condition, and they are orphaned on delete.
func (o object) onlySynced(createdBy provider) string {
	if o.notReady == nil {
		return ""
	}
	return o.notReady(createdBy)
}

// testName returns the name of the object in test output.
func (o object) testName() string {
	if o.file == "identity-kubernetes" {
		return "Identity(for-KubernetesAuth)"
	}
	return o.kind
}

var (
	reasonKubernetesAuth = "Infisical calls the Kubernetes API of kubernetesHost when the auth method is created, and the test has no Kubernetes API that Infisical can reach"
	reasonProjectRole    = "known gap: the ProjectRole Read of the Terraform provider returns an error instead of removing the resource when the role is not found, so a new ProjectRole is never created"
	reasonSecretReleased = "known bug of the released provider: its first refresh reads the secret with an empty ID and the API returns 400, so it cannot create a new Secret"
)

func updateBoth(set, check, want string) map[string]*update {
	u := &update{set: set, check: check, want: want}
	return map[string]*update{v1alpha1: u, v1alpha2: u}
}

const (
	projectGroup   = "project.crossplane.infisical.com"
	identityGroup  = "identity.crossplane.infisical.com"
	secretGroup    = "secret.crossplane.infisical.com"
	groupGroup     = "group.crossplane.infisical.com"
	secretSyncAPIs = "secretsync.crossplane.infisical.com"
)

// objects are all test objects. Every kind of the provider has at least one.
var objects = []object{
	{
		file: "project", name: "project", group: projectGroup, kind: "Project", parent: true,
		update: updateBoth(`{"description":"updated by e2e"}`, "status.atProvider.description", `"updated by e2e"`),
	},
	{
		file: "identity", name: "identity", group: identityGroup, kind: "Identity", parent: true,
		update: updateBoth(`{"role":"no-access"}`, "status.atProvider.role", `"no-access"`),
	},
	{
		file: "identity-kubernetes", name: "identity-kubernetes", group: identityGroup, kind: "Identity", parent: true,
	},
	{
		file: "group", name: "group", group: groupGroup, kind: "Group", parent: true,
		update: updateBoth(`{"name":"__PREFIX__-__RUN_ID__-updated"}`, "status.atProvider.name", `"__PREFIX__-__RUN_ID__-updated"`),
	},
	{
		file: "projectenvironment", name: "environment", group: projectGroup, kind: "ProjectEnvironment",
		update: updateBoth(`{"name":"e2e-updated"}`, "status.atProvider.name", `"e2e-updated"`),
	},
	{
		file: "secretfolder", name: "folder", group: secretGroup, kind: "SecretFolder",
		update: updateBoth(`{"description":"updated by e2e"}`, "status.atProvider.description", `"updated by e2e"`),
	},
	{
		file: "universalauth", name: "universal-auth", group: identityGroup, kind: "UniversalAuth",
		update: updateBoth(`{"accessTokenTtl":3600}`, "status.atProvider.accessTokenTtl", `3600`),
	},
	{
		file: "kubernetesauth", name: "kubernetes-auth", group: identityGroup, kind: "KubernetesAuth",
		notReady: func(provider) string { return reasonKubernetesAuth },
	},
	{
		file: "projectidentity", name: "project-identity", group: projectGroup, kind: "ProjectIdentity",
		update: map[string]*update{
			v1alpha1: {set: `{"roles":"[{\"role_slug\":\"member\"}]"}`, check: "status.atProvider.roles", want: `[{"role_slug":"member"}]`},
			v1alpha2: {set: `{"roles":[{"roleSlug":"member"}]}`, check: "status.atProvider.roles", want: `[{"roleSlug":"member"}]`},
		},
		up:   []check{{"spec.forProvider.roles", `[{"roleSlug":"viewer"}]`}},
		down: []check{{"spec.forProvider.roles", `[{"role_slug":"viewer"}]`}},
	},
	{
		file: "projectuser", name: "project-user", group: projectGroup, kind: "ProjectUser",
		update: map[string]*update{
			v1alpha1: {set: `{"roles":"[{\"role_slug\":\"member\"}]"}`, check: "status.atProvider.roles", want: `[{"role_slug":"member"}]`},
			v1alpha2: {set: `{"roles":[{"roleSlug":"member"}]}`, check: "status.atProvider.roles", want: `[{"roleSlug":"member"}]`},
		},
		up:   []check{{"spec.forProvider.roles", `[{"roleSlug":"viewer"}]`}},
		down: []check{{"spec.forProvider.roles", `[{"role_slug":"viewer"}]`}},
	},
	{
		file: "projectgroup", name: "project-group", group: projectGroup, kind: "ProjectGroup",
		update: map[string]*update{
			v1alpha1: {set: `{"roles":"[{\"role_slug\":\"member\"}]"}`, check: "status.atProvider.roles", want: `[{"role_slug":"member"}]`},
			v1alpha2: {set: `{"roles":[{"roleSlug":"member"}]}`, check: "status.atProvider.roles", want: `[{"roleSlug":"member"}]`},
		},
		up:   []check{{"spec.forProvider.roles", `[{"roleSlug":"viewer"}]`}},
		down: []check{{"spec.forProvider.roles", `[{"role_slug":"viewer"}]`}},
	},
	{
		file: "projectrole", name: "project-role", group: projectGroup, kind: "ProjectRole",
		notReady: func(p provider) string {
			if p == providerUnderTest {
				return reasonProjectRole
			}
			return ""
		},
		update: updateBoth(`{"description":"updated by e2e"}`, "status.atProvider.description", `"updated by e2e"`),
		up: []check{{"spec.forProvider.permissionsV2",
			`[{"action":["read"],"subject":"secrets","conditions":"{\"environment\":{\"$eq\":\"dev\"},\"secretPath\":{\"$eq\":\"/\"}}"}]`}},
		down: []check{{"spec.forProvider.permissions",
			`[{"action":["read"],"subject":"secrets","conditions":{"environment":{"$eq":"dev"},"secretPath":{"$eq":"/"}}}]`}},
	},
	{
		file: "accessapprovalpolicy", name: "access-approval-policy", group: projectGroup, kind: "AccessApprovalPolicy",
		update: updateBoth(`{"enforcementLevel":"hard"}`, "status.atProvider.enforcementLevel", `"hard"`),
		up:     []check{{"spec.forProvider.approvers", `[{"type":"user","username":"__USER_EMAIL__"}]`}},
		down:   []check{{"spec.forProvider.userApprovers", `["__USER_EMAIL__"]`}},
	},
	{
		file: "secretapprovalpolicy", name: "secret-approval-policy", group: projectGroup, kind: "SecretApprovalPolicy",
		update: updateBoth(`{"enforcementLevel":"hard"}`, "status.atProvider.enforcementLevel", `"hard"`),
		up:     []check{{"spec.forProvider.approvers", `[{"type":"user","username":"__USER_EMAIL__"}]`}},
		down:   []check{{"spec.forProvider.userApprovers", `["__USER_EMAIL__"]`}},
	},
	{
		file: "projecttemplate", name: "project-template", group: projectGroup, kind: "ProjectTemplate",
		update: updateBoth(`{"description":"updated by e2e"}`, "status.atProvider.description", `"updated by e2e"`),
		up: []check{
			{"spec.forProvider.environments", `[{"name":"development","slug":"dev","position":1}]`},
			{"spec.forProvider.roles", `[{"name":"E2E","slug":"e2e","permissions":[{"action":["read"],"subject":"secrets","conditions":"{\"environment\":{\"$eq\":\"dev\"}}"}]}]`},
		},
		down: []check{
			{"spec.forProvider.environments", `[{"name":"development","slug":"dev","position":1}]`},
			{"spec.forProvider.roles", `[{"name":"E2E","slug":"e2e","permissions":[{"action":["read"],"subject":"secrets","conditions":{"environment":{"$eq":"dev"}}}]}]`},
		},
	},
	{
		file: "secretsyncgithub", name: "secret-sync-github", group: secretSyncAPIs, kind: "SecretSyncGithub",
		update: map[string]*update{
			v1alpha1: {set: `{"description":"updated by e2e"}`, check: "status.atProvider.description", want: `"updated by e2e"`},
			v1alpha2: {set: `{"syncOptions":{"keySchema":"E2E_UPDATED_{{secretKey}}"}}`, check: "status.atProvider.syncOptions.keySchema", want: `"E2E_UPDATED_{{secretKey}}"`},
		},
		up: []check{
			{"spec.forProvider.destinationConfig", `{"scope":"repository","repositoryOwner":"__GITHUB_REPO_OWNER__","repositoryName":"__GITHUB_REPO_NAME__"}`},
			{"spec.forProvider.syncOptions", `{"initialSyncBehavior":"overwrite-destination","disableSecretDeletion":true,"keySchema":"E2E_{{secretKey}}"}`},
		},
		down: []check{
			{"spec.forProvider.destinationConfig", `{"scope":"repository","repository_owner":"__GITHUB_REPO_OWNER__","repository_name":"__GITHUB_REPO_NAME__"}`},
			{"spec.forProvider.syncOptions", `{"initial_sync_behavior":"overwrite-destination","disable_secret_deletion":true,"key_schema":"E2E_{{secretKey}}"}`},
		},
	},
	{
		file: "secret", name: "secret", group: secretGroup, kind: "Secret",
		notReady: func(p provider) string {
			if p == providerReleased {
				return reasonSecretReleased
			}
			return ""
		},
		update: map[string]*update{
			v1alpha2: {set: `{"secretReminder":{"note":"updated by e2e"}}`, check: "status.atProvider.secretReminder.note", want: `"updated by e2e"`},
		},
	},
}

// hasVersion reports whether the kind of the object has the API version.
func (o object) hasVersion(version string) bool {
	return version == v1alpha2 || !o.onlyV1alpha2
}

// objectsIn returns the test objects whose kind has the API version.
func objectsIn(version string) []object {
	var out []object
	for _, o := range objects {
		if o.hasVersion(version) {
			out = append(out, o)
		}
	}
	return out
}

// legacyObjects returns the test objects whose kind has both API versions,
// which are the kinds that the conversion and upgrade tests cover.
func legacyObjects() []object {
	return objectsIn(v1alpha1)
}

// checkFixtures returns the problems with the fixture files: every object
// needs a v1alpha2 fixture, and a v1alpha1 fixture only when its kind has
// v1alpha1.
func checkFixtures() []string {
	var problems []string
	for _, o := range objects {
		for _, version := range versions {
			path := filepath.Join("testdata", version, o.file+".yaml")
			_, err := os.Stat(path)
			switch {
			case o.hasVersion(version) && err != nil:
				problems = append(problems, fmt.Sprintf("%s: %v", path, err))
			case !o.hasVersion(version) && err == nil:
				problems = append(problems, fmt.Sprintf("%s exists, but %s has onlyV1alpha2 set", path, o.kind))
			}
		}
	}
	return problems
}

// objectByFile returns the test object of the fixture file.
func objectByFile(file string) object {
	for _, o := range objects {
		if o.file == file {
			return o
		}
	}
	panic("unknown test object " + file)
}

// changedShape reports whether the fields of the kind changed shape between
// v1alpha1 and v1alpha2.
func (o object) changedShape() bool {
	return len(o.up) > 0 || len(o.down) > 0
}
