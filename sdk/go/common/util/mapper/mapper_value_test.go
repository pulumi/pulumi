// Copyright 2026, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mapper

import (
	"errors"
	"math"
	"reflect"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type valueError string

func (e valueError) Error() string { return string(e) }

type valueText struct{ Text string }

func (v *valueText) UnmarshalText(text []byte) error {
	if string(text) == "invalid" {
		return errors.New("invalid text")
	}
	v.Text = string(text)
	return nil
}

func decodeTestValue(md Mapper, source, target any) FieldError {
	return md.DecodeValue(map[string]any{"value": source}, reflect.TypeFor[bag](), "value", target, false)
}

func TestDecodeValueConversions(t *testing.T) {
	t.Parallel()
	type namedInt int
	type namedString string
	type record struct{ Untagged int }
	for _, tt := range []struct {
		name   string
		source any
		target any
		want   any
	}{
		{"assignable struct ignores tags", record{42}, new(record), record{42}},
		{"named integers convert to their underlying type", namedInt(42), new(int), 42},
		{"strings convert to named string types", "hello", new(namedString), namedString("hello")},
		{"float to integer conversion truncates without error", 42.9, new(int), 42},
		{"integer conversion overflows without error", int64(257), new(int8), int8(1)},
		{"strings decode through TextUnmarshaler", "hello", new(valueText), valueText{"hello"}},
		{"decoding into any preserves the concrete numeric type", int32(42), new(any), int32(42)},
		{"values decode into interfaces they implement", valueError("hello"), new(error), valueError("hello")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.NoError(t, decodeTestValue(New(nil), tt.source, tt.target))
			assert.Equal(t, tt.want, reflect.ValueOf(tt.target).Elem().Interface())
		})
	}

	t.Run("source and destination pointer chains", func(t *testing.T) {
		t.Parallel()
		source := 42
		p := &source
		// Nil destination pointer chains are allocated; source chains are dereferenced.
		var target **int
		require.NoError(t, decodeTestValue(New(nil), &p, &target))
		require.NotNil(t, target)
		require.NotNil(t, *target)
		assert.Equal(t, 42, **target)
		// Existing destination pointers are reused rather than replaced.
		original := *target
		require.NoError(t, decodeTestValue(New(nil), 99, &target))
		assert.Same(t, original, *target)
		assert.Equal(t, 99, **target)
		// Decoding into any preserves the dereferenced source's concrete type.
		var untyped any
		require.NoError(t, decodeTestValue(New(nil), &p, &untyped))
		assert.IsType(t, int(0), untyped)
		assert.Equal(t, 42, untyped)
	})
}

func TestDecodeValueMissingAndInvalid(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name     string
		obj      map[string]any
		optional bool
		ignore   bool
	}{
		{"nil object reports a missing field", nil, false, false},
		{"nil interface reports a missing field", map[string]any{"value": nil}, false, false},
		{"optional nil interface leaves the destination unchanged", map[string]any{"value": nil}, true, false},
		{"IgnoreMissing permits absent keys", map[string]any{}, false, true},
		{"IgnoreMissing permits nil interface values", map[string]any{"value": nil}, false, true},
		{"optional and IgnoreMissing may be combined", nil, true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			target := 42
			err := New(&Opts{IgnoreMissing: tt.ignore}).DecodeValue(
				tt.obj, reflect.TypeFor[bag](), "value", &target, tt.optional)
			if tt.optional || tt.ignore {
				require.NoError(t, err)
			} else {
				require.IsType(t, &MissingError{}, err)
				assert.Equal(t, "value", err.Field())
			}
			assert.Equal(t, 42, target)
		})
	}
	for _, tt := range []struct {
		name   string
		source any
		target any
	}{
		{"nil target panics", 42, nil},
		{"non-pointer target panics", 42, 42},
		{"nil destination pointer panics", 42, (*int)(nil)},
		{"typed nil source pointer panics", (*int)(nil), new(int)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Panics(t, func() { _ = decodeTestValue(New(nil), tt.source, tt.target) })
		})
	}
	t.Run("destination pointers may be allocated before a type error", func(t *testing.T) {
		t.Parallel()
		var target *int
		err := decodeTestValue(New(nil), true, &target)
		require.IsType(t, &WrongTypeError{}, err)
		require.NotNil(t, target)
		assert.Zero(t, *target)
	})
	t.Run("values cannot decode into interfaces they do not implement", func(t *testing.T) {
		t.Parallel()
		var target error
		require.IsType(t, &WrongTypeError{}, decodeTestValue(New(nil), 42, &target))
	})
}

