package docs

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

func genCreationExampleSyntaxTypescript(r *schema.Resource) string {
	indentSize := 0
	buffer := bytes.Buffer{}
	write := func(format string, args ...interface{}) {
		buffer.WriteString(fmt.Sprintf(format, args...))
	}

	indent := func() {
		buffer.WriteString(strings.Repeat(" ", indentSize))
	}

	indended := func(f func()) {
		indentSize += 2
		f()
		indentSize -= 2
	}

	seenTypes := codegen.NewStringSet()
	var writeValue func(valueType schema.Type)
	writeValue = func(valueType schema.Type) {
		switch valueType {
		case schema.BoolType:
			write("true|false")
		case schema.IntType:
			write("0")
		case schema.NumberType:
			write("0.0")
		case schema.StringType:
			write("\"string\"")
		case schema.ArchiveType:
			write("new pulumi.asset.FileAsset(\"./file.txt\")")
		case schema.AssetType:
			write("new pulumi.asset.StringAsset(\"Hello, world!\")")
		}

		switch valueType := valueType.(type) {
		case *schema.ArrayType:
			write("[")
			writeValue(valueType.ElementType)
			write("]")
		case *schema.MapType:
			write("{\n")
			indended(func() {
				indent()
				write("\"string\": ")
				writeValue(valueType.ElementType)
				write("\n")
			})
			indent()
			write("}")
		case *schema.ObjectType:
			if seenTypes.Has(valueType.Token) && objectTypeHasRecursiveReference(valueType) {
				write("type(%s)", valueType.Token)
				return
			}

			seenTypes.Add(valueType.Token)
			write("{\n")
			indended(func() {
				for _, p := range valueType.Properties {
					indent()
					write("%s: ", p.Name)
					writeValue(p.Type)
					write(",\n")
				}
			})
			indent()
			write("}")
		case *schema.ResourceType:
			write("reference(%s)", valueType.Token)
		case *schema.EnumType:
			cases := make([]string, len(valueType.Elements))
			for index, c := range valueType.Elements {
				if stringCase, ok := c.Value.(string); ok && stringCase != "" {
					cases[index] = stringCase
				} else if intCase, ok := c.Value.(int); ok {
					cases[index] = strconv.Itoa(intCase)
				} else {
					if c.Name != "" {
						cases[index] = c.Name
					}
				}
			}

			write(fmt.Sprintf("%q", strings.Join(cases, "|")))
		case *schema.UnionType:
			if isUnionOfObjects(valueType) {
				possibleTypes := make([]string, len(valueType.ElementTypes))
				for index, elem := range valueType.ElementTypes {
					objectType := elem.(*schema.ObjectType)
					_, _, typeName := decomposeToken(objectType.Token)
					possibleTypes[index] = typeName
				}
				write("oneOf(" + strings.Join(possibleTypes, "|") + ")")
			}

			for _, elem := range valueType.ElementTypes {
				if isPrimitiveType(elem) {
					writeValue(elem)
					return
				}
			}
		case *schema.InputType:
			writeValue(valueType.ElementType)
		case *schema.OptionalType:
			writeValue(valueType.ElementType)
		case *schema.TokenType:
			writeValue(valueType.UnderlyingType)
		}
	}

	pkg, mod, name := decomposeToken(r.Token)
	if strings.Contains(mod, tokens.QNameDelimiter) && len(strings.Split(mod, tokens.QNameDelimiter)) == 2 {
		parts := strings.Split(mod, tokens.QNameDelimiter)
		if strings.EqualFold(parts[1], name) {
			mod = parts[0]
		}
	}

	mod = strings.Join(strings.Split(mod, tokens.QNameDelimiter), ".")
	resourceType := fmt.Sprintf("%s.%s.%s", pkg, mod, name)
	if mod == "" || mod == "index" {
		resourceType = fmt.Sprintf("%s.%s", pkg, name)
	}

	write("import * as pulumi from \"@pulumi/pulumi\";\n")
	write("import * as %s from \"@pulumi/%s\";\n", pkg, pkg)
	write("\n")
	write("const %s = new %s(\"%s\", {\n", camelCase(resourceName(r)), resourceType, camelCase(resourceName(r)))
	indended(func() {
		for _, p := range r.InputProperties {
			indent()
			write("%s: ", p.Name)
			writeValue(codegen.ResolvedType(p.Type))
			write(",\n")
		}
	})

	write("});\n")
	return buffer.String()
}
