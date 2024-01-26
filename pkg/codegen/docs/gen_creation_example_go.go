package docs

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/cgstrings"
	go_gen "github.com/pulumi/pulumi/pkg/v3/codegen/go"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

func normalizeGoModuleName(name string) string {
	return strings.ReplaceAll(name, "/", ".")
}

func computeGoTypeName(schemaType schema.Type, input bool) (typeName string, qualified bool) {
	switch schemaType {
	case schema.BoolType:
		if input {
			return "pulumi.Bool", true
		}
		return "bool", false
	case schema.IntType:
		if input {
			return "pulumi.Int", true
		}
		return "int", false
	case schema.NumberType:
		if input {
			return "pulumi.Float64", true
		}
		return "float64", false
	case schema.StringType:
		if input {
			return "pulumi.String", true
		}
		return "string", false
	case schema.ArchiveType:
		return "pulumi.Archive", true
	case schema.AssetType:
		return "pulumi.Asset", true
	}

	switch schemaType := schemaType.(type) {
	case *schema.ObjectType:
		pkg, mod, name := decomposeToken(schemaType.Token)
		mod = normalizeGoModuleName(mod)
		if mod == "index" || mod == "" {
			return pkg + "." + name, true
		} else {
			return mod + "." + name, true
		}
	case *schema.ResourceType:
		pkg, mod, name := decomposeToken(schemaType.Token)
		mod = normalizeGoModuleName(mod)
		if mod == "index" || mod == "" {
			return pkg + "." + name, true
		} else {
			return mod + "." + name, true
		}
	case *schema.EnumType:
		pkg, mod, name := decomposeToken(schemaType.Token)
		mod = normalizeGoModuleName(mod)
		if mod == "index" || mod == "" {
			return pkg + "." + name, true
		} else {
			return mod + "." + name, true
		}
	case *schema.InputType:
		return computeGoTypeName(schemaType.ElementType, true)
	}

	return "", false
}

func goEnumTitle(s string) string {
	if s == "" {
		return ""
	}
	if s[0] == '$' {
		return title(s[1:], "go")
	}
	s = cgstrings.UppercaseFirst(s)
	return cgstrings.ModifyStringAroundDelimeter(s, "-", func(next string) string {
		return "_" + cgstrings.UppercaseFirst(next)
	})
}

func goMakeSafeEnumName(name, typeName string) string {
	safeEnum := codegen.ExpandShortEnumName(name)
	return typeName + goEnumTitle(safeEnum)
}

