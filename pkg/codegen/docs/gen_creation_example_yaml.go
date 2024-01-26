package docs

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

// collapseToken converts an exact token to a token more suitable for
// display. For example, it converts
//
//	  fizz:index/buzz:Buzz => fizz:Buzz
//	  fizz:mode/buzz:Buzz  => fizz:mode:Buzz
//		 foo:index:Bar	      => foo:Bar
//		 foo::Bar             => foo:Bar
//		 fizz:mod:buzz        => fizz:mod:buzz
func collapseResourceTokenForYAML(token string) string {
	tokenParts := strings.Split(token, ":")

	if len(tokenParts) == 3 {
		title := func(s string) string {
			r := []rune(s)
			if len(r) == 0 {
				return ""
			}
			return strings.ToTitle(string(r[0])) + string(r[1:])
		}
		if mod := strings.Split(tokenParts[1], "/"); len(mod) == 2 && title(mod[1]) == tokenParts[2] {
			// aws:s3/bucket:Bucket => aws:s3:Bucket
			// We recourse to handle the case foo:index/bar:Bar => foo:index:Bar
			tokenParts = []string{tokenParts[0], mod[0], tokenParts[2]}
		}

		if tokenParts[1] == "index" || tokenParts[1] == "" {
			// foo:index:Bar => foo:Bar
			// or
			// foo::Bar => foo:Bar
			tokenParts = []string{tokenParts[0], tokenParts[2]}
		}
	}

	return strings.Join(tokenParts, ":")
}

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
			if seenTypes.Has(valueType.Token) && objectTypeHasRecursiveReference(valueType) {
				write("type(%s)", valueType.Token)
				return
			}

			seenTypes.Add(valueType.Token)
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
					cases[index] = strconv.Itoa(intCase)
				} else {
					if c.Name != "" {
						cases[index] = c.Name
					}
				}
			}

			write(strings.Join(cases, "|"))
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

	write("name: example\nruntime: yaml\nresources:\n")
	indended(func() {
		indent()
		write("%s:\n", camelCase(resourceName(r)))
		indended(func() {
			indent()
			write("type: %s\n", collapseResourceTokenForYAML(r.Token))
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
