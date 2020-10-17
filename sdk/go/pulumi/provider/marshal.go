package provider

import (
	"fmt"
	"reflect"

	"github.com/pulumi/pulumi/sdk/v2/go/common/resource"
)

type Unmarshaler interface {
	UnmarshalPropertyValue(v resource.PropertyValue) error
}

type Marshaler interface {
	MarshalPropertyValue() (resource.PropertyValue, error)
}

var unmarshalerType = reflect.TypeOf((*Unmarshaler)(nil)).Elem()

func isUnmarshaler(dest reflect.Value) (Unmarshaler, bool) {
	if dest.Kind() != reflect.Ptr {
		if !dest.CanAddr() {
			return nil, false
		}
		dest = dest.Addr()
	}
	if dest.Type().Implements(unmarshalerType) {
		return dest.Interface().(Unmarshaler), true
	}
	return nil, false
}

func unmarshalProperty(path string, v resource.PropertyValue, dest reflect.Value) error {
	if unmarshaler, ok := isUnmarshaler(dest); ok {
		return unmarshaler.UnmarshalPropertyValue(v)
	}

	for v.IsSecret() {
		v = v.SecretValue().Element
	}

	if v.IsComputed() || v.IsOutput() {
		return nil
	}

	switch dest.Kind() {
	case reflect.Bool:
		if !v.IsBool() {
			return failureError(typeMismatch(path, "bool", v))
		}
		dest.SetBool(v.BoolValue())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if !v.IsNumber() {
			return failureError(typeMismatch(path, "number", v))
		}
		dest.SetInt(int64(v.NumberValue()))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if !v.IsNumber() {
			return failureError(typeMismatch(path, "number", v))
		}
		dest.SetUint(uint64(v.NumberValue()))
	case reflect.Float32, reflect.Float64:
		if !v.IsNumber() {
			return failureError(typeMismatch(path, "number", v))
		}
		dest.SetFloat(v.NumberValue())
	case reflect.String:
		switch {
		case v.IsString():
			dest.SetString(v.StringValue())
		case v.IsAsset():
			dest.SetString(v.AssetValue().Path)
		case v.IsArchive():
			dest.SetString(v.ArchiveValue().Path)
		default:
			return failureError(typeMismatch(path, "string", v))
		}
	case reflect.Slice:
		if !v.IsArray() {
			return failureError(typeMismatch(path, "[]", v))
		}
		arrayValue := v.ArrayValue()
		slice := reflect.MakeSlice(dest.Type(), len(arrayValue), len(arrayValue))
		for i, e := range arrayValue {
			if err := unmarshalProperty(listElementPath(path, i), e, slice.Index(i)); err != nil {
				return err
			}
		}
		dest.Set(slice)
	case reflect.Map:
		if dest.Type().Key().Kind() != reflect.String {
			return fmt.Errorf("map schema must have string keys")
		}
		if !v.IsObject() {
			return failureError(typeMismatch(path, "object", v))
		}
		m := reflect.MakeMap(dest.Type())
		for k, e := range v.ObjectValue() {
			me := reflect.New(dest.Type().Elem()).Elem()
			if err := unmarshalProperty(objectPropertyPath(path, string(k)), e, me); err != nil {
				return err
			}
			m.SetMapIndex(reflect.ValueOf(string(k)), me)
		}
		dest.Set(m)
	case reflect.Struct:
		if !v.IsObject() {
			return failureError(typeMismatch(path, "object", v))
		}
		m := v.ObjectValue()
		for i := 0; i < dest.NumField(); i++ {
			f := dest.Field(i)
			desc, err := getFieldDesc(dest.Type().Field(i))
			if err != nil {
				return err
			}
			if desc == nil {
				continue
			}

			e, ok := m[resource.PropertyKey(desc.name)]
			if !ok || e.IsNull() {
				f.Set(reflect.Zero(f.Type()))
				continue
			}
			if err := unmarshalProperty(objectPropertyPath(path, desc.name), e, f); err != nil {
				return err
			}
		}
	case reflect.Ptr:
		if v.IsNull() {
			dest.Set(reflect.Zero(dest.Type()))
		} else {
			if dest.IsNil() {
				dest.Set(reflect.New(dest.Type().Elem()))
			}
			if err := unmarshalProperty(path, v, dest.Elem()); err != nil {
				return err
			}
		}
	case reflect.Interface:
		if dest.NumMethod() != 0 {
			return fmt.Errorf("unsupported type '%v' for property %v", dest.Type().Name(), path)
		}
		dest.Set(reflect.ValueOf(v))
	default:
		return fmt.Errorf("unsupported type '%v' for property %v", dest.Type().Name(), path)
	}

	return nil
}

func Unmarshal(m resource.PropertyMap, dest interface{}) error {
	v := reflect.ValueOf(dest)
	if v.Kind() != reflect.Ptr {
		return fmt.Errorf("dest type must be a pointer")
	}
	return unmarshalProperty("", resource.NewObjectProperty(m), v)
}

var marshalerType = reflect.TypeOf((*Marshaler)(nil)).Elem()

func marshalProperty(v reflect.Value) (resource.PropertyValue, error) {
	if v.Type().Implements(marshalerType) {
		return v.Interface().(Marshaler).MarshalPropertyValue()
	}

	switch v.Kind() {
	case reflect.Bool:
		return resource.NewBoolProperty(v.Bool()), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return resource.NewNumberProperty(float64(v.Int())), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return resource.NewNumberProperty(float64(v.Uint())), nil
	case reflect.Float32, reflect.Float64:
		return resource.NewNumberProperty(v.Float()), nil
	case reflect.String:
		return resource.NewStringProperty(v.String()), nil
	case reflect.Slice:
		s := make([]resource.PropertyValue, v.Len())
		for i := 0; i < v.Len(); i++ {
			e, err := marshalProperty(v.Index(i))
			if err != nil {
				return resource.PropertyValue{}, err
			}
			s[i] = e
		}
		return resource.NewArrayProperty(s), nil
	case reflect.Map:
		if v.Type().Key().Kind() != reflect.String {
			return resource.PropertyValue{}, fmt.Errorf("map values must have string keys")
		}
		m := make(resource.PropertyMap)
		for _, k := range v.MapKeys() {
			e, err := marshalProperty(v.MapIndex(k))
			if err != nil {
				return resource.PropertyValue{}, err
			}
			m[resource.PropertyKey(k.String())] = e
		}
		return resource.NewObjectProperty(m), nil
	case reflect.Struct:
		m := make(resource.PropertyMap)
		for i := 0; i < v.NumField(); i++ {
			f := v.Field(i)
			desc, err := getFieldDesc(v.Type().Field(i))
			if err != nil {
				return resource.PropertyValue{}, err
			}
			if desc == nil {
				continue
			}

			e, err := marshalProperty(f)
			if err != nil {
				return resource.PropertyValue{}, err
			}
			m[resource.PropertyKey(desc.name)] = e
		}
		return resource.NewObjectProperty(m), nil
	case reflect.Ptr:
		if v.IsNil() {
			return resource.NewNullProperty(), nil
		}
		return marshalProperty(v.Elem())
	default:
		return resource.PropertyValue{}, fmt.Errorf("unsupported type '%v'", v.Type().Name())
	}
}

func Marshal(src interface{}) (resource.PropertyMap, error) {
	v, err := marshalProperty(reflect.ValueOf(src))
	if err != nil {
		return nil, err
	}
	if !v.IsObject() {
		return nil, fmt.Errorf("marshaled properties must be a map, not a %v", v.TypeString())
	}
	return v.ObjectValue(), nil
}
