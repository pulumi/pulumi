package main

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/spf13/viper"
)

func valueFromRc(v *viper.Viper, name, iniSection string) any {
	if v.IsSet(name) {
		return v.Get(name)
	}
	sectionName := fmt.Sprintf("%s.%s", iniSection, name)
	if v.IsSet(sectionName) {
		return v.Get(sectionName)
	}
	defaultName := fmt.Sprintf("default.%s", name)
	if v.IsSet(defaultName) {
		return v.Get(defaultName)
	}
	return v.Get(name)
}

func defaultValueFromRc(v *viper.Viper, kind reflect.Kind, name, iniSection string) any {
	val := valueFromRc(v, name, iniSection)
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

func UnmashalOpts(v *viper.Viper, opts any, iniSection string) any {
	ref := reflect.ValueOf(opts)
	switch ref.Kind() {
	case reflect.Struct:
		rv := reflect.New(ref.Type()).Elem()
		for i := 0; i < ref.NumField(); i++ {
			if !rv.Field(i).CanSet() {
				panic(fmt.Sprintf("can't set field %s", ref.Type().Field(i).Name))
			}
			fieldName := dashedFieldName(ref.Type().Field(i).Name)
			switch rv.Field(i).Kind() {
			case reflect.Struct:
				rv.Field(i).Set(reflect.ValueOf(UnmashalOpts(v, rv.Field(i).Interface(), iniSection)))
			case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32,
				reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32,
				reflect.Uint64, reflect.Uintptr, reflect.String:
				rv.Field(i).Set(reflect.ValueOf(
					defaultValueFromRc(v, rv.Field(i).Kind(), fieldName, iniSection)))
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
