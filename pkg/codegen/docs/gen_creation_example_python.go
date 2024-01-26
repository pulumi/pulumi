package docs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi/pkg/v3/codegen/python"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

func pythonTokenToQualifiedName(pkg, module, member string) string {
	components := strings.Split(module, "/")
	for i, component := range components {
		components[i] = python.PyName(component)
	}
	module = strings.Join(components, ".")
	if module == "index" {
		module = ""
	}

	if module != "" {
		module = "." + module
	}

	return fmt.Sprintf("%s%s.%s", python.PyName(pkg), module, title(member, "python"))
}

func genCreationExampleSyntaxPython(r *schema.Resource) string {
	pkgDef, err := r.PackageReference.Definition()
	contract.Assertf(err == nil, "expected no error from getting package definition: %v", err)
	pythonInfo, hasInfo := pkgDef.Language["python"].(python.PackageInfo)
	if !hasInfo {
		pythonInfo = python.PackageInfo{}
	}
	compatibilities := make(map[string]string)
	compatibilities[pkgDef.Name] = pythonInfo.Compatibility
	argumentTypeName := func(objectType *schema.ObjectType) string {
		token := objectType.Token
		pkg, _, member := decomposeToken(token)
		module := pkgDef.TokenToModule(token)
		if strings.Contains(module, tokens.QNameDelimiter) && len(strings.Split(module, tokens.QNameDelimiter)) == 2 {
			parts := strings.Split(module, tokens.QNameDelimiter)
			if strings.EqualFold(parts[1], member) {
				module = parts[0]
			}
		}
		if lang, ok := pkgDef.Language["python"]; ok {
			if pkgInfo, ok := lang.(python.PackageInfo); ok {
				if m, ok := pkgInfo.ModuleNameOverrides[module]; ok {
					module = m
				}
			}

			if pkgInfo, ok := lang.(json.RawMessage); ok {
				var moduleNameOverrides map[string]string
				if err := json.Unmarshal(pkgInfo, &moduleNameOverrides); err == nil {
					if m, ok := moduleNameOverrides[module]; ok {
						module = m
					}
				}
			}
		}

		return pythonTokenToQualifiedName(pkg, module, member) + "Args"
	}

	resourceTypeName := func(resourceToken string) string {
		// Compute the resource type from the Pulumi type token.
		pkg, module, member := decomposeToken(resourceToken)
		if strings.Contains(module, tokens.QNameDelimiter) && len(strings.Split(module, tokens.QNameDelimiter)) == 2 {
			parts := strings.Split(module, tokens.QNameDelimiter)
			if strings.EqualFold(parts[1], member) {
				module = parts[0]
			}
		}
		if pythonLanguageInfo, ok := pkgDef.Language["python"]; ok {
			if pythonInfo, ok := pythonLanguageInfo.(python.PackageInfo); ok {
				if m, ok := pythonInfo.ModuleNameOverrides[module]; ok {
					module = m
				}
			}
		}

		return pythonTokenToQualifiedName(pkg, module, member)
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
			write("True|False")
		case schema.IntType:
			write("0")
		case schema.NumberType:
			write("0.0")
		case schema.StringType:
			write("\"string\"")
		case schema.ArchiveType:
			write("pulumi.FileAsset(\"./file.txt\")")
		case schema.AssetType:
			write("pulumi.StringAsset(\"Hello, world!\")")
		}

		switch valueType := valueType.(type) {
		case *schema.ArrayType:
			write("[\n")
			indended(func() {
				indent()
				writeValue(valueType.ElementType)
				write(",\n")
			})
			indent()
			write("]")
		case *schema.MapType:
			write("{\n")
			indended(func() {
				indent()
				write("'string': ")
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
			typeName := argumentTypeName(valueType)
			write("%s(\n", typeName)
			indended(func() {
				for _, p := range valueType.Properties {
					indent()
					write("%s=", python.PyName(p.Name))
					writeValue(p.Type)
					write(",\n")
				}
			})
			indent()
			write(")")
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

	pkg, _, name := decomposeToken(r.Token)
	pkg = python.PyName(pkg)

	write("import pulumi\n")
	write("import pulumi_%s as %s\n", pkg, pkg)
	write("\n")
	write("%s = %s(\"%s\",\n", camelCase(name), resourceTypeName(r.Token), camelCase(name))
	indended(func() {
		for index, p := range r.InputProperties {
			indent()
			write("%s=", python.PyName(p.Name))
			writeValue(codegen.ResolvedType(p.Type))
			if index < len(r.InputProperties)-1 {
				write(",")
			}
			write("\n")
		}
	})

	write(")\n")
	return buffer.String()
}
