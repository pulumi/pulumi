package docs

import (
	"bytes"
	"fmt"
	"github.com/pulumi/pulumi/pkg/v3/codegen/dotnet"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

func namespaceName(namespaces map[string]string, name string) string {
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

func propertyNameOverrides(properties []*schema.Property) map[string]string {
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

func resolvePropertyName(property string, overrides map[string]string) string {
	foundOverride, ok := overrides[property]
	if ok {
		return title(foundOverride, "csharp")
	}

	return title(property, "csharp")
}

func genCreationExampleSyntaxCSharp(r *schema.Resource) string {
	pkgDef, _ := r.PackageReference.Definition()
	csharpInfo, hasInfo := pkgDef.Language["csharp"].(dotnet.CSharpPackageInfo)
	if !hasInfo {
		csharpInfo = dotnet.CSharpPackageInfo{}
	}
	namespaces := make(map[string]map[string]string)
	compatibilities := make(map[string]string)
	packageNamespaces := csharpInfo.Namespaces
	namespaces[pkgDef.Name] = packageNamespaces
	compatibilities[pkgDef.Name] = csharpInfo.Compatibility
	argumentTypeName := func(objectType *schema.ObjectType) string {
		token := objectType.Token
		qualifier := "Inputs"
		suffix := "Args"
		pkg, _, member := decomposeToken(token)
		module := pkgDef.TokenToModule(token)
		resolvedNamespaces := namespaces[pkg]
		rootNamespace := namespaceName(resolvedNamespaces, pkg)
		namespace := namespaceName(resolvedNamespaces, module)
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

		if csharpLanguageInfo, ok := pkgDef.Language["csharp"]; ok {
			if resourceInfo, ok := csharpLanguageInfo.(dotnet.CSharpResourceInfo); ok {
				member = resourceInfo.Name
			}
		}

		namespaces := namespaces[pkg]
		rootNamespace := namespaceName(namespaces, pkg)

		namespace := namespaceName(namespaces, module)
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
			typeName := argumentTypeName(valueType)
			write("new %s\n", typeName)
			indent()
			write("{\n")
			overrides := propertyNameOverrides(valueType.Properties)
			indended(func() {
				for _, p := range valueType.Properties {
					indent()
					write("%s = ", resolvePropertyName(p.Name, overrides))
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
					cases[index] = fmt.Sprintf("%q", stringCase)
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

	resourceName := resourceTypeName(r.Token)
	pkg, mod, name := decomposeToken(r.Token)
	mod = title(strings.ReplaceAll(mod, "/", "."), "csharp")
	pkg = title(pkg, "csharp")

	write("using Pulumi;\n")
	write("using %s = Pulumi.%s;\n", pkg, pkg)

	write("\n")
	write("var %s = new %s(\"%s\", new () \n{\n", camelCase(name), resourceName, camelCase(name))
	inputPropertyOverrides := propertyNameOverrides(r.InputProperties)
	indended(func() {
		for _, p := range r.InputProperties {
			indent()
			write("%s = ", resolvePropertyName(p.Name, inputPropertyOverrides))
			writeValue(codegen.ResolvedType(p.Type))
			write(",\n")
		}
	})

	write("});\n")
	return buffer.String()
}
