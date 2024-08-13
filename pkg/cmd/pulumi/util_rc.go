package main

import (
	"fmt"
	"reflect"
	"strconv"
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

// UnmarshalArgs unmarshals the options from the given viper instance into a struct of type `T`.
func UnmarshalArgs[T any](v *viper.Viper, iniSection string) T {
	typ := reflect.TypeFor[T]()
	val := reflect.New(typ).Elem().Interface()
	return unmarshalArgs(v, val, iniSection).(T)
}

func unmarshalArgs(v *viper.Viper, opts any, iniSection string) any {
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
				rv.Field(i).Set(reflect.ValueOf(unmarshalArgs(v, rv.Field(i).Interface(), iniSection)))
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

// BindFlags binds all of the public fields of the struct type `T` as flags on the provided `cobra.Command`
// and hooks thme up to the provided `viper.Viper` instance.
// Use `UnmarshalArgs` to get the options back out from viper into a struct.
func BindFlags[T any](v *viper.Viper, cmd *cobra.Command) {
	typ := reflect.TypeFor[T]()
	if typ.Kind() != reflect.Struct {
		panic(fmt.Sprintf("BindFlags expects a struct type, got %v", typ.Kind()))
	}
	val := reflect.New(typ).Elem().Interface()
	bindFlags(v, cmd, val)
}

func bindFlags(v *viper.Viper, cmd *cobra.Command, opts any) {
	ref := reflect.ValueOf(opts)
	//nolint:exhaustive
	switch ref.Kind() {
	case reflect.Struct:
		rv := reflect.New(ref.Type()).Elem()
		for i := 0; i < ref.NumField(); i++ {
			fieldName := dashedFieldName(ref.Type().Field(i).Name)
			tag := rv.Type().Field(i).Tag.Get("args")
			if tag != "" {
				fieldName = tag
			}
			shortName := rv.Type().Field(i).Tag.Get("argsShort")
			usage := rv.Type().Field(i).Tag.Get("argsUsage")
			defaultValue := rv.Type().Field(i).Tag.Get("argsDefault")

			//nolint:exhaustive
			switch rv.Field(i).Kind() {
			case reflect.Struct:
				bindFlags(v, cmd, rv.Field(i).Interface())
			case reflect.Bool:
				d := defaultBool(defaultValue)
				cmd.PersistentFlags().BoolP(fieldName, shortName, d, usage)
				_ = v.BindPFlag(fieldName, cmd.PersistentFlags().Lookup(fieldName))
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32,
				reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32,
				reflect.Uint64, reflect.Uintptr:
				d := defaultInt(defaultValue)
				cmd.PersistentFlags().IntP(fieldName, shortName, d, usage)
				_ = v.BindPFlag(fieldName, cmd.PersistentFlags().Lookup(fieldName))
			case reflect.String:
				defaultString := ""
				if defaultValue != "" {
					defaultString = defaultValue
				}
				cmd.PersistentFlags().StringP(fieldName, shortName, defaultString, usage)
				_ = v.BindPFlag(fieldName, cmd.PersistentFlags().Lookup(fieldName))
			case reflect.Array, reflect.Slice:
				cmd.PersistentFlags().StringSliceP(fieldName, shortName, []string{}, usage)
				_ = v.BindPFlag(fieldName, cmd.PersistentFlags().Lookup(fieldName))
			default:
				contract.Failf("unexpected type %v", rv.Field(i).Kind())
			}
		}
	default:
		contract.Failf("unexpected type %v", ref.Kind())
	}
}

func defaultBool(defaultValue string) bool {
	if defaultValue != "" {
		if defaultValue == "true" {
			return true
		} else if defaultValue == "false" {
			return false
		} else {
			contract.Failf("unexpected default value %q for bool", defaultValue)
		}
	}
	return false
}

func defaultInt(defaultValue string) int {
	if defaultValue != "" {
		d, err := strconv.Atoi(defaultValue)
		if err != nil {
			contract.Failf("failed to parse default value %q as int: %v", defaultValue, err)
		}
		return d
	}
	return 0
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