func TestDecodeValueCollections(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name   string
		source any
	}{
		{"nil source slice replaces the destination with nil", []any(nil)},
		{"empty source slice replaces the destination with nil", []any{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			target := []string{"old"}
			require.NoError(t, decodeTestValue(New(nil), tt.source, &target))
			assert.Nil(t, target)
		})
	}
	for _, tt := range []struct {
		name   string
		source any
	}{
		{"nil source map replaces the destination with a non-nil empty map", map[string]any(nil)},
		{"empty source map replaces the destination with a non-nil empty map", map[string]any{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			target := map[string]string{"old": "entry"}
			require.NoError(t, decodeTestValue(New(nil), tt.source, &target))
			require.NotNil(t, target)
			assert.Empty(t, target)
		})
	}
	t.Run("recursive slice replacement", func(t *testing.T) {
		t.Parallel()
		source := []any{float64(42), float64(99)}
		backing := []int{1, 2, 3}
		target := backing
		require.NoError(t, decodeTestValue(New(nil), source, &target))
		assert.Equal(t, []int{42, 99}, target)
		assert.Equal(t, []int{1, 2, 3}, backing)
		target[0] = 0
		assert.Equal(t, float64(42), source[0])
	})
	t.Run("recursive map keys and values", func(t *testing.T) {
		t.Parallel()
		source := map[int64][]any{42: {float64(99)}}
		old := map[int][]int{1: {2}}
		target := old
		require.NoError(t, decodeTestValue(New(nil), source, &target))
		assert.Equal(t, map[int][]int{42: {99}}, target)
		assert.Equal(t, map[int][]int{1: {2}}, old)
	})
	t.Run("assignment takes precedence over collection conversion", func(t *testing.T) {
		t.Parallel()
		source := []int{42}
		var target []int
		require.NoError(t, decodeTestValue(New(nil), source, &target))
		target[0] = 99
		assert.Equal(t, 99, source[0])
		m := map[string]int{"a": 42}
		var dest map[string]int
		require.NoError(t, decodeTestValue(New(nil), m, &dest))
		dest["a"] = 99
		assert.Equal(t, 99, m["a"])
	})
}

func TestDecodeValueErrors(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name   string
		source any
		target any
		field  string
	}{
		{"slice element errors identify the failing index", []any{true}, new([]int), "value[0]"},
		{"map key errors identify the failing key", map[bool]int{true: 1}, new(map[int]int), "value[true] key"},
		{"map value errors identify the failing entry", map[string]bool{"a": true}, new(map[string]int), "value[a] value"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := decodeTestValue(New(nil), tt.source, tt.target)
			require.IsType(t, &WrongTypeError{}, err)
			assert.Equal(t, tt.field, err.Field())
		})
	}
	type required struct {
		Value int `json:"required"`
	}
	for _, tt := range []struct {
		name   string
		source any
		target any
		text   string
	}{
		{"UnmarshalText errors are wrapped in a FieldError", "invalid", new(valueText), "invalid text"},
		{
			"nested struct errors are wrapped in a FieldError",
			map[string]any{},
			new(required),
			"Missing required field 'required'",
		},
		{
			"objects cannot decode into scalars without a custom decoder",
			map[string]any{},
			new(int),
			"no custom decoder exists",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := decodeTestValue(New(nil), tt.source, tt.target)
			require.IsType(t, &fieldError{}, err)
			assert.Equal(t, "value", err.Field())
			assert.Contains(t, err.Error(), tt.text)
		})
	}
}

func TestDecodeValueStructIgnoreMissing(t *testing.T) {
	t.Parallel()
	type record struct {
		Required int `json:"required"`
		Optional int `json:"optional,omitempty"`
		Untagged int
	}
	// IgnoreMissing accepts absent required fields, but object decoding still resets all fields.
	target := record{Required: 1, Optional: 2, Untagged: 3}
	require.NoError(t, decodeTestValue(New(&Opts{IgnoreMissing: true}), map[string]any{}, &target))
	assert.Equal(t, record{}, target, "decoding an object replaces rather than merges the struct")
}

