/*
Copyright 2026 Infisical Inc.
*/

package conversion

import (
	"encoding/json"

	"github.com/crossplane/upjet/v2/pkg/types/name"
	"github.com/pkg/errors"
)

// ByResource are the field conversions of every Terraform resource whose
// fields changed shape between v1alpha1 and v1alpha2. The JSON formats of
// v1alpha1 are the formats of the Crossplane-specific legacy Terraform build
// (terraform-provider-infisical, tag crossplane-tf-provider/v0.0.20,
// directory crossplane/).
var ByResource = map[string][]Fields{
	"infisical_secret_sync_github": {
		JSONField("destinationConfig", "destinationConfig"),
		JSONField("syncOptions", "syncOptions"),
	},
	"infisical_project_identity": {JSONField("roles", "roles")},
	"infisical_project_user":     {JSONField("roles", "roles")},
	"infisical_project_group":    {JSONField("roles", "roles")},
	"infisical_project_template": {
		JSONField("roles", "roles", "conditions"),
		JSONField("environments", "environments"),
	},
	"infisical_project_role":           {projectRolePermissions()},
	"infisical_access_approval_policy": {approvers()},
	"infisical_secret_approval_policy": {approvers()},
}

// JSONField converts a v1alpha1 field that holds a JSON string to a v1alpha2 field that holds the same data as an object or a list.
// The keys of the JSON are the Terraform attribute names (snake case), and the keys of the object are the CRD field names (camel case).
// The values of the opaque keys stay as they are in the JSON, and they are JSON strings in v1alpha2
func JSONField(v1alpha1, v1alpha2 string, opaque ...string) Fields {
	return Fields{
		V1alpha1: []string{v1alpha1},
		V1alpha2: []string{v1alpha2},
		Up: func(in map[string]any) (map[string]any, error) {
			v, err := fromJSONString(in[v1alpha1], opaque)
			if err != nil || v == nil {
				return nil, err
			}
			return map[string]any{v1alpha2: v}, nil
		},
		Down: func(in map[string]any) (map[string]any, error) {
			s, err := toJSONString(in[v1alpha2], opaque)
			if err != nil {
				return nil, err
			}
			return map[string]any{v1alpha1: s}, nil
		},
	}
}

// projectRolePermissions converts the "permissions" JSON of v1alpha1, which is in the format of the V2 permissions, to "permissionsV2".
// The V1 "permissions" of v1alpha2 cannot be shown in v1alpha1. They are kept in the annotation.
func projectRolePermissions() Fields {
	return Fields{
		V1alpha1: []string{"permissions"},
		V1alpha2: []string{"permissionsV2", "permissions"},
		Up: func(in map[string]any) (map[string]any, error) {
			v, err := fromJSONString(in["permissions"], []string{"conditions"})
			if err != nil || v == nil {
				return nil, err
			}
			return map[string]any{"permissionsV2": v}, nil
		},
		Down: func(in map[string]any) (map[string]any, error) {
			v, ok := in["permissionsV2"]
			if !ok {
				return map[string]any{}, nil
			}
			s, err := toJSONString(v, []string{"conditions"})
			if err != nil {
				return nil, err
			}
			return map[string]any{"permissions": s}, nil
		},
	}
}

