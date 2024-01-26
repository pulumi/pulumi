package docs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi/pkg/v3/codegen/dotnet"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

func csharpNamespaceName(namespaces map[string]string, name string) string {
	if ns, ok := namespaces[name]; ok {
		return ns
	}

	// name could be a qualified module name so first split on /
	parts := strings.Split(name, tokens.QNameDelimiter)
	for i, part := range parts {
		names := strings.Split(part, "-")
		for j, name := range names {
			names[j] = title(name, "csharp")
		}
		parts[i] = strings.Join(names, "")
	}
	return strings.Join(parts, ".")
}

func csharpPropertyNameOverrides(properties []*schema.Property) map[string]string {
	overrides := make(map[string]string)
	for _, property := range properties {
		foundOverride := false
		if csharp, ok := property.Language["csharp"]; ok {
			if options, ok := csharp.(dotnet.CSharpPropertyInfo); ok {
				overrides[property.Name] = options.Name
				foundOverride = true
			}
		}

		if !foundOverride {
			overrides[property.Name] = property.Name
		}
	}

	return overrides
}

func resolveCSharpPropertyName(property string, overrides map[string]string) string {
	foundOverride, ok := overrides[property]
	if ok {
		return title(foundOverride, "csharp")
	}

	return title(property, "csharp")
}

func genCreationExampleSyntaxCSharp(r *schema.Resource) string {
	pkgDef, err := r.PackageReference.Definition()
	contract.Assertf(err == nil, "expected no error from getting package definition: %v", err)
	namespaces := make(map[string]map[string]string)
	compatibilities := make(map[string]string)
	csharpInfo, hasInfo := pkgDef.Language["csharp"].(dotnet.CSharpPackageInfo)
	if !hasInfo {
		if csharpInfoRaw, ok := pkgDef.Language["csharp"].(json.RawMessage); ok {
			err = json.Unmarshal(csharpInfoRaw, &csharpInfo)
			if err != nil {
				panic(err)
			}
		} else {
			csharpInfo = dotnet.CSharpPackageInfo{}
		}
	}

	namespaces[pkgDef.Name] = csharpInfo.Namespaces
	compatibilities[pkgDef.Name] = csharpInfo.Compatibility
	argumentTypeName := func(objectType *schema.ObjectType) string {
		token := objectType.Token
		qualifier := "Inputs"
		suffix := "Args"
		pkg, _, member := decomposeToken(token)
		module := pkgDef.TokenToModule(token)
		if strings.Contains(module, tokens.QNameDelimiter) && len(strings.Split(module, tokens.QNameDelimiter)) == 2 {
			parts := strings.Split(module, tokens.QNameDelimiter)
			if strings.ToLower(parts[1]) == strings.ToLower(member) {
				module = parts[0]
			}
		}
		resolvedNamespaces := namespaces[pkg]
		rootNamespace := csharpNamespaceName(resolvedNamespaces, pkg)
		namespace := csharpNamespaceName(resolvedNamespaces, module)
		if strings.ToLower(namespace) == "index" {
			namespace = ""
		}
		if namespace != "" {
			namespace = "." + namespace
		}
		if compatibilities[pkg] == "kubernetes20" {
			namespace = ".Types.Inputs" + namespace
		} else if qualifier != "" {
			namespace = namespace + "." + qualifier
		}
		member = member + suffix
		return fmt.Sprintf("%s%s.%s", rootNamespace, namespace, title(member, "csharp"))
	}

	resourceTypeName := func(resourceToken string) string {
		// Compute the resource type from the Pulumi type token.
		pkg, module, member := decomposeToken(resourceToken)
		if strings.Contains(module, tokens.QNameDelimiter) && len(strings.Split(module, tokens.QNameDelimiter)) == 2 {
			parts := strings.Split(module, tokens.QNameDelimiter)
			if strings.ToLower(parts[1]) == strings.ToLower(member) {
				module = parts[0]
			}
		}
		if csharpLanguageInfo, ok := pkgDef.Language["csharp"]; ok {
			if resourceInfo, ok := csharpLanguageInfo.(dotnet.CSharpResourceInfo); ok {
				member = resourceInfo.Name
			}
		}

		namespaces := namespaces[pkg]
		rootNamespace := csharpNamespaceName(namespaces, pkg)

		namespace := csharpNamespaceName(namespaces, module)
		if strings.ToLower(namespace) == "index" {
			namespace = ""
		}
		namespaceTokens := strings.Split(namespace, "/")
		for i, name := range namespaceTokens {
			namespaceTokens[i] = title(name, "csharp")
		}
		namespace = strings.Join(namespaceTokens, ".")

		if namespace != "" {
			namespace = "." + namespace
		}

		qualifiedMemberName := fmt.Sprintf("%s%s.%s", rootNamespace, namespace, title(member, "csharp"))
		return qualifiedMemberName
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
			write("new []\n")
			indent()
			write("{\n")
			indended(func() {
				indent()
				writeValue(valueType.ElementType)
				write("\n")
			})
			indent()
			write("}")
		case *schema.MapType:
			write("{\n")
			indended(func() {
				indent()
				write("[\"string\"] = ")
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
			write("new %s\n", typeName)
			indent()
			write("{\n")
			overrides := csharpPropertyNameOverrides(valueType.Properties)
			indended(func() {
				for _, p := range valueType.Properties {
					indent()
					write("%s = ", resolveCSharpPropertyName(p.Name, overrides))
					writeValue(p.Type)
					write(",\n")
				}
			})
			indent()
			write("}")
		case *schema.ResourceType:
			write("reference(%s)", valueType.Token)
		case *schema.EnumType:
			cases := make([]string, 0)
			for _, c := range valueType.Elements {
				if stringCase, ok := c.Value.(string); ok && stringCase != "" {
					cases = append(cases, stringCase)
				} else if intCase, ok := c.Value.(int); ok {
					cases = append(cases, strconv.Itoa(intCase))
				} else {
					if c.Name != "" {
						cases = append(cases, c.Name)
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

	resourceName := resourceTypeName(r.Token)
	pkg, _, name := decomposeToken(r.Token)
	pkg = title(pkg, "csharp")

	write("using Pulumi;\n")
	write("using %s = Pulumi.%s;\n", pkg, pkg)

	write("\n")
	write("var %s = new %s(\"%s\", new () \n{\n", camelCase(name), resourceName, camelCase(name))
	inputPropertyOverrides := csharpPropertyNameOverrides(r.InputProperties)
	indended(func() {
		for _, p := range r.InputProperties {
			indent()
			write("%s = ", resolveCSharpPropertyName(p.Name, inputPropertyOverrides))
			writeValue(codegen.ResolvedType(p.Type))
			write(",\n")
		}
	})

	write("});\n")
	return buffer.String()
}
