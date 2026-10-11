package config

import (
	"encoding/json"
	"math/rand"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/crossplane/upjet/v2/pkg/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	conv "github.com/infisical/provider-infisical/config/conversion"
)

// The property tests write random values into every converted field and check
// that clients of both API versions read back what they wrote. They only use
// the field converters and the JSON storage of the real types to get the
// expected values, and never the comparison of the conversion.

// trickyStrings are string values that are easy to confuse: numbers and other
// JSON values as text, spaces, the empty string, and words of the converters.
var trickyStrings = []string{
	"", "x", "100", "1e2", " 100", "100 ", "0100", "-0", "true", "null",
	"[]", "{}", `{"a":1}`, `{ "a": 1 }`, `a"b`, "ünï", "user", "group",
	"role_slug", "roleSlug",
}

var trickyNumbers = []float64{0, 1, 100, 1.5, -2}

// confusable are other strings that are the same value when they are parsed
// as JSON, or that look like it.
var confusable = map[string][]string{
	"100":        {"1e2", " 100", "100 ", "0100", "100.0"},
	"1e2":        {"100"},
	" 100":       {"100"},
	"100 ":       {"100"},
	"0100":       {"100"},
	"-0":         {"0"},
	"true":       {" true"},
	"null":       {"", " null"},
	"":           {"null", " "},
	"[]":         {"{}", " []"},
	"{}":         {"[]", " {}"},
	`{"a":1}`:    {`{ "a": 1 }`, `{"a":1.0}`},
	`{ "a": 1 }`: {`{"a":1}`},
}

// convertedKind is a kind with field conversions, and the fields that they
// convert.
type convertedKind struct {
	gvk                schema.GroupVersionKind
	v1alpha1, v1alpha2 []string
}

func convertedKinds(t *testing.T) []convertedKind {
	t.Helper()
	pc := GetProvider()
	var out []convertedKind
	for tfName, fields := range conv.ByResource {
		r, ok := pc.Resources[tfName]
		if !ok {
			t.Fatalf("no resource configuration for %s", tfName)
		}
		k := convertedKind{gvk: schema.GroupVersionKind{Group: r.ShortGroup + ".crossplane.infisical.com", Version: conv.VersionV1alpha2, Kind: r.Kind}}
		for _, f := range fields {
			k.v1alpha1 = append(k.v1alpha1, f.V1alpha1...)
			k.v1alpha2 = append(k.v1alpha2, f.V1alpha2...)
		}
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].gvk.Kind < out[j].gvk.Kind })
	return out
}

// randomV1alpha2 returns a stored v1alpha2 object with random values in the
// converted fields of spec.forProvider and spec.initProvider.
func randomV1alpha2(t *testing.T, s *runtime.Scheme, k convertedKind, r *rand.Rand) resource.Terraformed {
	t.Helper()
	obj, err := s.New(k.gvk)
	if err != nil {
		t.Fatal(err)
	}
	obj.GetObjectKind().SetGroupVersionKind(k.gvk)
	spec := reflect.ValueOf(obj).Elem().FieldByName("Spec")
	for _, params := range []string{"ForProvider", "InitProvider"} {
		p := spec.FieldByName(params)
		for _, field := range k.v1alpha2 {
			fill(r, fieldByJSONName(t, p, field), 0)
		}
	}
	tr := obj.(resource.Terraformed)
	tr.SetName("random")
	return stored(t, s, tr)
}

func fieldByJSONName(t *testing.T, v reflect.Value, name string) reflect.Value {
	t.Helper()
	for i := 0; i < v.NumField(); i++ {
		if strings.Split(v.Type().Field(i).Tag.Get("json"), ",")[0] == name {
			return v.Field(i)
		}
	}
	t.Fatalf("%s has no field %s", v.Type(), name)
	return reflect.Value{}
}

// fill sets a random value. Pointers are sometimes nil, and lists and maps
// are sometimes empty.
func fill(r *rand.Rand, v reflect.Value, depth int) {
	switch v.Kind() {
	case reflect.Ptr:
		if r.Intn(4) == 0 {
			v.Set(reflect.Zero(v.Type()))
			return
		}
		e := reflect.New(v.Type().Elem())
		fill(r, e.Elem(), depth)
		v.Set(e)
	case reflect.String:
		v.SetString(trickyStrings[r.Intn(len(trickyStrings))])
	case reflect.Float64:
		v.SetFloat(trickyNumbers[r.Intn(len(trickyNumbers))])
	case reflect.Bool:
		v.SetBool(r.Intn(2) == 0)
	case reflect.Slice:
		n := r.Intn(4)
		if depth > 3 {
			n = 0
		}
		list := reflect.MakeSlice(v.Type(), n, n)
		for i := 0; i < n; i++ {
			fill(r, list.Index(i), depth+1)
		}
		v.Set(list)
	case reflect.Map:
		n := r.Intn(3)
		m := reflect.MakeMapWithSize(v.Type(), n)
		for i := 0; i < n; i++ {
			val := reflect.New(v.Type().Elem()).Elem()
			fill(r, val, depth+1)
			m.SetMapIndex(reflect.ValueOf(trickyStrings[r.Intn(len(trickyStrings))]).Convert(v.Type().Key()), val)
		}
		v.Set(m)
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() && v.Type().Field(i).Tag.Get("json") != "-" {
				fill(r, v.Field(i), depth+1)
			}
		}
	}
}

