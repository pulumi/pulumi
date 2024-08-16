package main

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	ScopeSeparator = ":"
	FlagSeparator  = "."
)

func lookupArg(v *viper.Viper, scopes []string, name string) any {
	for _, scope := range scopes {
		key := scope + FlagSeparator + name
		if v.IsSet(key) {
			return v.Get(key)
		}
	}

	return nil
}

func defaultLookupArg(v *viper.Viper, scopes []string, kind reflect.Kind, name string) any {
	val := lookupArg(v, scopes, name)
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
		v, ok := val.(int64)
		if ok {
			return int(v)
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

func getScopes(cmd *cobra.Command) []string {
	hierarchy := []string{}
	for c := cmd; c != nil; c = c.Parent() {
		hierarchy = append(hierarchy, c.Name())
	}

	scopes := make([]string, len(hierarchy))
	prefix := ""
	for i := len(hierarchy) - 1; i >= 0; i-- {
		scopes[i] = prefix + hierarchy[i]
		if i < len(hierarchy)-1 {
			prefix += hierarchy[i] + ScopeSeparator
		}
	}

	return scopes
}

func LookupArg(v *viper.Viper, cmd *cobra.Command, name string) any {
	scopes := getScopes(cmd)
	return lookupArg(v, scopes, name)
}

// UnmarshalArgs unmarshals the options from the given viper instance into a struct of type `T`.
// A fieldname using camelcase will be read from the viper instance using a dashed fieldname.
// To override the fieldname, use the `args` tag.
// To set a default value, use the `argsDefault` tag.
// To set a usage description, use the `argsUsage` tag.
// To make a flag non-persistent (child commands don't inherit the flag), use the `argsNoPersist:"true"` tag.
func UnmarshalArgs[T any](v *viper.Viper, cmd *cobra.Command) T {
	scopes := getScopes(cmd)

	typ := reflect.TypeFor[T]()
	val := reflect.New(typ).Elem().Interface()

	return unmarshalArgs(v, scopes, val).(T)
}

func unmarshalArgs(v *viper.Viper, scopes []string, opts any) any {
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
				rv.Field(i).Set(reflect.ValueOf(unmarshalArgs(v, scopes, rv.Field(i).Interface())))
			case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32,
				reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32,
				reflect.Uint64, reflect.Uintptr, reflect.String:
				typ := rv.Field(i).Type()
				val := defaultLookupArg(v, scopes, rv.Field(i).Kind(), fieldName)
				if !reflect.ValueOf(val).CanConvert(typ) {
					contract.Failf("can't convert %v (%v) to %v", reflect.ValueOf(val).Kind(), val, typ)
				}
				refVal := reflect.ValueOf(val).Convert(typ)
				rv.Field(i).Set(refVal)
			case reflect.Slice | reflect.Array:
				values := defaultLookupArg(v, scopes, rv.Field(i).Kind(), fieldName).([]string)

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
	scope := getScopes(cmd)[0]

	ref := reflect.ValueOf(opts)
	//nolint:exhaustive
	switch ref.Kind() {
	case reflect.Struct:
		rv := reflect.New(ref.Type()).Elem()
		for i := 0; i < ref.NumField(); i++ {
			longName := dashedFieldName(ref.Type().Field(i).Name)
			tag := rv.Type().Field(i).Tag.Get("args")
			if tag != "" {
				longName = tag
			}
			shortName := rv.Type().Field(i).Tag.Get("argsShort")
			usage := rv.Type().Field(i).Tag.Get("argsUsage")
			defaultValue, defaultSet := rv.Type().Field(i).Tag.Lookup("argsDefault")
			noPersist := rv.Type().Field(i).Tag.Get("argsNoPersist") == "true"

			flagSet := cmd.PersistentFlags()
			if noPersist {
				flagSet = cmd.Flags()
			}

			if flagSet == nil {
				contract.Failf("no flags found for command %s", cmd.Name())
			}

			storeKey := scope + FlagSeparator + longName

			//nolint:exhaustive
			switch rv.Field(i).Kind() {
			case reflect.Struct:
				bindFlags(v, cmd, rv.Field(i).Interface())
			case reflect.Bool:
				d := defaultBool(defaultValue)
				flagSet.BoolP(longName, shortName, d, usage)
				if defaultSet {
					v.SetDefault(storeKey, d)
				}
				_ = v.BindPFlag(storeKey, flagSet.Lookup(longName))
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32,
				reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32,
				reflect.Uint64, reflect.Uintptr:
				d := defaultInt(defaultValue)
				flagSet.IntP(longName, shortName, d, usage)
				if defaultSet {
					v.SetDefault(storeKey, d)
				}
				_ = v.BindPFlag(storeKey, flagSet.Lookup(longName))
			case reflect.String:
				if rv.Type().Field(i).Tag.Get("argsType") == "var" {
					if defaultSet {
						contract.Failf("can't set default value with argsType:\"var\"")
					}
					value := reflect.New(rv.Field(i).Type()).Interface().(pflag.Value)
					flagSet.VarP(value, longName, shortName, usage)
				} else {
					flagSet.StringP(longName, shortName, defaultValue, usage)
				}
				if defaultSet {
					v.SetDefault(storeKey, defaultValue)
				}
				_ = v.BindPFlag(storeKey, flagSet.Lookup(longName))
			case reflect.Array, reflect.Slice:
				def := strings.Split(defaultValue, ",")
				if rv.Type().Field(i).Tag.Get("argsCommaSplit") == "false" {
					flagSet.StringArrayP(longName, shortName, def, usage)
				} else {
					flagSet.StringSliceP(longName, shortName, def, usage)
				}
				if defaultSet {
					v.SetDefault(storeKey, def)
				}
				_ = v.BindPFlag(storeKey, flagSet.Lookup(longName))
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
		}
		contract.Failf("unexpected default value %q for bool", defaultValue)
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

// TODO hack/pulumirc: These methods should probably go away
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
