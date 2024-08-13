package main

import (
	"reflect"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func valueFromRc(v *viper.Viper, name, iniSection string) any {
	if v.IsSet(name) {
		return v.Get(name)
	}
	sectionName := iniSection + "." + name
	if v.IsSet(sectionName) {
		return v.Get(sectionName)
	}
	defaultName := "default." + name
	if v.IsSet(defaultName) {
		return v.Get(defaultName)
	}
	return v.Get(name)
}

func defaultValueFromRc(v *viper.Viper, kind reflect.Kind, name, iniSection string) any {
	val := valueFromRc(v, name, iniSection)
	//nolint:exhaustive
	switch kind {
	case reflect.Bool:
		if val == nil {
			return false
		}
		return val
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if val == nil {
			return int(0)
		}
		return val
	case reflect.String:
		if val == nil {
			return ""
		}
		return val
	case reflect.Array | reflect.Slice:
		if val == nil {
			return []string{}
		}
		return val
	default:
		contract.Failf("unexpected type, %v", kind)
	}
	return nil
}

func dashedFieldName(name string) string {
	var result string
	for i, c := range name {
		if i > 0 && 'A' <= c && c <= 'Z' {
			result += "-"
		}
		result += strings.ToLower(string(c))
	}
	return result
}

func UnmarshalOpts[T any](v *viper.Viper, iniSection string) T {
	typ := reflect.TypeFor[T]()
	val := reflect.New(typ).Elem().Interface()
	return unmarshalOpts(v, val, iniSection).(T)
}

func unmarshalOpts(v *viper.Viper, opts any, iniSection string) any {
	ref := reflect.ValueOf(opts)
	//nolint:exhaustive
	switch ref.Kind() {
	case reflect.Struct:
		rv := reflect.New(ref.Type()).Elem()
		for i := 0; i < ref.NumField(); i++ {
			if !rv.Field(i).CanSet() {
				panic("can't set field " + ref.Type().Field(i).Name)
			}
			fieldName := dashedFieldName(ref.Type().Field(i).Name)

			tag := rv.Type().Field(i).Tag.Get("args")
			if tag != "" {
				fieldName = tag
			}

			//nolint:exhaustive
			switch rv.Field(i).Kind() {
			case reflect.Struct:
				rv.Field(i).Set(reflect.ValueOf(unmarshalOpts(v, rv.Field(i).Interface(), iniSection)))
			case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32,
				reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32,
				reflect.Uint64, reflect.Uintptr, reflect.String:
				rv.Field(i).Set(reflect.ValueOf(
					defaultValueFromRc(v, rv.Field(i).Kind(), fieldName, iniSection)))
			case reflect.Slice | reflect.Array:
				values := defaultValueFromRc(v, rv.Field(i).Kind(), fieldName, iniSection).([]string)

				rv.Field(i).Set(reflect.MakeSlice(
					rv.Field(i).Type(),
					len(values),
					cap(values)))
				for j := 0; j < len(values); j++ {
					rv.Field(i).Index(j).Set(reflect.ValueOf(values[j]))
				}

			default:
				contract.Failf("unexpected type %v", rv.Field(i).Kind())
			}
		}
		return rv.Interface()
	default:
		contract.Failf("unexpected type %v", ref.Kind())
	}
	return nil
}

func AddBoolConfig(v *viper.Viper, cmd *cobra.Command, name, shortname string, defaultValue bool, description string) {
	cmd.PersistentFlags().BoolP(name, shortname, defaultValue, description)
	_ = v.BindPFlag(name, cmd.PersistentFlags().Lookup(name))
}

func AddStringConfig(v *viper.Viper, cmd *cobra.Command, name, shortname, defaultValue, description string) {
	cmd.PersistentFlags().StringP(name, shortname, defaultValue, description)
	_ = v.BindPFlag(name, cmd.PersistentFlags().Lookup(name))
}

func AddStringSliceConfig(
	v *viper.Viper, cmd *cobra.Command, name, shortname string, defaultValue []string, description string,
) {
	cmd.PersistentFlags().StringSliceP(name, shortname, defaultValue, description)
	_ = v.BindPFlag(name, cmd.PersistentFlags().Lookup(name))
}

func AddIntConfig(v *viper.Viper, cmd *cobra.Command, name, shortname string, defaultValue int, description string) {
	cmd.PersistentFlags().IntP(name, shortname, defaultValue, description)
	_ = v.BindPFlag(name, cmd.PersistentFlags().Lookup(name))
}

func AddJSONConfig(v *viper.Viper, cmd *cobra.Command) {
	AddBoolConfig(v, cmd, "json", "j", false, "Emit output as JSON")
}
