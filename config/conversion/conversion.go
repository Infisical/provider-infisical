// note(daniel): package conversion converts fields between the v1alpha1 API, which was
// generated from the Crossplane-specific legacy Terraform build, and the
// v1alpha2 API, which is generated from the normal Terraform provider

// upjet copies all fields that have the same shape in both versions. This
// package converts the fields that changed shape, for example a JSON string in
// v1alpha1 that is an object in v1alpha2
package conversion

import (
	"encoding/json"

	"github.com/crossplane/crossplane-runtime/v2/pkg/fieldpath"
	ujconversion "github.com/crossplane/upjet/v2/pkg/config/conversion"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	// VersionV1alpha1 is the API version generated from the legacy build.
	VersionV1alpha1 = "v1alpha1"
	// VersionV1alpha2 is the API version generated from the normal provider.
	VersionV1alpha2 = "v1alpha2"

	AnnotationKey = "conversion.crossplane.infisical.com/fields"
)

// ParameterPrefixes are the paths of the parameter objects of a managed
// resource that hold the converted fields.
var ParameterPrefixes = []string{"spec.forProvider", "spec.initProvider", "status.atProvider"}

type Fields struct {
	V1alpha1 []string

	V1alpha2 []string

	// Up returns the v1alpha2 fields for the given v1alpha1 fields. The input
	// only holds the V1alpha1 fields that are set.
	Up func(v1alpha1 map[string]any) (map[string]any, error)
	// Down returns the v1alpha1 fields for the given v1alpha2 fields. The
	// input only holds the V1alpha2 fields that are set.
	Down func(v1alpha2 map[string]any) (map[string]any, error)
}

// Conversions returns the upjet conversions for both directions.
func (f Fields) Conversions() []ujconversion.Conversion {
	return []ujconversion.Conversion{
		&pavedConversion{fields: f, from: VersionV1alpha1, to: VersionV1alpha2},
		&pavedConversion{fields: f, from: VersionV1alpha2, to: VersionV1alpha1},
	}
}

// Owned returns all field names that the conversion handles, in both versions. upjet's identity and automatic conversions must skip them.
func (f Fields) Owned() []string {
	return append(append([]string{}, f.V1alpha1...), f.V1alpha2...)
}

var _ ujconversion.PavedConversion = &pavedConversion{}

type pavedConversion struct {
	fields   Fields
	from, to string
}

func (c *pavedConversion) Applicable(src, dst runtime.Object) bool {
	return src.GetObjectKind().GroupVersionKind().Version == c.from &&
		dst.GetObjectKind().GroupVersionKind().Version == c.to
}

func (c *pavedConversion) ConvertPaved(src, target *fieldpath.Paved) (bool, error) {
	if !c.Applicable(&unstructured.Unstructured{Object: src.UnstructuredContent()},
		&unstructured.Unstructured{Object: target.UnstructuredContent()}) {
		return false, nil
	}
	// upjet's identity conversion runs first and copies the metadata, so the target holds the annotation of the source.
	// read and write it on the target, so that the conversions of several field groups add up.
	stored, err := storedFields(target)
	if err != nil {
		return false, err
	}
	for _, prefix := range ParameterPrefixes {
		if c.to == VersionV1alpha2 {
			err = c.up(src, target, prefix, stored)
		} else {
			err = c.down(src, target, prefix, stored)
		}
		if err != nil {
			return false, errors.Wrapf(err, "cannot convert %v from %s to %s at %s", c.fields.Owned(), c.from, c.to, prefix)
		}
	}
	return true, errors.Wrap(setStoredFields(target, stored), "cannot store the v1alpha2 fields in an annotation")
}

func (c *pavedConversion) up(src, target *fieldpath.Paved, prefix string, stored map[string]any) error {
	in, err := pick(src, prefix, c.fields.V1alpha1)
	if err != nil {
		return err
	}
	saved := takeStored(stored, VersionV1alpha2, prefix, c.fields.V1alpha2)
	takeStored(stored, VersionV1alpha1, prefix, c.fields.V1alpha1)
	if err := deleteFields(target, prefix, c.fields.Owned()); err != nil {
		return err
	}
	out := map[string]any{}
	if len(in) > 0 {
		if out, err = c.fields.Up(in); err != nil {
			return err
		}
	}
	// use the saved v1alpha2 values while they still give the current v1alpha1 values, so that values that v1alpha1 cannot show are kept
	if len(saved) > 0 {
		if back, err := c.fields.Down(saved); err == nil && sameValues(back, in) {
			out = saved
		}
	}
	// keep the original v1alpha1 values for the way back
	putStored(stored, VersionV1alpha1, prefix, in)
	return setFields(target, prefix, out)
}