// mutateV1alpha2 returns a copy of a stored v1alpha2 object with small
// changes in the converted fields that are easy to confuse with no change.
func mutateV1alpha2(t *testing.T, s *runtime.Scheme, k convertedKind, obj resource.Terraformed, r *rand.Rand) resource.Terraformed {
	t.Helper()
	out := obj.DeepCopyObject().(resource.Terraformed)
	spec := reflect.ValueOf(out).Elem().FieldByName("Spec")
	for _, params := range []string{"ForProvider", "InitProvider"} {
		p := spec.FieldByName(params)
		for _, field := range k.v1alpha2 {
			mutate(r, fieldByJSONName(t, p, field))
		}
	}
	return stored(t, s, out)
}

// mutate changes some values: a string to a confusable string, an empty
// string to no value, and no value to an empty string.
func mutate(r *rand.Rand, v reflect.Value) {
	switch v.Kind() {
	case reflect.Ptr:
		switch {
		case v.IsNil() && v.Type().Elem().Kind() == reflect.String && r.Intn(5) == 0:
			e := reflect.New(v.Type().Elem())
			v.Set(e)
		case v.IsNil():
		case v.Elem().Kind() == reflect.String && v.Elem().String() == "" && r.Intn(3) == 0:
			v.Set(reflect.Zero(v.Type()))
		default:
			mutate(r, v.Elem())
		}
	case reflect.String:
		if others := confusable[v.String()]; len(others) > 0 && r.Intn(3) == 0 {
			v.SetString(others[r.Intn(len(others))])
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			mutate(r, v.Index(i))
		}
	case reflect.Map:
		for _, key := range v.MapKeys() {
			val := reflect.New(v.Type().Elem()).Elem()
			val.Set(v.MapIndex(key))
			mutate(r, val)
			v.SetMapIndex(key, val)
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() {
				mutate(r, v.Field(i))
			}
		}
	}
}

// rewriteLegacyText changes the JSON text of the given v1alpha1 fields the
// way a legacy client can write it: other spacing and key order, or an empty
// value of the same JSON type.
func rewriteLegacyText(t *testing.T, obj resource.Terraformed, names []string, r *rand.Rand) {
	t.Helper()
	for _, prefix := range []string{"spec.forProvider", "spec.initProvider"} {
		for _, n := range names {
			text, ok := value(t, obj, prefix+"."+n).(string)
			if !ok {
				continue
			}
			var parsed any
			if err := json.Unmarshal([]byte(text), &parsed); err != nil {
				continue
			}
			variants := []string{"", "null"}
			if raw, err := json.MarshalIndent(parsed, " ", "   "); err == nil {
				variants = append(variants, string(raw))
			}
			switch parsed.(type) {
			case []any:
				variants = append(variants, "[]", " [ ] ")
			case map[string]any:
				variants = append(variants, "{}", " { } ")
			}
			setField(t, obj, prefix+"."+n, variants[r.Intn(len(variants))])
		}
	}
}

// fields returns the JSON of the given fields of all parameter objects.
func fields(t *testing.T, obj resource.Terraformed, names []string) string {
	t.Helper()
	out := map[string]any{}
	for _, prefix := range []string{"spec.forProvider", "spec.initProvider"} {
		for _, n := range names {
			if v := value(t, obj, prefix+"."+n); v != nil {
				out[prefix+"."+n] = v
			}
		}
	}
	return jsonOf(t, out)
}

// setFields sets the given fields of all parameter objects to the values of
// another object, and deletes the fields that the other object does not have.
func setFields(t *testing.T, obj, from resource.Terraformed, names []string) {
	t.Helper()
	for _, prefix := range []string{"spec.forProvider", "spec.initProvider"} {
		for _, n := range names {
			setField(t, obj, prefix+"."+n, value(t, from, prefix+"."+n))
		}
	}
}

// fresh returns a copy of the object without the saved values of the
// conversion, as a client that never used the other API version wrote it.
func fresh(obj resource.Terraformed) resource.Terraformed {
	out := obj.DeepCopyObject().(resource.Terraformed)
	a := out.GetAnnotations()
	delete(a, conv.AnnotationKey)
	out.SetAnnotations(a)
	return out
}