// approvers converts the user and group lists of the approval policies in
// v1alpha1 to the "approvers" and "bypassers" sets of v1alpha2. Users are
// identified by their username, and groups by their ID.
func approvers() Fields {
	type pair struct{ users, groups, set string }
	pairs := []pair{
		{users: "userApprovers", groups: "groupApprovers", set: "approvers"},
		{users: "userBypassers", groups: "groupBypassers", set: "bypassers"},
	}
	return Fields{
		V1alpha1: []string{"userApprovers", "groupApprovers", "userBypassers", "groupBypassers"},
		V1alpha2: []string{"approvers", "bypassers"},
		Up: func(in map[string]any) (map[string]any, error) {
			out := map[string]any{}
			for _, p := range pairs {
				var set []any
				users, err := stringList(in[p.users])
				if err != nil {
					return nil, errors.Wrap(err, p.users)
				}
				for _, u := range users {
					set = append(set, map[string]any{"type": "user", "username": u})
				}
				groups, err := stringList(in[p.groups])
				if err != nil {
					return nil, errors.Wrap(err, p.groups)
				}
				for _, g := range groups {
					set = append(set, map[string]any{"type": "group", "id": g})
				}
				if len(set) > 0 {
					out[p.set] = set
				}
			}
			return out, nil
		},
		Down: func(in map[string]any) (map[string]any, error) {
			out := map[string]any{}
			for _, p := range pairs {
				entries, ok := in[p.set].([]any)
				if !ok {
					continue
				}
				var users, groups []any
				for _, e := range entries {
					m, ok := e.(map[string]any)
					if !ok {
						return nil, errors.Errorf("%s: unexpected entry %v", p.set, e)
					}
					switch m["type"] {
					case "user":
						if u, ok := m["username"].(string); ok && u != "" {
							users = append(users, u)
						}
					case "group":
						if g, ok := m["id"].(string); ok && g != "" {
							groups = append(groups, g)
						}
					}
				}
				if len(users) > 0 {
					out[p.users] = users
				}
				if len(groups) > 0 {
					out[p.groups] = groups
				}
			}
			return out, nil
		},
	}
}

func stringList(v any) ([]string, error) {
	if v == nil {
		return nil, nil
	}
	list, ok := v.([]any)
	if !ok {
		return nil, errors.Errorf("expected a list, got %T", v)
	}
	out := make([]string, 0, len(list))
	for _, e := range list {
		s, ok := e.(string)
		if !ok {
			return nil, errors.Errorf("expected a string, got %T", e)
		}
		out = append(out, s)
	}
	return out, nil
}

// fromJSONString parses a v1alpha1 JSON string and returns the v1alpha2 value.
func fromJSONString(v any, opaque []string) (any, error) {
	s, ok := v.(string)
	if !ok {
		return nil, errors.Errorf("expected a JSON string, got %T", v)
	}
	if s == "" {
		return nil, nil
	}
	var parsed any
	if err := json.Unmarshal([]byte(s), &parsed); err != nil {
		return nil, errors.Wrap(err, "cannot parse the JSON string")
	}
	return renameKeys(parsed, opaque, func(k string) string { return name.NewFromSnake(k).LowerCamelComputed }, opaqueToString)
}

// toJSONString returns the v1alpha1 JSON string for a v1alpha2 value.
func toJSONString(v any, opaque []string) (string, error) {
	renamed, err := renameKeys(v, opaque, func(k string) string { return name.NewFromCamel(k).Snake }, opaqueFromString)
	if err != nil {
		return "", err
	}
	raw, err := json.Marshal(renamed)
	return string(raw), err
}

// renameKeys renames the keys of all objects in v. The values of opaque keys
// are not renamed but converted with convertOpaque.
func renameKeys(v any, opaque []string, rename func(string) string, convertOpaque func(any) (any, error)) (any, error) {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			var err error
			if contains(opaque, k) {
				val, err = convertOpaque(val)
			} else {
				val, err = renameKeys(val, opaque, rename, convertOpaque)
			}
			if err != nil {
				return nil, errors.Wrap(err, k)
			}
			out[rename(k)] = val
		}
		return out, nil
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			r, err := renameKeys(val, opaque, rename, convertOpaque)
			if err != nil {
				return nil, err
			}
			out[i] = r
		}
		return out, nil
	default:
		return v, nil
	}
}

// opaqueToString converts an opaque JSON value of v1alpha1 to the JSON string
// that v1alpha2 holds.
func opaqueToString(v any) (any, error) {
	if v == nil {
		return nil, nil
	}
	if s, ok := v.(string); ok {
		return s, nil
	}
	raw, err := json.Marshal(v)
	return string(raw), err
}

// opaqueFromString converts the JSON string of v1alpha2 back to the opaque
// JSON value of v1alpha1.
func opaqueFromString(v any) (any, error) {
	s, ok := v.(string)
	if !ok || s == "" {
		return v, nil
	}
	var out any
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		// Not JSON: keep the string as it is.
		return s, nil //nolint:nilerr // a plain string is a valid value
	}
	return out, nil
}

func contains(list []string, s string) bool {
	for _, e := range list {
		if e == s {
			return true
		}
	}
	return false
}