func (c *pavedConversion) down(src, target *fieldpath.Paved, prefix string, stored map[string]any) error {
	in, err := pick(src, prefix, c.fields.V1alpha2)
	if err != nil {
		return err
	}
	saved := takeStored(stored, VersionV1alpha1, prefix, c.fields.V1alpha1)
	takeStored(stored, VersionV1alpha2, prefix, c.fields.V1alpha2)
	if err := deleteFields(target, prefix, c.fields.Owned()); err != nil {
		return err
	}
	out := map[string]any{}
	if len(in) > 0 {
		if out, err = c.fields.Down(in); err != nil {
			return err
		}
	}
	// use the saved original v1alpha1 values while they still give the current v1alpha2 values, so that a v1alpha1 client gets back exactly what it wrote
	if len(saved) > 0 {
		if forth, err := c.fields.Up(saved); err == nil && sameValues(forth, in) {
			out = saved
		}
	}
	// keep the v1alpha2 values for the way back
	putStored(stored, VersionV1alpha2, prefix, in)
	return setFields(target, prefix, out)
}

// takeStored removes the saved values of the given version and fields from stored, and returns them keyed by field name
func takeStored(stored map[string]any, version, prefix string, fields []string) map[string]any {
	out := map[string]any{}
	for _, f := range fields {
		k := storedKey(version, prefix, f)
		if v, ok := stored[k]; ok {
			out[f] = v
			delete(stored, k)
		}
	}
	return out
}

func putStored(stored map[string]any, version, prefix string, values map[string]any) {
	for f, v := range values {
		stored[storedKey(version, prefix, f)] = v
	}
}

func storedKey(version, prefix, field string) string {
	return version + ":" + prefix + "." + field
}

// pick returns the given fields of the parameter object that are set.
func pick(p *fieldpath.Paved, prefix string, fields []string) (map[string]any, error) {
	out := map[string]any{}
	for _, f := range fields {
		v, err := p.GetValue(prefix + "." + f)
		if fieldpath.IsNotFound(err) || (err == nil && v == nil) {
			continue
		}
		if err != nil {
			return nil, errors.Wrapf(err, "cannot get %s.%s", prefix, f)
		}
		out[f] = v
	}
	return out, nil
}

func deleteFields(p *fieldpath.Paved, prefix string, fields []string) error {
	for _, f := range fields {
		if err := p.DeleteField(prefix + "." + f); err != nil && !fieldpath.IsNotFound(err) {
			return errors.Wrapf(err, "cannot delete %s.%s", prefix, f)
		}
	}
	return nil
}

func setFields(p *fieldpath.Paved, prefix string, fields map[string]any) error {
	for f, v := range fields {
		if err := p.SetValue(prefix+"."+f, v); err != nil {
			return errors.Wrapf(err, "cannot set %s.%s", prefix, f)
		}
	}
	return nil
}

// storedFields returns the v1alpha2 values that are kept in the annotation, keyed by their field path
func storedFields(p *fieldpath.Paved) (map[string]any, error) {
	stored := map[string]any{}
	v, err := p.GetValue("metadata.annotations")
	if fieldpath.IsNotFound(err) || v == nil {
		return stored, nil
	}
	if err != nil {
		return nil, errors.Wrap(err, "cannot get the annotations")
	}
	annotations, ok := v.(map[string]any)
	if !ok {
		return stored, nil
	}
	raw, ok := annotations[AnnotationKey].(string)
	if !ok || raw == "" {
		return stored, nil
	}
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		return nil, errors.Wrapf(err, "cannot parse the %s annotation", AnnotationKey)
	}
	return stored, nil
}

func setStoredFields(p *fieldpath.Paved, stored map[string]any) error {
	annotations := map[string]any{}
	if v, err := p.GetValue("metadata.annotations"); err == nil {
		if m, ok := v.(map[string]any); ok {
			annotations = m
		}
	}
	if len(stored) == 0 {
		if _, ok := annotations[AnnotationKey]; !ok {
			return nil
		}
		delete(annotations, AnnotationKey)
	} else {
		raw, err := json.Marshal(stored)
		if err != nil {
			return err
		}
		annotations[AnnotationKey] = string(raw)
	}
	if len(annotations) == 0 {
		return deleteFields(p, "metadata", []string{"annotations"})
	}
	return p.SetValue("metadata.annotations", annotations)
}

// sameValues reports whether two sets of field values are the same once they
// are stored. omitempty drops empty values (null, "", [] and {}) when an object
// is stored, so an empty value is the same as no value. A JSON string is
// compared by its parsed value, so key order and spacing do not matter.
func sameValues(a, b any) bool {
	ja, err := prunedJSON(a)
	if err != nil {
		return false
	}
	jb, err := prunedJSON(b)
	return err == nil && ja == jb
}

// prunedJSON returns the JSON of v without its empty values.
func prunedJSON(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return "", err
	}
	raw, err = json.Marshal(pruneEmpty(generic))
	return string(raw), err
}

// pruneEmpty returns v without its empty values, or nil if v is empty. The
// elements of a list are kept in place, because their position has a meaning.
func pruneEmpty(v any) any {
	switch t := v.(type) {
	case string:
		if t == "" {
			return nil
		}
		var parsed any
		if err := json.Unmarshal([]byte(t), &parsed); err == nil {
			if _, ok := parsed.(string); !ok {
				return pruneEmpty(parsed)
			}
		}
		return t
	case map[string]any:
		out := map[string]any{}
		for k, val := range t {
			if p := pruneEmpty(val); p != nil {
				out[k] = p
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	case []any:
		if len(t) == 0 {
			return nil
		}
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = pruneEmpty(val)
		}
		return out
	default:
		return v
	}
}