func TestDecodeValueCustomDecoderPrecedence(t *testing.T) {
	t.Parallel()
	calls := 0
	md := New(&Opts{CustomDecoders: Decoders{
		reflect.TypeFor[valueText](): func(_ Mapper, obj map[string]any) (any, error) {
			calls++
			if _, fail := obj["fail"]; fail {
				return nil, errors.New("custom failure")
			}
			return valueText{"custom"}, nil
		},
		reflect.TypeFor[map[string]string](): func(Mapper, map[string]any) (any, error) {
			t.Fatal("custom decoder must not be used for maps")
			return nil, nil
		},
	}})
	// A custom decoder takes precedence over tagged struct decoding for object sources.
	var target valueText
	require.NoError(t, decodeTestValue(md, map[string]any{}, &target))
	assert.Equal(t, valueText{"custom"}, target)
	assert.Equal(t, 1, calls)
	// Assignable sources bypass custom decoders.
	require.NoError(t, decodeTestValue(md, valueText{"assigned"}, &target))
	assert.Equal(t, valueText{"assigned"}, target)
	// String sources use TextUnmarshaler rather than custom object decoders.
	require.NoError(t, decodeTestValue(md, "text", &target))
	assert.Equal(t, valueText{"text"}, target)
	assert.Equal(t, 1, calls)
	// Custom decoder failures are wrapped with the destination field name.
	err := decodeTestValue(md, map[string]any{"fail": true}, &target)
	require.IsType(t, &fieldError{}, err)
	assert.Equal(t, "value", err.Field())
	assert.Contains(t, err.Error(), "custom failure")
	// Map-to-map decoding recursively converts entries without calling custom decoders.
	var m map[string]string
	require.NoError(t, decodeTestValue(md, map[string]any{"a": "b"}, &m))
	assert.Equal(t, map[string]string{"a": "b"}, m)
}

func TestEncodeValueKinds(t *testing.T) {
	t.Parallel()
	type namedBool bool
	type namedString string
	type namedSlice []byte
	type namedKey string
	type namedMap map[namedKey]any
	type namedStruct struct {
		Value int `json:"value"`
	}
	value := 42
	pointer := &value
	for _, tt := range []struct {
		name   string
		source any
		want   any
	}{
		{"named booleans encode as bool", namedBool(true), true},
		{"strings are neither escaped nor UTF-8 validated", "\"\\\n\xff", "\"\\\n\xff"},
		{"named strings encode as string", namedString("hello"), "hello"},
		{"pointer chains encode the final value", &pointer, float64(42)},
		{"non-nil empty slices remain non-nil", []int{}, []any{}},
		{"bytes encode as numeric elements rather than base64", []byte{0, 255}, []any{float64(0), float64(255)}},
		{"named slices encode recursively", namedSlice{42}, []any{float64(42)}},
		{"nil slice elements are retained", []any{nil, (*int)(nil), []int(nil)}, []any{nil, nil, nil}},
		{"non-nil empty maps remain non-nil", map[string]int{}, map[string]any{}},
		{
			"named maps accept string-kind keys and encode values recursively",
			namedMap{"a": []int{42}, "b": nil},
			map[string]any{"a": []any{float64(42)}, "b": nil},
		},
		{"named structs encode their tagged fields", namedStruct{42}, map[string]any{"value": float64(42)}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := New(nil).EncodeValue(tt.source)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
	// TestMapperEncodeValue already covers int, uint, and float32; cover the other kinds here.
	type namedInt int64
	type namedUint uint64
	type namedFloat float64
	for _, tt := range []struct {
		name   string
		source any
		want   float64
	}{
		{"int8 encodes as float64", int8(-42), -42},
		{"int16 encodes as float64", int16(-42), -42},
		{"int32 encodes as float64", int32(-42), -42},
		{"int64 encodes as float64", int64(-42), -42},
		{"uint8 encodes as float64", uint8(42), 42},
		{"uint16 encodes as float64", uint16(42), 42},
		{"uint32 encodes as float64", uint32(42), 42},
		{"uint64 encodes as float64", uint64(42), 42},
		{"uintptr encodes as float64", uintptr(42), 42},
		{"float64 retains its value", float64(42.5), 42.5},
		{"named signed integers encode as float64", namedInt(-42), -42},
		{"named unsigned integers encode as float64", namedUint(42), 42},
		{"named floats encode as float64", namedFloat(42.5), 42.5},
		{"large integers may lose precision", uint64(1<<53 + 1), 1 << 53},
		{"positive infinity is preserved", math.Inf(1), math.Inf(1)},
		{"negative infinity is preserved", math.Inf(-1), math.Inf(-1)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := New(nil).EncodeValue(tt.source)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
	t.Run("NaN is preserved", func(t *testing.T) {
		t.Parallel()
		got, err := New(nil).EncodeValue(math.NaN())
		require.NoError(t, err)
		assert.True(t, math.IsNaN(got.(float64)))
	})
}

func TestEncodeValueUnsupportedKinds(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name   string
		source any
	}{
		{"arrays panic rather than encoding as slices", [1]int{42}},
		{"complex64 values panic", complex64(1)},
		{"complex128 values panic", complex128(1)},
		{"channels panic", make(chan int)},
		{"functions panic", func() {}},
		{"unsafe pointers panic", unsafe.Pointer(new(int))},
		{"maps with non-string-kind keys panic", map[int]string{1: "a"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Panics(t, func() { _, _ = New(nil).EncodeValue(tt.source) })
		})
	}
}

func TestEncodeValueErrorAggregation(t *testing.T) {
	t.Parallel()
	type invalid struct {
		Good int  `json:"good"`
		A    *int `json:"a"`
		B    *int `json:"b"`
	}
	// Struct encoding collects missing-field errors while retaining successfully encoded fields.
	v := invalid{Good: 42}
	got, err := New(nil).EncodeValue(v)
	require.Error(t, err)
	assert.Equal(t, map[string]any{"good": float64(42)}, got)
	require.Len(t, err.Failures(), 2)
	for i, key := range []string{"a", "b"} {
		failure := err.Failures()[i]
		require.IsType(t, &MissingError{}, failure)
		assert.Equal(t, key, failure.(FieldError).Field())
	}
	for _, tt := range []struct {
		name   string
		source any
	}{
		{"slice encoding collects all element errors and returns nil", []any{v, v}},
		{"map encoding collects all value errors and returns nil", map[string]any{"first": v, "second": v}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := New(nil).EncodeValue(tt.source)
			require.Error(t, err)
			assert.Nil(t, got)
			require.Len(t, err.Failures(), 4)
		})
	}
	// IgnoreMissing omits required nil fields without reporting errors.
	got, err = New(&Opts{IgnoreMissing: true}).EncodeValue(v)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"good": float64(42)}, got)
}

