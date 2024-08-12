package main

import (
	"fmt"
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/spf13/viper"
)

func boolFromRc(v *viper.Viper, name, iniSection string) bool {
	if v.IsSet(name) {
		return v.GetBool(name)
	}
	sectionName := fmt.Sprintf("%s.%s", iniSection, name)
	if v.IsSet(sectionName) {
		return v.GetBool(sectionName)
	}
	defaultName := fmt.Sprintf("default.%s", name)
	if v.IsSet(defaultName) {
		return v.GetBool(defaultName)
	}
	val := v.GetBool(name)
	return val
}

func stringFromRc(v *viper.Viper, name, iniSection string) string {
	if v.IsSet(name) {
		return v.GetString(name)
	}
	sectionName := fmt.Sprintf("%s.%s", iniSection, name)
	if v.IsSet(sectionName) {
		return v.GetString(sectionName)
	}
	defaultName := fmt.Sprintf("default.%s", name)
	if v.IsSet(defaultName) {
		return v.GetString(defaultName)
	}
	val := v.GetString(name)
	return val
}

func intFromRc(v *viper.Viper, name, iniSection string) int {
	if v.IsSet(name) {
		return v.GetInt(name)
	}
	sectionName := fmt.Sprintf("%s.%s", iniSection, name)
	if v.IsSet(sectionName) {
		return v.GetInt(sectionName)
	}
	defaultName := fmt.Sprintf("default.%s", name)
	if v.IsSet(defaultName) {
		return v.GetInt(defaultName)
	}
	val := v.GetInt(name)
	return val
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
			fieldName := ref.Type().Field(i).Name
			switch rv.Field(i).Kind() {
			case reflect.String:
				rv.Field(i).Set(reflect.ValueOf(stringFromRc(v, fieldName, iniSection)))
			case reflect.Bool:
				rv.Field(i).Set(reflect.ValueOf(boolFromRc(v, fieldName, iniSection)))
			case reflect.Int:
				rv.Field(i).Set(reflect.ValueOf(intFromRc(v, fieldName, iniSection)))
			}
		}
		return rv.Interface()
	default:
		contract.Failf("unexpected type")
	}
	return nil
}
