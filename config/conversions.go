/*
Copyright 2026 Infisical Inc.
*/

package config

import (
	"encoding/json"
	"fmt"
	"strings"

	// Note: the CRD schema change report is embedded.
	_ "embed"

	ujconfig "github.com/crossplane/upjet/v2/pkg/config"
	ujconversion "github.com/crossplane/upjet/v2/pkg/config/conversion"
	"github.com/pkg/errors"

	"github.com/infisical/provider-infisical/config/conversion"
)

//go:embed crd-schema-changes.json
var crdSchemaChanges []byte

func configureConversions(pc *ujconfig.Provider) error {
	changedPaths, err := schemaChangePaths(pc.RootGroup)
	if err != nil {
		return err
	}
	owned := map[string][]string{}
	for tfName, fields := range conversion.ByResource {
		r, ok := pc.Resources[tfName]
		if !ok {
			continue
		}
		for _, f := range fields {
			owned[tfName] = append(owned[tfName], f.Owned()...)
		}
		// skip all changes under the owned fields in the automatic conversions. upjet only skips exact paths
		for _, p := range changedPaths[fmt.Sprintf("%s/%s", r.ShortGroup, r.Kind)] {
			if ownsPath(owned[tfName], p) {
				r.AutoConversionRegistrationOptions.AutoRegisterExcludePaths = append(r.AutoConversionRegistrationOptions.AutoRegisterExcludePaths, p)
			}
		}
	}

	if err := ujconfig.ExcludeTypeChangesFromIdentity(pc, crdSchemaChanges); err != nil {
		return errors.Wrap(err, "cannot exclude the type changes from the identity conversion")
	}
	for tfName, r := range pc.Resources {
		exclude := append(r.AutoConversionRegistrationOptions.IdentityConversionExcludePaths, owned[tfName]...)
		// the first conversion is the default identity conversion of upjet, which does not know about the changed fields. Replace it
		r.Conversions[0] = ujconversion.NewIdentityConversionExpandPaths(ujconversion.AllVersions, ujconversion.AllVersions,
			ujconversion.DefaultPathPrefixes(), exclude...)
	}
	if err := ujconfig.RegisterAutoConversions(pc, crdSchemaChanges); err != nil {
		return errors.Wrap(err, "cannot register the automatic API conversions")
	}
	for tfName, fields := range conversion.ByResource {
		r, ok := pc.Resources[tfName]
		if !ok {
			continue
		}
		for _, f := range fields {
			r.Conversions = append(r.Conversions, f.Conversions()...)
		}
	}
	return nil
}

// schemaChangePaths returns the changed field paths of every CRD, keyed by "<short group>/<kind>"
func schemaChangePaths(rootGroup string) (map[string][]string, error) {
	var report map[string]struct {
		Versions map[string]struct {
			Changes []struct {
				Path string `json:"path"`
			} `json:"changes"`
		} `json:"versions"`
	}
	if err := json.Unmarshal(crdSchemaChanges, &report); err != nil {
		return nil, errors.Wrap(err, "cannot parse the CRD schema change report")
	}
	out := map[string][]string{}
	for key, r := range report {
		// the key is "<short group>.<root group>/<kind>"
		short := strings.Replace(key, "."+rootGroup+"/", "/", 1)
		for _, v := range r.Versions {
			for _, c := range v.Changes {
				out[short] = append(out[short], c.Path)
			}
		}
	}
	return out, nil
}

// ownsPath reports whether the CRD field path is one of the owned fields of a parameter object, or below one
func ownsPath(owned []string, path string) bool {
	for _, prefix := range conversion.ParameterPrefixes {
		for _, f := range owned {
			base := prefix + "." + f
			if path == base || strings.HasPrefix(path, base+".") || strings.HasPrefix(path, base+"[") {
				return true
			}
		}
	}
	return false
}