func genCreationExampleSyntaxGo(r *schema.Resource) string {
	argumentTypeName := func(objectType *schema.ObjectType) string {
		token := objectType.Token
		pkg, mod, member := decomposeToken(token)
		if strings.Contains(mod, tokens.QNameDelimiter) && len(strings.Split(mod, tokens.QNameDelimiter)) == 2 {
			parts := strings.Split(mod, tokens.QNameDelimiter)
			if strings.EqualFold(parts[1], member) {
				mod = parts[0]
			}
		}
		mod = normalizeGoModuleName(mod)
		if mod == "index" || mod == "" {
			return pkg + "." + member + "Args"
		}
		return mod + "." + member + "Args"
	}

	indentSize := 0
	buffer := bytes.Buffer{}
	write := func(format string, args ...interface{}) {
		buffer.WriteString(fmt.Sprintf(format, args...))
	}

	indent := func() {
		buffer.WriteString(strings.Repeat(" ", indentSize))
	}

	indented := func(f func()) {
		indentSize += 2
		f()
		indentSize -= 2
	}

	seenTypes := codegen.NewStringSet()
	var writeValue func(valueType schema.Type, plain bool, optional bool)
	writeValue = func(valueType schema.Type, plain bool, optional bool) {
		switch valueType {
		case schema.BoolType:
			if plain && optional {
				write("pulumi.BoolRef(true|false)")
			} else if plain && !optional {
				write("true|false")
			} else {
				write("pulumi.Bool(true|false)")
			}
		case schema.IntType:
			if plain && optional {
				write("pulumi.IntRef(0)")
			} else if plain && !optional {
				write("0")
			} else {
				write("pulumi.Int(0)")
			}
		case schema.NumberType:
			if plain && optional {
				write("pulumi.Float64Ref(0.0)")
			} else if plain && !optional {
				write("0.0")
			} else {
				write("pulumi.Float64(0.0)")
			}
		case schema.StringType:
			if plain && optional {
				write("pulumi.StringRef(\"string\")")
			} else if plain && !optional {
				write("\"string\"")
			} else {
				write("pulumi.String(\"string\")")
			}
		case schema.ArchiveType:
			write("pulumi.NewFileArchive(\"./file.txt\")")
		case schema.AssetType:
			write("pulumi.NewStringAsset(\"Hello, world!\")")
		}

		switch valueType := valueType.(type) {
		case *schema.ArrayType:
			elementTypeName, qualified := computeGoTypeName(valueType.ElementType, !plain)
			if qualified {
				elementTypeName = elementTypeName + "Array"
			} else {
				elementTypeName = "[]" + elementTypeName
			}

			write("%s{\n", elementTypeName)
			indented(func() {
				indent()
				writeValue(valueType.ElementType, false, false)
				write("\n")
			})

			indent()
			write("}")

		case *schema.MapType:
			elementTypeName, qualified := computeGoTypeName(valueType.ElementType, !plain)
			if qualified {
				elementTypeName = elementTypeName + "Map"
			} else {
				elementTypeName = "map[string]" + elementTypeName
			}

			write("%s{\n", elementTypeName)
			indented(func() {
				indent()
				write("\"string\": ")
				writeValue(valueType.ElementType, false, false)
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
			write("&%s{\n", typeName)
			indented(func() {
				for _, p := range valueType.Properties {
					indent()
					write("%s: ", title(p.Name, "go"))
					writeValue(p.Type, p.Plain, !p.IsRequired())
					write(",\n")
				}
			})

			indent()
			write("}")
		case *schema.ResourceType:
			write("reference(%s)", valueType.Token)
		case *schema.EnumType:
			_, _, enumTypeName := decomposeToken(valueType.Token)
			cases := make([]string, len(valueType.Elements))
			for index, c := range valueType.Elements {
				cases[index] = goMakeSafeEnumName(c.Name, enumTypeName)
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
				if isPrimitiveType(codegen.ResolvedType(elem)) {
					writeValue(elem, false, false)
					return
				}
			}
		case *schema.InputType:
			plain = false
			writeValue(valueType.ElementType, plain, optional)
		case *schema.OptionalType:
			writeValue(valueType.ElementType, plain, optional)
		case *schema.TokenType:
			writeValue(codegen.ResolvedType(valueType.UnderlyingType), plain, optional)
		}
	}

	pkg, mod, name := decomposeToken(r.Token)
	if strings.Contains(mod, tokens.QNameDelimiter) && len(strings.Split(mod, tokens.QNameDelimiter)) == 2 {
		parts := strings.Split(mod, tokens.QNameDelimiter)
		if strings.EqualFold(parts[1], name) {
			mod = parts[0]
		}
	}

	pkgDef, err := r.PackageReference.Definition()
	contract.Assertf(err == nil, "expected no error from getting package definition: %v", err)
	importPath := ""
	if goInfo, ok := pkgDef.Language["go"].(go_gen.GoPackageInfo); ok {
		importPath = goInfo.ImportBasePath
	}

	write("import (\n")
	indented(func() {
		indent()
		write("\"github.com/pulumi/pulumi/sdk/v3/go/pulumi\"\n")
		indent()
		if importPath != "" && strings.HasPrefix("github.com/pulumi/pulumi-", importPath) {
			write("\"%s/%s\"\n", importPath, mod)
		} else {
			if mod == "index" || mod == "" {
				write("\"github.com/pulumi/pulumi-%s/sdk/v3/go/%s\"\n", pkg, pkg)
			} else {
				write("\"github.com/pulumi/pulumi-%s/sdk/v3/go/%s/%s\"\n", pkg, pkg, mod)
			}
		}
	})

	write(")\n\n")
	if mod == "index" || mod == "" {
		mod = pkg
	}

	mod = normalizeGoModuleName(mod)
	write("%s, err := %s.New%s(\"%s\", &%s.%sArgs{\n", camelCase(name), mod, name, camelCase(name), mod, name)
	indented(func() {
		for _, p := range r.InputProperties {
			indent()
			write("%s: ", title(p.Name, "go"))
			writeValue(p.Type, p.Plain, !p.IsRequired())
			write(",\n")
		}
		write("})\n")
	})

	return buffer.String()
}
