package pulumi

import (
	"reflect"
)

type Dynamic struct {
	v interface{}
}

func (Dynamic) ElementType() reflect.Type {
	return dynamicType
}

// NewDynamicValue creates a new dynamic value.
func NewDynamicValue(v interface{}) Dynamic {
	switch v := v.(type) {
	case Dynamic:
		return v
	case Asset:
		return NewDynamicAsset(v)
	case Archive:
		return NewDynamicArchive(v)
	case Output:
		return NewDynamicOutput(v.ApplyT(func(v interface{}) (Dynamic, error) {
			dv := NewDynamicValue(v)
			return dv, nil
		}).(DynamicOutput))
	case Resource:
		return NewDynamicResource(v)
	}

	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Bool:
		return NewDynamicBool(rv.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return NewDynamicNumber(float64(rv.Int()))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return NewDynamicNumber(float64(rv.Uint()))
	case reflect.Float32, reflect.Float64:
		return NewDynamicNumber(rv.Float())
	case reflect.Ptr, reflect.Interface:
		// Dereference non-nil pointers and interfaces.
		if rv.IsNil() {
			return Dynamic{}
		}
		return NewDynamicValue(rv.Elem().Interface())
	case reflect.String:
		return NewDynamicString(rv.String())
	case reflect.Array, reflect.Slice:
		if rv.IsNil() {
			return Dynamic{}
		}

		// If an array or a slice, create a new array by recursing into elements.
		arr := make([]Dynamic, rv.Len())
		for i := range arr {
			arr[i] = NewDynamicValue(rv.Index(i).Interface())
		}
		return NewDynamicArray(arr)
	case reflect.Map:
		if rv.Type().Key().Kind() != reflect.String {
			panic(fmt.Errorf("map keys must be strings (got %v)", rv.Type().Key()))
		}

		if rv.IsNil() {
			return Dynamic{}
		}

		// For maps, only support string-based keys, and recurse into the values.
		m := make(map[string]Dynamic, rv.Len())
		for iter := rv.MapRange(); iter.Next(); {
			m[iter.Key().String()] = NewDynamicValue(iter.Value().Interface())
		}
		return NewDynamicMap(m)
	case reflect.Struct:
		fields := reflect.VisibleFields(rv.Type())

		m := make(map[string]Dynamic, len(fields))
		for _, f := range fields {
			m[f.Name] = NewDynamicValue(rv.FieldByIndex(f.Index).Interface())
		}
		return NewDynamicMap(m)
	default:
		panic(fmt.Errorf("cannot convert value of type %T to Dynamic", v))
	}
}

// NewDynamicBool creates a new dynamic bool value.
func NewDynamicBool(v bool) Dynamic { return Dynamic{v: v} }

// NewDynamicNumber creates a new dynamic number value.
func NewDynamicNumber(v float64) Dynamic { return Dynamic{v: v} }

// NewDynamicString creates a new dynamic string value.
func NewDynamicString(v string) Dynamic { return Dynamic{v: v} }

// NewDynamicArray creates a new dynamic array value.
func NewDynamicArray(v []Dynamic) Dynamic { return Dynamic{v: v} }

// NewDynamicAsset creates a new dynamic asset value.
func NewDynamicAsset(v Asset) Dynamic { return Dynamic{v: v} }

// NewDynamicArchive creates a new dynamic archive value.
func NewDynamicArchive(v Archive) Dynamic { return Dynamic{v: v} }

// NewDynamicMap creates a new dynamic map value.
func NewDynamicMap(v map[string]Dynamic) Dynamic { return Dynamic{v: v} }

// NewDynamicOutput creates a new dynamic output value.
func NewDynamicOutput(v DynamicOutput) Dynamic { return Dynamic{v: v} }

// NewDynamicResource creates a new dynamic resource value.
func NewDynamicResource(v Resource) Dynamic { return Dynamic{v: v} }

// BoolValue fetches the underlying bool value (panicking if it isn't a bool).
func (v Dynamic) BoolValue() bool { return v.v.(bool) }

