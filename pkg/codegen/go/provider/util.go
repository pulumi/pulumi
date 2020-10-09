package provider

import (
	"encoding/json"
	"go/ast"
	"io"
	"io/ioutil"
	"reflect"
	"strings"
	"unicode"

	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
)

func pascalCase(s string) string {
	if len(s) == 0 {
		return s
	}

	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func camelCase(s string) string {
	if len(s) == 0 {
		return s
	}

	runes := []rune(s)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

func getPulumiPropertyDesc(field *ast.Field) ([]string, bool) {
	if field.Tag == nil {
		return nil, false
	}

	tag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))
	value, ok := tag.Lookup("pulumi")
	if !ok {
		return nil, false
	}
	return strings.Split(value, ","), true
}

func writeTempFile(pattern string, write func(w io.Writer) error) (string, error) {
	f, err := ioutil.TempFile("", "output_types_*.go")
	if err != nil {
		return "", err
	}
	defer f.Close()

	return f.Name(), write(f)
}

func rawMessage(v interface{}) json.RawMessage {
	bytes, err := json.Marshal(v)
	contract.Assert(err == nil)
	return json.RawMessage(bytes)
}