//nolint:staticcheck // The mapper intentionally supports nonstandard json tag options.
func TestValueStructTags(t *testing.T) {
	t.Parallel()
	type tagged struct {
		Both     int `json:"first" pulumi:"second"` // Both recognized tags produce keys.
		Empty    int `json:",omitempty"`            // An empty key does not fall back to the field name.
		Skipped  int `json:"skipped,skip"`          // Skipped keys are recognized even with incompatible values.
		Dash     int `json:"-"`                     // A dash key skips encoding and decoding.
		Untagged int // Untagged fields are ignored and reset on object decoding.
		EmptyTag int `json:""`                  // An empty tag is not a recognized tag.
		Optional int `json:"optional,optional"` // Optional missing fields reset to zero.
		Omit     int `json:"omit,omitempty"`    // Omitempty also makes decoding optional.
	}
	md := New(nil)
	want := map[string]any{
		"first": float64(3), "second": float64(3),
		"": float64(0), "optional": float64(0), "omit": float64(0),
	}
	got, err := md.EncodeValue(tagged{Both: 3, Skipped: 4, Dash: 5, Untagged: 6, EmptyTag: 7})
	require.NoError(t, err)
	assert.Equal(t, want, got)
	want["skipped"], want["-"] = true, true // Skipped keys are recognized without decoding their values.
	target := tagged{Untagged: 99, Optional: 99, Omit: 99}
	delete(want, "optional")
	delete(want, "omit")
	require.NoError(t, decodeTestValue(md, want, &target))
	assert.Equal(t, tagged{Both: 3}, target)

	for _, tt := range []struct {
		name   string
		ignore bool
	}{
		{"key matching is case-sensitive", false},
		{"IgnoreUnrecognized permits incorrectly cased keys", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			md := New(&Opts{IgnoreUnrecognized: tt.ignore})
			var target tagged
			input := map[string]any{"first": 3, "second": 3, "First": 4}
			err := decodeTestValue(md, input, &target)
			if tt.ignore {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "Unrecognized field 'First'")
			}
		})
	}
}

