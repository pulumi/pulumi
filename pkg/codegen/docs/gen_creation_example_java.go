package docs

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

func genCreationExampleSyntaxJava(r *schema.Resource) string {
	argumentTypeName := func(objectType *schema.ObjectType) string {
		token := objectType.Token
		_, _, member := decomposeToken(token)
		return member + "Args"
	}

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
			write("new FileAsset(\"./file.txt\")")
		case schema.AssetType:
			write("new StringAsset(\"Hello, world!\")")
		}

		switch valueType := valueType.(type) {
		case *schema.ArrayType:
			if isPrimitiveType(valueType.ElementType) {
				write("List.of(")
				writeValue(valueType.ElementType)
				write(")")
			} else {
				write("List.of(\n")
				indended(func() {
					indent()
					writeValue(valueType.ElementType)
					write("\n")
				})
				indent()
				write(")")
			}

		case *schema.MapType:
			write("Map.ofEntries(\n")
			indended(func() {
				indent()
				write("Map.entry(\"string\", ")
				writeValue(valueType.ElementType)
				write(")\n")
			})
			indent()
			write(")")
		case *schema.ObjectType:
			if seenTypes.Has(valueType.Token) && objectTypeHasRecursiveReference(valueType) {
				write("type(%s)", valueType.Token)
				return
			}

			seenTypes.Add(valueType.Token)
			typeName := argumentTypeName(valueType)
			write("%s.builder()\n", typeName)
			indended(func() {
				for _, p := range valueType.Properties {
					indent()
					write(".%s(", p.Name)
					writeValue(p.Type)
					write(")\n")
				}

				indent()
				write(".build()")
			})
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

	_, _, name := decomposeToken(r.Token)
	write("import com.pulumi.Pulumi;\n")
	write("import java.util.List;\n")
	write("import java.util.Map;\n")
	write("\n")
	write("var %s = new %s(\"%s\", %sArgs.builder()\n", camelCase(name), name, camelCase(name), name)
	indended(func() {
		for _, p := range r.InputProperties {
			indent()
			write(".%s(", p.Name)
			writeValue(codegen.ResolvedType(p.Type))
			write(")\n")
		}
		indent()
		write(".build());\n")
	})

	return buffer.String()
}