// NumberValue fetches the underlying number value (panicking if it isn't a number).
func (v Dynamic) NumberValue() float64 { return v.v.(float64) }

// StringValue fetches the underlying string value (panicking if it isn't a string).
func (v Dynamic) StringValue() string { return v.v.(string) }

// AssetValue fetches the underlying asset value (panicking if it isn't an asset).
func (v Dynamic) AssetValue() Asset { return v.v.(Asset) }

// ArchiveValue fetches the underlying archive value (panicking if it isn't an archive).
func (v Dynamic) ArchiveValue() Archive { return v.v.(Archive) }

// ArrayValue fetches the underlying array value (panicking if it isn't a array).
func (v Dynamic) ArrayValue() []Dynamic { return v.v.([]Dynamic) }

// MapValue fetches the underlying map value (panicking if it isn't a object).
func (v Dynamic) MapValue() map[string]Dynamic { return v.v.(map[string]Dynamic) }

// OutputValue fetches the underlying output value (panicking if it isn't a output).
func (v Dynamic) OutputValue() DynamicOutput { return v.v.(DynamicOutput) }

// ResourceValue fetches the underlying resource value (panicking if it isn't a resource reference).
func (v Dynamic) ResourceValue() Resource { return v.v.(Resource) }

// IsNull returns true if the underlying value is a null.
func (v Dynamic) IsNull() bool {
	return v.v == nil
}

// IsBool returns true if the underlying value is a bool.
func (v Dynamic) IsBool() bool {
	_, is := v.v.(bool)
	return is
}

// IsNumber returns true if the underlying value is a number.
func (v Dynamic) IsNumber() bool {
	_, is := v.v.(float64)
	return is
}

// IsString returns true if the underlying value is a string.
func (v Dynamic) IsString() bool {
	_, is := v.v.(string)
	return is
}

// IsArray returns true if the underlying value is an array.
func (v Dynamic) IsArray() bool {
	_, is := v.v.([]Dynamic)
	return is
}

// IsAsset returns true if the underlying value is an asset.
func (v Dynamic) IsAsset() bool {
	_, is := v.v.(*Asset)
	return is
}

// IsArchive returns true if the underlying value is an archive.
func (v Dynamic) IsArchive() bool {
	_, is := v.v.(*Archive)
	return is
}

// IsMap returns true if the underlying value is a map.
func (v Dynamic) IsMap() bool {
	_, is := v.v.(map[string]Dynamic)
	return is
}

// IsOutput returns true if the underlying value is an output value.
func (v Dynamic) IsOutput() bool {
	_, is := v.v.(DynamicOutput)
	return is
}

// IsResource returns true if the underlying value is a resource value.
func (v Dynamic) IsResource() bool {
	_, is := v.v.(Resource)
	return is
}

var dynamicType = reflect.TypeOf((*Dynamic)(nil)).Elem()

// DynamicOutput is an Output that returns Dynamic values.
type DynamicOutput struct{ *OutputState }

func (DynamicOutput) ElementType() reflect.Type {
	return dynamicType
}

func (o DynamicOutput) Unwrap() DynamicOutput {
	result := newOutput(o.join, dynamicType, o.dependencies()...)
	go func() {
		secret, deps := false, ([]Resource)(nil)
		for {
			v, known, vSecret, vDeps, err := o.getState().await(context.TODO())
			if err != nil || !known {
				result.getState().fulfill(nil, known, secret, deps, err)
				return
			}

			secret = secret || vSecret
			deps = append(deps, vDeps...)

			dv := v.(Dynamic)
			if !dv.IsOutput() {
				result.getState().resolve(dv, true, secret, deps)
				return
			}
		}
	}()
	return result.(DynamicOutput)
}

func (o DynamicOutput) Apply(f func(v Dynamic) (Dynamic, error)) DynamicOutput {
	return o.ApplyT(f).(DynamicOutput)
}

func init() {
	RegisterOutputType(DynamicOutput{})
}
