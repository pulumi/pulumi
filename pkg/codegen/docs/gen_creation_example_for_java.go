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

func javaNamespaceName(namespaces map[string]string, name string) string {
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

func genCreationExampleSyntaxJava(r *schema.Resource) string {
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

	pkg, mod, name := decomposeToken(r.Token)
	mod = title(strings.ReplaceAll(mod, "/", "."), "java")
	pkg = title(pkg, "java")

	write("import com.pulumi.Pulumi;;\n")
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
