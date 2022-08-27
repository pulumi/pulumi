// Copyright 2016-2020, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dotnet

import (
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"
	"unicode"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model/format"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

// isReservedWord returns true if s is a C# reserved word as per
// https://docs.microsoft.com/en-us/dotnet/csharp/language-reference/language-specification/lexical-structure#keywords
func isReservedWord(s string) bool {
	switch s {
	case "abstract", "as", "base", "bool", "break", "byte", "case", "catch", "char", "checked", "class", "const",
		"continue", "decimal", "default", "delegate", "do", "double", "else", "enum", "event", "explicit", "extern",
		"false", "finally", "fixed", "float", "for", "foreach", "goto", "if", "implicit", "in", "int", "interface",
		"internal", "is", "lock", "long", "namespace", "new", "null", "object", "operator", "out", "override",
		"params", "private", "protected", "public", "readonly", "ref", "return", "sbyte", "sealed", "short",
		"sizeof", "stackalloc", "static", "string", "struct", "switch", "this", "throw", "true", "try", "typeof",
		"uint", "ulong", "unchecked", "unsafe", "ushort", "using", "virtual", "void", "volatile", "while":
		return true
	// Treat contextual keywords as keywords, as we don't validate the context around them.
	case "add", "alias", "ascending", "async", "await", "by", "descending", "dynamic", "equals", "from", "get",
		"global", "group", "into", "join", "let", "nameof", "on", "orderby", "partial", "remove", "select", "set",
		"unmanaged", "value", "var", "when", "where", "yield":
		return true
	default:
		return false
	}
}

// isLegalIdentifierStart returns true if it is legal for c to be the first character of a C# identifier as per
// https://docs.microsoft.com/en-us/dotnet/csharp/language-reference/language-specification/lexical-structure
func isLegalIdentifierStart(c rune) bool {
	return c == '_' || c == '@' ||
		unicode.In(c, unicode.Lu, unicode.Ll, unicode.Lt, unicode.Lm, unicode.Lo, unicode.Nl)
}

// isLegalIdentifierPart returns true if it is legal for c to be part of a C# identifier (besides the first character)
// as per https://docs.microsoft.com/en-us/dotnet/csharp/language-reference/language-specification/lexical-structure
func isLegalIdentifierPart(c rune) bool {
	return c == '_' ||
		unicode.In(c, unicode.Lu, unicode.Ll, unicode.Lt, unicode.Lm, unicode.Lo, unicode.Nl, unicode.Mn, unicode.Mc,
			unicode.Nd, unicode.Pc, unicode.Cf)
}

// makeValidIdentifier replaces characters that are not allowed in C# identifiers with underscores. A reserved word is
// prefixed with @. No attempt is made to ensure that the result is unique.
func makeValidIdentifier(name string) string {
	var builder strings.Builder
	for i, c := range name {
		if i == 0 && c == '@' {
			builder.WriteRune(c)
			continue
		}
		if !isLegalIdentifierPart(c) {
			builder.WriteRune('_')
		} else {
			if i == 0 && !isLegalIdentifierStart(c) {
				builder.WriteRune('_')
			}
			builder.WriteRune(c)
		}
	}
	name = builder.String()
	if isReservedWord(name) {
		return "@" + name
	}
	return name
}

// propertyName returns a name as a valid identifier in title case.
func propertyName(name string) string {
	return makeValidIdentifier(Title(name))
}

func makeSafeEnumName(name, typeName string) (string, error) {
	// Replace common single character enum names.
	safeName := codegen.ExpandShortEnumName(name)

	// If the name is one illegal character, return an error.
	if len(safeName) == 1 && !isLegalIdentifierStart(rune(safeName[0])) {
		return "", fmt.Errorf("enum name %s is not a valid identifier", safeName)
	}

	// Capitalize and make a valid identifier.
	safeName = strings.Title(makeValidIdentifier(safeName))

	// If there are multiple underscores in a row, replace with one.
	regex := regexp.MustCompile(`_+`)
	safeName = regex.ReplaceAllString(safeName, "_")

	// If the enum name starts with an underscore, add the type name as a prefix.
	if strings.HasPrefix(safeName, "_") {
		safeName = typeName + safeName
	}

	// "Equals" conflicts with a method on the EnumType struct, change it to EqualsValue.
	if safeName == "Equals" {
		safeName = "EqualsValue"
	}

	return safeName, nil
}

// Provides code for a method which will be placed in the program preamble if deemed
// necessary. Because many Terraform functions are complex, it is much prettier to
// encapsulate them as their own function in the preamble.
func getHelperMethodIfNeeded(functionName string) (string, bool) {
	switch functionName {
	case "filebase64":
		return `private static string ReadFileBase64(string path) {
		return Convert.ToBase64String(Encoding.UTF8.GetBytes(File.ReadAllText(path)))
	}`, true
	case "filebase64sha256":
		return `private static string ComputeFileBase64Sha256(string path) {
		var fileData = Encoding.UTF8.GetBytes(File.ReadAllText(path));
		var hashData = SHA256.Create().ComputeHash(fileData);
		return Convert.ToBase64String(hashData);
	}`, true
	case "sha1":
		return `private static string ComputeSHA1(string input) {
		return BitConverter.ToString(
			SHA1.Create().ComputeHash(Encoding.UTF8.GetBytes(input))
		).Replace("-","").ToLowerInvariant());
	}`, true
	default:
		return "", false
	}
}

// LowerCamelCase sets the first character to lowercase
// LowerCamelCase("LowerCamelCase") -> "lowerCamelCase"
func LowerCamelCase(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	return string(append([]rune{unicode.ToLower(runes[0])}, runes[1:]...))
}
func linearizeAndSetupGenerator(program *pcl.Program, options GenerateProgramOptions) ([]pcl.Node, *generator, error) {
	// Linearize the nodes into an order appropriate for procedural code generation.
	nodes := pcl.Linearize(program)

	// Import C#-specific schema info.
	namespaces := make(map[string]map[string]string)
	compatibilities := make(map[string]string)
	tokenToModules := make(map[string]func(x string) string)
	functionArgs := make(map[string]string)
	packages, err := program.PackageSnapshots()
	if err != nil {
		return nil, nil, err
	}
	for _, p := range packages {
		if err := p.ImportLanguages(map[string]schema.Language{"csharp": Importer}); err != nil {
			return nil, nil, err
		}

		csharpInfo, hasInfo := p.Language["csharp"].(CSharpPackageInfo)
		if !hasInfo {
			csharpInfo = CSharpPackageInfo{}
		}
		packageNamespaces := csharpInfo.Namespaces
		namespaces[p.Name] = packageNamespaces
		compatibilities[p.Name] = csharpInfo.Compatibility
		tokenToModules[p.Name] = p.TokenToModule

		for _, f := range p.Functions {
			if f.Inputs != nil {
				functionArgs[f.Inputs.Token] = f.Token
			}
		}
	}

	g := &generator{
		program:                     program,
		namespaces:                  namespaces,
		compatibilities:             compatibilities,
		tokenToModules:              tokenToModules,
		functionArgs:                functionArgs,
		functionInvokes:             map[string]*schema.Function{},
		generateOptions:             options,
		generatingComponentResource: true,
	}

	g.Formatter = format.NewFormatter(g)

	for _, n := range nodes {
		if r, ok := n.(*pcl.Resource); ok && requiresAsyncInit(r) {
			g.asyncInit = true
			break
		}
	}
	return nodes, g, nil
}

// Prints the given rootname with a special prefix when a component resource is being generated, allowing it
// to reference component resource members or input arguments
func (g *generator) genRootNameWithPrefix(w io.Writer, rootName string, expr *model.ScopeTraversalExpression) {
	fmtString := "%s"
	switch expr.Parts[0].(type) {
	case *pcl.ConfigVariable:
		fmtString = "args.%s"
	}
	g.Fgenf(w, fmtString, rootName)
}

func decomposeTokenHelper(r *pcl.Resource) (string, string, string, hcl.Diagnostics) {
	var pkg, module, member string
	var diags hcl.Diagnostics
	if r.IsModule {
		// Terraform child modules are differentiated by their local path. Creating a unique identifier out it.
		pkg = makeValidIdentifier(path.Base(r.Definition.Labels[1]))
		member = "Index"
	} else {
		pkg, module, member, diags = r.DecomposeToken()
	}
	return pkg, module, member, diags
}
