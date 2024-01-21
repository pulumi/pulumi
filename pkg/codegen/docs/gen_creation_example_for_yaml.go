package docs

import (
	"bytes"
	"fmt"
	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"strings"
)

func genCreationExampleSyntaxYAML(r *schema.Resource) string {
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

	var writeValue func(valueType schema.Type)
	writeValue = func(valueType schema.Type) {
		switch valueType {
		case schema.BoolType:
			write("True|False")
		case schema.IntType:
			write("0")
		case schema.NumberType:
			write("0.0")
		case schema.StringType:
			write("\"string\"")
		case schema.ArchiveType:
			write("\n")
			indended(func() {
				indent()
				write("Fn::FileAsset: ./file.txt")
			})
		case schema.AssetType:
			write("\n")
			indended(func() {
				indent()
				write("Fn::StringAsset: \"example content\"")
			})
		}

		switch valueType := valueType.(type) {
		case *schema.ArrayType:
			if isPrimitiveType(valueType.ElementType) {
				write("[")
				writeValue(valueType.ElementType)
				write("]")
			} else {
				write("[")
				writeValue(valueType.ElementType)
				write("\n")
				indent()
				write("]")
			}
		case *schema.MapType:
			write("\n")
			indended(func() {
				indent()
				write("\"string\": ")
				writeValue(valueType.ElementType)
				if !isPrimitiveType(valueType.ElementType) {
					write("\n")
				}
			})
		case *schema.ObjectType:
			write("\n")
			indended(func() {
				for index, p := range valueType.Properties {
					indent()
					write("%s: ", p.Name)
					writeValue(p.Type)
					if index != len(valueType.Properties)-1 {
						write("\n")
					}
				}
			})
		case *schema.ResourceType:
			write("reference(%s)", valueType.Token)
		case *schema.EnumType:
			cases := make([]string, len(valueType.Elements))
			for index, c := range valueType.Elements {
				if stringCase, ok := c.Value.(string); ok && stringCase != "" {
					cases[index] = stringCase
				} else if intCase, ok := c.Value.(int); ok {
					cases[index] = fmt.Sprintf("%d", intCase)
				} else {
					if c.Name != "" {
						cases[index] = c.Name
					}
				}
			}

			write(strings.Join(cases, "|"))
		case *schema.UnionType:
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
		}
	}

	write("name: example\nruntime: yaml\nresources:\n")
	indended(func() {
		indent()
		write("%s:\n", camelCase(resourceName(r)))
		indended(func() {
			indent()
			write("type: %s\n", r.Token)
			indent()
			write("properties:\n")
			indended(func() {
				for _, p := range r.InputProperties {
					indent()
					write("%s: ", p.Name)
					writeValue(codegen.ResolvedType(p.Type))
					write("\n")
				}
			})
		})
	})
	return buffer.String()
}