// legacyData returns the data that the v1alpha1 fields of a v1alpha1 object
// hold, as the v1alpha2 values that a client without saved values gets. The
// same data can be different JSON text in v1alpha1, for example null and no
// key.
func legacyData(t *testing.T, s *runtime.Scheme, k convertedKind, v1 resource.Terraformed) string {
	t.Helper()
	return fields(t, stored(t, s, up(t, s, fresh(v1))), k.v1alpha2)
}

// legacyDataOf returns the data that a v1alpha2 object shows in v1alpha1.
func legacyDataOf(t *testing.T, s *runtime.Scheme, k convertedKind, v2 resource.Terraformed) string {
	t.Helper()
	return legacyData(t, s, k, stored(t, s, down(t, s, fresh(v2))))
}

// checkConversionProperties checks the properties for one random case of the
// kind.
func checkConversionProperties(t *testing.T, s *runtime.Scheme, k convertedKind, r *rand.Rand) {
	t.Helper()
	a := randomV1alpha2(t, s, k, r)
	// b is other random values, or small changes of a.
	b := randomV1alpha2(t, s, k, r)
	if r.Intn(2) == 0 {
		b = mutateV1alpha2(t, s, k, a, r)
	}
	// The v1alpha1 values that a legacy client writes: the v1alpha1 view of
	// b, as one object without saved values.
	bAsV1 := stored(t, s, down(t, s, fresh(b)))

	// No data loss: a v1alpha2 object stays the same after a legacy client
	// changes only a label.
	v1 := stored(t, s, down(t, s, a))
	v1.SetLabels(map[string]string{"changed": "label"})
	if got, want := fields(t, stored(t, s, up(t, s, v1)), k.v1alpha2), fields(t, a, k.v1alpha2); got != want {
		t.Errorf("a v1alpha1 label update changed the v1alpha2 values:\nwant %s\ngot  %s", want, got)
	}

	// Legacy writes win: a legacy client writes new values into all fields,
	// and then into one field at a time.
	writes := [][]string{k.v1alpha1}
	for _, f := range k.v1alpha1 {
		writes = append(writes, []string{f})
	}
	for _, names := range writes {
		v1 := stored(t, s, down(t, s, a))
		setFields(t, v1, bAsV1, names)
		if r.Intn(2) == 0 {
			rewriteLegacyText(t, v1, names, r)
		}
		v1 = stored(t, s, v1)
		written := fields(t, v1, k.v1alpha1)
		v2 := stored(t, s, up(t, s, v1))
		if got := fields(t, stored(t, s, down(t, s, v2)), k.v1alpha1); got != written {
			t.Errorf("a v1alpha1 client wrote %v and read back other values:\nwrote %s\nread  %s", names, written, got)
		}
		if got, want := legacyDataOf(t, s, k, v2), legacyData(t, s, k, v1); got != want {
			t.Errorf("a v1alpha1 client wrote %v, but v1alpha2 holds other data:\nwant %s\ngot  %s", names, want, got)
		}
	}

	// v1alpha2 writes are shown: a legacy client created the object, and a
	// v1alpha2 client writes new values.
	legacy := stored(t, s, down(t, s, fresh(a)))
	v2 := stored(t, s, up(t, s, fresh(legacy)))
	setFields(t, v2, b, k.v1alpha2)
	v2 = stored(t, s, v2)
	view := stored(t, s, down(t, s, v2))
	if got, want := legacyData(t, s, k, view), legacyDataOf(t, s, k, v2); got != want {
		t.Errorf("a v1alpha2 client wrote new values, but v1alpha1 shows other data:\nwant %s\ngot  %s", want, got)
	}
	view.SetLabels(map[string]string{"changed": "label"})
	if got, want := fields(t, stored(t, s, up(t, s, view)), k.v1alpha2), fields(t, v2, k.v1alpha2); got != want {
		t.Errorf("a v1alpha1 label update undid the v1alpha2 values:\nwant %s\ngot  %s", want, got)
	}
}

// TestConversionProperties checks the properties with fixed random seeds.
func TestConversionProperties(t *testing.T) {
	s := setup(t)
	for _, k := range convertedKinds(t) {
		t.Run(k.gvk.Kind, func(t *testing.T) {
			for seed := int64(1); seed <= 300; seed++ {
				checkConversionProperties(t, s, k, rand.New(rand.NewSource(seed)))
				if t.Failed() {
					t.Fatalf("failed with seed %d", seed)
				}
			}
		})
	}
}

// FuzzConversionProperties searches for other random cases:
//
//	go test ./config/ -run '^$' -fuzz FuzzConversionProperties -fuzztime 1m
func FuzzConversionProperties(f *testing.F) {
	for seed := int64(1); seed <= 10; seed++ {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, seed int64) {
		s := setup(t)
		for _, k := range convertedKinds(t) {
			checkConversionProperties(t, s, k, rand.New(rand.NewSource(seed)))
			if t.Failed() {
				t.Fatalf("%s failed with seed %d", k.gvk.Kind, seed)
			}
		}
	})
}
