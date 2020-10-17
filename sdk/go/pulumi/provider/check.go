package provider

import (
	"fmt"
	"reflect"

	"github.com/pulumi/pulumi/sdk/v2/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource/plugin"
)

func typeMismatch(path, expected string, actual resource.PropertyValue) plugin.CheckFailure {
	return plugin.CheckFailure{
		Property: resource.PropertyKey(path),
		Reason:   fmt.Sprintf("expected a %v value, received a %v", expected, actual.TypeString()),
	}
}

func missingRequiredProperty(path, key string) plugin.CheckFailure {
	return plugin.CheckFailure{
		Property: resource.PropertyKey(path),
		Reason:   fmt.Sprintf("missing required property %v", key),
	}
}

func failureError(f plugin.CheckFailure) error {
	return fmt.Errorf("%v: %v", f.Property, f.Reason)
}

type checker struct {
	failures []plugin.CheckFailure
}

var propertyValueType = reflect.TypeOf((*resource.PropertyValue)(nil)).Elem()

func (c *checker) checkProperty(path string, v resource.PropertyValue, schema reflect.Type) error {
	if schema == propertyValueType {
		return nil
	}

	for v.IsSecret() {
		v = v.SecretValue().Element
	}

	if v.IsComputed() || v.IsOutput() {
		return nil
	}

	switch schema.Kind() {
	case reflect.Bool:
		if !v.IsBool() {
			c.failures = append(c.failures, typeMismatch(path, "bool", v))
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if !v.IsNumber() {
			c.failures = append(c.failures, typeMismatch(path, "number", v))
		}
	case reflect.Float32, reflect.Float64:
		if !v.IsNumber() {
			c.failures = append(c.failures, typeMismatch(path, "number", v))
		}
	case reflect.String:
		if !v.IsString() && !v.IsArchive() && !v.IsAsset() {
			c.failures = append(c.failures, typeMismatch(path, "string", v))
		}
	case reflect.Slice:
		if !v.IsArray() {
			c.failures = append(c.failures, typeMismatch(path, "[]", v))
		}
		for i, e := range v.ArrayValue() {
			if err := c.checkProperty(listElementPath(path, i), e, schema.Elem()); err != nil {
				return err
			}
		}
	case reflect.Map:
		if schema.Key().Kind() != reflect.String {
			return fmt.Errorf("map schema must have string keys")
		}
		if !v.IsObject() {
			c.failures = append(c.failures, typeMismatch(path, "object", v))
		} else {
			for k, e := range v.ObjectValue() {
				if err := c.checkProperty(objectPropertyPath(path, string(k)), e, schema.Elem()); err != nil {
					return err
				}
			}
		}
	case reflect.Struct:
		if !v.IsObject() {
			c.failures = append(c.failures, typeMismatch(path, "object", v))
		} else {
			m := v.ObjectValue()
			for i := 0; i < schema.NumField(); i++ {
				f := schema.Field(i)
				desc, err := getFieldDesc(f)
				if err != nil {
					return err
				}
				if desc == nil {
					continue
				}

				e, ok := m[resource.PropertyKey(desc.name)]
				if !ok || e.IsNull() {
					if desc.required {
						c.failures = append(c.failures, missingRequiredProperty(path, desc.name))
					}
					continue
				}

				if err := c.checkProperty(objectPropertyPath(path, desc.name), e, f.Type); err != nil {
					return err
				}
			}
		}
	case reflect.Ptr:
		if !v.IsNull() {
			return c.checkProperty(path, v, schema.Elem())
		}
	case reflect.Interface:
		if schema.NumMethod() != 0 {
			return fmt.Errorf("unsupported type '%v'", schema.Name())
		}
	default:
		return fmt.Errorf("unsupported type '%v'", schema.Name())
	}

	return nil
}

func Check(inputs resource.PropertyMap, schema reflect.Type) (resource.PropertyMap, []plugin.CheckFailure, error) {
	c := &checker{}
	if err := c.checkProperty("", resource.NewObjectProperty(inputs), schema); err != nil {
		return nil, nil, err
	}
	return inputs, c.failures, nil
}
