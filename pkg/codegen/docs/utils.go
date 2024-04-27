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

// Pulling out some of the repeated strings tokens into constants would harm readability, so we just ignore the
// goconst linter's warning.
//
//nolint:lll, goconst
package docs

import (
	"strings"
	"unicode"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/dotnet"
	go_gen "github.com/pulumi/pulumi/pkg/v3/codegen/go"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func isDotNetTypeNameBoundary(prev rune, next rune) bool {
	// For C# type names, which are PascalCase are qualified using "." as the separator.
	return prev == rune('.') && unicode.IsUpper(next)
}

func isPythonTypeNameBoundary(prev rune, next rune) bool {
	// For Python, names are snake_cased (Duh?).
	return (prev == rune('_') && unicode.IsLower(next))
}

// wbr inserts HTML <wbr> in between case changes, e.g. "fooBar" becomes "foo<wbr>Bar".
func wbr(s string) string {
	runes := slice.Prealloc[rune](len(s))
	var prev rune
	for i, r := range s {
		if i != 0 &&
			// For TS, JS and Go, property names are camelCase and types are PascalCase.
			((unicode.IsLower(prev) && unicode.IsUpper(r)) ||
				isDotNetTypeNameBoundary(prev, r) ||
				isPythonTypeNameBoundary(prev, r)) {
			runes = append(runes, []rune("<wbr>")...)
		}
		runes = append(runes, r)
		prev = r
	}
	return string(runes)
}

// tokenToName returns the resource name from a Pulumi token.
func tokenToName(tok string) string {
	components := strings.Split(tok, ":")
	contract.Assertf(len(components) == 3, "malformed token %v", tok)
	return components[2]
}

func tokenToPackageName(tok string) string {
	components := strings.Split(tok, ":")
	contract.Assertf(len(components) == 3, "malformed token %v", tok)
	return components[0]
}

func title(s, lang string) string {
	switch lang {
	case "go":
		return go_gen.Title(s)
	case "csharp":
		return dotnet.Title(s)
	default:
		return strings.Title(s)
	}
}

func modFilenameToDisplayName(name string) string {
	parts := strings.Split(name, "/")
	return parts[len(parts)-1]
}

func getModuleLink(name string) string {
	return strings.ToLower(name) + "/"
}

func getResourceLink(name string) string {
	link := strings.ToLower(name)
	// Handle URL generation for resources named `index`. We prepend a double underscore
	// here, since a link of .../<module>/index has trouble resolving and returns a 404 in
	// the browser, likely due to `index` being some sort of reserved keyword.
	if link == "index" {
		return "--" + link
	}
	return link
}

func getFunctionLink(name string) string {
	return strings.ToLower(name)
}

// isExternalType checks if the type is external to the given package.
func isExternalType(t schema.Type, pkg schema.PackageReference) (isExternal bool) {
	switch typ := t.(type) {
	case *schema.ObjectType:
		return typ.PackageReference != nil && !codegen.PkgEquals(typ.PackageReference, pkg)
	case *schema.ResourceType:
		return typ.Resource != nil && pkg != nil && !codegen.PkgEquals(typ.Resource.PackageReference, pkg)
	case *schema.EnumType:
		return pkg != nil && !codegen.PkgEquals(typ.PackageReference, pkg)
	}
	return
}

// Iterate character by character and remove underscores if that underscore
// is at the very front of an identifier, follows a special character, and is not a delimeter
// within an identifier.
func removeLeadingUnderscores(s string) string {
	var sb strings.Builder
	lastChar := ' '
	for _, ch := range s {
		if ch != '_' || (unicode.IsLetter(lastChar)) {
			sb.WriteRune(ch)
		}
		lastChar = ch
	}
	return sb.String()
}