//nolint:staticcheck // Exercise overriding the mapper's nonstandard json tag options.
func TestValueCustomTags(t *testing.T) {
	t.Parallel()
	type tagged struct {
		Value   int  `custom:"value" json:"ignored"` // Custom tag names replace the defaults.
		Nil     *int `custom:"nil,maybe"`            // Custom optional options permit nil and missing fields.
		Skipped int  `custom:"skipped,exclude"`      // Custom skip options exclude recognized fields.
	}
	md := New(&Opts{Tags: []string{"custom"}, OptionalTags: []string{"maybe"}, SkipTags: []string{"exclude"}})
	got, err := md.EncodeValue(tagged{Value: 42})
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"value": float64(42)}, got)
	var target tagged
	require.NoError(t, decodeTestValue(md, map[string]any{"value": 42, "skipped": true}, &target))
	assert.Equal(t, tagged{Value: 42}, target)
	// An explicitly empty tag list disables all field mapping.
	got, err = New(&Opts{Tags: []string{}}).EncodeValue(tagged{})
	require.NoError(t, err)
	assert.Equal(t, map[string]any{}, got)

	for _, tt := range []struct {
		name   string
		source any
	}{
		{"unknown tag options panic", struct {
			Value int `json:"value,unknown"`
		}{}},
		{"overridden optional options no longer recognize optional", struct {
			Value int `json:"value,optional"`
		}{}},
		{"overridden skip options no longer recognize skip", struct {
			Value int `json:"value,skip"`
		}{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			md := New(&Opts{OptionalTags: []string{}, SkipTags: []string{}})
			assert.Panics(t, func() { _, _ = md.EncodeValue(tt.source) })
			assert.Panics(t, func() {
				_ = decodeTestValue(md, map[string]any{}, reflect.New(reflect.TypeOf(tt.source)).Interface())
			})
		})
	}
}

//nolint:staticcheck // The mapper intentionally supports nonstandard json tag options.
func TestEncodeValueOptionalZeroValues(t *testing.T) {
	t.Parallel()
	zero := 0
	source := struct {
		Bool    bool           `json:"bool,omitempty"`    // False is retained.
		Int     int            `json:"int,optional"`      // Zero is retained.
		String  string         `json:"string,omitempty"`  // Empty strings are retained.
		Slice   []int          `json:"slice,omitempty"`   // Non-nil empty slices are retained.
		Map     map[string]int `json:"map,omitempty"`     // Non-nil empty maps are retained.
		Pointer *int           `json:"pointer,omitempty"` // Pointers to zero are retained.
		NilMap  map[string]int `json:"nilMap,omitempty"`  // Nil maps are omitted.
		NilList []int          `json:"nilList,optional"`  // Nil slices are omitted.
		NilAny  any            `json:"nilAny,omitempty"`  // Nil interfaces are omitted.
	}{Slice: []int{}, Map: map[string]int{}, Pointer: &zero}
	got, err := New(nil).EncodeValue(source)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{
		"bool": false, "int": float64(0), "string": "", "slice": []any{},
		"map": map[string]any{}, "pointer": float64(0),
	}, got)
}

func TestValueEmbeddedFields(t *testing.T) {
	t.Parallel()
	type Embedded struct {
		Value int `json:"value"`
	}
	type Ignored struct {
		Other int `json:"other"`
	}
	type Scalar int
	type outer struct {
		Embedded     // Embedded struct fields are flattened.
		*Ignored     // Embedded pointers to structs are ignored.
		Scalar       // Other anonymous fields are ignored.
		First    int `pulumi:"duplicate"` // Earlier values for duplicate keys are overwritten.
		Last     int `json:"duplicate"`   // The last value wins during encoding.
	}
	source := outer{Embedded: Embedded{42}, Ignored: &Ignored{99}, Scalar: 99, First: 1, Last: 2}
	got, err := New(nil).EncodeValue(source)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"value": float64(42), "duplicate": float64(2)}, got)
	// Decoding a duplicate key populates both tagged fields.
	var target outer
	require.NoError(t, decodeTestValue(New(nil), got, &target))
	assert.Equal(t, outer{Embedded: Embedded{42}, First: 2, Last: 2}, target)

	// Go field selection favors the outer field over an embedded field with the same name.
	type shadow struct {
		Embedded
		Value int `json:"value"`
	}
	got, err = New(nil).EncodeValue(shadow{Embedded: Embedded{1}, Value: 2})
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"value": float64(2)}, got)
	var dest shadow
	require.NoError(t, decodeTestValue(New(nil), got, &dest))
	assert.Equal(t, shadow{Value: 2}, dest)
}
