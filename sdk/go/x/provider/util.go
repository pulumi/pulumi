package provider

import (
	"fmt"
	"reflect"
	"strings"
	"unicode"
)

func listElementPath(listPath string, index int) string {
	return fmt.Sprintf("%v[%v]", listPath, index)
}

func objectPropertyPath(objectPath, key string) string {
	if strings.ContainsAny(key, `."[]`) {
		return fmt.Sprintf(`%v["%s"]`, objectPath, strings.ReplaceAll(key, `"`, `\"`))
	}
	if objectPath == "" {
		return key
	}
	return fmt.Sprintf("%v.%v", objectPath, key)
}

type fieldDesc struct {
	name      string
	required  bool
	immutable bool
	//minItems   int
	//maxItems   int
	deprecated bool
	secret     bool
}

func camelCase(fieldName string) string {
	runes := []rune(fieldName)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

func getFieldDesc(field reflect.StructField) (*fieldDesc, error) {
	if field.PkgPath != "" {
		return nil, nil
	}

	value, ok := field.Tag.Lookup("pulumi")
	if !ok {
		return nil, nil
	}

	opts := strings.Split(value, ",")

	desc := &fieldDesc{name: opts[0]}
	if desc.name == "" {
		desc.name = camelCase(field.Name)
	}

	if field.Type.Kind() != reflect.Ptr {
		desc.required = true
	}

	for _, opt := range opts[1:] {
		switch opt {
		case "required":
			desc.required = true
		case "optional":
			desc.required = false
		case "immutable":
			desc.immutable = true
		case "deprecated":
			desc.deprecated = true
		case "secret":
			desc.secret = true
		default:
			return nil, fmt.Errorf("unknown option '%v' in tag for struct field %v", opt, field.Name)
		}
	}
	return desc, nil
}
